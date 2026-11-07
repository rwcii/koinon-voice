import AVFAudio
import Foundation
import SidecarProtocol

/// play plays the request's audio frames as they arrive, 16-bit signed little-endian mono PCM
/// at the sample rate its params give: through the default output with voice processing on
/// (ADR 002, Decision 3), or into a file. The Kokoro helper's audio plays this way. Its first
/// audio event and result are speak's; a cancel stops playback at once.
@MainActor
func play(_ request: Request, _ out: Output) async throws {
    struct P: Decodable {
        var sampleRate: Double
        var output: String?
        enum CodingKeys: String, CodingKey {
            case output
            case sampleRate = "sample_rate"
        }
    }
    struct FirstAudio: Encodable {
        var sinceRequestMS: Double
        var sinceFirstInputMS: Double
        enum CodingKeys: String, CodingKey {
            case sinceRequestMS = "since_request_ms"
            case sinceFirstInputMS = "since_first_input_ms"
        }
    }
    struct Result: Encodable {
        var audioSeconds: Double
        var outputLatencyMS: Double
        enum CodingKeys: String, CodingKey {
            case audioSeconds = "audio_seconds"
            case outputLatencyMS = "output_latency_ms"
        }
    }
    let p = try request.params(P.self)
    guard p.sampleRate > 0, let format = AVAudioFormat(standardFormatWithSampleRate: p.sampleRate, channels: 1) else {
        throw HelperError(code: "bad_request", message: "sample_rate \(p.sampleRate) is not a sample rate")
    }

    let sink: AudioSink
    if let path = p.output {
        sink = try FileSink(path: path)
    } else {
        try await requireMicrophone()
        sink = DeviceSink.shared
    }
    try sink.prepare(sampleRate: p.sampleRate)
    let requested = Date()
    var firstInput: Date?
    var sentFirst = false
    var frames: AVAudioFramePosition = 0

    // A cancel ends the audio stream, so the loop ends with the task cancelled.
    for await pcm in request.audio {
        if firstInput == nil { firstInput = Date() }
        guard let buffer = pcmBuffer(pcm, format) else { continue }
        frames += AVAudioFramePosition(buffer.frameLength)
        try await sink.write(buffer) {
            guard !sentFirst else { return }
            sentFirst = true
            let now = Date()
            out.event(request.id, "first_audio", FirstAudio(
                sinceRequestMS: now.timeIntervalSince(requested) * 1000,
                sinceFirstInputMS: now.timeIntervalSince(firstInput ?? requested) * 1000))
        }
    }
    try await withTaskCancellationHandler {
        try Task.checkCancellation()
        await sink.finish()
        try Task.checkCancellation()
    } onCancel: {
        Task { @MainActor in sink.stop() }
    }
    out.result(request.id, Result(audioSeconds: Double(frames) / p.sampleRate, outputLatencyMS: sink.latency * 1000))
}

/// pcmBuffer turns 16-bit signed little-endian mono samples into a buffer of format, a
/// standard mono format; an odd last byte is dropped.
func pcmBuffer(_ pcm: Data, _ format: AVAudioFormat) -> AVAudioPCMBuffer? {
    let n = pcm.count / 2
    guard n > 0, let buffer = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: AVAudioFrameCount(n)),
          let samples = buffer.floatChannelData?[0] else { return nil }
    pcm.withUnsafeBytes { raw in
        for i in 0..<n {
            samples[i] = Float(Int16(littleEndian: raw.loadUnaligned(fromByteOffset: i * 2, as: Int16.self))) / 32768
        }
    }
    buffer.frameLength = AVAudioFrameCount(n)
    return buffer
}
