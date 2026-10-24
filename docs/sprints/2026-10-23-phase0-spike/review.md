# Sprint 2026-10-23 — Phase 0 spike — Review

Reviewer: Codex. Driver: Claude.

## Plan (ad2afc1)

Peer review, changes requested:

1. **F1 (P2). The required local open-weight run had no owner.** Criterion 1 requires one, but
   chunk 02 left it to "any local server the maintainer installs". Disposition: fixed. Chunk
   02 installs Ollama and one open-weight model chosen at the start of the chunk, records the
   version and digest, and its scorecard is not complete without that row; the definition of
   done and `sprint.md` say so.
2. **F2 (P2). Time to first spoken word was not measured.** Chunk 02 timed tokens and chunk 04
   timed synthesis separately. Disposition: fixed. Chunk 03 records, for every engine and
   item, request to first audio sample at the output, with synthesis starting on the first
   complete sentence; first token, first sentence and synthesis first audio stay as
   diagnostic columns. `sprint.md` lists criterion 1's spoken latency under chunk 03.

Refusal review, changes requested:

3. **R1 (P2). Local-speech and engine quotes lacked their question.** Disposition: fixed.
   PDR 001, Decision 2 now states the design session's fourth question and its trade-off, that
   "all VTT TTV transitions happen locally" opened the maintainer's reply without answering a
   numbered question, and the scope. ADR 002, Decision 2 refers to that provenance and states
   that "we can also look at local mac models and modes" was raised by the maintainer, not in
   answer to a question.
4. **Stale Gate A wording in PDR 001.** Disposition: fixed. Decision 3 and the conclusion say
   that Gate A approved Phase 0's decision at c699013 and that the order of Phases 1 to 3 is a
   proposal with no maintainer decision.

Also updated: the definition of done records that Apple Intelligence is now on and the model
answered a prompt.

Status note: the reviewer's sandbox cannot write commit statuses, so neither `peer-review` nor
`refusal-review` is set on ad2afc1; the comments were the verdicts. They were on the first
`rwcii/koinon-voice` repository, which was deleted and recreated on 2026-10-23 because its
Actions created no check suites, and went with it. The approving verdicts on the merged plan,
8f9c859, are on the new repository's pull request #2:
[peer review](https://github.com/rwcii/koinon-voice/pull/2#issuecomment-6375491807) and
[refusal review](https://github.com/rwcii/koinon-voice/pull/2#issuecomment-6375492089).
