# Changelog

All notable changes to Koinon Voice are recorded here. Add visible changes under Unreleased.

## Unreleased

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
