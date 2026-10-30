import Foundation
import Testing
@testable import KokoroText

@Test func koinonHasOnePronunciation() {
    #expect(prepare("Codex and Claude are waiting in Koinon.") == "Codex and Claude are waiting in [Koinon](/kˈYnˌɑn/).")
    #expect(prepare("the koinon daemon, KOINON's board") == "the [koinon](/kˈYnˌɑn/) daemon, [KOINON](/kˈYnˌɑn/)'s board")
}

@Test func sessionNamesAreLeftToMisaki() {
    // The maintainer accepted claude-koinon-voice-b9 as Misaki says it (DECISIONS.md#kv-12-listen).
    #expect(prepare("claude-koinon-voice-b9 asks.") == "claude-koinon-voice-b9 asks.")
    #expect(prepare("Koinons and koinonia") == "Koinons and koinonia")
}

@Test func linksKeepOnlyTheirText() {
    #expect(prepare("See [the board](https://example.invalid/) and [x](/hˈæk/).") == "See the board and x.")
}

@Test func pcmIsLittleEndianAndClipped() {
    #expect(pcm16([0, 1, -1, 2, -2, 0.5]) == Data([0, 0, 0xff, 0x7f, 0x01, 0x80, 0xff, 0x7f, 0x01, 0x80, 0x00, 0x40]))
}

@Test func chunksKeepWholeSamples() {
    let parts = chunks(Data(repeating: 1, count: 10), size: 5)
    #expect(parts.map(\.count) == [4, 4, 2])
    #expect(chunks(Data(), size: 4).isEmpty)
}

@Test func missingModelNamesTheFileAndInstall() throws {
    let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: dir) }

    guard case .failure(let e) = findModel(dir) else {
        Issue.record("an empty directory has a model")
        return
    }
    #expect(e.description.contains(dir.appendingPathComponent("kokoro-v1_0.safetensors").path))
    #expect(e.description.contains("koinon-voice kokoro install"))

    try Data("model".utf8).write(to: dir.appendingPathComponent(modelFile))
    guard case .failure(let v) = findModel(dir) else {
        Issue.record("a directory with no voice has a model")
        return
    }
    #expect(v.description.contains("voices") && v.description.contains("koinon-voice kokoro install"))

    try FileManager.default.createDirectory(at: dir.appendingPathComponent(voicesDir), withIntermediateDirectories: true)
    try Data("voice".utf8).write(to: dir.appendingPathComponent("voices/af_heart.safetensors"))
    guard case .success(let model) = findModel(dir) else {
        Issue.record("no model")
        return
    }
    #expect(model.voices == ["af_heart"])
    // SHA-256 of "model".
    #expect(try sha256(model.modelURL) == "9372c470eeadd5ecd9c3c74c2b3cb633f8e2f2fad799250a0f70d652b6b825e4")
}
