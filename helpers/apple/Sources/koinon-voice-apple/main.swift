// koinon-voice-apple is the bridge's macOS helper (ADR 002, Decision 1). With no arguments
// it speaks the sidecar protocol on stdin and stdout. `record SECONDS FILE` records the
// default input to a 16 kHz mono WAV file, for the maintainer's test recordings.
import AVFAudio
import Foundation
import FoundationModels
import Speech

let helperVersion = "0.1.0"

/// Info is what the hello reports about this Mac, for the scorecard's record.
struct Info: Encodable {
    var os = ProcessInfo.processInfo.operatingSystemVersionString
    var foundationModels = frameworkVersion("FoundationModels")
    var speech = frameworkVersion("Speech")
    var model = modelAvailability()
    var voice = AVSpeechSynthesisVoice(language: "en-US")?.identifier ?? ""
    var transcriberAvailable = SpeechTranscriber.isAvailable

    enum CodingKeys: String, CodingKey {
        case os, speech, model, voice
        case foundationModels = "foundation_models"
        case transcriberAvailable = "transcriber_available"
    }
}

/// frameworkVersion reads a system framework's source version, such as 3525.2.2 for Speech;
/// its bundle version says only 1.0.
func frameworkVersion(_ name: String) -> String {
    let url = URL(fileURLWithPath: "/System/Library/Frameworks/\(name).framework/Resources/version.plist")
    let source = NSDictionary(contentsOf: url)?["SourceVersion"]
    guard let n = (source as? NSNumber)?.uint64Value ?? (source as? String).flatMap({ UInt64($0) }) else { return "unknown" }
    // SourceVersion packs A.B.C.D.E with three digits for each part after A.
    var parts = [n / 1_000_000_000_000]
    for shift in [1_000_000_000, 1_000_000, 1000, 1] as [UInt64] { parts.append(n / shift % 1000) }
    while parts.count > 3, parts.last == 0 { parts.removeLast() }
    return parts.map(String.init).joined(separator: ".")
}

func modelAvailability() -> String {
    switch SystemLanguageModel.default.availability {
    case .available: return "available"
    case .unavailable(let reason): return "unavailable: \(reason)"
    }
}

let args = CommandLine.arguments.dropFirst()
switch args.first {
case nil:
    await serve(handlers: ["summarize": summarize, "transcribe": transcribe, "speak": speak], info: Info())
case "record":
    let rest = Array(args.dropFirst())
    guard rest.count == 2, let seconds = Double(rest[0]), seconds > 0 else {
        FileHandle.standardError.write(Data("usage: koinon-voice-apple record SECONDS FILE.wav\n".utf8))
        exit(2)
    }
    do {
        try await record(seconds: seconds, to: rest[1])
    } catch {
        let reason = (error as? HelperError)?.message ?? "\(error)"
        FileHandle.standardError.write(Data("koinon-voice-apple record: \(reason)\n".utf8))
        exit(1)
    }
case "version":
    print(helperVersion)
default:
    FileHandle.standardError.write(Data("usage: koinon-voice-apple [record SECONDS FILE.wav | version]\n".utf8))
    exit(2)
}
