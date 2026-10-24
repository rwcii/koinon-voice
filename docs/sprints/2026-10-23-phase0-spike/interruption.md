# Interruption

Chunk 05's live check 5 (definition of done): with the bridge speaking through the speakers,
the maintainer interrupts, and the transcript should hold the interruption and none of the
bridge's own speech. The captured input stays under `~/.local/share/koinon-voice/` and is not
in Git (definition of done, Private data); this file holds the transcripts, timings and
hashes.

## Run

- Date: 2026-10-24, on the maintainer's Mac (Apple M2 Max, macOS 26.5, build 25F71), from
  the maintainer's own Terminal window, which has microphone access.
- Devices: input the EMEET SmartCam Nova 4K, output the DLZ Creator USB mixer (4 output
  channels), both at 48 kHz. With voice processing on, the input reaches the helper as 20
  channels at 48 kHz, of which channel 0 is the processed voice.
- Helper: `koinon-voice-apple` 0.1.0 built from this chunk's head (SHA-256 prefix
  `e01baf6525fdf171`), `duplex` with `AVAudioEngine`, `setVoiceProcessingEnabled(true)` on the
  input node, a player node for the synthesized speech and a streaming `SpeechTranscriber`
  (en-US, volatile results) on the processed input. Barge-in is the first non-empty
  transcript heard while the speech is playing; playback then stops.
- Speech: text 03 of `tts-summaries.md` (14.7 s), voice
  `com.apple.voice.premium.en-US.Zoe`, the configured voice (scorecard-speech.md). The bench
  listens for 25 s.
- Commands: `koinon-voice bench interrupt --control`, in which nobody speaks, then
  `koinon-voice bench interrupt`, in which the maintainer says "Stop, wait, I have a
  question." a few seconds in.

## Results

| Run | Barge-in | Transcript | Interruption words missed | Bridge words in the transcript | Capture SHA-256 prefix |
| --- | --- | --- | --- | --- | --- |
| control, 11:39 | none | (empty) | — | none | `ba914889b98d9c97` |
| interrupt, 11:43 | 4,126 ms after the start | "Wait, I have a question." | stop | none | `fca78dd04a478663` |

- **Echo cancellation held.** In the control run the 25.5 s capture is nearly silent while
  the bridge speaks (per-second peaks of 0 to 0.05 of full scale after the first 3 s, at most
  0.08 before), and the transcript is empty. In the interrupt run no bridge word reached the
  transcript either.
- **Barge-in worked, but the first word was lost before recognition.** The capture is silent
  for the first 1.4 s and holds the maintainer's voice from 1.4 to 2.6 s (peaks 0.14 to 0.24):
  "Stop", said over the bridge's speech, was suppressed by the voice processing itself, as
  echo, and only "wait, I have a question" reached the transcriber. The bench scores the run
  as failed for the missing word. An interruption word for the product needs to outlast the
  voice processing's adaptation to double talk, or the bridge needs another cue.
- **Scoring after review.** Peer review of this chunk made the bench stricter: a run fails
  without voice processing; an interruption run fails without a barge-in during playback or
  with an interruption that has no words; and every word of the interruption must be heard,
  common words included, which only leak attribution leaves out. Under these rules the
  control still passes, and the interrupt run still fails, on "stop" alone.
- **Barge-in latency.** The transcriber's first non-empty partial result came about 1.5 s
  after the maintainer finished speaking, and playback stopped then, 4.1 s after the start.

## Fixed during the run

The first live runs found two faults in `duplex`, which `bench interrupt` had not exercised
before; both are fixed in this chunk, and a probe (a small Swift program outside the
repository) showed each.

1. **The engine failed to start**, with `-10875` from `PerformCommand(*outputNode,
   kAUInitialize)`. The engine's main mixer and output did not exist yet when voice processing
   was enabled on the input. Creating the mixer first lets the engine start; the maintainer
   heard the probe's tone through the DLZ Creator with voice processing on.
2. **The processed input reached the transcriber as silence.** `AVAudioConverter`'s downmix
   of the 20-channel voice-processing layout to mono drops channel 0, so a 0.5 signal on
   channel 0 came out at 0. The helper now copies channel 0 and converts from mono. It also
   writes one diagnostic line at the end of a run: the input's channels, the seconds and peak
   heard, and how much of the speech played.

With only the first fix, a control run printed "Speaking…" but played nothing, wrote a capture
with no audio (a 4,096-byte header) and had to be stopped; why it played nothing was not
established, and it did not recur after the second fix. Installing that rebuilt helper by
copying it over the old file then made macOS kill it at start (`signal: killed`) with a valid
ad hoc signature; removing the old file first fixed it. Install the helper as a new file.

## For chunk 06

- **Ordinary playback was silent through the DLZ Creator.** In the probe, a tone through
  `AVAudioEngine` without voice processing (the mixer feeding the 4-channel output in 2
  channels) was not heard, while the same tone with voice processing, in 4, 2 or 1 channels,
  was. The maintainer confirmed the mixer does not need waking for a first sound. That path is
  the one the helper's `speak` uses (`DeviceSink`, for `summarize --speak`), so the bridge's
  ordinary speech may be silent on multichannel outputs; `bench tts` plays its renditions with
  `afplay` and does not use it. Not investigated.
- The interruption cue and its latency above, for ADR 002's interruption design.
