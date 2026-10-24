import AVFAudio
import AVFoundation
import Foundation
import Speech

/// duplex speaks a text through the default output while it listens on the default input
/// with Apple's voice processing (echo cancellation), and streams what it hears: the
/// interruption check (definition of done, live check 5). Events: "speaking" when playback
/// starts, "partial" and "final" transcripts, and "barge_in" when speech is heard during
/// playback (playback then stops, unless stop_on_speech is false). It listens for seconds
/// and writes the processed input to capture, a 16 kHz mono WAV file, if given.
@MainActor
func duplex(_ request: Request, _ out: Output) async throws {
    struct P: Decodable {
        var text: String
        var voice: String?
        var seconds: Double?
        var capture: String?
        var stopOnSpeech: Bool?
        enum CodingKeys: String, CodingKey {
            case text, voice, seconds, capture
            case stopOnSpeech = "stop_on_speech"
        }
    }
    struct Heard: Encodable {
        var text: String
        var start: Double
        var end: Double
    }
    struct Result: Encodable {
        var transcript: String
        var finals: [Heard]
        var voice: String
        var spokenSeconds: Double
        var bargeInMS: Int?
        var listenedSeconds: Double
        var voiceProcessing: Bool
        enum CodingKeys: String, CodingKey {
            case transcript, finals, voice
            case spokenSeconds = "spoken_seconds"
            case bargeInMS = "barge_in_ms"
            case listenedSeconds = "listened_seconds"
            case voiceProcessing = "voice_processing"
        }
    }
    let p = try request.params(P.self)
    let seconds = p.seconds ?? 20
    switch AVCaptureDevice.authorizationStatus(for: .audio) {
    case .authorized:
        break
    case .notDetermined:
        guard await AVCaptureDevice.requestAccess(for: .audio) else { throw microphoneDenied }
    default:
        throw microphoneDenied
    }

    // Synthesize first: the whole text is a few buffers, and the player's format is theirs.
    let synth = AVSpeechSynthesizer()
    let utterance = AVSpeechUtterance(string: p.text)
    let voice = p.voice.flatMap { AVSpeechSynthesisVoice(identifier: $0) } ?? AVSpeechSynthesisVoice(language: "en-US")
    utterance.voice = voice
    var buffers: [AVAudioPCMBuffer] = []
    for await b in synthesize(synth, utterance) { buffers.append(b) }
    guard let speechFormat = buffers.first?.format else {
        throw HelperError(code: "engine_error", message: "the synthesizer produced no audio")
    }
    let spokenSeconds = buffers.reduce(0.0) { $0 + Double($1.frameLength) / speechFormat.sampleRate }

    let engine = AVAudioEngine()
    // The mixer and output must exist before voice processing is enabled; created after it,
    // the output unit fails to initialize (-10875) when the engine starts.
    _ = engine.mainMixerNode
    let input = engine.inputNode
    do {
        try input.setVoiceProcessingEnabled(true)
    } catch {
        throw HelperError(code: "unavailable", message: "voice processing: \(error.localizedDescription)")
    }
    let player = AVAudioPlayerNode()
    engine.attach(player)
    engine.connect(player, to: engine.mainMixerNode, format: speechFormat)

    let transcriber = SpeechTranscriber(locale: Locale(identifier: "en-US"), transcriptionOptions: [],
                                        reportingOptions: [.volatileResults], attributeOptions: [.audioTimeRange])
    let micFormat = input.outputFormat(forBus: 0)
    guard micFormat.sampleRate > 0 else { throw HelperError(code: "unavailable", message: "no input device") }
    // With voice processing the input has many channels (20 on a USB camera) and the processed
    // voice is channel 0; a converter's downmix of that layout drops it, so take channel 0.
    guard let monoFormat = AVAudioFormat(standardFormatWithSampleRate: micFormat.sampleRate, channels: 1),
          let analyzerFormat = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [transcriber], considering: monoFormat),
          let toAnalyzer = AVAudioConverter(from: monoFormat, to: analyzerFormat),
          let captureFormat = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true),
          let toCapture = AVAudioConverter(from: monoFormat, to: captureFormat) else {
        throw HelperError(code: "engine_error", message: "cannot convert \(micFormat) for the transcriber")
    }
    var captureFile: AVAudioFile?
    if let path = p.capture {
        // A capture is the person's voice: only they may read the file.
        let mask = umask(0o077)
        defer { umask(mask) }
        captureFile = try AVAudioFile(forWriting: URL(fileURLWithPath: path), settings: captureFormat.settings,
                                      commonFormat: .pcmFormatInt16, interleaved: true)
    }

    let (inputs, feed) = AsyncStream<AnalyzerInput>.makeStream()
    let analyzer = SpeechAnalyzer(modules: [transcriber])
    try await analyzer.start(inputSequence: inputs)

    let state = DuplexState()
    let started = Date()
    let id = request.id
    let stopOnSpeech = p.stopOnSpeech ?? true
    let results = Task { () -> [Heard] in
        var finals: [Heard] = []
        for try await r in transcriber.results {
            let text = String(r.text.characters).trimmingCharacters(in: .whitespacesAndNewlines)
            let heard = Heard(text: text, start: r.range.start.seconds, end: r.range.end.seconds)
            if r.isFinal {
                if !text.isEmpty { finals.append(heard) }
                out.event(id, "final", heard)
            } else {
                out.event(id, "partial", heard)
            }
            if !text.isEmpty, let ms = state.bargeIn(Int(Date().timeIntervalSince(started) * 1000)) {
                out.event(id, "barge_in", ["since_start_ms": ms])
                if stopOnSpeech { player.stop() }
            }
        }
        return finals
    }

    input.installTap(onBus: 0, bufferSize: 4096, format: micFormat) { tapped, _ in
        guard let buffer = firstChannel(tapped, as: monoFormat) else { return }
        state.heard(buffer)
        if let converted = convert(buffer, with: toAnalyzer, to: analyzerFormat) {
            feed.yield(AnalyzerInput(buffer: converted))
        }
        if let file = captureFile, let converted = convert(buffer, with: toCapture, to: captureFormat) {
            try? file.write(from: converted)
        }
    }
    engine.prepare()
    try engine.start()
    // Speech heard after the last buffer has played is not an interruption.
    let last = buffers.count - 1
    for (i, b) in buffers.enumerated() {
        player.scheduleBuffer(b, completionCallbackType: .dataPlayedBack) { _ in
            if i == last { state.stopPlaying() }
        }
    }
    player.play()
    state.startPlaying()
    out.event(id, "speaking", ["text": p.text])

    try await Task.sleep(for: .seconds(seconds))
    let played = player.lastRenderTime.flatMap { player.playerTime(forNodeTime: $0) }
        .map { Double($0.sampleTime) / speechFormat.sampleRate } ?? 0
    FileHandle.standardError.write(Data(String(format: "duplex: input %d ch, %.1f s heard, peak %.0f%%; %.1f s of %.1f s spoken played\n",
                                               micFormat.channelCount, state.heardSeconds, state.peak * 100, played, spokenSeconds).utf8))
    input.removeTap(onBus: 0)
    player.stop()
    engine.stop()
    feed.finish()
    try await analyzer.finalizeAndFinishThroughEndOfInput()
    let finals = try await results.value
    captureFile = nil
    out.result(id, Result(
        transcript: finals.map(\.text).joined(separator: " "), finals: finals,
        voice: voice?.identifier ?? "", spokenSeconds: spokenSeconds, bargeInMS: state.bargeInMS,
        listenedSeconds: Date().timeIntervalSince(started), voiceProcessing: input.isVoiceProcessingEnabled))
}

/// DuplexState records whether playback is under way and when speech first interrupted it.
final class DuplexState: @unchecked Sendable {
    private let lock = NSLock()
    private var playing = false
    private(set) var bargeInMS: Int?
    private(set) var heardSeconds = 0.0
    private(set) var peak: Float = 0

    /// heard counts one mono input buffer.
    func heard(_ buffer: AVAudioPCMBuffer) {
        guard let samples = buffer.floatChannelData?[0] else { return }
        var top: Float = 0
        for i in 0..<Int(buffer.frameLength) { top = max(top, abs(samples[i])) }
        lock.lock()
        heardSeconds += Double(buffer.frameLength) / buffer.format.sampleRate
        peak = max(peak, top)
        lock.unlock()
    }

    func startPlaying() {
        lock.lock()
        playing = true
        lock.unlock()
    }

    func stopPlaying() {
        lock.lock()
        playing = false
        lock.unlock()
    }

    /// bargeIn records the first speech heard during playback and returns its time, once.
    func bargeIn(_ ms: Int) -> Int? {
        lock.lock()
        defer { lock.unlock() }
        guard playing, bargeInMS == nil else { return nil }
        bargeInMS = ms
        playing = false
        return ms
    }
}

/// firstChannel copies channel 0 of a deinterleaved float buffer into format, a mono format
/// at the same rate.
func firstChannel(_ buffer: AVAudioPCMBuffer, as format: AVAudioFormat) -> AVAudioPCMBuffer? {
    guard let source = buffer.floatChannelData?[0],
          let mono = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: buffer.frameLength),
          let target = mono.floatChannelData?[0] else { return nil }
    target.update(from: source, count: Int(buffer.frameLength))
    mono.frameLength = buffer.frameLength
    return mono
}

/// convert resamples one tapped buffer into format.
func convert(_ buffer: AVAudioPCMBuffer, with converter: AVAudioConverter, to format: AVAudioFormat) -> AVAudioPCMBuffer? {
    let capacity = AVAudioFrameCount(Double(buffer.frameLength) * format.sampleRate / buffer.format.sampleRate) + 1
    guard let out = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: capacity) else { return nil }
    var supplied = false
    var error: NSError?
    converter.convert(to: out, error: &error) { _, status in
        if supplied {
            status.pointee = .noDataNow
            return nil
        }
        supplied = true
        status.pointee = .haveData
        return buffer
    }
    return error == nil && out.frameLength > 0 ? out : nil
}
