---
adr: "002"
title: "Local Speech Engine Sidecars"
status: draft
created: "2026-10-23"
updated: "2026-10-23"
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
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Speech recognition, speech synthesis, voice activity detection and the wake word run in local
sidecar processes that the Go bridge starts and drives over stdio. The Go module stays
CGO-free. On macOS, a Swift helper provides Apple's on-device engines; other engines (Cactus
with Whistle, Parakeet, whisper.cpp, Kokoro, openWakeWord) can be helpers behind the same
interface. Phase 0 measures them and chooses the defaults.

This record is a proposal from the design session of 2026-10-23.

---

## Table of Contents

1. Problem Context
2. Decision 1: Engines are sidecar processes behind one interface
3. Decision 2: Phase 0 chooses the default engines by measurement
4. Conclusion
5. Document History

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

### Scope

Covers speech engines. Text summarization adapters are ADR 003.

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

### Trade-offs Accepted

Audio crosses a process boundary; Phase 0 measures the added latency.

---

## 3. Decision 2: Phase 0 chooses the default engines by measurement

### Decision

Phase 0 measures recognition (Apple SpeechAnalyzer, Cactus Whistle with keyword biasing,
Parakeet), synthesis (Apple, Kokoro), wake word ("Koinon" with openWakeWord and with
recognition plus phrase matching) and echo cancellation on the maintainer's machine, and
records the results. This record is revised with the chosen defaults.

**Provenance:** Maintainer, 2026-10-23, in the design session; the full context of the first
quote, the answer to where speech processing happens, is in PDR 001, Decision 2: "I would like
to consider local running models.  we can look at local mac models, or even emergent micro
models such as https://huggingface.co/Cactus-Compute/whistle or similar
https://github.com/cactus-compute/cactus." Later, raised by the maintainer, not in answer to a
question, after the agent had proposed engine sidecars and a Phase 0 bake-off: "we can also
look at local mac models and modes." Reason: the maintainer asked to consider these engines.
Scope: which engines Phase 0 measures; the sidecar mechanism in Decision 1 is a proposal.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Pick from published benchmarks | Choose without measuring | Published figures are vendor runs on read English, not Koinon's vocabulary |

### Rationale

1. Peer names, repository names and issue jargon are where general recognition fails.

---

## Conclusion

Speech engines are local sidecar processes behind one stdio interface, which keeps the Go
module CGO-free and lets Phase 0 choose engines by measurement.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
