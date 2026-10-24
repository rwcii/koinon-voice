import AVFAudio
import Foundation
import Speech

/// Segment is one finalized stretch of a transcript.
struct Segment: Encodable, Sendable {
    var text: String
    var start: Double
    var end: Double
}

/// transcribe runs SpeechAnalyzer with SpeechTranscriber, or with DictationTranscriber when
/// transcriber is "dictation", over a 16 kHz mono file. Keywords, if given, are the
/// analyzer's contextual strings: words the transcriber favours.
func transcribe(_ request: Request, _ out: Output) async throws {
    struct P: Decodable {
        var path: String
        var locale: String?
        var keywords: [String]?
        var transcriber: String?
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
    let module: any SpeechModule
    let stream: AsyncThrowingStream<Segment, Error>
    switch p.transcriber ?? "speech" {
    case "speech":
        let t = SpeechTranscriber(locale: locale, transcriptionOptions: [], reportingOptions: [], attributeOptions: [.audioTimeRange])
        module = t
        stream = segments(of: t.results) { r in Segment(text: String(r.text.characters), start: r.range.start.seconds, end: r.range.end.seconds) }
    case "dictation":
        let t = DictationTranscriber(locale: locale, preset: .shortDictation)
        module = t
        stream = segments(of: t.results) { r in Segment(text: String(r.text.characters), start: r.range.start.seconds, end: r.range.end.seconds) }
    default:
        throw HelperError(code: "bad_request", message: "transcriber \(p.transcriber ?? "") is not speech or dictation")
    }
    let assets = await AssetInventory.status(forModules: [module])
    let context = AnalysisContext()
    if let keywords = p.keywords, !keywords.isEmpty {
        context.contextualStrings[.general] = keywords
    }
    let started = Date()
    let results = Task { () -> [Segment] in
        var segments: [Segment] = []
        for try await s in stream {
            segments.append(s)
        }
        return segments
    }
    // The context goes in with the file, so it applies from the first sample.
    let analyzer: SpeechAnalyzer
    do {
        analyzer = try await SpeechAnalyzer(inputAudioFile: file, modules: [module], analysisContext: context, finishAfterFile: true)
    } catch {
        results.cancel()
        throw HelperError(code: "engine_error", message: "\(error)")
    }
    let segments = try await results.value
    withExtendedLifetime(analyzer) {} // the analyzer runs until its results end
    let text = segments.map(\.text).joined().trimmingCharacters(in: .whitespacesAndNewlines)
    out.result(request.id, Result(
        text: text, segments: segments, locale: locale.identifier,
        audioSeconds: Double(file.length) / format.sampleRate,
        elapsedMS: Int(Date().timeIntervalSince(started) * 1000), assets: "\(assets)"))
}

/// segments maps a transcriber's results to segments, so either transcriber feeds one loop.
func segments<S: AsyncSequence & Sendable>(of results: S, _ segment: @escaping @Sendable (S.Element) -> Segment) -> AsyncThrowingStream<Segment, Error> where S.Element: Sendable {
    AsyncThrowingStream { continuation in
        let task = Task {
            do {
                for try await r in results { continuation.yield(segment(r)) }
                continuation.finish()
            } catch {
                continuation.finish(throwing: error)
            }
        }
        continuation.onTermination = { _ in task.cancel() }
    }
}
