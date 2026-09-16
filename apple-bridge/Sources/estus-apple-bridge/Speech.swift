import AVFoundation
import Foundation
import Speech

/// The Mac's own speech for Estus: recorded audio to text, and answers to audio.
/// Everything runs on-device; audio is written only to temporary files that are
/// removed right after.
enum Speech {
    static let maxSeconds = 180.0
    static let locale = Locale(identifier: "pt_BR")
    static let formats: Set<String> = ["wav", "m4a", "caf", "ogg", "oga", "opus", "mp3", "aiff"]

    // MARK: /transcribe

    static func transcribe(_ body: Data, filename: String) async -> HTTPResponse {
        let ext = (filename as NSString).pathExtension.lowercased()
        guard formats.contains(ext) else {
            return .error(422, "Formato de áudio não suportado.")
        }
        guard !body.isEmpty else {
            return .json(200, .object(["text": .string(""), "seconds": .number(0)]))
        }
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("estus-audio-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: dir) }
        let url = dir.appendingPathComponent("audio.\(ext)")
        do {
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            try body.write(to: url)
        } catch {
            return .error(500, "Não consegui transcrever o áudio.")
        }
        let file: AVAudioFile
        do {
            file = try AVAudioFile(forReading: url)
        } catch {
            // Telegram sends Ogg/Opus, which AVAudioFile may refuse to open;
            // afconvert reads more containers than it does, so try once through
            // plain 16 kHz mono (all the speech model needs) before giving up.
            let converted = dir.appendingPathComponent("audio-16k.caf")
            guard let status = try? await run("/usr/bin/afconvert", ["-f", "caff", "-d", "LEI16@16000", "-c", "1", url.path, converted.path]),
                  status == 0,
                  let readable = try? AVAudioFile(forReading: converted)
            else {
                return .error(422, "Formato de áudio não suportado.")
            }
            file = readable
        }
        let seconds = Double(file.length) / file.processingFormat.sampleRate
        if seconds > maxSeconds {
            return .error(422, "Áudio longo demais — mande até 3 minutos.")
        }
        let started = Date()
        do {
            let text = try await transcribe(file)
            log(String(format: "transcribed %.1fs of audio in %.2fs", seconds, Date().timeIntervalSince(started)))
            return .json(200, .object(["text": .string(text), "seconds": .number(seconds)]))
        } catch {
            log("transcribe failed: \(error)")
            return .error(500, "Não consegui transcrever o áudio.")
        }
    }

    static func transcribe(_ file: AVAudioFile) async throws -> String {
        let transcriber = SpeechTranscriber(locale: locale, transcriptionOptions: [], reportingOptions: [], attributeOptions: [])
        // The pt-BR model is downloaded once, the first time it's needed.
        if let request = try await AssetInventory.assetInstallationRequest(supporting: [transcriber]) {
            try await request.downloadAndInstall()
        }
        let analyzer = SpeechAnalyzer(modules: [transcriber])
        async let text = transcriber.results.reduce("") { $0 + String($1.text.characters) }
        if let last = try await analyzer.analyzeSequence(from: file) {
            try await analyzer.finalizeAndFinish(through: last)
        } else {
            await analyzer.cancelAndFinishNow()
        }
        return try await text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    // MARK: /speak

    static func speak(_ body: Data) async -> HTTPResponse {
        guard let payload = try? JSONValue.parse(body), payload.objectValue != nil else {
            return .error(400, "body must be a JSON object")
        }
        let text = (payload["text"]?.stringValue ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else {
            return .error(422, "Nada para ler.")
        }
        let voice = payload["voice"]?.stringValue ?? "Luciana"
        // `say -v <bad name>` silently falls back to the default voice instead of
        // failing, so check against the installed voices ourselves. `say -v '?'`
        // — what the README tells the owner to run — prints "Eddy (Portuguese
        // (Brazil))" where AVSpeechSynthesisVoice only knows "Eddy", so accept
        // both forms and hand `say` whatever was configured, unchanged.
        let wanted = voice.lowercased()
        let installed = AVSpeechSynthesisVoice.speechVoices().contains { known in
            let name = known.name.lowercased()
            return wanted == name || wanted.hasPrefix(name + " (")
        }
        guard installed else {
            return .error(422, "A voz \(voice) não está instalada neste Mac.")
        }
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("estus-speak-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: dir) }
        do {
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            let input = dir.appendingPathComponent("text.txt")
            let aiff = dir.appendingPathComponent("voice.aiff")
            let m4a = dir.appendingPathComponent("voice.m4a")
            // What is being read aloud is the owner's own data: keep the file
            // that carries it readable only by this user.
            guard FileManager.default.createFile(atPath: input.path, contents: Data(text.utf8), attributes: [.posixPermissions: 0o600]) else {
                return .error(500, "Não consegui gerar a voz.")
            }
            // The voice was checked above, so a failure here is `say` itself
            // going wrong — nothing the owner can fix by picking another voice.
            guard try await run("/usr/bin/say", ["-v", voice, "-o", aiff.path, "-f", input.path]) == 0 else {
                return .error(500, "Não consegui gerar a voz.")
            }
            guard try await run("/usr/bin/afconvert", ["-f", "m4af", "-d", "aac", aiff.path, m4a.path]) == 0 else {
                return .error(500, "Não consegui gerar a voz.")
            }
            return HTTPResponse(status: 200, body: try Data(contentsOf: m4a), contentType: "audio/mp4")
        } catch {
            log("speak failed: \(error)")
            return .error(500, "Não consegui gerar a voz.")
        }
    }

    /// Runs a system tool and returns its exit status.
    static func run(_ path: String, _ args: [String]) async throws -> Int32 {
        try await withCheckedThrowingContinuation { continuation in
            let process = Process()
            process.executableURL = URL(fileURLWithPath: path)
            process.arguments = args
            process.standardOutput = FileHandle.nullDevice
            process.standardError = FileHandle.nullDevice
            process.terminationHandler = { continuation.resume(returning: $0.terminationStatus) }
            do {
                try process.run()
            } catch {
                continuation.resume(throwing: error)
            }
        }
    }
}
