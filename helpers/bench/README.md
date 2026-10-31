# Bench helpers

`bench_helper.py` runs one engine of the speech and wake bench behind the sidecar protocol
(`internal/sidecar`): Cactus Whistle and Parakeet for recognition, Kokoro for synthesis
(chunk 04), and openWakeWord and phrase matching (Silero VAD, then Whistle) for wake
detection (chunk 05). It is for the bench only; ADR 002 decides whether any of them becomes a
product engine.

Kokoro moved to the native helper `koinon-voice-kokoro` (`helpers/kokoro`): `bench tts` runs
that one, and no command starts this helper's `kokoro` engine. It stays as the record of
Phase 0's measurement.

Install the engines into a virtual environment outside the repository, as
`requirements.txt` describes, then run `koinon-voice bench stt`, `bench tts` or `bench wake`,
which start the helper with that environment's Python. The helper turns off the libraries'
telemetry before it loads them.
