import Foundation
import FoundationModels

enum Routes {
    static func handle(_ request: HTTPRequest) async -> HTTPResponse {
        switch (request.method, request.path) {
        case ("GET", "/health"):
            return health()
        case ("POST", "/chat"):
            return await chat(request.body)
        case ("POST", "/transcribe"):
            return await Speech.transcribe(request.body, filename: request.headers["x-filename"] ?? "")
        case ("POST", "/speak"):
            return await Speech.speak(request.body)
        case ("POST", "/generate-image"):
            return await Images.generate(request.body)
        case (_, "/health"), (_, "/chat"), (_, "/transcribe"), (_, "/speak"), (_, "/generate-image"):
            return .error(405, "method not allowed")
        default:
            return .error(404, "not found")
        }
    }

    // MARK: /health

    static func availabilityReason() -> String? {
        switch SystemLanguageModel.default.availability {
        case .available:
            return nil
        case .unavailable(let reason):
            switch reason {
            case .deviceNotEligible: return "deviceNotEligible"
            case .appleIntelligenceNotEnabled: return "appleIntelligenceNotEnabled"
            case .modelNotReady: return "modelNotReady"
            @unknown default: return "unavailable"
            }
        }
    }

    static func health() -> HTTPResponse {
        let reason = availabilityReason()
        return .json(200, .object([
            "available": .bool(reason == nil),
            "reason": .string(reason ?? ""),
        ]))
    }

    // MARK: /chat

    struct Message {
        let role: String
        let content: String
    }

    static let transcriptBudget = 3000

    static func chat(_ body: Data) async -> HTTPResponse {
        let payload: JSONValue
        do {
            payload = try JSONValue.parse(body)
        } catch {
            return .error(400, error.localizedDescription)
        }
        guard payload.objectValue != nil else {
            return .error(400, "body must be a JSON object")
        }

        let instructions = payload["instructions"]?.stringValue ?? ""
        let messages = (payload["messages"]?.arrayValue ?? []).compactMap { m -> Message? in
            guard let content = m["content"]?.stringValue else { return nil }
            return Message(role: m["role"]?.stringValue ?? "user", content: content)
        }
        guard let last = messages.last,
              !last.content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return .error(400, "messages must end with a non-empty user message")
        }

        if let reason = availabilityReason() {
            return .error(500, "Apple Intelligence indisponível: \(reason)")
        }

        let recorder = ToolCallRecorder()
        let endpoint = payload["tool_endpoint"]?.stringValue ?? ""
        let token = payload["tool_token"]?.stringValue ?? ""
        var tools: [any Tool] = []
        var seen = Set<String>()
        for spec in payload["tools"]?.arrayValue ?? [] {
            guard let name = spec["name"]?.stringValue, !name.isEmpty else {
                return .error(400, "every tool needs a name")
            }
            guard seen.insert(name).inserted else {
                return .error(400, "duplicate tool name \(name)")
            }
            do {
                let schema = try SchemaConverter.generationSchema(toolName: name, jsonSchema: spec["parameters"])
                tools.append(BridgeTool(
                    name: name,
                    description: spec["description"]?.stringValue ?? name,
                    parameters: schema,
                    endpoint: endpoint,
                    token: token,
                    recorder: recorder
                ))
            } catch {
                return .error(400, error.localizedDescription)
            }
        }
        if !tools.isEmpty && endpoint.isEmpty {
            return .error(400, "tool_endpoint is required when tools are provided")
        }

        var fullInstructions = instructions
        let history = renderTranscript(Array(messages.dropLast()))
        if !history.isEmpty {
            if !fullInstructions.isEmpty { fullInstructions += "\n\n" }
            fullInstructions += "Conversation so far:\n" + history
        }

        let started = Date()
        do {
            let session = LanguageModelSession(
                model: .default,
                tools: tools,
                instructions: fullInstructions.isEmpty ? nil : fullInstructions
            )
            let response = try await session.respond(to: last.content)
            let calls = await recorder.records
            log(String(format: "chat ok in %.1fs (%d tool calls)", Date().timeIntervalSince(started), calls.count))
            return .json(200, .object([
                "text": .string(response.content),
                "tool_calls": .array(calls.map { $0.json }),
            ]))
        } catch {
            let (status, message) = describe(error)
            log("chat failed (\(status)): \(message) — \(error)")
            return .error(status, message)
        }
    }

    /// Renders earlier messages compactly, keeping the most recent ones within `transcriptBudget` characters.
    static func renderTranscript(_ messages: [Message]) -> String {
        var lines: [String] = []
        var used = 0
        for message in messages.reversed() {
            let label = message.role == "assistant" ? "Assistant" : "User"
            let text = message.content
                .replacingOccurrences(of: "\n\n", with: "\n")
                .trimmingCharacters(in: .whitespacesAndNewlines)
            var line = "\(label): \(text)"
            let remaining = transcriptBudget - used
            if line.count > remaining {
                if lines.isEmpty && remaining > 40 {
                    // Newest message alone is too long: keep its tail.
                    line = "\(label): …" + String(text.suffix(remaining - label.count - 3))
                } else {
                    break
                }
            }
            lines.append(line)
            used += line.count + 1
        }
        return lines.reversed().joined(separator: "\n")
    }

    static func describe(_ error: any Error) -> (Int, String) {
        if let e = error as? LanguageModelSession.GenerationError {
            switch e {
            case .exceededContextWindowSize:
                return (422, "A conversa excedeu a janela de contexto do modelo local (~4k tokens). Encurte a mensagem ou inicie uma nova conversa.")
            case .guardrailViolation:
                return (422, "O pedido foi bloqueado pelas proteções de segurança do Apple Intelligence.")
            case .refusal:
                return (422, "O modelo local se recusou a responder a este pedido.")
            case .unsupportedLanguageOrLocale:
                return (422, "Idioma ou região não suportado pelo modelo local.")
            case .unsupportedGuide:
                return (422, "O schema de uma ferramenta usa um recurso não suportado pelo modelo local.")
            case .assetsUnavailable:
                return (500, "Os recursos do modelo local não estão disponíveis no momento (Apple Intelligence pode estar baixando ou desativado).")
            case .decodingFailure:
                return (500, "O modelo local gerou uma resposta que não pôde ser decodificada.")
            case .rateLimited:
                return (500, "O modelo local está limitando requisições. Tente novamente em instantes.")
            case .concurrentRequests:
                return (500, "O modelo local já está processando outra requisição nesta sessão.")
            @unknown default:
                return (500, "Erro de geração do modelo local: \(e.localizedDescription)")
            }
        }
        if let e = error as? LanguageModelSession.ToolCallError {
            return (500, "Falha ao executar a ferramenta \(e.tool.name): \(e.underlyingError.localizedDescription)")
        }
        return (500, "Erro inesperado: \(error.localizedDescription)")
    }
}
