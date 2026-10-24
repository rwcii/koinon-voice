import AVFAudio
import AVFoundation
import Foundation

/// record captures the default input for the given seconds as a 16 kHz mono 16-bit WAV
/// file, the format transcribe takes.
func record(seconds: Double, to path: String) async throws {
    // Without access macOS hands the engine silence, not an error, so ask first.
    switch AVCaptureDevice.authorizationStatus(for: .audio) {
    case .authorized:
        break
    case .notDetermined:
        guard await AVCaptureDevice.requestAccess(for: .audio) else { throw microphoneDenied }
    default:
        throw microphoneDenied
    }
    let engine = AVAudioEngine()
    let input = engine.inputNode
    let source = input.outputFormat(forBus: 0)
    guard source.sampleRate > 0 else { throw HelperError(code: "unavailable", message: "no input device") }
    guard let target = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true),
          let converter = AVAudioConverter(from: source, to: target) else {
        throw HelperError(code: "engine_error", message: "cannot convert \(source) to 16 kHz mono")
    }
    // A recording is the person's voice: only they may read the file.
    let mask = umask(0o077)
    defer { umask(mask) }
    let file = try AVAudioFile(forWriting: URL(fileURLWithPath: path), settings: target.settings, commonFormat: .pcmFormatInt16, interleaved: true)
    var failure: Error?
    var peak: Int16 = 0
    input.installTap(onBus: 0, bufferSize: 4096, format: source) { buffer, _ in
        let capacity = AVAudioFrameCount(Double(buffer.frameLength) * target.sampleRate / source.sampleRate) + 1
        guard let out = AVAudioPCMBuffer(pcmFormat: target, frameCapacity: capacity) else { return }
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
        if let error { failure = error; return }
        if let samples = out.int16ChannelData?[0] {
            for i in 0..<Int(out.frameLength) { peak = max(peak, samples[i] == .min ? .max : abs(samples[i])) }
        }
        do { try file.write(from: out) } catch { failure = error }
    }
    try engine.start()
    FileHandle.standardError.write(Data("Recording for \(Int(seconds)) seconds…\n".utf8))
    try await Task.sleep(for: .seconds(seconds))
    input.removeTap(onBus: 0)
    engine.stop()
    if let failure { throw failure }
    if peak == 0 {
        throw HelperError(code: "bad_audio", message: "the recording is silent: check the microphone access in System Settings > Privacy & Security > Microphone, and the input device")
    }
    FileHandle.standardError.write(Data(String(format: "Done: %.1f s, peak %.0f%% of full scale.\n", Double(file.length) / 16000, Double(peak) / 327.67).utf8))
}

let microphoneDenied = HelperError(code: "unavailable", message: "no microphone access: allow this terminal in System Settings > Privacy & Security > Microphone, then run it again")
