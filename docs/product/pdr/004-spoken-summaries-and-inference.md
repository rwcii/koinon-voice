---
pdr: "004"
title: "Spoken Summaries and Inference"
status: draft
created: "2026-10-23"
updated: "2026-10-26"
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related_adrs:
  - "003"
related_pdrs:
  - "001"
  - "002"
tags:
  - voice
  - content
  - integration
---

# PDR 004: Spoken Summaries and Inference

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon items are written to be read, not heard. Koinon Voice speaks a short summary of each
item, which needs summarization. The maintainer accepts external inference for it if local
models are not enough. This record states that, and the scope of external use that the
maintainer decided after the Phase 0 proof of concept, which measured local and external
engines on synthetic items.

---

## Table of Contents

1. Problem Context
2. Decision 1: Items are summarized for speech, locally first
3. Decision 2: External inference per host
4. Conclusion
5. Document History

---

## 1. Problem Context

### The Problem

A decision item carries a title, context, a recommendation, commands and links. Read aloud
verbatim it is long and full of hashes and paths. A useful spoken version names who asks, what
is being decided, the options, the recommendation and any deadline.

### Product Requirements

1. Each item has a short spoken summary that is accurate about the ask and the recommendation.
2. The maintainer knows when text left the machine to produce it.

### Scope

Covers summarization for speech and the use of external inference. Speech recognition and
synthesis stay local (PDR 001, Decision 2). The engine adapters are ADR 003. A spoken-summary
field written by the agent that creates an item would be a Koinon change, raised separately if
adopted.

---

## 2. Decision 1: Items are summarized for speech, locally first

### Decision

Koinon Voice speaks a summary of an item, not its text, with the full item on screen
(PDR 002, Decision 3). It summarizes with a local model where that is good enough. External
inference may be used where it is needed; when it is, the screen shows and the speech says
which provider produced the summary.

**Provenance:** Maintainer, 2026-10-23, asked who the maintainer discusses an item with (the
agents through Koinon, or a local model in the bridge): "we need to do some sumarization for
voice to work correctly.  this may need external inferrance.  I'm open to this if needed."
On engines: "I would like to consider local running models." and "for engines, we could also
look at openrouter or openai for models too." The provider label was proposed in the design
session.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Read items verbatim | Speak the item's text | Too long and not speakable |
| External only | Always summarize with a hosted model | The maintainer prefers local models where they suffice |
| Local only | Never use external inference | The maintainer is open to it if needed |

### Rationale

1. The maintainer decided summarization, the local preference and openness to external use.

### User Impact

The maintainer hears a few sentences per item, and hears when a provider produced them.

---

## 3. Decision 2: External inference per host

### Decision

The maintainer enables external inference per host, and it is off until then. Where it is on,
Claude Haiku summarizes; where it is off, the local Qwen3-4B does. A spoken request to
summarize one item externally stays open.

The measurements behind it (`docs/sprints/2026-10-23-phase0-spike/results.md`): on 20
synthetic items, Haiku was correct on all 20, and among the quickest to speak at
about a hundredth of a cent per item; the local models missed a fact in 6 and 8 of 20, most
often a dropped option or an invented recommendation, which misleads the maintainer about a
decision. Haiku's listenability rating was the lowest, from spelling agent names out, which
the prompt can change; the ratings, three per engine, separate engines only coarsely.

**Provenance:** Maintainer, 2026-10-23, asked about scope and provider: "let's discuss what a
POC would look like here." and "for engines, we could also look at openrouter or openai for
models too." Phase 0 measured accuracy, listenability, latency and cost for local and external
engines, including Apple's on-device model, local open-weight models, Claude, OpenAI and
OpenRouter, and proposed this decision. Maintainer, 2026-10-26, approved board item
kv-pdr004-d2, "Koinon Voice: external summarization scope: per host, Claude Haiku where on,
local Qwen3-4B where off (PDR 004, Decision 2)?", with the note: "You can use external
inference for this".

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Decide now | Pick a scope and provider before measuring | The maintainer asked for a proof of concept first |
| Local only, everywhere | Never send an item out | The local models missed a fact in 6 and 8 of 20 items |
| External on for all hosts | One switch | The maintainer approved the per-host proposal |

### Rationale

1. The measurements showed that the local models miss facts that Claude Haiku keeps, and the
   maintainer chose where external inference is used.

### User Impact

A host the maintainer has not enabled sends nothing out, and its summaries are less often
correct. Where it is enabled, Claude Haiku summarizes, labeled as Decision 1 says.

---

## Conclusion

Items are summarized for speech, with local models preferred and external inference
acceptable when needed and labeled. The maintainer enables external inference per host, with
Claude Haiku where it is on and the local Qwen3-4B where it is off.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-24 | Decision 2: Phase 0's proposal for the maintainer's decision |
| 2026-10-26 | Decision 2 decided by the maintainer: external inference per host |
