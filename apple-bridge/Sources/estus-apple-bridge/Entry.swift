import Foundation

// @main + async static main(), not a plain main.swift + dispatchMain(): the
// Swift runtime only aliases its main-actor executor to the OS's real main
// thread under this specific entry-point shape — confirmed live, a version
// using dispatchMain() crashed AppKit's NSWindow with "should only be
// instantiated on the main thread" even from inside DispatchQueue.main.sync.
// This is the same shape game-art-generator (the reference this bridge's
// foreground-activation trick is ported from) uses.
@main
struct Entry {
    static func main() async {
        log("process started, pid=\(ProcessInfo.processInfo.processIdentifier), insideBundle=\(AppBundleRelauncher.isRunningInsideAppBundle)")
        if !AppBundleRelauncher.isRunningInsideAppBundle {
            do {
                try AppBundleRelauncher.relaunchInsideAppBundle() // never returns
            } catch {
                log("failed to relaunch inside an app bundle: \(error)")
                exit(1)
            }
        }
        log("running the server, insideBundle=true")

        // Go's AppleBridge.stop() sends SIGKILL to the launcher process
        // above, which can't be caught — so this process (the one actually
        // serving) instead watches whether its launcher is still alive and
        // exits on its own once it isn't, or `open`'s child would keep
        // running orphaned, deaf to Go's next AppleBridge.ensure() trying to
        // start a fresh one on the same port.
        if let launcherPID = AppBundleRelauncher.launcherPID() {
            let watchdog = DispatchSource.makeTimerSource(queue: .main)
            watchdog.schedule(deadline: .now() + 2, repeating: 2)
            watchdog.setEventHandler {
                if !AppBundleRelauncher.launcherIsAlive(launcherPID) {
                    log("launcher (pid \(launcherPID)) is gone; exiting")
                    exit(0)
                }
            }
            watchdog.resume()
            withExtendedLifetime(watchdog) {
                // never returns
            }
        }

        startServer() // synchronous — nothing here suspends, still the real main thread
        // RunLoop.main.run() would be the real fix (a normal AppKit app
        // always has one continuously draining window-server notifications,
        // which is what keeps NSApplication.isActive honest) but the
        // Swift runtime refuses to let it be called from an async context
        // at all, unconditionally — ForegroundActivation.ensure()/retreat()
        // instead pump the run loop briefly around each AppKit call.
        while true {
            try? await Task.sleep(for: .seconds(3600))
        }
    }

    static func startServer() {
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
            runningServer = server // keep alive; nothing else references it
        } catch {
            log("failed to start: \(error)")
            exit(1)
        }
    }

    nonisolated(unsafe) private static var runningServer: HTTPServer?
}
