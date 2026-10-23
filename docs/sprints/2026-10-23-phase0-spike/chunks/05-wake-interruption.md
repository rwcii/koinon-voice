# Chunk 05 — Wake and interruption

Read `../decision.md` and `../definition-of-done.md` first. Depends on chunk 04.

## Outcome

- **`koinon-voice bench wake`** feeds audio files to each wake detector and reports misses on
  the maintainer's "Koinon" recordings at three distances and false wakes per hour on the
  background recordings (definition of done, live check 4). Detectors:
  - openWakeWord with a custom "Koinon" model ("coin-on"), trained with openWakeWord's
    synthetic-sample procedure; the model file stays outside the repository and its hash and
    training settings are recorded.
  - Phrase matching: speech detection, then the best recognition engine from chunk 04 with
    "Koinon" as a biased keyword where the engine supports it.
- **Interruption**: the Apple helper gains a `duplex` capability that plays synthesized speech
  through the default output while capturing the microphone with Apple's voice-processing
  input (echo cancellation), and streams the captured transcript.

## Tests

The definition of done's scoring tests for wake counts from synthetic event logs, and the
sidecar protocol tests extended to the `duplex` frames with the synthetic helper.

## Live checks

Definition of done, live checks 4 and 5. The evidence goes to `scorecard-wake.md` and
`interruption.md`.

## Done

Tests pass, both detectors are scored, and the interruption check is recorded with its
transcript, or with what failed.
