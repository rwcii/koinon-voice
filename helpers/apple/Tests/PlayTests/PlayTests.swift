import AVFAudio
import Foundation
import Testing
@testable import koinon_voice_apple
import SidecarProtocol

/// Frames is what play wrote, decoded.
final class Frames: @unchecked Sendable {
    private let lock = NSLock()
    private var reader = FrameReader()
    private var messages: [[String: Any]] = []

    func write(_ data: Data) {
        lock.lock()
        defer { lock.unlock() }
        reader.append(data)
        while case .json(let body)? = try? reader.next() {
            if let m = try? JSONSerialization.jsonObject(with: body) as? [String: Any] { messages.append(m) }
        }
    }

    var all: [[String: Any]] {
        lock.lock()
        defer { lock.unlock() }
        return messages
    }
}

func playRequest(output: String, sampleRate: Int) -> (Request, AsyncStream<Data>.Continuation) {
    let body = Data(#"{"type":"request","id":1,"method":"play","params":{"sample_rate":\#(sampleRate),"output":"\#(output)"}}"#.utf8)
    let (input, _) = AsyncStream<Data>.makeStream()
    let (audio, frames) = AsyncStream<Data>.makeStream()
    return (Request(id: 1, body: body, input: input, audio: audio), frames)
}

/// pcm is n samples of 16-bit little-endian PCM at value v.
func pcm(_ n: Int, _ v: Int16) -> Data {
    var d = Data()
    for _ in 0..<n { withUnsafeBytes(of: v.littleEndian) { d.append(contentsOf: $0) } }
    return d
}

func tempFile() -> String {
    FileManager.default.temporaryDirectory.appendingPathComponent("play-\(UUID().uuidString).wav").path
}

// Check 11: play writes the frames it receives to a FileSink at the given rate.
@MainActor @Test func playWritesFramesAtRate() async throws {
    let path = tempFile()
    defer { try? FileManager.default.removeItem(atPath: path) }
    let (request, frames) = playRequest(output: path, sampleRate: 24000)
    frames.yield(pcm(2400, 16384))
    frames.yield(pcm(2400, -16384))
    frames.finish()
    let got = Frames()
    try await play(request, Output(write: got.write))

    let file = try AVAudioFile(forReading: URL(fileURLWithPath: path))
    #expect(file.fileFormat.sampleRate == 24000)
    #expect(file.length == 4800)
    let buffer = try #require(AVAudioPCMBuffer(pcmFormat: file.processingFormat, frameCapacity: 4800))
    try file.read(into: buffer)
    let samples = try #require(buffer.floatChannelData?[0])
    #expect(samples[0] == 0.5 && samples[4799] == -0.5)

    let names = got.all.map { ($0["type"] as? String ?? "") + ":" + ($0["event"] as? String ?? "") }
    #expect(names == ["event:first_audio", "result:"])
    let result = got.all.last?["data"] as? [String: Any]
    #expect(result?["audio_seconds"] as? Double == 0.2)
}

// Check 11: a cancel stops play: no frame after it is written, and there is no result.
@MainActor @Test func cancelStopsPlay() async throws {
    let path = tempFile()
    defer { try? FileManager.default.removeItem(atPath: path) }
    let (request, frames) = playRequest(output: path, sampleRate: 24000)
    let got = Frames()
    let task = Task { @MainActor in try await play(request, Output(write: got.write)) }
    frames.yield(pcm(2400, 100))
    while got.all.isEmpty { await Task.yield() } // the first frame is written
    task.cancel()
    frames.finish() // the server ends a cancelled request's audio
    frames.yield(pcm(2400, 200))
    await #expect(throws: CancellationError.self) { try await task.value }
    #expect(got.all.allSatisfy { $0["type"] as? String != "result" })
    let file = try AVAudioFile(forReading: URL(fileURLWithPath: path))
    #expect(file.length == 2400)
}
