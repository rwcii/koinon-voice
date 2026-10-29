# Sprint 2026-10-29 — Speak with Kokoro — Definition of done

Read `decision.md` first. The numbers below refer to its acceptance criteria. The maintainer
approved it at Gate A (DECISIONS.md#kv-12-gate-a) and kept one voice, `af_heart`, for this
sprint (DECISIONS.md#kv-12-voices; the other voices are #32, after GA).

## What the native candidates are

Checked on 2026-10-29 from each project's repository (README, `Package.swift`, documentation
and license through the GitHub API). Nothing below was built or run yet; chunk 01 starts by
doing that.

| Candidate | Runtime | Text to phonemes | License | Platforms | Status |
| --- | --- | --- | --- | --- | --- |
| `mlalma/kokoro-ios` (KokoroSwift) | MLX Swift 0.30.2 (exact pin in its `Package.swift`) | MisakiSwift 1.0.6: Misaki's dictionaries, then a BART model on MLX for words not in them | MIT; MisakiSwift Apache-2.0; MLX Swift MIT | macOS 15, iOS 18; not the iOS simulator | Chosen |
| `FluidInference/FluidAudio` (`KokoroAneManager`) | Core ML, 7 stages | Misaki lexicon, then a BART G2P model on Core ML | Apache-2.0 | macOS 15, iOS 18 | Fallback only |
| ONNX Runtime with an ONNX export | ONNX Runtime | None included; needs MisakiSwift or another | MIT; model Apache-2.0 | macOS, iOS | Not chosen: two pieces to join |
| sherpa-onnx, kokoro-onnx and others that use espeak-ng | various | espeak-ng | espeak-ng is GPL-3.0 | | Excluded: GPL |

Why kokoro-ios:

- **Same model as Phase 0.** Its test app (`mlalma/KokoroTestApp`) ships
  `kokoro-v1_0.safetensors` with SHA-256 `4e9ecdf03b8b6cf906070390237feda473dc13327cb8d56a43deaa374c02acd8`,
  327,115,152 bytes. That is the file Phase 0 rated, from `mlx-community/Kokoro-82M-bf16` at
  revision `a71e4d38b236d968966a2002c4c895dbd12b1c3c` (`scorecard-speech.md`, Engines), and the
  same MLX runtime family as Phase 0's `mlx-audio`.
- **Same text-to-phoneme step as Phase 0.** Phase 0 used Misaki (`misaki[en]==0.9.4`);
  MisakiSwift is its Swift port.
- **FluidAudio's documented crash.** Its `KokoroAne.md`, "Known OS issues", lists a BNNS
  segfault in Core ML on iOS and macOS 26.4 to 26.5.x, fixed in macOS 26.6, and says that no
  iOS version or routing is yet shown safe for long sessions (issue #889 there). The
  maintainer's Mac runs macOS 26.5.

**Xcode is needed.** MLX Swift's README says that SwiftPM on the command line cannot build its
Metal shaders and that `xcodebuild` can. The maintainer's Mac has only the Command Line Tools
(`xcode-select -p` is `/Library/Developer/CommandLineTools`; `xcodebuild` and `metal` are not
found). Before chunk 01 the maintainer installs Xcode, which iOS and iPadOS builds (criterion 7)
also need. FluidAudio builds with SwiftPM alone, but it is the route with the crash above.

What is not known yet, and chunk 01 checks first:

- That kokoro-ios builds with `xcodebuild` and runs here.
- That it reads Phase 0's voice files (`voices/af_heart.safetensors` in the same snapshot);
  the test app loads `voices.npz` instead.
- That its output matches Phase 0's in length and sound for the same text, voice and speed.
- How it pronounces "Koinon" and the other names in the six texts.

If kokoro-ios fails one of these, chunk 01 stops and the plan is corrected (`sprint` skill)
before any later chunk. FluidAudio is the fallback only on macOS 26.6 or later.

## Design that the chunks build

- **A Kokoro helper.** `helpers/kokoro`, a Swift package with the executable
  `koinon-voice-kokoro`, on kokoro-ios and MisakiSwift at exact versions, built by
  `scripts/build-kokoro-helper.sh` with `xcodebuild`. It is a separate process from the Apple
  helper so a crash in the model does not stop recognition or playback (ADR 002, Decision 1,
  Rationale 2). It opens no audio device (criterion 3).
- **Its protocol.** The sidecar protocol of `internal/sidecar` (`SidecarProtocol` in Swift).
  The hello names the helper, its version, the model file's SHA-256 and the voices found.
  Method `synthesize`, params `{text, voice, speed}`: audio frames of 16-bit mono PCM at
  24 kHz, an event `first_audio {since_request_ms}`, and the result
  `{audio_seconds, elapsed_ms, phonemes}`. A cancel ends it with no more frames.
- **Model files.** `koinon-voice kokoro install` downloads `kokoro-v1_0.safetensors`,
  `config.json` and the `af_heart` voice file from `mlx-community/Kokoro-82M-bf16` at
  revision `a71e4d38b236d968966a2002c4c895dbd12b1c3c` over HTTPS into
  `~/.local/share/koinon-voice/kokoro/`, mode 0700, and checks each file's SHA-256 against
  values committed in the source. `koinon-voice kokoro status` checks them again. No Python
  is used (criterion 1).
- **Playback.** The Apple helper gains method `play`: params `{sample_rate}`, then audio
  frames as input, played through the shared `DeviceSink` with voice processing on (ADR 002,
  Decision 3), with the same `first_audio` event and result as `speak`, and a cancel that
  stops playback at once.
- **Speaking.** A Go speaker in `internal/speech` sends each sentence to
  `koinon-voice-kokoro`, and forwards its audio frames to the Apple helper's `play` as they
  arrive. The next sentence is synthesized while the one before it plays (criterion 4).
- **Configuration.** The engines file gains `[speech]` with `engine = "kokoro"` (default) or
  `"apple"`, `voice` (default `af_heart` for Kokoro; the Apple voice identifier for Apple)
  and `speed` (default 1.1, Kokoro only). The present top-level `voice` keeps its meaning
  for `engine = "apple"`.

## Checks

Go, isolated from the person's environment (WDR 006, Decision 2). A synthetic Kokoro helper
and a synthetic Apple helper, built from the test binary as the existing helper tests do,
record what they receive:

1. `synthesize` sends `{text, voice, speed}` with the defaults `af_heart` and 1.1, and
   the values the engines file gives.
2. Audio frames from the Kokoro helper reach the Apple helper's `play` in order and unchanged,
   with `sample_rate` 24000.
3. Sentence 2 is requested before sentence 1's playback ends.
4. Cancel during sentence 1 stops synthesis and playback; no frame is sent after it.
5. A Kokoro helper that exits mid-sentence gives an error that names it; the Apple helper's
   request is cancelled.
6. With `engine = "apple"`, `speak` is used as now, with the configured Apple voice.
7. A missing model or helper gives the error that names the missing file or binary and the
   install command, and exit status 1, before any audio (criterion 5).
8. `kokoro install` into a temporary directory from a loopback HTTPS test server: a correct
   file is kept with mode 0600 in a 0700 directory; a file with a wrong SHA-256 is refused
   and removed; an interrupted download leaves no file under the final name.
9. `kokoro status` reports each file present, missing or changed.

Swift (`helpers/kokoro`, `helpers/apple`), in their test scripts with no model:

10. The Kokoro helper's hello and its error for a missing model file.
11. `play` writes received frames to a `FileSink` at the given rate; cancel stops it.

On the maintainer's Mac, recorded in `scorecard-kokoro-native.md` (WDR 006):

12. **Native engine (1, 2).** `koinon-voice-kokoro` runs with no Python process (checked with
    `ps` during synthesis) and reports the model's SHA-256 `4e9ecdf0…`.
13. **Measured again (6).** `bench tts` with the native engine on the six texts of
    `tts-summaries.md`: time to first audio (median and each text), audio length, and clicks
    (sample-to-sample jumps above 0.3 of full scale, as Phase 0 counted them), at speed 1.0
    against Phase 0's Kokoro (787 ms median, 15.3 s and 3 clicks for text 01), and at 1.1.
14. **Blind rating (6).** The maintainer rates the native recordings at 1.1 blind, shuffled
    with Phase 0's recordings in `~/.local/share/koinon-voice/bench/tts/`, as Phase 0 did.
15. **Audio path (3, 4).** `helper --say` and `summarize --speak` play Kokoro through the
    maintainer's output device; a stop during the first sentence ends it; the maintainer
    confirms that it plays.
16. **Names.** The phonemes and the sound of "Koinon", "Codex" and "Claude" in the six texts
    are recorded, and the maintainer says whether each is acceptable.
17. **Stability.** 100 syntheses of text 01 in one helper process with no crash.
18. **Later platforms (7).** The scorecard cites where kokoro-ios, MisakiSwift and MLX Swift
    document iOS and iPadOS support. Nothing is built for iOS.

## Publication gates

- Each chunk: the `check` skill (go vet, gofmt, the race suite, `git diff --check`, the four
  `CGO_ENABLED=0` builds, the changelog rule and the private-content scan), Codex
  `peer-review` and `refusal-review` on the frozen head, CI on Linux and macOS, squash
  auto-merge into `develop` (WDR 003, WDR 004).
- No model file, voice file, audio or recording in Git, CI or logs.
- #12 closes when criteria 1 to 7 have their evidence, including the maintainer's rating.
