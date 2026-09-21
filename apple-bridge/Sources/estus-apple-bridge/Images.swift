import Foundation
import ImageIO
import ImagePlayground
import UniformTypeIdentifiers

/// On-device image generation via Apple's Image Playground — illustrated
/// styles only (animation, illustration, sketch), never a photo. This is the
/// fallback generate_image on the Go side reaches for when no OpenAI key is
/// configured (tools_images.go), not the primary path.
///
/// ImageCreator refuses to run outside a real foreground app ("Image
/// creation is not available when the application is hidden or running in
/// the background") — confirmed live. Entry.swift's AppBundleRelauncher
/// already gets this process running from inside a proper .app bundle;
/// ForegroundActivation.ensure() below is the other half, briefly showing a
/// window and Dock icon right before generation and hiding them again after
/// (ForegroundActivation.retreat()) — a real, if brief, visible cost the Mac
/// pays for this to work at all, not just an implementation detail.
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
            try await ForegroundActivation.ensure()
            defer { ForegroundActivation.retreat() }
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
