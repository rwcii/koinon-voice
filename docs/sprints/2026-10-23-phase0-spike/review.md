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

## Chunk 02, summarize (e774d39)

[Peer review](https://github.com/rwcii/koinon-voice/pull/4#issuecomment-6378366124), changes
requested:

1. **F1 (P1). A compatible server could echo any key into the scorecard.** Redaction matched
   only `sk-`, `pk-` and `rk-` shapes. Disposition: fixed. Each adapter scrubs its own
   configured key from its errors before they leave it, and key-shaped text is still
   redacted; a test runs an echoing server through `Run` and `WriteScorecard`.
2. **F2 (P2). A stream that ended early passed as a summary.** Disposition: fixed. The
   OpenAI-compatible adapter needs a finish reason or `[DONE]`, the Anthropic adapter needs
   `end_turn` or `stop_sequence`, and a token-limit stop is incomplete too; all are the new
   `ErrIncomplete`, with tests for partial text at EOF and for each stop.
3. **F3 (P2). A negation hid a later claim in its sentence.** Disposition: fixed. A denial
   must end on the claim it denies, with only filler words between the negation and it, so
   unrelated negations and mixed claims ("does not recommend Kokoro but recommends Ava")
   count; tests cover both fields. The recorded summaries were rescored: no verdict changed
   from this, and Haiku's SYN-05 changed from the earlier key fix.
4. **F4 (P2). Ordinary playback did not name an external provider.** Disposition: fixed.
   Outside the blind rating, an external summary is spoken as "From PROVIDER." before its
   text and its screen line says `external: PROVIDER`; the blind rating still hides both
   until the ratings are in. A test covers external and local playback.

[Refusal review](https://github.com/rwcii/koinon-voice/pull/4#issuecomment-6378366349):
approved.

With F1 to F4 fixed, both reviews approved e6e2dbf, which merged as 6d1b90d:
[peer review](https://github.com/rwcii/koinon-voice/pull/4#issuecomment-6378422323) and
[refusal review](https://github.com/rwcii/koinon-voice/pull/4#issuecomment-6378422499).
Chunk 03 moves F4's provider label after the summary ("Summary by PROVIDER."), so that the
label is not the first spoken word that criterion 1 times; the screen line is unchanged.

## Chunk 03, Apple helper (0a22fcb)

[Peer review](https://github.com/rwcii/koinon-voice/pull/5#issuecomment-6379992503), changes
requested:

1. **F1 (P1). A deadline could not end a request blocked on a helper that stopped reading.**
   Open wrote the request before its deadline watcher started, and a deadline failed the
   requests before killing the helper, so it waited on an event callback blocked writing to
   it. Disposition: fixed. The watcher starts before the first write, and on a deadline the
   bridge kills the process group first, which releases any blocked write or callback, and
   then fails the requests; Close does the same. A request whose write the kill cut short
   reports the deadline. Tests cover a 256 KiB request to a helper that never reads and an
   event callback blocked writing input.
2. **F2 (P2). Canceling a finished request could kill the helper.** The watcher's select
   could pick the canceled context over the closed result. Disposition: fixed. Whoever
   removes a request from those in flight decides its outcome, so a result and a
   cancellation exclude each other and a late cancel does nothing. A test cancels 200
   finished calls and then makes another. This was the Linux CI failure in
   `TestHelperCommand`.
3. **F3 (P2). A summary whose playback failed was still rated.** Disposition: fixed. In
   `--blind`, a failed playback is reported as "playback failed (REASON); not rated." with
   neither the summary text nor the engine, and no rating is asked for or counted. No
   playback failed in the recorded run, so its ratings stand.

The three new sidecar tests fail on 0a22fcb and pass after the fix. Also corrected: the
definition of done gives the runner image version, 20261013.0424.1, and the runner's macOS;
20261012.601 was the provisioner's version.

[Refusal review](https://github.com/rwcii/koinon-voice/pull/5#issuecomment-6379992662):
approved. The provider label's move after the summary was accepted, and the check's
limitation recorded in the scorecard was carried forward.

With F1 to F3 fixed, both reviews approved 5a2b516, which merged as 747c6c3:
[peer review](https://github.com/rwcii/koinon-voice/pull/5#issuecomment-6380052405) and
[refusal review](https://github.com/rwcii/koinon-voice/pull/5#issuecomment-6380052585).

## Chunk 04, speech bench (ff53c26)

[Peer review](https://github.com/rwcii/koinon-voice/pull/6#issuecomment-6381064645), changes
requested:

1. **F1 (P2). A failed helper restart panicked the recognition bench.** After a request killed
   its helper, a failed start left no helper and the next file dereferenced it. Disposition:
   fixed. A file with no helper tries a new start; if that fails, the file is a failed row
   scored as nothing heard, and the bench goes on to the next engine. A test with a timed-out
   request and a failing restart covers it.
2. **F2 (P2). The WAV reader divided by zero on a 1-bit header and accepted truncated
   data.** Disposition: fixed. Bit depths other than 8, 16, 24 and 32, no channels, a chunk
   that runs past the end of the file, and data that ends inside a frame are `ErrBadAudio`;
   every chunk is checked against the file's size before it is read, which bounds memory.
   Tests cover each with synthetic files.
3. **F3 (P2). Medians of even counts took the upper middle value.** Disposition: fixed. Both
   medians are now sample medians, with odd and even tests. Every set has 60 files and the
   synthesis bench six texts, so the scorecard's medians were recomputed from its recorded
   per-file latencies, the manifests' lengths and the renditions, without a new run: Apple's
   first audio is 489 ms and Kokoro's 787 ms, and the recognition latencies and real-time
   factors moved by at most 1 ms and 0.001.

The three tests fail on ff53c26: the restart and WAV tests panic, and the median test gets
536 ms.

[Refusal review](https://github.com/rwcii/koinon-voice/pull/6#issuecomment-6381064802),
changes requested:

4. **R1 (P2). Keeping Zoe until Kokoro is wired in had no quoted decision.** Disposition:
   fixed. The scorecard now gives the question put to the maintainer on 2026-10-23, its
   options (Evan or Ava, from the same Apple synthesizer, or leaving Zoe) and the reason for
   asking (`summarize --speak` speaks only Apple voices until a Kokoro engine exists, and
   ADR 002 decides product engines), with the maintainer's answer quoted.

With F1 to F3 and R1 fixed, both reviews approved b9f11e2, which merged as 93e1008:
[peer review](https://github.com/rwcii/koinon-voice/pull/6#issuecomment-6381108404) and
[refusal review](https://github.com/rwcii/koinon-voice/pull/6#issuecomment-6381108577).

## Chunk 05, wake and interruption (dc65532)

[Peer review](https://github.com/rwcii/koinon-voice/pull/7#issuecomment-6388579784), changes
requested:

1. **F1 (P2). Failed background recordings counted toward the hours.** A recording's hours
   were added before its error was checked, so a failed one diluted the false-wake rate and a
   wholly failed set read as no false wakes. Disposition: fixed. Failed hours are a separate
   count and column, the rate is over the hours scored, and a set with none scored shows its
   rate as unavailable. Tests cover a mixed set and a wholly failed one.
2. **F2 (P2). An interruption run passed without a barge-in.** The verdict looked only at
   the transcript's words. Disposition: fixed. A run fails without voice processing, and an
   interruption run fails without a barge-in during playback; the result gives each reason.
   A regression test in which the whole phrase is heard after playback ends, with no
   barge-in, fails with "no barge-in during playback", apart from the silent echo control.
3. **F3 (P2). Common words vanished from the interruption.** "No." scored as passed with an
   empty transcript, because "no" was filtered out as a common word. Disposition: fixed.
   Every word of the interruption must be heard; common words are left out only when
   attributing transcript words to the bridge as leaks. An interruption with no words to
   check fails. Tests cover "No." missed in an empty transcript, "No." heard as "No!", and an
   interruption with no words.
4. **F4 (P2). The meeting recording of criterion 4 was missing.** Disposition: fixed, after
   the maintainer chose to record one (scorecard-wake.md, Decisions 4). The recorder gains a
   `meeting` source, the maintainer recorded 20 minutes of a meeting playing in the room, and
   the bench, rerun on every set, woke on none of it (openWakeWord's highest score 0.047).
   The takes' counts are unchanged, and openWakeWord's rate is now 0.57 false wakes per hour
   over 1.75 h.
5. **Nonblocking. The scorecard said the first, model-loading request was unscored.**
   Disposition: fixed in the scorecard, which now says the first request is scored and its
   Latency includes the model load.

[Refusal review](https://github.com/rwcii/koinon-voice/pull/7#issuecomment-6388580013),
changes requested:

6. **R1 (P2). v3's and v4's settings had no quoted decisions.** Disposition: fixed. The
   scorecard's Decisions section gives each question as put to the maintainer on 2026-10-24,
   its options and the quoted answer, with the scope of each, and does the same for the music
   and meeting recordings. v2's larger training set is identified as the experimenter's
   choice, and the earlier choice of the background sources as recorded in a handoff without
   its question or answer.

On dc65532 the new `internal/speech` tests do not build (no `FailedHours`, `Said` or
`Verdict`, and `PerHour` has one result), and in `cmd/koinon-voice` the wake table lacks the
failed-hours column and the run with no barge-in prints "Result: passed".
