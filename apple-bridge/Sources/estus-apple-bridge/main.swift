import Foundation

let defaultPort: UInt16 = 8765
let portEnv = ProcessInfo.processInfo.environment["APPLE_BRIDGE_PORT"] ?? ""
let port: UInt16
if portEnv.isEmpty {
    port = defaultPort
} else if let p = UInt16(portEnv), p > 0 {
    port = p
} else {
    log("invalid APPLE_BRIDGE_PORT=\(portEnv)")
    exit(2)
}

do {
    let server = try HTTPServer(host: "127.0.0.1", port: port) { request in
        await Routes.handle(request)
    }
    server.start()
    log("estus-apple-bridge starting on 127.0.0.1:\(port) (apple intelligence: \(Routes.availabilityReason() ?? "available"))")
    withExtendedLifetime(server) {
        dispatchMain()
    }
} catch {
    log("failed to start: \(error)")
    exit(1)
}
