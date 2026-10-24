import AVFAudio
import AVFoundation
import Foundation

/// record captures one channel of the default input for the given seconds as a 16 kHz mono
/// 16-bit WAV file, the format transcribe takes. Channel is 1-based: a mixer such as a
/// 14-channel USB interface carries the microphone on one channel, and folding all of them
/// into one gives silence.
func record(seconds: Double, to path: String, channel: Int = 1) async throws {
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
    let channels = Int(source.channelCount)
    guard channel >= 1, channel <= channels else {
        throw HelperError(code: "bad_request", message: "the input device has \(channels) channels; there is no channel \(channel)")
    }
    guard let mono = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: source.sampleRate, channels: 1, interleaved: false),
          let target = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true),
          let converter = AVAudioConverter(from: mono, to: target) else {
        throw HelperError(code: "engine_error", message: "cannot convert \(source) to 16 kHz mono")
    }
    // A recording is the person's voice: only they may read the file.
    let mask = umask(0o077)
    defer { umask(mask) }
    let file = try AVAudioFile(forWriting: URL(fileURLWithPath: path), settings: target.settings, commonFormat: .pcmFormatInt16, interleaved: true)
    var failure: Error?
    var peak: Int16 = 0
    // Every channel's peak, to name the one with sound when the chosen one is silent.
    var peaks = [Float](repeating: 0, count: channels)
    // The tap's format must be the input's; the chosen channel is copied out of it.
    input.installTap(onBus: 0, bufferSize: 4096, format: source) { buffer, _ in
        guard let data = buffer.floatChannelData,
              let one = AVAudioPCMBuffer(pcmFormat: mono, frameCapacity: buffer.frameLength),
              let dst = one.floatChannelData?[0] else { return }
        let frames = Int(buffer.frameLength)
        // Non-interleaved float input has a pointer per channel; interleaved has one.
        let interleaved = buffer.format.isInterleaved
        for c in 0..<channels {
            for i in 0..<frames {
                let v = interleaved ? data[0][i * channels + c] : data[c][i]
                peaks[c] = max(peaks[c], abs(v))
                if c == channel - 1 { dst[i] = v }
            }
        }
        one.frameLength = buffer.frameLength
        let capacity = AVAudioFrameCount(Double(frames) * target.sampleRate / source.sampleRate) + 1
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
            return one
        }
        if let error { failure = error; return }
        if let samples = out.int16ChannelData?[0] {
            for i in 0..<Int(out.frameLength) { peak = max(peak, samples[i] == .min ? .max : abs(samples[i])) }
        }
        do { try file.write(from: out) } catch { failure = error }
    }
    try engine.start()
    let channelNote = channels > 1 ? ", channel \(channel) of \(channels)" : ""
    FileHandle.standardError.write(Data("Recording for \(Int(seconds.rounded())) seconds\(channelNote)…\n".utf8))
    try await Task.sleep(for: .seconds(seconds))
    input.removeTap(onBus: 0)
    engine.stop()
    if let failure { throw failure }
    if peak == 0 {
        throw HelperError(code: "bad_audio", message: "the recording is silent: " + silenceHint(peaks, channel: channel))
    }
    FileHandle.standardError.write(Data(String(format: "Done: %.1f s, peak %.0f%% of full scale.\n", Double(file.length) / 16000, Double(peak) / 327.67).utf8))
}

/// silenceHint says where sound was, if anywhere: a channel above 1% of full scale is one a
/// person speaking reaches; below that is the device's own noise.
func silenceHint(_ peaks: [Float], channel: Int) -> String {
    let levels = peaks.enumerated().map { String(format: "%d: %.1f%%", $0.offset + 1, $0.element * 100) }.joined(separator: ", ")
    if let loudest = peaks.indices.max(by: { peaks[$0] < peaks[$1] }), peaks[loudest] > 0.01 {
        return String(format: "channel %d has sound (peak %.0f%%); record that channel instead. Peak by channel: %@", loudest + 1, peaks[loudest] * 100, levels)
    }
    if peaks.count > 1 {
        return "no channel of the input device has sound, so the device is not sending the microphone to the Mac: check its routing to USB and the channel's gain. Peak by channel: \(levels)"
    }
    return "check the microphone access in System Settings > Privacy & Security > Microphone, and the input device"
}

let microphoneDenied = HelperError(code: "unavailable", message: "no microphone access: allow this terminal in System Settings > Privacy & Security > Microphone, then run it again")
