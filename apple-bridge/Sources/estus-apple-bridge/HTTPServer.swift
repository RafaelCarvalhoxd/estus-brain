import Foundation
import Network

struct HTTPRequest: Sendable {
    let method: String
    let path: String
    let headers: [String: String]  // lowercased keys
    let body: Data
}

struct HTTPResponse: Sendable {
    let status: Int
    let body: Data
    var contentType = "application/json; charset=utf-8"

    static func json(_ status: Int, _ value: JSONValue) -> HTTPResponse {
        HTTPResponse(status: status, body: value.serialized())
    }

    static func error(_ status: Int, _ message: String) -> HTTPResponse {
        .json(status, .object(["error": .string(message)]))
    }
}

typealias HTTPHandler = @Sendable (HTTPRequest) async -> HTTPResponse

enum HTTPLimits {
    static let maxHeaderBytes = 64 * 1024
    static let maxBodyBytes = 1024 * 1024
    // A few minutes of recorded speech; everything else stays small.
    static let maxAudioBodyBytes = 10 * 1024 * 1024
    static let readTimeoutSeconds = 30.0

    static func maxBody(for path: String) -> Int {
        path == "/transcribe" ? maxAudioBodyBytes : maxBodyBytes
    }
}

func log(_ message: String) {
    let ts = ISO8601DateFormatter().string(from: Date())
    FileHandle.standardError.write(Data("[\(ts)] \(message)\n".utf8))
}

/// Minimal HTTP/1.1 server on Network.framework: one request per connection, `Connection: close`.
final class HTTPServer: Sendable {
    private let listener: NWListener
    private let queue = DispatchQueue(label: "estus.apple-bridge.listener")
    private let handler: HTTPHandler

    init(host: String, port: UInt16, handler: @escaping HTTPHandler) throws {
        guard let nwPort = NWEndpoint.Port(rawValue: port) else {
            throw JSONValue.ParseError(message: "invalid port \(port)")
        }
        let params = NWParameters.tcp
        params.allowLocalEndpointReuse = true
        params.requiredLocalEndpoint = .hostPort(host: NWEndpoint.Host(host), port: nwPort)
        self.listener = try NWListener(using: params)
        self.handler = handler
    }

    func start() {
        let handler = self.handler
        listener.stateUpdateHandler = { state in
            switch state {
            case .ready:
                log("listening on \(self.listener.parameters.requiredLocalEndpoint.map { "\($0)" } ?? "?")")
            case .failed(let error):
                log("listener failed: \(error)")
                exit(1)
            default:
                break
            }
        }
        listener.newConnectionHandler = { connection in
            HTTPConnection(connection: connection, handler: handler).start()
        }
        listener.start(queue: queue)
    }
}

/// State for a single connection. All mutable state is touched only on `queue`.
private final class HTTPConnection: @unchecked Sendable {
    private let connection: NWConnection
    private let handler: HTTPHandler
    private let queue = DispatchQueue(label: "estus.apple-bridge.connection")
    private var buffer = Data()
    private var requestReceived = false
    private var sentContinue = false
    private var finished = false

    init(connection: NWConnection, handler: @escaping HTTPHandler) {
        self.connection = connection
        self.handler = handler
    }

    func start() {
        connection.stateUpdateHandler = { [self] state in
            switch state {
            case .failed, .cancelled:
                finished = true
            default:
                break
            }
        }
        connection.start(queue: queue)
        queue.asyncAfter(deadline: .now() + HTTPLimits.readTimeoutSeconds) { [self] in
            if !requestReceived && !finished {
                send(.error(408, "request timeout"))
            }
        }
        receive()
    }

    private func receive() {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 64 * 1024) { [self] data, _, isComplete, error in
            if finished || requestReceived { return }
            if let data, !data.isEmpty { buffer.append(data) }
            switch parse() {
            case .needMore:
                if isComplete || error != nil {
                    connection.cancel()
                } else {
                    receive()
                }
            case .failure(let status, let message):
                requestReceived = true
                send(.error(status, message))
            case .request(let request):
                requestReceived = true
                let handler = self.handler
                Task {
                    let response = await handler(request)
                    self.queue.async { self.send(response) }
                }
            }
        }
    }

    private enum ParseResult {
        case needMore
        case failure(Int, String)
        case request(HTTPRequest)
    }

    private func parse() -> ParseResult {
        let separator = Data("\r\n\r\n".utf8)
        guard let headerEnd = buffer.range(of: separator) else {
            if buffer.count > HTTPLimits.maxHeaderBytes {
                return .failure(431, "request headers too large")
            }
            return .needMore
        }
        let headerData = buffer.subdata(in: buffer.startIndex..<headerEnd.lowerBound)
        guard let headerText = String(data: headerData, encoding: .utf8)
            ?? String(data: headerData, encoding: .isoLatin1) else {
            return .failure(400, "malformed request")
        }
        var lines = headerText.components(separatedBy: "\r\n")
        guard !lines.isEmpty else { return .failure(400, "malformed request") }
        let requestLine = lines.removeFirst().split(separator: " ", omittingEmptySubsequences: true)
        guard requestLine.count == 3, requestLine[2].hasPrefix("HTTP/1.") else {
            return .failure(400, "malformed request line")
        }
        var headers: [String: String] = [:]
        for line in lines where !line.isEmpty {
            guard let colon = line.firstIndex(of: ":") else {
                return .failure(400, "malformed header")
            }
            let key = line[..<colon].trimmingCharacters(in: .whitespaces).lowercased()
            let value = line[line.index(after: colon)...].trimmingCharacters(in: .whitespaces)
            headers[key] = value
        }
        if let te = headers["transfer-encoding"], te.lowercased() != "identity" {
            return .failure(501, "transfer-encoding not supported; send Content-Length")
        }
        var contentLength = 0
        if let cl = headers["content-length"] {
            guard let n = Int(cl), n >= 0 else { return .failure(400, "invalid Content-Length") }
            contentLength = n
        }
        var path = String(requestLine[1])
        if let q = path.firstIndex(of: "?") { path = String(path[..<q]) }
        let limit = HTTPLimits.maxBody(for: path)
        if contentLength > limit {
            return .failure(413, "request body exceeds \(limit / (1024 * 1024)) MB")
        }
        let bodyStart = headerEnd.upperBound
        let available = buffer.endIndex - bodyStart
        if available < contentLength {
            if !sentContinue, headers["expect"]?.lowercased() == "100-continue" {
                sentContinue = true
                connection.send(content: Data("HTTP/1.1 100 Continue\r\n\r\n".utf8), completion: .idempotent)
            }
            return .needMore
        }
        let body = buffer.subdata(in: bodyStart..<(bodyStart + contentLength))
        return .request(HTTPRequest(
            method: String(requestLine[0]).uppercased(),
            path: path,
            headers: headers,
            body: body
        ))
    }

    private func send(_ response: HTTPResponse) {
        if finished { return }
        finished = true
        let head = "HTTP/1.1 \(response.status) \(Self.reason(response.status))\r\n"
            + "Content-Type: \(response.contentType)\r\n"
            + "Content-Length: \(response.body.count)\r\n"
            + "Connection: close\r\n\r\n"
        var payload = Data(head.utf8)
        payload.append(response.body)
        connection.send(content: payload, contentContext: .finalMessage, isComplete: true, completion: .contentProcessed { [self] _ in
            connection.cancel()
        })
    }

    private static func reason(_ status: Int) -> String {
        switch status {
        case 200: return "OK"
        case 400: return "Bad Request"
        case 404: return "Not Found"
        case 405: return "Method Not Allowed"
        case 408: return "Request Timeout"
        case 413: return "Payload Too Large"
        case 422: return "Unprocessable Entity"
        case 431: return "Request Header Fields Too Large"
        case 500: return "Internal Server Error"
        case 501: return "Not Implemented"
        case 503: return "Service Unavailable"
        default: return "Status"
        }
    }
}
