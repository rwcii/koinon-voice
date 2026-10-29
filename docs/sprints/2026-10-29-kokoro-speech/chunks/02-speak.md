# Chunk 02 — Speak with Kokoro

Read `../decision.md` and `../definition-of-done.md` first. Chunk 01 has merged.

## Outcome

1. The Apple helper's `play` method, as the definition of done describes: params
   `{sample_rate}`, audio frames as input, played through `DeviceSink` with voice processing
   on, or through `FileSink` with `output`; the `first_audio` event and the result of
   `speak`; cancel stops playback at once. Its hello lists the capability `play`.
2. A Go speaker in `internal/speech` with one interface for both engines:
   - Apple: `speak` as now.
   - Kokoro: start `koinon-voice-kokoro` and open one Apple helper `play` request per text;
     for each sentence, call `synthesize` and forward its frames to `play` as they arrive,
     and call `synthesize` for the next sentence while the frames before it play. A stop
     cancels both requests. An exit of either helper ends the text with an error that names
     it and cancels the other request.
3. The engines file's `[speech]` table (`engine`, `voice`, `speed`) with the defaults in the
   definition of done. `summarize --speak` and `helper --say` use the speaker. `helper --say`
   gains `--engine apple|kokoro` and `--speed`; `--voice` selects the engine's voice. A
   missing model or helper gives the error of check 7.
4. The time to first spoken word that `summarize --speak` records is measured the same way
   for both engines: from the first sentence's request to the Apple helper's `first_audio`.
5. README, CHANGELOG and the engines file example describe Kokoro as the default voice and
   `engine = "apple"` as the choice. ADR 002, Decision 2's sentence "Apple's voice stays until
   Kokoro is wired in" gets a history line that Kokoro is wired in, with this chunk's PR.

## Tests

Definition of done, checks 1 to 7 and 11.

## Evidence

Checks 14 and 15, added to `scorecard-kokoro-native.md`:

- `helper --say` and `summarize --speak` on the maintainer's Mac through the output device,
  and a stop during the first sentence.
- The blind rating of the native recordings at 1.1 beside Phase 0's, with the scores and the
  maintainer's words.

#12 closes after this chunk merges, with the scorecard as its evidence.
