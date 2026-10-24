import AVFAudio
import Foundation
import Speech

/// transcribe runs SpeechAnalyzer with SpeechTranscriber over a 16 kHz mono file.
func transcribe(_ request: Request, _ out: Output) async throws {
    struct P: Decodable {
        var path: String
        var locale: String?
    }
    struct Segment: Encodable {
        var text: String
        var start: Double
        var end: Double
    }
    struct Result: Encodable {
        var text: String
        var segments: [Segment]
        var locale: String
        var audioSeconds: Double
        var elapsedMS: Int
        var assets: String
        enum CodingKeys: String, CodingKey {
            case text, segments, locale, assets
            case audioSeconds = "audio_seconds"
            case elapsedMS = "elapsed_ms"
        }
    }
    let p = try request.params(P.self)
    let file: AVAudioFile
    do {
        file = try AVAudioFile(forReading: URL(fileURLWithPath: p.path))
    } catch {
        throw HelperError(code: "bad_audio", message: "cannot read \(p.path): \(error.localizedDescription)")
    }
    let format = file.fileFormat
    guard format.sampleRate == 16000, format.channelCount == 1 else {
        throw HelperError(code: "bad_audio", message: "want 16 kHz mono, got \(Int(format.sampleRate)) Hz with \(format.channelCount) channels")
    }
    guard file.length > 0 else {
        throw HelperError(code: "bad_audio", message: "the file has no audio")
    }

    let locale = Locale(identifier: p.locale ?? "en-US")
    let transcriber = SpeechTranscriber(locale: locale, transcriptionOptions: [], reportingOptions: [], attributeOptions: [.audioTimeRange])
    let assets = await AssetInventory.status(forModules: [transcriber])
    let analyzer = SpeechAnalyzer(modules: [transcriber])
    let started = Date()
    let results = Task { () -> [Segment] in
        var segments: [Segment] = []
        for try await r in transcriber.results {
            let range = r.range
            segments.append(Segment(text: String(r.text.characters), start: range.start.seconds, end: range.end.seconds))
        }
        return segments
    }
    do {
        try await analyzer.start(inputAudioFile: file, finishAfterFile: true)
    } catch {
        results.cancel()
        throw HelperError(code: "engine_error", message: "\(error)")
    }
    let segments = try await results.value
    let text = segments.map(\.text).joined().trimmingCharacters(in: .whitespacesAndNewlines)
    out.result(request.id, Result(
        text: text, segments: segments, locale: locale.identifier,
        audioSeconds: Double(file.length) / format.sampleRate,
        elapsedMS: Int(Date().timeIntervalSince(started) * 1000), assets: "\(assets)"))
}
