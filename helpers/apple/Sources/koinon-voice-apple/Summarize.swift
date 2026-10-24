import Foundation
import FoundationModels

/// summarize streams one summary from Foundation Models. The bridge sends the instructions
/// and prompt (chunk 02's), so every engine gets the same text.
func summarize(_ request: Request, _ out: Output) async throws {
    struct P: Decodable {
        var instructions: String
        var prompt: String
    }
    struct Delta: Encodable { var text: String }
    struct Result: Encodable {
        var text: String
        var inputTokens: Int?
        var outputTokens: Int?
        enum CodingKeys: String, CodingKey {
            case text
            case inputTokens = "input_tokens"
            case outputTokens = "output_tokens"
        }
    }
    let p = try request.params(P.self)
    let model = SystemLanguageModel.default
    if case .unavailable(let reason) = model.availability {
        throw HelperError(code: "unavailable", message: "\(reason)")
    }
    let session = LanguageModelSession(model: model, instructions: p.instructions)
    var text = ""
    do {
        for try await snapshot in session.streamResponse(to: p.prompt) {
            // Each snapshot is the whole answer so far; the bridge wants what is new.
            let next = snapshot.content
            if next.hasPrefix(text) {
                let delta = String(next.dropFirst(text.count))
                if !delta.isEmpty { out.event(request.id, "delta", Delta(text: delta)) }
            }
            text = next
        }
    } catch let error as LanguageModelSession.GenerationError {
        switch error {
        case .guardrailViolation, .refusal:
            throw HelperError(code: "refused", message: "\(error)")
        case .exceededContextWindowSize:
            throw HelperError(code: "incomplete", message: "\(error)")
        case .assetsUnavailable:
            throw HelperError(code: "unavailable", message: "\(error)")
        default:
            throw HelperError(code: "engine_error", message: "\(error)")
        }
    }
    // Token counts are the model's own; they cost nothing, so a failure leaves them unknown.
    let input = try? await model.tokenCount(for: Instructions(p.instructions)) + model.tokenCount(for: p.prompt)
    let output = try? await model.tokenCount(for: text)
    out.result(request.id, Result(text: text, inputTokens: input, outputTokens: output))
}
