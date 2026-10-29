# Sprint 2026-10-29 — Speak with Kokoro — Chunks

Each chunk is one pull request from `develop`, built with the `ship` skill after the chunks it
depends on have merged, with one pull request in flight at a time. Each chunk passes its part
of `definition-of-done.md` and updates the documents for the behavior it changes in the same
pull request.

| Chunk | Outcome | Criteria | Checks | Depends on |
| --- | --- | --- | --- | --- |
| [01 Kokoro helper](chunks/01-kokoro-helper.md) | `helpers/kokoro` (`koinon-voice-kokoro` on kokoro-ios), `koinon-voice kokoro install` and `status`, and `bench tts` with the native engine; checks 12, 13, 16 and 17 on the maintainer's Mac. | 1, 2, 6 (measurement), 7 | 8–10, 12, 13, 16–18 | Xcode installed by the maintainer |
| [02 Speak with Kokoro](chunks/02-speak.md) | The Apple helper's `play`, the Go speaker that joins the two helpers, `[speech]` in the engines file, and Kokoro as the default voice of `summarize --speak` and `helper --say`; checks 14 and 15. | 3, 4, 5, 6 (rating) | 1–7, 11, 14, 15 | 01 |

Chunk 01 begins with its feasibility steps (build, voice file, output against Phase 0). If one
fails, the chunk stops and the plan is corrected before it continues (`sprint` skill).

Gate A approved `decision.md` at 533b3dc (DECISIONS.md#kv-12-gate-a). The maintainer kept one
voice for this sprint and moved the comparison of other voices to after GA, #32
(DECISIONS.md#kv-12-voices).
