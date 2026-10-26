---
adr: "002"
title: "Local Speech Engine Sidecars"
status: accepted
created: "2026-10-23"
updated: "2026-10-26"
parent_adr: ""
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related:
  - "001"
  - "003"
tags:
  - architecture
  - build
  - integration
---

# ADR 002: Local Speech Engine Sidecars

**Date:** 2026-10-23
**Status:** Accepted
**Authors:** Robert Capps

---

## Executive Summary

Speech recognition, speech synthesis, voice activity detection and the wake word run in local
sidecar processes that the Go bridge starts and drives over stdio. The Go module stays
CGO-free. On macOS, a Swift helper provides Apple's on-device engines and owns the audio
devices, with Apple's voice processing for echo cancellation; other engines run as helpers
behind the same interface.

The record was proposed in the design session of 2026-10-23 and revised on 2026-10-24 from
Phase 0's measurements (`docs/sprints/2026-10-23-phase0-spike/results.md`). The maintainer
decided the default engines, the audio path and native runtimes for the engines on
2026-10-26, and the record is accepted.

---

## Table of Contents

1. Problem Context
2. Decision 1: Engines are sidecar processes behind one interface
3. Decision 2: Default engines, from Phase 0
4. Decision 3: The Swift helper owns the audio, with voice processing
5. Decision 4: Native runtimes, no Python
6. Conclusion
7. Document History

---

## 1. Problem Context

### The Problem

The candidate local engines need C bindings or Swift: Apple's SpeechAnalyzer, Foundation
Models, speech synthesis and voice-processing audio are Swift-only; Cactus, whisper.cpp and
Kokoro runtimes are C, C++ or Python. Koinon's build is one CGO-free Go binary for four
targets (WDR 004), and the maintainer asked to consider several local models (PDR 001,
Decision 2).

### Requirements

1. All speech processing runs locally (PDR 001, Decision 2).
2. The Go module builds CGO-free for linux and darwin, amd64 and arm64.
3. Engines can be swapped and compared without changing the bridge.
4. The bridge does not hear its own speech, so the maintainer can interrupt it (PDR 003,
   Decision 3).

### Scope

Covers speech engines and the audio path. Text summarization adapters are ADR 003.

---

## 2. Decision 1: Engines are sidecar processes behind one interface

### Decision

Each engine runs as a separate local process that the bridge starts and speaks to over stdio
with a small framed protocol: audio in and transcripts out for recognition, text in and audio
out for synthesis, events for speech detection and wake. The macOS helper is written in Swift
and built on the maintainer's machine. Ordinary tests use a synthetic helper (WDR 006).

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| cgo bindings | Link engines into the Go binary | Breaks the CGO-free four-target build; one engine crash takes the bridge down |
| Python services | Run engines as local Python servers | Adds an interpreter runtime that Koinon retired |
| Swift bridge | Write the whole bridge in Swift | macOS only; the maintainer asked for Koinon's language |

### Rationale

1. Koinon already drives native CLIs as processes; the same pattern keeps the module CGO-free.
2. A process boundary isolates engine crashes and lets Phase 0 compare engines directly.
3. Phase 0 ran the Swift helper and five engines in a Python helper behind the protocol, and
   compared them without changing the bridge.

### Trade-offs Accepted

Audio crosses a process boundary. Phase 0 did not measure that cost separately; a whole
recognition request for a phrase of a few seconds took 16 ms with the quickest engine, which
bounds it.

---

## 3. Decision 2: Default engines, from Phase 0

### Decision

From the measurements in Phase 0's results:

- **Recognition:** Apple `SpeechTranscriber`, in the Swift helper. Koinon's names (the word
  "Koinon", peers and repositories) are resolved after recognition by matching against the
  names the connected daemons report.
- **Synthesis:** Kokoro, at speed 1.1. Apple's voice stays until Kokoro is wired in, as the
  maintainer decided.
- **Wake word:** openWakeWord with a "Koinon" model, retrained until its miss rate is one the
  maintainer accepts. Until then push-to-talk is the reliable way to speak to the bridge, and
  the wake word is offered as experimental.

**Provenance:** Maintainer, 2026-10-23, in the design session; the full context of the first
quote, the answer to where speech processing happens, is in PDR 001, Decision 2: "I would like
to consider local running models.  we can look at local mac models, or even emergent micro
models such as https://huggingface.co/Cactus-Compute/whistle or similar
https://github.com/cactus-compute/cactus." Later, raised by the maintainer, not in answer to a
question, after the agent had proposed engine sidecars and a Phase 0 bake-off: "we can also
look at local mac models and modes." Reason: the maintainer asked to consider these engines.
Scope: which engines Phase 0 measures. The defaults were proposed from those measurements;
maintainer, 2026-10-26, approved board item kv-adr002-d2, "Koinon Voice: accept the default
speech engines from Phase 0 (ADR 002, Decision 2)?", which recommended accepting the three
defaults as proposed, with no note.
On the voice, maintainer, 2026-10-23, after the blind synthesis rating, asked which voice to
use until a Kokoro engine exists: "try kokoro at 1.1 and 1.2, stay with zoe until kokaro is
wired in" (scorecard-speech.md).

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ---------------- |
| Pick from published benchmarks | Choose without measuring | Published figures are vendor runs on read English, not Koinon's vocabulary |
| Parakeet for recognition | The most accurate overall, especially in noise | Within 1.6 points of Apple, and needs a Python runtime; the alternative if noise matters more |
| Whistle with keyword biasing | The lowest error rate on Koinon's names | Heard the decision verbs worst, and a misheard "confirm" defeats the read-back (PDR 002, Decision 2) |
| Apple `DictationTranscriber` with the keyword list | Heard "Koinon" in most places, and some full peer names | Heard fewer decision verbs than `SpeechTranscriber`, with a higher overall error rate, about the same rate on Koinon's terms, and more than twice the latency |
| Apple speech synthesis | Quicker to first audio | The maintainer rated it lower on every text and asked to stop using it |
| Phrase matching for the wake word | Recognition plus the word "Koinon" | Missed most takes beyond half a metre, and keeps a recognizer running on every sound |

### Rationale

1. A misheard verb is the costly recognition error; Apple and Parakeet heard the verbs best.
2. No engine heard "Koinon" without a list, so names need their own step with any engine.
3. The maintainer's rating decides the voice.
4. Neither wake detector is good enough yet. openWakeWord's results point to its model, which
   can be trained again, though Phase 0 did not isolate the cause; push-to-talk does not
   depend on it (PDR 003, Decision 1).

### Trade-offs Accepted

Name matching after recognition was measured on Phase 0's transcripts only (#11). It resolves
peer and repository names, but the bare word "Koinon" also matches ordinary words such as
"canon" and "can in", so where the bridge resolves it is left to Phase 1's design. Kokoro
starts about 0.3 s later than Apple's voice.

---

## 4. Decision 3: The Swift helper owns the audio, with voice processing

### Decision

On macOS the Swift helper captures the microphone and plays all of the bridge's speech
through one `AVAudioEngine` with Apple's voice processing on, whichever engine synthesized the
speech. An interruption is any speech heard during playback: the bridge stops speaking and
acts on what follows, not on a cue word.

**Provenance:** Maintainer, 2026-10-26, approved board item kv-adr002-d3, "Koinon Voice: play
all speech through the Swift helper with voice processing (ADR 002, Decision 3)?", which
proposed this decision from Phase 0's evidence and named its cost, with no note.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ---------------- |
| Each engine plays its own audio | Kokoro's helper plays what it renders | Echo cancellation needs the played audio as its reference |
| A cue word such as "stop" | Interrupt only on the word | Phase 0's "Stop", said over the bridge, was suppressed as echo |

### Rationale

1. In Phase 0, no word of the bridge's speech reached the transcript with voice processing.
2. Ordinary playback without voice processing was silent through the maintainer's
   multichannel USB mixer, while the same audio with voice processing was heard.

### Trade-offs Accepted

The first word said over the bridge's speech can be lost. A Kokoro summary crosses two
processes before it plays.

---

## 5. Decision 4: Native runtimes, no Python

### Decision

Koinon Voice's engines run natively, without Python. Kokoro and openWakeWord, which ran in a
Python helper in Phase 0, run in native runtimes, for example ONNX Runtime or Core ML, in a
helper process (Decision 1). Their results are measured again against Phase 0's.

**Provenance:** Maintainer, 2026-10-26, on board item kv-adr002-d4, "Koinon Voice: how do
Kokoro and openWakeWord ship: managed Python or native runtimes (ADR 002, Decision 4)?". The
item offered A, a managed Python environment, and B, native runtimes, and recommended A for
Phase 1. The maintainer chose B in the verdict note: "B - NO Python.  This needs to run
natively on a mac, and eventually on a iphone/ipad, etc.  let's not start with tech debt that
will need to be ripped out."

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Managed Python environment | Koinon Voice keeps a private Python environment for the two helpers, from pinned packages | The maintainer: Koinon Voice runs natively on a Mac and later on iPhone and iPad, and a Python environment is debt to remove later |

### Rationale

1. The maintainer decided it, for the platforms named above.

### Trade-offs Accepted

Kokoro and openWakeWord need native code that Koinon Voice does not have yet, and Phase 0's
measurements of them do not carry over until they are repeated.

---

## Conclusion

Speech engines are local sidecar processes behind one stdio interface, which keeps the Go
module CGO-free and let Phase 0 compare them. From its measurements, the maintainer chose
Apple recognition with Koinon names resolved after it, Kokoro's voice and openWakeWord
retrained for the wake word. The Swift helper owns the audio with voice processing, and the
engines run in native runtimes without Python.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-24 | Revised from Phase 0: proposed default engines, the audio path with voice processing, and the open runtime question |
| 2026-10-24 | Decision 2's trade-off: name matching measured on Phase 0's transcripts (#11) |
| 2026-10-26 | Decisions 3 and 4 decided by the maintainer: the audio path with voice processing, and native runtimes without Python |
| 2026-10-26 | Decision 2 decided by the maintainer (board item kv-adr002-d2); the record is accepted |
