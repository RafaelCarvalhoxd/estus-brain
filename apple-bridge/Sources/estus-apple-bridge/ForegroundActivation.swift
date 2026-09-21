import AppKit
import Foundation

// ImageCreator refuses to run until NSApplication.isActive is actually
// true, which needs a real, visible window plus a moment for
// finishLaunching/activate to land — the game-art-generator reference polls
// for exactly that instead of assuming one activate() call is synchronous.
//
// .regular activation policy, not .accessory: confirmed live that an
// accessory (Dock-less) app never reports isActive == true no matter how
// long this waits, so ImageCreator refuses every time; a regular app with a
// real visible window does become active within ~1s. retreat() below is
// what keeps that Dock icon and window from lingering between requests —
// it only actually works because Entry.swift runs a real RunLoop.main.run()
// forever; without one draining window-server notifications, isActive
// never updated after hide(), and the next request saw a stale "already
// active" and failed instantly against a hidden, accessory app.
//
// DispatchQueue.main.sync, not @MainActor: NSWindow's own thread check
// wants GCD's actual main queue specifically — confirmed live by a crash
// from an @MainActor-isolated call here ("NSWindow should only be
// instantiated on the main thread!").
@available(macOS 15.2, *)
enum ForegroundActivation {
    // Only ever touched inside DispatchQueue.main.sync, always the same
    // serial queue — safe in practice, just not provable to the compiler.
    nonisolated(unsafe) private static var window: NSWindow?

    static func ensure() async throws {
        DispatchQueue.main.sync { activateOnce(); pump() }
        for _ in 0..<80 {
            if DispatchQueue.main.sync(execute: { NSApplication.shared.isActive }) {
                return
            }
            try? await Task.sleep(for: .milliseconds(100))
            DispatchQueue.main.sync { activateOnce(); pump() }
        }
        throw BridgeSetupError("Image Playground precisa que o processo esteja em primeiro plano, e ele nunca ficou ativo")
    }

    /// Deliberately does nothing now. It used to hide the window and demote
    /// back to .accessory after each generation — confirmed live, live
    /// enough times to be sure: once this process hides/demotes itself even
    /// once, it can never reactivate again for the rest of its life (every
    /// later request failed instantly with the same "hidden or running in
    /// the background" ImageCreator refusal, no matter how long ensure()
    /// waited or how the run loop was pumped). A Dock icon and a tiny
    /// window staying up for as long as the bridge runs is a real, visible
    /// cost — but the alternative measured was "works exactly once per
    /// process," which is worse.
    static func retreat() {}

    /// No real run loop drains window-server notifications in this process
    /// (Entry.swift keeps it alive with an async sleep, and RunLoop.main.run()
    /// can't be called from an async context at all) — so isActive doesn't
    /// reflect activate() without this.
    private static func pump() {
        RunLoop.main.run(until: Date().addingTimeInterval(0.4))
    }

    private static func activateOnce() {
        let app = NSApplication.shared
        if app.activationPolicy() != .regular {
            _ = app.setActivationPolicy(.regular)
        }
        if !NSRunningApplication.current.isFinishedLaunching {
            app.finishLaunching()
        }
        let w = window ?? makeWindow()
        window = w
        w.makeKeyAndOrderFront(nil)
        w.orderFrontRegardless()
        app.activate()
        _ = NSRunningApplication.current.activate(options: [.activateAllWindows])
    }

    private static func makeWindow() -> NSWindow {
        let w = NSWindow(contentRect: NSRect(x: 40, y: 40, width: 280, height: 70), styleMask: [.titled, .miniaturizable], backing: .buffered, defer: false)
        w.isReleasedWhenClosed = false
        w.title = "Estus"
        let label = NSTextField(labelWithString: "Geração de imagem (Apple Intelligence) precisa desta janela aberta.")
        label.frame = NSRect(x: 12, y: 8, width: 256, height: 40)
        label.lineBreakMode = .byWordWrapping
        label.maximumNumberOfLines = 3
        label.font = .systemFont(ofSize: 11)
        w.contentView?.addSubview(label)
        w.collectionBehavior = [.moveToActiveSpace]
        return w
    }
}
