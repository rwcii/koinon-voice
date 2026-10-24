# Bench helpers

`bench_helper.py` runs one engine of chunk 04's speech bench behind the sidecar protocol
(`internal/sidecar`): Cactus Whistle and Parakeet for recognition, Kokoro for synthesis. It
is for the bench only; ADR 002 decides whether any of them becomes a product engine.

Install the engines into a virtual environment outside the repository, as
`requirements.txt` describes, then run `koinon-voice bench stt` or `koinon-voice bench tts`,
which start the helper with that environment's Python. The helper turns off the libraries'
telemetry before it loads them.
