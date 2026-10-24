"""Bench helper for chunk 04: one recognition or synthesis engine behind the sidecar protocol.

Usage: python bench_helper.py ENGINE, where ENGINE is whistle, parakeet or kokoro. It speaks
protocol v1 (internal/sidecar) on stdin and stdout: 4-byte big-endian frame length, kind
byte 'J', then one JSON message. It serves

- transcribe {path, keywords?} -> {text, elapsed_ms}, for whistle and parakeet;
- synthesize {text, voice, output} -> event first_audio {since_request_ms}, then
  {audio_seconds, elapsed_ms}, for kokoro, writing a 16-bit mono WAV file.

The model loads on the first request, so the hello is immediate; the bench sends a warm-up
request first and does not score it. The helper runs from a virtual environment outside the
repository (ADR 002); it is for the bench only and sends no telemetry.
"""

import json
import os
import struct
import sys
import time
import wave

# No usage reports from the libraries: the bench's audio is the maintainer's voice.
os.environ.setdefault("NEEDLE_TELEMETRY", "0")
os.environ.setdefault("DO_NOT_TRACK", "1")
os.environ.setdefault("HF_HUB_DISABLE_TELEMETRY", "1")

VERSION = 1
HELPER_VERSION = "0.1.0"

# Engine name -> (capabilities, model id).
ENGINES = {
    "whistle": (["transcribe"], "Cactus-Compute/whistle"),
    "parakeet": (["transcribe"], "mlx-community/parakeet-tdt-0.6b-v2"),
    "kokoro": (["synthesize"], "mlx-community/Kokoro-82M-bf16"),
}

# Frames go to the real stdout; anything a library prints (spaCy's model install, progress
# bars) goes to stderr, so it cannot break a frame.
out = os.fdopen(os.dup(1), "wb")
os.dup2(2, 1)
sys.stdout = sys.stderr


def send(message):
    body = b"J" + json.dumps(message).encode()
    out.write(struct.pack(">I", len(body)) + body)
    out.flush()


def read_frame(stream):
    head = stream.read(4)
    if len(head) < 4:
        return None
    (n,) = struct.unpack(">I", head)
    body = stream.read(n)
    if len(body) < n:
        return None
    if body[:1] != b"J":
        return {}  # audio frames are not used by the bench
    return json.loads(body[1:])


def package_version(name):
    try:
        from importlib.metadata import version

        return version(name)
    except Exception:
        return "unknown"


class Engine:
    def __init__(self, name):
        self.name = name
        self.model = None

    def load(self):
        if self.model is not None:
            return
        if self.name == "whistle":
            import needle

            self.model = needle
        elif self.name == "parakeet":
            from mlx_audio.stt.utils import load

            self.model = load(ENGINES["parakeet"][1])
        elif self.name == "kokoro":
            from mlx_audio.tts.utils import load_model

            self.model = load_model(ENGINES["kokoro"][1])

    def transcribe(self, params):
        self.load()
        path = params["path"]
        start = time.perf_counter()
        if self.name == "whistle":
            keywords = params.get("keywords") or None
            text = self.model.transcribe(path, language="en", keywords=keywords)["text"]
        else:
            text = self.model.generate(path).text
        return {"text": text.strip(), "elapsed_ms": (time.perf_counter() - start) * 1000}

    def synthesize(self, request_id, params, started):
        import numpy as np

        self.load()
        rate = getattr(self.model, "sample_rate", 24000)
        chunks = []
        first = None
        for result in self.model.generate(text=params["text"], voice=params.get("voice") or "af_heart", speed=1.0, lang_code="a"):
            audio = np.nan_to_num(np.array(result.audio, dtype=np.float32))
            if first is None and audio.size:
                first = (time.perf_counter() - started) * 1000
                send({"type": "event", "id": request_id, "event": "first_audio", "data": {"since_request_ms": first}})
            chunks.append(audio)
        audio = np.concatenate(chunks) if chunks else np.zeros(0, dtype=np.float32)
        if not audio.size or not np.abs(audio).max() > 0:
            raise HelperError("bad_audio", "the engine produced no sound")
        old = os.umask(0o077)
        try:
            with wave.open(params["output"], "wb") as w:
                w.setnchannels(1)
                w.setsampwidth(2)
                w.setframerate(rate)
                w.writeframes((np.clip(audio, -1, 1) * 32767).astype("<i2").tobytes())
        finally:
            os.umask(old)
        return {"audio_seconds": audio.size / rate, "elapsed_ms": (time.perf_counter() - started) * 1000}


class HelperError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ENGINES:
        print("usage: bench_helper.py whistle|parakeet|kokoro", file=sys.stderr)
        return 2
    name = sys.argv[1]
    capabilities, model = ENGINES[name]
    package = "cactus-needle" if name == "whistle" else "mlx-audio"
    engine = Engine(name)
    send({
        "type": "hello",
        "protocol": VERSION,
        "helper": "koinon-voice-bench-" + name,
        "version": HELPER_VERSION,
        "capabilities": capabilities,
        "info": {"engine": name, "model": model, "package": package, "package_version": package_version(package),
                 "python": sys.version.split()[0]},
    })
    stdin = sys.stdin.buffer
    while True:
        m = read_frame(stdin)
        if m is None:
            return 0
        if m.get("type") != "request":
            continue
        started = time.perf_counter()
        request_id = m.get("id")
        try:
            method = m.get("method")
            params = m.get("params") or {}
            if method not in capabilities:
                raise HelperError("unknown_method", str(method))
            if method == "transcribe":
                data = engine.transcribe(params)
            else:
                data = engine.synthesize(request_id, params, started)
            send({"type": "result", "id": request_id, "data": data})
        except HelperError as e:
            send({"type": "error", "id": request_id, "code": e.code, "message": e.message})
        except Exception as e:  # an engine failure is this request's error, not the helper's end
            send({"type": "error", "id": request_id, "code": "engine_error", "message": f"{type(e).__name__}: {e}"})


if __name__ == "__main__":
    sys.exit(main())
