// koinon-voice-kokoro is the bridge's Kokoro helper (ADR 002, Decisions 1 and 4). It speaks
// the sidecar protocol on stdin and stdout: method synthesize, params {text, voice, speed},
// sends one sentence's audio as frames of 16-bit signed little-endian mono PCM at 24 kHz,
// then its result. It opens no audio device; the Apple helper plays the audio.
import Foundation
import KokoroSwift
import KokoroText
import MLX
import SidecarProtocol

let helperVersion = "0.1.0"

/// Audio frames of 200 ms at 24 kHz.
let frameBytes = 4800 * 2

let defaultModels = FileManager.default.homeDirectoryForCurrentUser
    .appendingPathComponent(".local/share/koinon-voice/kokoro").path

struct Info: Encodable {
    var models: String
    var model: String
    var modelSHA256: String
    var voices: [String]
    var sampleRate: Int
    var error: String?

    enum CodingKeys: String, CodingKey {
        case models, model, voices, error
        case modelSHA256 = "model_sha256"
        case sampleRate = "sample_rate"
    }
}

/// Output writes frames to stdout, one at a time.
final class Output: @unchecked Sendable {
    private let lock = NSLock()

    private func write(_ wire: Data) {
        lock.lock()
        defer { lock.unlock() }
        FileHandle.standardOutput.write(wire)
    }

    func send<D: Encodable>(_ m: Outgoing<D>) {
        if let wire = try? encodeMessage(m) { write(wire) }
    }

    func audio(_ id: UInt64, _ pcm: Data) {
        if let wire = try? Frame.audio(id: id, pcm: pcm).encoded() { write(wire) }
    }

    func error(_ id: UInt64, _ code: String, _ message: String) {
        send(Outgoing<Empty>(type: "error", id: id, code: code, message: message))
    }
}

struct SynthesizeParams: Decodable {
    var text: String
    var voice: String?
    var speed: Float?
}

/// Engine holds the model and voices. Only the synthesis queue uses it.
final class Engine {
    let model: Model
    private var tts: KokoroTTS?
    private var voices: [String: MLXArray] = [:]

    init(model: Model) { self.model = model }

    func load() -> KokoroTTS {
        if let tts { return tts }
        let t = KokoroTTS(modelPath: model.modelURL)
        tts = t
        return t
    }

    func voice(_ name: String) throws -> MLXArray {
        if let v = voices[name] { return v }
        guard model.voices.contains(name) else {
            throw HelperError(code: "bad_request", message: "no voice \(name); voices: \(model.voices.joined(separator: ", "))")
        }
        guard let v = try MLX.loadArrays(url: model.voiceURL(name))["voice"] else {
            throw HelperError(code: "engine_error", message: "voice \(name) has no array named voice")
        }
        voices[name] = v
        return v
    }

    /// warm runs one short synthesis, so the first request does not wait for MLX's setup.
    func warm() {
        guard let name = model.voices.contains("af_heart") ? "af_heart" : model.voices.first,
              let v = try? voice(name) else { return }
        _ = try? load().generateAudio(voice: v, language: .enUS, text: "Ready.", speed: 1.0)
    }
}

struct HelperError: Error {
    var code: String
    var message: String
}

/// Server runs requests one at a time on its synthesis queue, in the order they came.
final class Server: @unchecked Sendable {
    let out = Output()
    let engine: Engine?
    let unavailable: String?
    private let queue = DispatchQueue(label: "koinon-voice-kokoro.synthesis")
    private let lock = NSLock()
    private var cancelled: Set<UInt64> = []

    init(engine: Engine?, unavailable: String?) {
        self.engine = engine
        self.unavailable = unavailable
        if let engine { queue.async { engine.warm() } }
    }

    func cancel(_ id: UInt64) {
        lock.lock()
        cancelled.insert(id)
        lock.unlock()
    }

    /// takeCancelled reports whether the request was cancelled and forgets it.
    private func takeCancelled(_ id: UInt64) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        return cancelled.remove(id) != nil
    }

    func request(id: UInt64, method: String, body: Data) {
        guard method == "synthesize" else {
            out.error(id, "unknown_method", method)
            return
        }
        let requested = Date()
        queue.async { [self] in
            do {
                try synthesize(id: id, body: body, requested: requested)
            } catch let e as HelperError {
                out.error(id, e.code, e.message)
            } catch {
                out.error(id, "engine_error", "\(error)")
            }
        }
    }

    private func synthesize(id: UInt64, body: Data, requested: Date) throws {
        // A request cancelled before its sentence is synthesized ends with no frames.
        if takeCancelled(id) { throw HelperError(code: "cancelled", message: "cancelled") }
        guard let engine else { throw HelperError(code: "unavailable", message: unavailable ?? "no model") }
        let p: SynthesizeParams
        do {
            guard let decoded = try JSONDecoder().decode(Params<SynthesizeParams>.self, from: body).params else {
                throw HelperError(code: "bad_request", message: "no params")
            }
            p = decoded
        } catch let e as HelperError {
            throw e
        } catch {
            throw HelperError(code: "bad_request", message: "\(error)")
        }
        guard !p.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw HelperError(code: "bad_request", message: "no text")
        }
        let voice = try engine.voice(p.voice ?? "af_heart")
        let started = Date()
        let (samples, tokens) = try engine.load().generateAudio(voice: voice, language: .enUS, text: prepare(p.text), speed: p.speed ?? 1.1)
        let elapsed = Date().timeIntervalSince(started)
        // kokoro-ios cannot stop a sentence it has started; its frames are not sent.
        if takeCancelled(id) { throw HelperError(code: "cancelled", message: "cancelled") }

        struct FirstAudio: Encodable {
            var sinceRequestMS: Double
            enum CodingKeys: String, CodingKey { case sinceRequestMS = "since_request_ms" }
        }
        struct Result: Encodable {
            var audioSeconds: Double
            var elapsedMS: Double
            var phonemes: String
            enum CodingKeys: String, CodingKey {
                case phonemes
                case audioSeconds = "audio_seconds"
                case elapsedMS = "elapsed_ms"
            }
        }
        let frames = chunks(pcm16(samples), size: frameBytes)
        if !frames.isEmpty {
            out.send(Outgoing(type: "event", id: id, event: "first_audio", data: FirstAudio(sinceRequestMS: Date().timeIntervalSince(requested) * 1000)))
        }
        for f in frames { out.audio(id, f) }
        let phonemes = (tokens ?? []).map { ($0.phonemes ?? "") + $0.whitespace }.joined()
        out.send(Outgoing(type: "result", id: id, data: Result(
            audioSeconds: Double(samples.count) / Double(KokoroTTS.Constants.samplingRate),
            elapsedMS: elapsed * 1000,
            phonemes: phonemes.trimmingCharacters(in: .whitespaces))))
    }

    /// drain waits for the requests already queued.
    func drain() { queue.sync {} }
}

func serve(models: String) {
    let dir = URL(fileURLWithPath: models)
    var info = Info(models: models, model: modelFile, modelSHA256: "", voices: [], sampleRate: KokoroTTS.Constants.samplingRate)
    let server: Server
    switch findModel(dir) {
    case .failure(let e):
        // The hello still comes, so the bridge can say what is missing and how to install it.
        info.error = e.description
        server = Server(engine: nil, unavailable: e.description)
    case .success(let model):
        info.voices = model.voices
        do {
            info.modelSHA256 = try sha256(model.modelURL)
        } catch {
            info.error = "\(model.modelURL.path): \(error)"
        }
        server = Server(engine: Engine(model: model), unavailable: nil)
    }
    server.out.send(Outgoing.hello(helper: "koinon-voice-kokoro", version: helperVersion, capabilities: ["synthesize"], info: info))

    var reader = FrameReader()
    while true {
        let chunk = FileHandle.standardInput.availableData
        if chunk.isEmpty { break }
        reader.append(chunk)
        while true {
            let frame: Frame?
            do {
                frame = try reader.next()
            } catch {
                FileHandle.standardError.write(Data("koinon-voice-kokoro: \(error)\n".utf8))
                exit(2)
            }
            guard let frame else { break }
            guard case .json(let body) = frame,
                  let m = try? JSONDecoder().decode(Incoming.self, from: body), let id = m.id else { continue }
            switch m.type {
            case "request": server.request(id: id, method: m.method ?? "", body: body)
            case "cancel": server.cancel(id)
            default: continue
            }
        }
    }
    // stdin closed: the bridge is done. Finish what is queued, then exit.
    server.drain()
}

var args = Array(CommandLine.arguments.dropFirst())
var models = defaultModels
while let a = args.first {
    args.removeFirst()
    switch a {
    case "--models" where !args.isEmpty:
        models = args.removeFirst()
    case "version":
        print(helperVersion)
        exit(0)
    default:
        FileHandle.standardError.write(Data("usage: koinon-voice-kokoro [--models DIR] | version\n".utf8))
        exit(2)
    }
}
serve(models: models)
