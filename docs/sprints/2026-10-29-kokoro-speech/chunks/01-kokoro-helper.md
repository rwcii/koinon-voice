# Chunk 01 — Kokoro helper

Read `../decision.md` and `../definition-of-done.md` first.

## Before it starts

The maintainer has installed Xcode (definition of done, "Xcode is needed"), and
`xcodebuild -version` prints a version.

## Feasibility steps

Do these first, outside the repository, in `~/.local/share/koinon-voice/kokoro-spike/`, and
record each result in `scorecard-kokoro-native.md`:

1. Build a minimal command-line tool on kokoro-ios at its current release, with MisakiSwift
   and MLX Swift at the versions its `Package.swift` pins, with `xcodebuild`.
2. Load `kokoro-v1_0.safetensors` (SHA-256 `4e9ecdf0…`) and `voices/af_heart.safetensors`
   from the Phase 0 snapshot of `mlx-community/Kokoro-82M-bf16`
   (`a71e4d38b236d968966a2002c4c895dbd12b1c3c`). If kokoro-ios takes only `voices.npz`, record
   that, and read the voice from `mlalma/KokoroTestApp`'s `voices.npz` at a pinned commit with
   its SHA-256.
3. Render text 01 of `tts-summaries.md` at speed 1.0 and compare it with Phase 0's
   `~/.local/share/koinon-voice/bench/tts/01-kokoro.wav`: length (Phase 0: 15.3 s), clicks
   (Phase 0: 3) and the phonemes of each word.

If a step fails, stop and report it on the board with what failed; the plan is corrected
before the chunk goes on.

## Outcome

1. `helpers/kokoro`: a Swift package with the executable `koinon-voice-kokoro`, depending on
   kokoro-ios and MisakiSwift at exact versions, and on `SidecarProtocol` from
   `helpers/apple` (a local package dependency, so both helpers share one protocol
   implementation). `scripts/build-kokoro-helper.sh` builds it with `xcodebuild` into
   `~/.local/share/koinon-voice/bin/`.
2. The helper speaks the protocol in the definition of done: its hello (helper, version,
   model SHA-256, voices found); `synthesize {text, voice, speed}` with audio frames of
   16-bit mono PCM at 24 kHz, the `first_audio` event and the result
   `{audio_seconds, elapsed_ms, phonemes}`; cancel. It reads its model directory from
   `--models DIR` (default `~/.local/share/koinon-voice/kokoro/`) and opens no audio device.
3. `koinon-voice kokoro install [--dir DIR]` and `koinon-voice kokoro status [--dir DIR]`,
   with the files, revision and SHA-256 values in the definition of done committed in the
   source. `install` writes each file under a temporary name and renames it after its
   SHA-256 matches.
4. `bench tts` runs the native engine as `kokoro` with `--kokoro-helper PATH`, in place of
   the Python bench helper's Kokoro. It keeps Phase 0's report columns and adds clicks, and
   takes `--speed` (default 1.1). Phase 0's Python helper keeps its other engines.
5. README describes the helper, its build, `kokoro install` and `kokoro status`. CHANGELOG
   has an entry. `helpers/bench/README.md` says that Kokoro moved to the native helper.

## Tests

Definition of done, checks 8, 9 and 10, and the existing `bench tts` tests with a synthetic
`koinon-voice-kokoro` that sends known frames.

## Evidence

`scorecard-kokoro-native.md` in this sprint directory, with checks 12, 13, 16, 17 and 18,
the helper's hello, the exact versions and the commands that produced it. The maintainer
listens to text 01 and the names (check 16) during the chunk; the blind rating (check 14)
is in chunk 02.
