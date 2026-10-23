---
pdr: "003"
title: "Listening States and Alerts"
status: draft
created: "2026-10-23"
updated: "2026-10-23"
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related_adrs:
  - "002"
related_pdrs:
  - "001"
  - "002"
tags:
  - voice
  - notifications
  - ux
---

# PDR 003: Listening States and Alerts

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice listens only when the maintainer arms it. While armed with the microphone open,
it acts only after the wake word "Koinon" or while push-to-talk is held. Alerts play whether
or not it is armed, follow the macOS Focus mode, and use one voice. The maintainer can
interrupt it while it speaks.

---

## Table of Contents

1. Problem Context
2. Decision 1: Armed listening, wake word and push-to-talk
3. Decision 2: Alerts play while disarmed and follow Focus
4. Decision 3: One voice, and interruption from Phase 1
5. Conclusion
6. Document History

---

## 1. Problem Context

### The Problem

Spoken words carry the maintainer's authority (PDR 002), so what the bridge treats as the
maintainer speaking matters. An always-open microphone would act on a television, a meeting
or another person. An alert the maintainer cannot hear, or one that speaks during a meeting,
fails in the other direction.

### Product Requirements

1. Nothing is captured while the maintainer has not armed listening.
2. While the microphone is open, the bridge acts only on speech meant for it.
3. Alerts reach the maintainer when they should and stay quiet when they should.

### Scope

Covers listening states, the wake word, push-to-talk, alerts, the voice and interruption.
The weight of spoken words is PDR 002. Wake word and speech engines are measured in Phase 0
(ADR 002).

---

## 2. Decision 1: Armed listening, wake word and push-to-talk

### Decision

Koinon Voice has three listening states. **Disarmed** (the default): the microphone is closed
and nothing is captured. **Armed**: the microphone is open, and audio is checked only for the
wake word and otherwise discarded. **Engaged**: after the wake word, speech is transcribed and
acted on, with a short follow-up window, then the bridge returns to Armed. Holding the
push-to-talk key goes straight to Engaged without the wake word. The wake word is "Koinon",
pronounced "coin-on"; a custom word or name comes later.

**Provenance:** Maintainer, 2026-10-23, asked what counts as the maintainer speaking
(push-to-talk, a wake word, or always listening): "listening should be armed, or it doesnt
listen.  A wake word would be a good thing to have of the mic is open." Asked whether to add
push-to-talk alongside the wake word: "add push-to-talk". Asked which wake word: "koinon
(coin-on).  eventually custom word/name." The three-state model was proposed in the design
session from these answers.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Always listening | Transcribe everything while running | The maintainer said it listens only when armed |
| Push-to-talk only | No wake word | The maintainer wanted a wake word while the microphone is open |
| Wake word only | No push-to-talk | The maintainer asked for push-to-talk as well |

### Rationale

1. The maintainer decided arming, the wake word and push-to-talk.
2. Discarding non-wake audio keeps what is captured to what is meant for the bridge.

### User Impact

The maintainer arms the bridge, then says "Koinon" or holds a key to speak to it.

### Success Criteria

Phase 0 measures false wakes and missed wakes for "Koinon" (Phase 0 decision, criterion 4).

---

## 3. Decision 2: Alerts play while disarmed and follow Focus

### Decision

Alerts (a sound or a short spoken line when something needs the maintainer) play whether
listening is armed or not; arming governs the microphone only. The macOS Focus mode sets alert
behavior from Phase 1: for example, silent in a meeting and spoken while working.

**Provenance:** Maintainer, 2026-10-23, asked whether alerts play while disarmed: "yes alerts
play while disarmed". Asked whether macOS Focus modes control alert behavior from Phase 1:
"yes."

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Alerts only while armed | Disarming silences everything | The maintainer said alerts play while disarmed |
| Bridge-only quiet settings | Its own quiet hours, ignoring Focus | The maintainer chose Focus |

### Rationale

1. The maintainer decided both.

### User Impact

The maintainer hears what needs attention without arming the microphone, and Focus keeps it
quiet when the maintainer is busy.

---

## 4. Decision 3: One voice, and interruption from Phase 1

### Decision

Koinon Voice uses one voice for all speech for now. The maintainer can interrupt it while it
speaks from Phase 1, which requires echo cancellation so the bridge does not hear itself.

**Provenance:** Maintainer, 2026-10-23, asked whether to use one voice or a voice per agent or
host: "one voice is fine for now." Asked whether interrupting is in Phase 1, given that it
makes echo cancellation a Phase 1 requirement: "phase 1."

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| A voice per agent or host | Distinct voices identify the speaker | Not needed now, by the maintainer's answer |
| No interruption until later | Speech plays to the end | The maintainer put interruption in Phase 1 |

### Rationale

1. The maintainer decided both.

### User Impact

The maintainer can cut the bridge off mid-sentence and speak.

---

## Conclusion

Koinon Voice listens only when armed, acts on the wake word "Koinon" or push-to-talk, alerts
while disarmed under the Focus mode, speaks with one voice and can be interrupted from
Phase 1. Each decision is the maintainer's answer of 2026-10-23.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
