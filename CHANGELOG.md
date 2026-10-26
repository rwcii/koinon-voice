# Changelog

All notable changes to Koinon Voice are recorded here. Add visible changes under Unreleased.

## Unreleased

- The summary prompt gives the asker as a person says it, the agent family and then the
  project ("Codex, working in Koinon Voice"), and asks the engine to say other session names
  the same way and to keep only the figures that the decision needs (#13).
- `koinon-voice summarize --blind` shows each rated item, as the engines receive it, before
  its summaries play, so a summary's rating can judge its accuracy against the item as well
  as how it sounds (#13).
- The records, the vendored process guides and the shared skills follow Koinon v0.5.0
  (8abe1b9). `handoff` and `pickup` name the handoff directory of a session that runs outside a
  Git checkout (rwcii/koinon#485), and WDR 001 and WDR 004 cite the maintainer's verdicts that
  keep the required `peer-review` and `refusal-review` statuses and the `pre-commit` block
  (rwcii/koinon#493).
- The records, the vendored process guides and the shared skills follow Koinon at e89d370,
  after its #487 records and docs (rwcii/koinon#492, #494). WDR 002 adopts Koinon's Decisions 1
  to 4, which now say a peer's request is a teammate's request, and drops the differences
  Koinon's record now carries. WDR 004 adopts Decision 6: a release lists every failed CI run
  since the previous release tag, as the release facts describe. The `pair` skill drops its
  once-only limits.
- Agent-facing text says what to do (rwcii/koinon#487). The shared skills are taken from
  Koinon at 0cf3979 (rwcii/koinon#491), without their limits and caveats: `sprint` has no
  gates A and B, and `release` merges and publishes with no second approval. The skill facts,
  AGENTS.md and CONTRIBUTING.md say that a peer's work request is a teammate's request. WDR 002
  records the maintainer's decision, WDR 001 adopts Koinon's records without their limit
  text, and WDR 005 adds the progress note.
- ADR 001 is revised to Koinon PDR 009, approved on rwcii/koinon#470: each connector runs
  `koinon mcp --maintainer` on stdio, locally or over SSH, with the daemon's maintainer secret
  from the Keychain, in place of SSH forwards and dashboard logins. It remains a draft for the
  maintainer's decision.
- `koinon-voice bench wake` records how many speech segments phrase matching's voice
  activity step found per file, and its Misses table counts the takes missed with no speech
  found (#10).
- `koinon-voice bench names` resolves Koinon's names in a recognition scorecard's
  transcripts after recognition, by sound against the keyword list, and reports the error
  rates before and after, with wrong resolutions in held-out text (#11). Its report on
  Phase 0's transcripts is `scorecard-names.md`.
- `summarize`'s check counts a denial after the noun it denies ("Deadline is not specified")
  and no longer reads "due to" as a deadline (#9). Scorecards gain a `Not for speech` column
  for commit hashes, paths, URLs and Markdown in a summary, summary cells keep line breaks
  as `<br>`, and `summarize --rescore FILE` checks a scorecard's recorded summaries again
  without running an engine; Phase 0's scorecard is rescored with it.
- Phase 0 results: `results.md` summarizes the scorecards and the Koinon map with a proposal
  for each choice, and ADR 001 to ADR 003 and PDR 004, Decision 2 are revised from them as
  drafts for the maintainer's decision. Follow-up work is filed as #8 to #15.
- The wake and interruption bench: `koinon-voice corpus wake --distance near|mid|far` records
  takes of "Koinon" and `corpus background --source music|meeting|podcast|coffee` long
  background recordings; `koinon-voice bench wake` scores openWakeWord with a custom "Koinon"
  model and phrase matching (Silero VAD, then Whistle biased to Koinon names) for misses per
  distance and false wakes per hour. The Apple helper gains `duplex`, which speaks through the
  speakers while it listens with Apple's voice processing (echo cancellation) and stops on
  barge-in; `koinon-voice bench interrupt` runs the interruption check and its echo control.
- The speech bench: `koinon-voice corpus record` prompts each phrase of a committed list of
  60 Koinon phrases and records it, clean or with background noise, from one channel of the
  input device into a private corpus with a SHA-256 manifest; `koinon-voice bench stt`
  scores Apple SpeechAnalyzer, Parakeet and Cactus Whistle (with and without keyword
  biasing) on it, with word error rate overall and on Koinon terms, latency and real-time
  factor; `koinon-voice bench tts` times Apple speech synthesis and Kokoro to first audio and
  takes a blind listenability rating. Parakeet, Whistle and Kokoro run in a Python bench
  helper (`helpers/bench`) from a virtual environment outside the repository.
- The Apple helper (`helpers/apple`, a Swift package) behind a framed stdio sidecar protocol:
  Foundation Models summaries, SpeechAnalyzer transcription of 16 kHz mono files, and
  AVSpeechSynthesizer speech to the output device or a file. `summarize` gains the `apple`
  engine, and `--speak` now streams each summary into the helper's speech sentence by sentence
  and records every engine's time to its first spoken word; `--blind` rates each summary
  right after it is heard. `koinon-voice helper` prints the helper's hello and can
  transcribe a file or speak a text. An external engine's spoken summary ends with "Summary
  by PROVIDER." The engines file's `voice` sets the speaking voice, and `koinon-voice-apple
  record SECONDS FILE` records a 16 kHz mono file, refusing without microphone access or when
  the recording is silent.
- `koinon-voice summarize` turns decision items into spoken summaries with each engine in
  `~/.config/koinon-voice/engines.toml` (Claude through the Anthropic SDK, and OpenAI,
  OpenRouter or a local oMLX server through one OpenAI-compatible adapter), with API keys
  from the macOS Keychain, 20 synthetic fixture items with answer keys, a deterministic
  correctness check, streamed timings, cost, `--speak` and a `--blind` listenability rating,
  and a Markdown scorecard. An external engine's summary is shown and spoken with its
  provider's name.
- Phase 0 Koinon map: what Koinon v0.4.1 offers a maintainer client today, and the gaps
  reported on rwcii/koinon#464.
- Draft product and architecture records from the 2026-10-23 design session (PDR 001 to
  PDR 004, ADR 001 to ADR 003) and the Phase 0 spike decision.
- Repository set up as a peer of rwcii/koinon: Go module, `koinon-voice version`, Koinon's
  shared agent skills and their facts, the ADR, PDR and WDR series with WDR 001 to WDR 006,
  CI on Linux and macOS, the branch policy script, and a commit-msg hook and required
  check that refuse tool attribution and non-maintainer commit identities.
