# Chunk 03 — Apple helper

Read `../decision.md` and `../definition-of-done.md` first. Depends on chunk 02.

## Outcome

- **Sidecar protocol** (ADR 002, Decision 1) in `internal/sidecar`: the bridge starts a helper
  process and exchanges length-prefixed JSON frames on stdin and stdout, with binary audio
  frames for PCM. The first frame is a hello with the protocol version and the helper's
  capabilities (`summarize`, `transcribe`, `speak`). Each request has an ID and a deadline.
  The bridge kills the helper's process group on a deadline, a crash or exit.
- **Swift helper** in `helpers/apple/` (a Swift package, built with `swift build -c
  release`), with three capabilities:
  - `summarize`: Foundation Models with the chunk 02 prompt; reports
    `unavailable` with Apple's reason when the model is not available.
  - `transcribe`: SpeechAnalyzer with SpeechTranscriber on a 16 kHz mono file, returning the
    text, timing and the locale's asset version.
  - `speak`: AVSpeechSynthesizer to a file or the default output, returning time to first
    audio.
- **Apple engine** in `summarize`, using the helper.
- **Time to first spoken word** (criterion 1): `summarize --speak` streams each summary into
  the helper's `speak` and records, for every engine and item, the time from sending the
  summarization request to the first audio sample the helper writes to the output device.
  Synthesis starts on the first complete sentence and does not wait for the full summary;
  the scorecard keeps first token, first sentence and synthesis first audio as separate
  columns beside it. The chunk re-runs the scorecard with this column.
- **CI**: the macOS job builds the helper when the runner's SDK has FoundationModels and
  Speech, and reports the SDK version either way (definition of done, The machine).

## Tests

The definition of done's sidecar protocol tests, with a synthetic helper built from Go in the
test. The Swift helper has its own `swift test` for frame encoding, run on the maintainer's Mac
and in CI where the SDK allows.

## Live check

With Apple Intelligence on, the Apple engine runs in the chunk 02 scorecard, and every
engine's row gets its time to first spoken word. Transcription and synthesis are exercised on
one short recording to prove the path; scoring them is chunk 04.

## Done

Tests pass, the helper builds on the maintainer's Mac, the Apple row is in the scorecard or
recorded as unavailable with Apple's reason, every row has its time to first spoken word,
and the CI SDK result is recorded.
