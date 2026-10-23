# Sprint 2026-10-23 — Phase 0 spike — Chunks

Each chunk is one pull request from `develop`, built with the `ship` skill after the chunks it
depends on have merged, with one pull request in flight at a time. Each chunk passes its part
of `definition-of-done.md` and updates the documents for the behavior it changes in the same
pull request. All chunks deliver #1.

| Chunk | Outcome | Criteria | Depends on |
| --- | --- | --- | --- |
| [01 Koinon map](chunks/01-koinon-map.md) | `koinon-map.md` records what Koinon v0.4.1 offers a maintainer client and the gaps, tried against a scratch daemon, with a summary on rwcii/koinon#464. | 6 | — |
| [02 Summarize](chunks/02-summarize.md) | `koinon-voice summarize` with the Anthropic and OpenAI-compatible adapters, Keychain keys, synthetic fixtures with answer keys, and the scorecard. | 1 | — |
| [03 Apple helper](chunks/03-apple-helper.md) | The sidecar protocol, a Swift helper for Apple's model, recognition and synthesis, and the Apple engine in `summarize`. | 1, 2, 3 (Apple) | 02 |
| [04 Speech bench](chunks/04-speech-bench.md) | `koinon-voice bench stt` and `bench tts`, the phrase list and private corpus layout, Whistle, Parakeet and Kokoro helpers, and the scores. | 2, 3 | 03 |
| [05 Wake and interruption](chunks/05-wake-interruption.md) | `koinon-voice bench wake` for "Koinon" with openWakeWord and phrase matching, and the interruption check with echo cancellation. | 4, 5 | 04 |
| [06 Results](chunks/06-results.md) | The scorecards summarized, ADR 001 to ADR 003 and PDR 004, Decision 2 revised as proposals for the maintainer's decision. | 7 | 01 to 05 |

Gate A approved `decision.md` at c699013. Maintainer, 2026-10-23, asked "Do you approve
`decision.md` at `c699013` (Gate A)?": "gate A approved, proceed with the plan".
