# Chunk 04 — Speech bench

Read `../decision.md` and `../definition-of-done.md` first. Depends on chunk 03.

## Outcome

- **Phrase list** `corpus-phrases.md`: about 60 short utterances in Koinon's vocabulary:
  peer names (`codex-koinon-voice-af`), repositories, issue and pull request numbers, "approve",
  "reject", "confirm", "Koinon, what's waiting?", room names and free questions. Committed as
  text; the recordings stay private (definition of done, Private data).
- **Corpus layout** under `~/.local/share/koinon-voice/corpus/{clean,noisy}/NN.wav` with a
  manifest of phrase number and SHA-256; `koinon-voice corpus record` prompts each phrase and
  records it through the Apple helper, or the maintainer records the files another way.
- **`koinon-voice bench stt`** runs each recognition engine over the corpus and reports word
  error rate overall and on Koinon terms, real-time factor and latency per file.
- **`koinon-voice bench tts`** speaks a set of summaries with each synthesis engine and
  reports time to first audio, for the maintainer's blind rating.
- **Engine helpers** speaking the chunk 03 protocol, for the bench only:
  - Apple (chunk 03) for recognition and synthesis.
  - Cactus Whistle: through Cactus's command-line or library interface, with keyword biasing
    seeded from the phrase list's peer and repository names.
  - Parakeet: through a maintained Apple-silicon runtime (an MLX or Core ML port), chosen and
    recorded in the chunk.
  - Kokoro-82M: through its MLX or Core ML port, chosen and recorded in the chunk.
  Python-based helpers live in `helpers/bench/` and run from a virtual environment outside the
  repository; ADR 002 decides whether any becomes a product engine.

## Tests

The definition of done's scoring tests for word error rate, including hyphenated and numeric
Koinon terms, and the edge cases for empty, silent and wrong-rate audio.

## Live checks

Definition of done, live checks 2 and 3. The evidence goes to `scorecard-speech.md`, with
each engine's version, model file hash and what was installed.

## Done

Tests pass, every engine is scored on both corpus sets or recorded as not runnable with the
reason, and the scorecard is committed.
