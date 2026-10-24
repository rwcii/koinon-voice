import AVFAudio
import Foundation
import SidecarProtocol

/// speak synthesizes each input frame's text as it arrives, in order, to the default output
/// or to a file. The first audio event marks the first sample: the first one the output
/// device renders, or the first one written to the file.
@MainActor
func speak(_ request: Request, _ out: Output) async throws {
    struct P: Decodable {
        var voice: String?
        var output: String?
    }
    struct Text: Decodable { var text: String }
    struct FirstAudio: Encodable {
        var sinceRequestMS: Double
        var sinceFirstInputMS: Double
        enum CodingKeys: String, CodingKey {
            case sinceRequestMS = "since_request_ms"
            case sinceFirstInputMS = "since_first_input_ms"
        }
    }
    struct Result: Encodable {
        var voice: String
        var audioSeconds: Double
        var outputLatencyMS: Double
        enum CodingKeys: String, CodingKey {
            case voice
            case audioSeconds = "audio_seconds"
            case outputLatencyMS = "output_latency_ms"
        }
    }
    let p = try request.params(P.self)
    let voice = p.voice.flatMap(AVSpeechSynthesisVoice.init(identifier:)) ?? AVSpeechSynthesisVoice(language: "en-US")
    guard let voice else { throw HelperError(code: "unavailable", message: "no en-US voice") }

    // One request plays at a time: the bridge speaks summaries in turn.
    let sink: AudioSink = try p.output.map { try FileSink(path: $0) } ?? DeviceSink.shared
    // The request opens before the summary starts, so the output is ready by its first sentence.
    try sink.prepare()
    let requested = Date()
    var firstInput: Date?
    var sentFirst = false
    let synth = AVSpeechSynthesizer()
    var frames: AVAudioFramePosition = 0
    var sampleRate = 0.0

    for await body in request.input {
        guard let text = try? JSONDecoder().decode(Params<Text>.self, from: body).params?.text,
              !text.trimmingCharacters(in: .whitespaces).isEmpty else { continue }
        if firstInput == nil { firstInput = Date() }
        let utterance = AVSpeechUtterance(string: text)
        utterance.voice = voice
        // One utterance at a time, in order; each buffer plays as soon as it is synthesized.
        for await buffer in synthesize(synth, utterance) {
            frames += AVAudioFramePosition(buffer.frameLength)
            sampleRate = buffer.format.sampleRate
            try await sink.write(buffer) {
                guard !sentFirst else { return }
                sentFirst = true
                let now = Date()
                out.event(request.id, "first_audio", FirstAudio(
                    sinceRequestMS: now.timeIntervalSince(requested) * 1000,
                    sinceFirstInputMS: now.timeIntervalSince(firstInput ?? requested) * 1000))
            }
        }
    }
    await sink.finish()
    out.result(request.id, Result(
        voice: voice.identifier, audioSeconds: sampleRate > 0 ? Double(frames) / sampleRate : 0,
        outputLatencyMS: sink.latency * 1000))
}

/// synthesize yields one utterance's PCM buffers as the synthesizer produces them.
func synthesize(_ synth: AVSpeechSynthesizer, _ utterance: AVSpeechUtterance) -> AsyncStream<AVAudioPCMBuffer> {
    AsyncStream { continuation in
        synth.write(utterance) { buffer in
            guard let pcm = buffer as? AVAudioPCMBuffer else { return }
            if pcm.frameLength == 0 {
                continuation.finish() // the synthesizer's end of utterance
            } else {
                continuation.yield(pcm)
            }
        }
    }
}

@MainActor
protocol AudioSink: AnyObject {
    /// write queues a buffer; first runs once its first sample has gone out.
    func write(_ buffer: AVAudioPCMBuffer, first: @escaping () -> Void) async throws
    /// prepare gets the output ready before the first buffer.
    func prepare() throws
    /// finish returns when everything written has gone out.
    func finish() async
    var latency: Double { get }
}

/// DeviceSink plays through the default output with one engine kept running across requests,
/// so a request's timing does not include starting the engine.
@MainActor
final class DeviceSink: AudioSink {
    static let shared = DeviceSink()
    private let engine = AVAudioEngine()
    private let player = AVAudioPlayerNode()
    private var format: AVAudioFormat?
    private var pending = 0
    private var drained: CheckedContinuation<Void, Never>?
    private var watching = false

    var latency: Double { engine.outputNode.presentationLatency }

    private init() { engine.attach(player) }

    func prepare() throws {
        // The system voices synthesize 22.05 kHz mono; write reconnects for any other format.
        guard format == nil, let f = AVAudioFormat(standardFormatWithSampleRate: 22050, channels: 1) else { return }
        engine.connect(player, to: engine.mainMixerNode, format: f)
        try engine.start()
        format = f
    }

    func write(_ buffer: AVAudioPCMBuffer, first: @escaping () -> Void) async throws {
        if format != buffer.format {
            player.stop()
            engine.stop()
            engine.connect(player, to: engine.mainMixerNode, format: buffer.format)
            try engine.start()
            format = buffer.format
        }
        if !player.isPlaying {
            // A stopped player's timeline restarts at zero, so the first rendered sample of
            // this request is the first sample time above zero.
            player.play()
            watchFirst(first)
        }
        pending += 1
        player.scheduleBuffer(buffer, completionCallbackType: .dataPlayedBack) { [weak self] _ in
            Task { @MainActor in self?.played() }
        }
    }

    private func watchFirst(_ first: @escaping () -> Void) {
        guard !watching else { return }
        watching = true
        Task {
            while true {
                if let node = player.lastRenderTime, let t = player.playerTime(forNodeTime: node), t.sampleTime > 0 {
                    break
                }
                try? await Task.sleep(for: .milliseconds(1))
            }
            watching = false
            first()
        }
    }

    private func played() {
        pending -= 1
        if pending == 0, let d = drained {
            drained = nil
            d.resume()
        }
    }

    func finish() async {
        if pending > 0 {
            await withCheckedContinuation { drained = $0 }
        }
        player.stop()
    }
}

/// FileSink writes a request's audio to a file.
@MainActor
final class FileSink: AudioSink {
    private let url: URL
    private var file: AVAudioFile?
    var latency: Double { 0 }

    init(path: String) throws {
        url = URL(fileURLWithPath: path)
    }

    func prepare() throws {}

    func write(_ buffer: AVAudioPCMBuffer, first: @escaping () -> Void) async throws {
        if file == nil {
            file = try AVAudioFile(forWriting: url, settings: buffer.format.settings)
        }
        try file?.write(from: buffer)
        first()
    }

    func finish() async { file = nil }
}
