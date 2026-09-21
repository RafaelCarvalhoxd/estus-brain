import Foundation

// ImageCreator (Images.swift) refuses to run outside a real foreground app —
// confirmed live, and the same fix a public tool (github.com/wojtczyk/
// game-art-generator) already ships: relaunch the plain executable from
// inside a proper .app bundle via `open`, which is what LaunchServices
// treats as a foreground-capable app. This has to happen at process start,
// before the HTTP port is bound — by the time a /generate-image request
// arrives, this process already is the server, too late to hand the port
// to a relaunched copy of itself.
enum AppBundleRelauncher {
    private static let wrapperPIDKey = "ESTUS_BRIDGE_WRAPPER_PID"
    private static let bundleIdentifier = "com.estus.applebridge.wrapper"

    static var isRunningInsideAppBundle: Bool {
        Bundle.main.bundleURL.pathExtension == "app" && Bundle.main.bundleIdentifier != nil
    }

    /// Relaunches the current executable inside a wrapper .app and never
    /// returns — the caller's process just keeps existing so the same PID Go
    /// already tracks (AppleBridge.ensure/stop) stays valid, while the
    /// actual server runs in the launched app.
    static func relaunchInsideAppBundle() throws -> Never {
        let bundleURL = try wrapperBundle()
        log("wrapper bundle ready at \(bundleURL.path)")
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/open")
        // No -W: confirmed live that `open -W` can hang indefinitely
        // waiting for the launched app to quit when this process itself
        // runs detached/backgrounded — `open -n` alone still launches the
        // app instantly either way, so nothing is gained by waiting here.
        process.arguments = ["-n", bundleURL.path]

        var environment = ProcessInfo.processInfo.environment
        environment[wrapperPIDKey] = String(ProcessInfo.processInfo.processIdentifier)
        process.environment = environment
        process.standardInput = FileHandle(forReadingAtPath: "/dev/null")
        process.standardOutput = FileHandle.standardOutput
        process.standardError = FileHandle.standardError

        try process.run()
        log("launched `open -n \(bundleURL.path)`")
        dispatchMain()
    }

    /// True once, in the wrapped process, if its launcher already died —
    /// `kill -9` (what Go's Process.Kill sends) can't be caught, so the
    /// wrapped process instead notices its launcher is gone and exits on
    /// its own; otherwise a killed launcher would leave this one running
    /// forever, deaf to AppleBridge.stop.
    static func launcherPID() -> pid_t? {
        guard let raw = ProcessInfo.processInfo.environment[wrapperPIDKey], let pid = pid_t(raw) else {
            return nil
        }
        return pid
    }

    static func launcherIsAlive(_ pid: pid_t) -> Bool {
        kill(pid, 0) == 0
    }

    private static func wrapperBundle() throws -> URL {
        let fm = FileManager.default
        let support = try fm.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
        let bundleURL = support.appendingPathComponent("EstusBrain/AppleBridgeWrapper/estus-apple-bridge.app", isDirectory: true)
        let macOSURL = bundleURL.appendingPathComponent("Contents/MacOS", isDirectory: true)
        let executableURL = macOSURL.appendingPathComponent("estus-apple-bridge", isDirectory: false)
        let infoPlistURL = bundleURL.appendingPathComponent("Contents/Info.plist", isDirectory: false)

        guard let currentExecutable = Bundle.main.executableURL, fm.isExecutableFile(atPath: currentExecutable.path) else {
            throw BridgeSetupError("could not resolve the running executable's own path")
        }
        // A freshly-registered bundle costs LaunchServices a real, multi-
        // second delay the first time `open` launches it (confirmed live —
        // long enough to blow past AppleBridge.ensure's timeout on the Go
        // side). Rewriting the bundle on every boot re-triggers that delay
        // forever; only rewrite when the binary inside is actually stale.
        if fm.fileExists(atPath: executableURL.path), fm.contentsEqual(atPath: currentExecutable.path, andPath: executableURL.path) {
            return bundleURL
        }
        try? fm.removeItem(at: bundleURL)
        try fm.createDirectory(at: macOSURL, withIntermediateDirectories: true)
        try fm.copyItem(at: currentExecutable, to: executableURL)
        try fm.setAttributes([.posixPermissions: 0o755], ofItemAtPath: executableURL.path)

        let plist: [String: Any] = [
            "CFBundleExecutable": "estus-apple-bridge",
            "CFBundleIdentifier": bundleIdentifier,
            "CFBundleName": "Estus Apple Bridge",
            "CFBundlePackageType": "APPL",
            "CFBundleShortVersionString": "1.0",
        ]
        let plistData = try PropertyListSerialization.data(fromPropertyList: plist, format: .xml, options: 0)
        try plistData.write(to: infoPlistURL, options: .atomic)
        return bundleURL
    }
}

struct BridgeSetupError: Error, CustomStringConvertible {
    let description: String
    init(_ description: String) { self.description = description }
}
