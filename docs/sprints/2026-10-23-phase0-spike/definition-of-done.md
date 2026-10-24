# Sprint 2026-10-23 — Phase 0 spike — Definition of done

Read `decision.md` first. The numbers below refer to its acceptance criteria.

## The machine and what it has

Measurements run on the maintainer's Mac: Apple M2 Max, 32 GB, macOS 26.5, Swift 6.3.2 with
the macOS 26.5 SDK (Command Line Tools), Go 1.27.3. Checked on 2026-10-23:

- `SpeechTranscriber` supports en-US, and the en-US assets are installed.
- `SystemLanguageModel.default.availability` was `unavailable(appleIntelligenceNotEnabled)`.
  The maintainer turned on Apple Intelligence the same day; after the model download it
  reports `available` and answered a one-sentence summary prompt. Should it become
  unavailable again, criterion 1 records Apple's reason, and the rest of the sprint does not
  depend on it.
- No local model server (Ollama, LM Studio) is installed, and the system Python is 3.9.
  Chunk 02 installs a local model server and one open-weight model for criterion 1. Engines
  that need a newer Python run in a virtual environment outside the repository, created with
  a Homebrew Python. Each chunk records what it installed, with versions and model digests.

Whether GitHub's `macos-latest` runner has the macOS 26 SDK is not known. Chunk 03 finds out;
until it is shown, the Swift helper is built and tested on the maintainer's Mac and its CI
build step reports the SDK it found.

Chunk 03 found it on pull request #5: `macos-latest` was the `macos-26-arm64` image
20261013.0424.1 on macOS 26.6.2 (25G83), with Xcode 26.6 (17F113) and the macOS 26.5 SDK,
which has Foundation Models and Speech; CI built the helper and its six Swift tests passed
([run 40063033352](https://github.com/rwcii/koinon-voice/actions/runs/40063033352)).

## Private data

- Audio of the maintainer's voice, the corpus recordings and background recordings live under
  `~/.local/share/koinon-voice/` (mode 0700) and never enter Git, CI or a log. Evidence
  commits scores, commands, engine versions and hashes of the audio files.
- Real Koinon items reach an external engine only when the maintainer lists their references
  for the scorecard; all other external runs use the synthetic fixtures.
- API keys are read from the macOS Keychain (ADR 003, Decision 2) and never printed.

## Tests (synthetic, in the Go suite)

- **Summarize (1).** Each adapter against an `httptest` server that records requests: the
  prompt holds the item fields, the response becomes the summary, usage becomes the cost, a
  4xx or 5xx is reported per engine without stopping the others, and a refusal or empty
  answer is a failed row, not a summary. A key missing from the Keychain fails that engine
  with a clear message. No test reaches a provider.
- **Fixtures (1).** At least 20 synthetic decision items covering short and long context,
  several options, commands, links, a deadline and none; each has an answer key of asker,
  ask, options, recommendation and deadline for the correctness check.
- **Sidecar protocol (1 to 5).** A synthetic helper, built from Go in the test, exercises the
  framed stdio protocol: request and response, an engine error, a crash mid-stream, a slow
  helper past its deadline and a helper that writes junk. The bridge reports each and leaves
  no process behind.
- **Scoring (2, 4).** Word error rate on known pairs, including Koinon terms with hyphens and
  digits; false-wake and miss counts from synthetic event logs.
- **Isolation (WDR 006, Decision 2).** Tests use temporary directories for state and corpus,
  no audio device and no Keychain; a test that would need one is skipped with its reason.

## Integration points exercised for real (named live checks)

Run on the maintainer's Mac and recorded in the chunk's evidence file with versions:

1. **Summarization scorecard.** All configured engines on the fixtures and on the items the
   maintainer lists; the maintainer rates listenability blind (engine names hidden).
2. **Corpus.** The maintainer reads the committed phrase list once in a quiet room and once
   with background noise; every recognition engine is scored on those files.
3. **Synthesis.** Apple and Kokoro speak the same summaries; the maintainer rates them blind.
4. **Wake word.** The maintainer says "Koinon" at three distances; false wakes are counted on
   at least one hour of background recordings fed to each detector.
5. **Interruption.** With the bridge speaking through the speakers, the maintainer interrupts;
   the transcript holds the interruption and none of the bridge's own speech.
6. **Koinon map.** Each read, notice and act path is tried against a scratch Koinon daemon
   with temporary state, never the maintainer's running daemon.

## Edge cases that must have tests

- A provider that streams nothing, times out or returns malformed JSON.
- An OpenRouter or OpenAI-compatible response without usage fields: cost recorded as unknown.
- A fixture with no recommendation or no deadline: the answer key says so and the check
  expects the summary not to invent one.
- A Swift helper missing, not executable or of the wrong protocol version.
- An empty or silent audio file, and a file at the wrong sample rate.

## Gate

The `check` skill, the Go workflow on Linux and macOS and the `attribution` check, and the
other family's `peer-review` and `refusal-review` statuses on each chunk's head. The live
checks above are recorded before #1 closes.
