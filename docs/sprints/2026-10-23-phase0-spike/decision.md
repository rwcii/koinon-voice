# Sprint 2026-10-23 — Phase 0 spike — Decision

This sprint is Phase 0 of Koinon Voice (PDR 001, Decision 3). It settles, with measurements on
the maintainer's machine, the engine and integration choices that Phase 1 builds on. The
product decisions it serves are PDR 001 to PDR 004, from the maintainer's design session of
2026-10-23; the proposals it tests are ADR 001 to ADR 003. Claude drives the plan; Codex
reviews.

## Problem

Koinon Voice's product shape is decided, but each choice below it rests on vendor claims or
on proposals:

- Published speech recognition figures are vendor-run tests on read English. None covers
  Koinon's vocabulary: peer names such as `codex-koinon-voice-af`, repository names, issue
  numbers, and the words "approve", "reject" and "confirm" (PDR 002, Decision 2).
- The wake word "Koinon" (PDR 003, Decision 1) has no measured false-wake or miss rate with
  any engine.
- Interruption in Phase 1 (PDR 003, Decision 3) needs echo cancellation that has not been
  shown on this machine.
- Whether a local model summarizes items well enough, and where external inference earns its
  place, is open (PDR 004, Decision 2).
- The Koinon interface for acting as the maintainer is requested (rwcii/koinon#464); what
  Koinon v0.4.1 offers today for reading items, change notices and recording verdicts is not
  mapped.

## Acceptance criteria

1. **Summarization.** `koinon-voice summarize` turns a decision item, from a fixture file or
   the local daemon, into a spoken summary with each configured engine: Apple's on-device
   model, at least one local open-weight model, Claude, OpenAI and OpenRouter. A scorecard
   records, per engine and item, correctness (asker, ask, options, recommendation, deadline),
   the maintainer's blind listenability rating, time to first spoken word and cost.
2. **Recognition.** Apple SpeechAnalyzer, Cactus Whistle (with keyword biasing seeded from
   peer and repository names) and Parakeet are scored on a recorded corpus of the
   maintainer's own speech of Koinon phrases, clean and with background noise: word error
   rate overall and on Koinon terms, and latency.
3. **Synthesis.** Apple speech synthesis and Kokoro are scored for time to first audio and the
   maintainer's listenability rating, on the same summaries.
4. **Wake word.** "Koinon" is scored with openWakeWord and with recognition plus phrase
   matching: misses across distances, and false wakes per hour against music, a meeting
   recording and speech that is not addressed to the bridge.
5. **Interruption.** A recorded check shows the maintainer interrupting synthesized speech on
   this machine, with echo cancellation, and the bridge transcribing the interruption without
   its own speech.
6. **Koinon map.** A record of what Koinon v0.4.1 offers a non-browser client for reading
   waiting items, change notices, posting and recording verdicts, and the gaps, linked to
   rwcii/koinon#464.
7. **Results.** The evidence is committed under this sprint, and ADR 001 to ADR 003 and
   PDR 004, Decision 2 are revised from it as proposals for the maintainer's decision.

## Constraints

- **Local speech.** All audio is processed on the maintainer's machine (PDR 001, Decision 2).
- **Private recordings.** Recordings of the maintainer's voice and the corpus audio stay out
  of Git and out of CI; the evidence commits scores, commands and engine versions only.
- **External inference.** External engines receive only synthetic items, and real items the
  maintainer picks for the scorecard (PDR 004). Keys come from the macOS Keychain and never
  enter Git, logs or CI (ADR 003, Decision 2).
- **Tests.** The Go suite uses synthetic helpers and fixtures, temporary state and no
  network or audio device, isolated from the person's environment (WDR 006, Decision 2). CI
  never contacts a model provider.
- **Build.** The Go module stays CGO-free for the four targets (WDR 004); engine helpers are
  separate processes (ADR 002, Decision 1).
- **Koinon.** Koinon Voice proposes Koinon changes through Koinon issues; it does not change
  rwcii/koinon from this repository.
- **Process.** Gates A and B (WDR 002, Decision 5); one open pull request per target branch
  (WDR 004, Decision 2).

## Issues

This sprint delivers #1. It depends on, and does not
deliver, rwcii/koinon#464.
