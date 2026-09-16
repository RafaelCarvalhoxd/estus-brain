import Foundation
import FoundationModels

// MARK: - JSON Schema -> GenerationSchema

struct SchemaConversionError: Error, LocalizedError {
    let message: String
    var errorDescription: String? { message }
}

/// Converts the JSON Schema subset used by the backend into a `DynamicGenerationSchema`.
enum SchemaConverter {
    static func generationSchema(toolName: String, jsonSchema: JSONValue?) throws -> GenerationSchema {
        let root = convert(jsonSchema ?? .object(["type": .string("object")]),
                           name: sanitize(toolName) + "_arguments",
                           description: nil,
                           forceObject: true)
        do {
            return try GenerationSchema(root: root, dependencies: [])
        } catch {
            throw SchemaConversionError(message: "invalid parameters schema for tool \(toolName): \(error.localizedDescription)")
        }
    }

    /// `name` is a unique path-based name, used for named (object/enum) schemas so names never collide.
    private static func convert(_ schema: JSONValue, name: String, description: String?, forceObject: Bool = false) -> DynamicGenerationSchema {
        let desc = schema["description"]?.stringValue ?? description
        let type = forceObject ? "object" : resolveType(schema)

        switch type {
        case "object":
            let props = schema["properties"]?.objectValue ?? [:]
            let required = Set((schema["required"]?.arrayValue ?? []).compactMap { $0.stringValue })
            // Required properties first, then alphabetical, for a stable order.
            let keys = props.keys.sorted { a, b in
                let ra = required.contains(a), rb = required.contains(b)
                return ra == rb ? a < b : ra
            }
            let properties = keys.map { key -> DynamicGenerationSchema.Property in
                let propSchema = props[key]!
                return DynamicGenerationSchema.Property(
                    name: key,
                    description: propSchema["description"]?.stringValue,
                    schema: convert(propSchema, name: name + "_" + sanitize(key), description: nil),
                    isOptional: !required.contains(key)
                )
            }
            return DynamicGenerationSchema(name: name, description: desc, properties: properties)

        case "array":
            let items = schema["items"] ?? .object(["type": .string("string")])
            return DynamicGenerationSchema(
                arrayOf: convert(items, name: name + "_item", description: nil),
                minimumElements: schema["minItems"]?.intValue,
                maximumElements: schema["maxItems"]?.intValue
            )

        case "integer":
            return DynamicGenerationSchema(type: Int.self)

        case "number":
            return DynamicGenerationSchema(type: Double.self)

        case "boolean":
            return DynamicGenerationSchema(type: Bool.self)

        default: // "string" and anything unknown
            if let choices = schema["enum"]?.arrayValue, !choices.isEmpty {
                let strings = choices.map { value -> String in
                    if let s = value.stringValue { return s }
                    return value.serializedString()
                }
                return DynamicGenerationSchema(name: name, description: desc, anyOf: strings)
            }
            return DynamicGenerationSchema(type: String.self)
        }
    }

    private static func resolveType(_ schema: JSONValue) -> String {
        switch schema["type"] {
        case .string(let t)?:
            return t
        case .array(let ts)?:
            // e.g. ["string", "null"] -> "string"
            return ts.compactMap { $0.stringValue }.first { $0 != "null" } ?? "string"
        default:
            if schema["properties"] != nil { return "object" }
            if schema["items"] != nil { return "array" }
            return "string"
        }
    }

    private static func sanitize(_ s: String) -> String {
        String(s.map { $0.isLetter || $0.isNumber || $0 == "_" ? $0 : "_" })
    }
}

// MARK: - GeneratedContent -> JSON

extension GeneratedContent {
    var jsonValue: JSONValue {
        switch kind {
        case .null:
            return .null
        case .bool(let b):
            return .bool(b)
        case .number(let d):
            if d.rounded() == d, abs(d) < 1e15 { return .int(Int(d)) }
            return .number(d)
        case .string(let s):
            return .string(s)
        case .array(let items):
            return .array(items.map { $0.jsonValue })
        case .structure(let properties, _):
            var object: [String: JSONValue] = [:]
            for (key, value) in properties {
                let v = value.jsonValue
                if v != .null { object[key] = v }  // omit unset optionals
            }
            return .object(object)
        @unknown default:
            return .null
        }
    }
}

// MARK: - Tool call recording

struct ToolCallRecord: Sendable {
    let name: String
    let arguments: JSONValue
    let result: JSONValue
    let error: String?

    var json: JSONValue {
        var o: [String: JSONValue] = [
            "name": .string(name),
            "arguments": arguments,
            "result": result,
        ]
        if let error { o["error"] = .string(error) }
        return .object(o)
    }
}

actor ToolCallRecorder {
    private(set) var records: [ToolCallRecord] = []
    func append(_ record: ToolCallRecord) { records.append(record) }
}

// MARK: - Dynamic tool

/// A FoundationModels tool whose schema comes from the request and whose execution is
/// delegated to the backend over HTTP.
struct BridgeTool: Tool {
    typealias Arguments = GeneratedContent
    typealias Output = String

    static let maxOutputChars = 2000

    let name: String
    let description: String
    let parameters: GenerationSchema
    let endpoint: String
    let token: String
    let recorder: ToolCallRecorder

    @concurrent
    func call(arguments: GeneratedContent) async throws -> String {
        var args = arguments.jsonValue
        if args.objectValue == nil { args = .object([:]) }
        log("tool call \(name) \(args.serializedString())")

        let outcome = await invokeBackend(arguments: args)
        switch outcome {
        case .success(let result):
            await recorder.append(ToolCallRecord(name: name, arguments: args, result: result, error: nil))
            return Self.truncate(result.serializedString())
        case .failure(let message):
            log("tool \(name) failed: \(message)")
            await recorder.append(ToolCallRecord(name: name, arguments: args, result: .null, error: message))
            return Self.truncate("Erro: \(message)")
        }
    }

    private enum Outcome {
        case success(JSONValue)
        case failure(String)
    }

    private func invokeBackend(arguments: JSONValue) async -> Outcome {
        var base = endpoint
        while base.hasSuffix("/") { base.removeLast() }
        guard !base.isEmpty, let url = URL(string: base + "/" + name) else {
            return .failure("tool_endpoint inválido")
        }
        var request = URLRequest(url: url, timeoutInterval: 60)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        request.httpBody = arguments.serialized()

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await URLSession.shared.data(for: request)
        } catch {
            return .failure(error.localizedDescription)
        }
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        let body = try? JSONValue.parse(data)

        if (200..<300).contains(status) {
            if let body, let result = body["result"] { return .success(result) }
            if let error = body?["error"]?.stringValue { return .failure(error) }
            return .success(body ?? .string(String(decoding: data, as: UTF8.self)))
        }
        if let message = body?["error"]?.stringValue { return .failure(message) }
        return .failure("HTTP \(status)")
    }

    private static func truncate(_ s: String) -> String {
        s.count <= maxOutputChars ? s : String(s.prefix(maxOutputChars)) + "…"
    }
}
