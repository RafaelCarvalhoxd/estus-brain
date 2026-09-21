import Foundation
import ImageIO
import ImagePlayground
import UniformTypeIdentifiers

/// On-device image generation via Apple's Image Playground — illustrated
/// styles only (animation, illustration, sketch), never a photo. This is the
/// fallback generate_image on the Go side reaches for when no OpenAI key is
/// configured (tools_images.go), not the primary path.
///
/// ImageCreator refuses to run in this process as built: "Image creation is
/// not available when the application is hidden or running in the
/// background" — confirmed against the real framework, not just from docs.
/// estus-apple-bridge is a plain command-line daemon (main.swift's
/// dispatchMain, no windows); wrapping it in an NSApplication with
/// .accessory or .regular activation policy and explicitly activating it
/// did NOT clear this — both still failed the same way. The API appears to
/// require an actual app-bundle GUI session (launched via LaunchServices,
/// not a bare executable), which this bridge does not have and can't get
/// without becoming a very different kind of process. The route is kept
/// because it costs nothing to leave in — the Go side already tries OpenAI
/// first and only reaches this as a fallback — and returns this same clear
/// error rather than silently pretending to work.
enum Images {
    // MARK: /generate-image

    static func generate(_ body: Data) async -> HTTPResponse {
        guard let payload = try? JSONValue.parse(body), payload.objectValue != nil else {
            return .error(400, "body must be a JSON object")
        }
        let prompt = (payload["prompt"]?.stringValue ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        guard !prompt.isEmpty else {
            return .error(422, "Descreva o que gerar.")
        }
        guard #available(macOS 15.2, *) else {
            return .error(500, "O Image Playground exige macOS 15.2 ou mais recente.")
        }
        let started = Date()
        do {
            let creator = try await ImageCreator()
            let style = creator.availableStyles.first ?? .illustration
            var data: Data?
            for try await image in creator.images(for: [.text(prompt)], style: style, limit: 1) {
                data = pngData(from: image.cgImage)
                break
            }
            guard let data else {
                return .error(500, "O Image Playground não devolveu nenhuma imagem.")
            }
            log(String(format: "generated image in %.1fs", Date().timeIntervalSince(started)))
            return .json(200, .object(["image_base64": .string(data.base64EncodedString())]))
        } catch {
            log("generate-image failed: \(error)")
            return .error(500, "Não consegui gerar a imagem: \(error.localizedDescription)")
        }
    }

    private static func pngData(from cgImage: CGImage) -> Data? {
        let data = NSMutableData()
        guard let dest = CGImageDestinationCreateWithData(data, UTType.png.identifier as CFString, 1, nil) else {
            return nil
        }
        CGImageDestinationAddImage(dest, cgImage, nil)
        guard CGImageDestinationFinalize(dest) else { return nil }
        return data as Data
    }
}
