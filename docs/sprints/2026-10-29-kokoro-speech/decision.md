# Sprint 2026-10-29 — Speak with Kokoro — Decision

This sprint delivers #12: Koinon Voice speaks with Kokoro, in a native runtime without Python,
through the Swift helper's voice-processing engine. Claude drives the plan; Codex reviews.

**Source:** #12, and three decisions of the maintainer:

- ADR 002, Decision 2 (board item kv-adr002-d2, 2026-10-26): Kokoro, at speed 1.1, is the
  voice. Apple's voice stays until Kokoro is wired in.
- ADR 002, Decision 3 (board item kv-adr002-d3, 2026-10-26): all speech plays through the
  Swift helper's `AVAudioEngine` with voice processing on, whichever engine synthesized it.
- ADR 002, Decision 4 (board item kv-adr002-d4, 2026-10-26): "B - NO Python.  This needs to
  run natively on a mac, and eventually on a iphone/ipad, etc.  let's not start with tech debt
  that will need to be ripped out."

## Problem

Kokoro ran in Phase 0 only in the Python bench helper (`helpers/bench`, through `mlx-audio`
and `misaki`). The maintainer rated it 4.7 against Apple's Zoe at 3.2 and asked to stop using
Zoe (`docs/sprints/2026-10-23-phase0-spike/scorecard-speech.md`, Synthesis). Today:

- `summarize --speak` and `helper --say` speak only Apple voices, with Zoe in the engines
  file.
- No native code runs Kokoro: neither its model nor the step that turns text into the
  phonemes it reads.
- Phase 0's Kokoro measurements came from the Python helper and do not carry over to a native
  engine (ADR 002, Decision 4, Trade-offs).

## Acceptance criteria

1. **Native engine.** Kokoro-82M synthesizes speech on the maintainer's Mac in a helper
   process (ADR 002, Decision 1) with no Python at run time or at installation, text to
   phonemes included (ADR 002, Decision 4). The model's source, revision and SHA-256 are
   recorded, and the model is stored outside the repository.
2. **Voice and speed.** The engine speaks with a Kokoro voice chosen in the engines file, by
   default `af_heart`, the voice the maintainer rated in Phase 0, at speed 1.1 by default
   (ADR 002, Decision 2).
3. **Audio path.** Kokoro's audio plays through the Swift helper's voice-processing engine,
   the same path as Apple's voice now (ADR 002, Decision 3). The engine itself opens no audio
   device.
4. **Streaming and stopping.** A text plays sentence by sentence: the first sentence plays
   while the next ones are synthesized, and a stop request ends playback and synthesis.
5. **The default voice.** `summarize --speak` and `helper --say` speak with Kokoro by default.
   Apple's voice stays available as a choice in the engines file. When the Kokoro model or
   its helper is missing, the command says what is missing and how to install it.
6. **Measured again.** `bench tts` runs the native engine on Phase 0's six texts
   (`tts-summaries.md`) and reports time to first audio, audio length and clicks against
   Phase 0's Kokoro (787 ms median to first audio, 15.3 s for text 01 at speed 1.0, 3 clicks).
   The maintainer rates it blind, beside Phase 0's recordings, as in Phase 0.
7. **Later platforms.** The runtime and the text-to-phoneme step that the sprint chooses are
   available on iOS and iPadOS, as their publishers document. This sprint builds for macOS
   only.

## Constraints

- **Build.** The Go module stays CGO-free for the four targets (WDR 004). Native code lives
  in a helper (ADR 002, Decision 1).
- **No Python** at run time or installation (ADR 002, Decision 4). A model converted by its
  publisher is used where one exists.
- **Tests.** The Go suite uses a synthetic helper and no model, audio device or network,
  isolated from the person's environment (WDR 006, Decision 2).
- **Privacy.** No model, audio or recording goes into Git, CI or logs.
- **Process.** Gates A and B of the `sprint` skill (WDR 002); one open pull request per target branch
  (WDR 004, Decision 2).

## Issues

This sprint delivers #12. It does not run openWakeWord natively: that follows the retrained
model of #10. It does not build the voice loop or the iOS app.
