// What the Kokoro helper does without the model: the text it gives Misaki, the PCM it sends
// and the model files it needs.
import CryptoKit
import Foundation

/// The phonemes of names whose pronunciation the maintainer chose. Misaki reads them from its
/// inline form `[word](/phonemes/)`.
public let lexicon: [String: String] = [
    // "KOY-non", in upper and lower case (DECISIONS.md#kv-12-listen; scorecard-kokoro-native.md, check 16).
    "koinon": "kˈYnˌɑn",
]

/// prepare turns a sentence into Misaki's input. Markdown links keep only their text, also
/// links inside links, and the brackets that remain are removed, so a summary cannot set
/// phonemes. Then each lexicon word, in any case, gets its fixed phonemes. A lexicon word
/// joined to others by a hyphen, as in a session name, is left to Misaki.
///
/// MisakiSwift 1.0.6 reads the position of an inline form in UTF-16 units as a position in
/// characters (EnglishG2P.preprocess), so the two must agree for each character before the
/// form. The text is composed (NFC) and its line ends made "\n" first; a lexicon word after a
/// character of more than one UTF-16 unit, such as an emoji, is left to Misaki.
public func prepare(_ text: String) -> String {
    var out = text.precomposedStringWithCanonicalMapping.replacingOccurrences(of: "\r\n", with: "\n")
    while true {
        let next = out.replacingOccurrences(of: #"\[([^\[\]]*)\]\([^\)]*\)"#, with: "$1", options: .regularExpression)
        if next == out { break }
        out = next
    }
    out.removeAll { $0 == "[" || $0 == "]" }
    for (word, phonemes) in lexicon {
        let pattern = #"(?<![\p{L}\p{M}\p{N}_-])"# + NSRegularExpression.escapedPattern(for: word) + #"(?![\p{L}\p{M}\p{N}_-])"#
        let re = try! NSRegularExpression(pattern: pattern, options: [.caseInsensitive])
        let matches = re.matches(in: out, range: NSRange(out.startIndex..., in: out))
        for m in matches.reversed() {
            guard let r = Range(m.range, in: out) else { continue }
            let before = out[..<r.lowerBound]
            guard before.utf16.count == before.count else { continue }
            out.replaceSubrange(r, with: "[" + out[r] + "](/" + phonemes + "/)")
        }
    }
    return out
}

/// pcm16 converts samples in -1...1 to 16-bit signed little-endian PCM, clipping the rest.
public func pcm16(_ samples: [Float]) -> Data {
    var out = Data(capacity: samples.count * 2)
    for x in samples {
        let v = Int16((max(-1, min(1, x)) * 32767).rounded()).littleEndian
        withUnsafeBytes(of: v) { out.append(contentsOf: $0) }
    }
    return out
}

/// chunks splits PCM into audio frames of at most size bytes, each a whole number of samples.
public func chunks(_ pcm: Data, size: Int) -> [Data] {
    let size = max(2, size - size % 2)
    return stride(from: 0, to: pcm.count, by: size).map { pcm.subdata(in: pcm.startIndex + $0..<pcm.startIndex + min($0 + size, pcm.count)) }
}

/// The model file, relative to the model directory, and its voices' directory. These are the
/// paths `koinon-voice kokoro install` writes (internal/kokoro).
public let modelFile = "kokoro-v1_0.safetensors"
public let voicesDir = "voices"

public let installCommand = "koinon-voice kokoro install"

/// ModelError is a model directory the helper cannot synthesize from.
public struct ModelError: Error, CustomStringConvertible {
    public var description: String
}

/// Model is what the helper found in its model directory.
public struct Model {
    public var dir: URL
    public var voices: [String]

    public var modelURL: URL { dir.appendingPathComponent(modelFile) }

    public func voiceURL(_ name: String) -> URL {
        dir.appendingPathComponent(voicesDir).appendingPathComponent(name + ".safetensors")
    }
}

/// findModel checks the model directory: the model file and at least one voice. The error
/// names the missing file and the install command.
public func findModel(_ dir: URL) -> Result<Model, ModelError> {
    let fm = FileManager.default
    let model = dir.appendingPathComponent(modelFile)
    guard fm.fileExists(atPath: model.path) else {
        return .failure(ModelError(description: "no Kokoro model: \(model.path) is missing; run \(installCommand)"))
    }
    let names = (try? fm.contentsOfDirectory(atPath: dir.appendingPathComponent(voicesDir).path)) ?? []
    let voices = names.filter { $0.hasSuffix(".safetensors") }.map { String($0.dropLast(".safetensors".count)) }.sorted()
    guard !voices.isEmpty else {
        return .failure(ModelError(description: "no Kokoro voice: \(dir.appendingPathComponent(voicesDir).path) has no .safetensors file; run \(installCommand)"))
    }
    return .success(Model(dir: dir, voices: voices))
}

/// sha256 is a file's SHA-256 in lower-case hex. The file is mapped, not copied.
public func sha256(_ url: URL) throws -> String {
    let data = try Data(contentsOf: url, options: .alwaysMapped)
    return SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
}
