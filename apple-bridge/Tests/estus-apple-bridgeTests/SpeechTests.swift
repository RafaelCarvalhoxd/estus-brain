import AVFoundation
import Foundation
import Testing

@testable import estus_apple_bridge

@Test func transcribesPortugueseSpeech() async throws {
    let dir = FileManager.default.temporaryDirectory.appendingPathComponent("bridge-test-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: dir) }
    let aiff = dir.appendingPathComponent("frase.aiff")
    let said = try await Speech.run("/usr/bin/say", ["-v", "Luciana", "-o", aiff.path, "Gastei quarenta e cinco reais no mercado"])
    #expect(said == 0)

    let response = await Speech.transcribe(try Data(contentsOf: aiff), filename: "frase.aiff")
    #expect(response.status == 200)
    let json = try JSONValue.parse(response.body)
    #expect(json["text"]?.stringValue?.lowercased().contains("mercado") == true)
}

/// Telegram's voice notes are Opus, which AVAudioFile refuses to open; only the
/// conversion fallback makes them transcribable.
@Test func transcribesCompressedOpusAudio() async throws {
    let dir = FileManager.default.temporaryDirectory.appendingPathComponent("bridge-test-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: dir) }
    let aiff = dir.appendingPathComponent("frase.aiff")
    let caf = dir.appendingPathComponent("frase.caf")
    #expect(try await Speech.run("/usr/bin/say", ["-v", "Luciana", "-o", aiff.path, "Comprei pão na padaria hoje"]) == 0)
    #expect(try await Speech.run("/usr/bin/afconvert", ["-f", "caff", "-d", "opus@48000", "-c", "1", aiff.path, caf.path]) == 0)

    let response = await Speech.transcribe(try Data(contentsOf: caf), filename: "frase.caf")
    #expect(response.status == 200)
    let json = try JSONValue.parse(response.body)
    #expect(json["text"]?.stringValue?.lowercased().contains("padaria") == true)
}

@Test func emptyAudioHasNoText() async throws {
    let response = await Speech.transcribe(Data(), filename: "vazio.wav")
    #expect(response.status == 200)
    #expect(try JSONValue.parse(response.body)["text"]?.stringValue == "")
}

@Test func rejectsUnknownFormats() async {
    #expect(await Speech.transcribe(Data([1, 2, 3]), filename: "audio.xyz").status == 422)
    #expect(await Speech.transcribe(Data([1, 2, 3]), filename: "audio.wav").status == 422)
}

@Test func speaksAsPlayableAudio() async throws {
    let response = await Speech.speak(Data(#"{"text":"Olá, tudo certo?","voice":"Luciana"}"#.utf8))
    #expect(response.status == 200)
    #expect(response.contentType == "audio/mp4")
    let url = FileManager.default.temporaryDirectory.appendingPathComponent("speak-\(UUID().uuidString).m4a")
    try response.body.write(to: url)
    defer { try? FileManager.default.removeItem(at: url) }
    #expect(try AVAudioFile(forReading: url).length > 0)
}

/// `say -v '?'` prints "Eddy (Portuguese (Brazil))" — the form the README tells
/// the owner to configure — while AVSpeechSynthesisVoice only knows "Eddy".
@Test func acceptsTheVoiceNameAsSayPrintsIt() async {
    #expect(await Speech.speak(Data(#"{"text":"oi","voice":"Eddy (Portuguese (Brazil))"}"#.utf8)).status == 200)
    #expect(await Speech.speak(Data(#"{"text":"oi","voice":"luciana"}"#.utf8)).status == 200)
}

@Test func speakNeedsTextAndAnInstalledVoice() async {
    #expect(await Speech.speak(Data(#"{"text":"   "}"#.utf8)).status == 422)
    #expect(await Speech.speak(Data(#"{"text":"oi","voice":"Nao Existe"}"#.utf8)).status == 422)
    #expect(await Speech.speak(Data("nope".utf8)).status == 400)
}

@Test func audioGetsTheLargerBodyLimit() {
    #expect(HTTPLimits.maxBody(for: "/transcribe") == 10 * 1024 * 1024)
    #expect(HTTPLimits.maxBody(for: "/chat") == 1024 * 1024)
}
