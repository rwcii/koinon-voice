---
pdr: "001"
title: "Two-Way Voice Bridge"
status: draft
created: "2026-10-23"
updated: "2026-10-23"
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related_adrs:
  - "001"
  - "002"
related_pdrs:
  - "002"
  - "003"
  - "004"
tags:
  - voice
  - flow
  - integration
---

# PDR 001: Two-Way Voice Bridge

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice lets the maintainer talk with the agent sessions that Koinon connects, and lets
them talk back. It is a local bridge on the maintainer's machine that connects to the Koinon
daemon there, to remote daemons over SSH, or to both, and presents one dashboard that is the
two-way voice interface. All speech recognition and speech synthesis run on that machine.

This record states the product's shape and its phasing. How spoken words act on Koinon is
PDR 002, when the bridge listens is PDR 003, and how it summarizes is PDR 004.

---

## Table of Contents

1. Problem Context
2. Decision 1: A two-way voice bridge with one dashboard
3. Decision 2: Speech is processed locally
4. Decision 3: Phases, with the phone deferred
5. Conclusion
6. Document History

---

## 1. Problem Context

### The Problem

The maintainer runs agent teams through Koinon on this machine and on remote hosts. Decisions,
messages and stalled sessions wait on the dashboard until the maintainer looks at it, and each
daemon has its own dashboard. Nothing tells the maintainer by sound that something needs
attention, and nothing lets the maintainer answer without reading and typing.

### Product Requirements

1. The maintainer hears that something needs attention, asks about it, discusses it and gives
   a decision, by voice.
2. Agents' replies reach the maintainer as speech.
3. Local and remote Koinon daemons appear in one place.
4. Audio of the maintainer's voice and of agents' messages stays on the maintainer's machine.

### Scope

Covers the product's shape, where speech runs and the phases. The weight of spoken words is
PDR 002; listening states and alerts are PDR 003; summaries and external inference are
PDR 004. The bridge's structure is ADR 001 and the speech engines are ADR 002.

---

## 2. Decision 1: A two-way voice bridge with one dashboard

### Decision

Koinon Voice is a local voice bridge. It connects to the Koinon daemon on the maintainer's
machine, to one or more remote daemons over SSH, or to both, and presents a unified dashboard
on the maintainer's machine. The dashboard is the voice interface: the maintainer talks to
the agents and they talk back. A typical exchange: an item needs the maintainer's attention;
the bridge alerts with a sound or a spoken prompt; the maintainer asks for the item and
discusses it; the maintainer's decision is recorded on the item.

**Provenance:** Maintainer, 2026-10-23, in the design session, answering what the project is,
which direction comes first and what Koinon Voice is to Koinon:
"this is a 2 way voice project.  I can talk to the agents, they can talk back.  koinon-voice
can connect to a local koinon session,  a remote koinon session over SSH, or both. it
presents a unified dashboard on my local machine that becomes the two way voice
interaction."; "both. examples, a new item needs my attention, it can alert me to the new
issue with a sound or a voice prompt.  i can then ask it for the issue, and we can discuss
the issue.  my decision is sent to the decision record."; "it's a local voice bridge to
coordinate local and remote koinon sessions." The options offered for the last question were
a client acting with the maintainer's identity, or a new agent family on the peer list.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| One-way read-out | Speak Koinon events; no voice input | The maintainer asked for both directions |
| A voice agent family | Koinon Voice joins as a peer like claude or codex | The maintainer described a bridge for the maintainer, not another agent |
| A dashboard per daemon | Voice on each daemon's own dashboard | The maintainer asked for one unified dashboard |

### Rationale

1. The maintainer described this product.

### User Impact

The maintainer hears what needs attention across every connected daemon and answers by voice,
with the item on screen.

### Success Criteria

The maintainer handles a waiting decision on a local or remote daemon from alert to recorded
verdict without typing.

---

## 3. Decision 2: Speech is processed locally

### Decision

Speech recognition and speech synthesis run on the maintainer's machine. No audio leaves it.
Summarizing text for speech may use external inference under PDR 004.

**Provenance:** Maintainer, 2026-10-23: "all VTT TTV transitions happen locally." and, on
engines, "I would like to consider local running models."

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Cloud speech services | Send audio to a hosted recognizer or voice | Sends the maintainer's voice and agents' messages off the machine |

### Rationale

1. The maintainer decided it.
2. Koinon keeps runtime data on the machine; local speech keeps that true for audio.

### User Impact

Speaking to the bridge sends no audio anywhere.

---

## 4. Decision 3: Phases, with the phone deferred

### Decision

Koinon Voice is built in phases: 0, a spike that settles engines and Koinon integration with
evidence; 1, two-way voice on the maintainer's machine against the local daemon, in the
browser dashboard; 2, remote daemons over SSH in the unified view; 3, ambient use without the
dashboard open (menu bar, hotkey, wake word in the background). A phone surface is deferred,
not dropped.

**Provenance:** Maintainer, 2026-10-23, asked where Koinon Voice runs (the Mac next to the
daemon, a phone, or the browser dashboard): "I'd love to support all 3. let's discuss
phasing." Later: "phone, is deffered for now." The phase order was proposed in the design
session and is part of the Phase 0 plan's gate A.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Native Mac app first | Menu bar app before the dashboard | The maintainer described the dashboard as the voice interface; a page on localhost can use the microphone |
| Phone in an early phase | Voice from the phone | Deferred by the maintainer; reaching the machine from outside is a separate security question |

### Rationale

1. Phase 0 settles the choices that every later phase depends on, from measurements.
2. Each phase is usable on its own.

### User Impact

The maintainer gets local two-way voice first, then remote daemons, then use without the
dashboard open.

---

## Conclusion

Koinon Voice is a local, two-way voice bridge with one dashboard over local and remote Koinon
daemons, with all speech processed on the maintainer's machine, built in phases with the
phone deferred. The shape, local speech and the phone deferral are the maintainer's words;
the phase order is a proposal awaiting gate A of the Phase 0 plan.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
