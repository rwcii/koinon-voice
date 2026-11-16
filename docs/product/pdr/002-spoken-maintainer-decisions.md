---
pdr: "002"
title: "Spoken Maintainer Decisions"
status: draft
created: "2026-10-23"
updated: "2026-11-16"
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related_adrs:
  - "001"
related_pdrs:
  - "001"
  - "003"
tags:
  - voice
  - identity
  - flow
---

# PDR 002: Spoken Maintainer Decisions

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

In Koinon, a post or decision the maintainer makes on the dashboard is the maintainer's
instruction and approval (Koinon PDR 005, Decision 3). In Koinon Voice, words the maintainer
speaks carry the same weight. A spoken decision on a decision item is read back and confirmed
before it is recorded; the screen shows the full item and the exact verdict while that
happens.

---

## Table of Contents

1. Problem Context
2. Decision 1: Spoken words carry the maintainer's dashboard weight
3. Decision 2: Read-back for decisions only
4. Decision 3: The item is on screen with the spoken summary
5. Decision 4: A verdict names its user and computer, and each outcome is said
6. Conclusion
7. Document History

---

## 1. Problem Context

### The Problem

A voice bridge that relays the maintainer's words as ordinary peer messages would make every
spoken answer advisory, and the maintainer would still have to approve on the dashboard. A
bridge whose words count as the maintainer's must not record a misheard decision.

### Product Requirements

1. A spoken answer acts as the maintainer's own dashboard action.
2. A recorded decision is the one the maintainer meant.
3. The maintainer can see what is being decided, not only hear a summary.

### Scope

Covers the weight of spoken words and how a decision is confirmed. When the bridge listens is
PDR 003. The Koinon interface that carries the maintainer's authority is ADR 001 and
rwcii/koinon#464.

---

## 2. Decision 1: Spoken words carry the maintainer's dashboard weight

### Decision

Words the maintainer speaks to Koinon Voice are the maintainer's dashboard posts and
decisions, with the same weight: an instruction or approval, as Koinon PDR 005, Decision 3
gives a dashboard post. Koinon Voice acts through the maintainer's identity in Koinon, not as
an agent session.

**Provenance:** Maintainer, 2026-10-23, asked whether spoken words carry the weight of the
maintainer's dashboard decisions, with the option offered that voice could draft and route
while approvals needed an explicit confirmation or the dashboard: "sppoken words are maintainer
dashboard decisions. same weight."

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Spoken words as drafts | Voice drafts; the maintainer approves on the dashboard | The maintainer decided spoken words carry full weight |
| Spoken words as peer messages | The bridge posts as an agent session | Koinon records an agent session's posts as that agent's, and the maintainer's spoken words carry the maintainer's weight |

### Rationale

1. The maintainer decided it.

### User Impact

The maintainer decides by voice; nothing else is needed for the decision to take effect.

---

## 3. Decision 2: Read-back for decisions only

### Decision

Before Koinon Voice records a verdict on a decision item, it speaks the exact verdict it will
record and records it when the maintainer confirms. Other spoken input (questions, messages,
room posts) is not read back.

**Provenance:** Maintainer, 2026-10-23, asked whether the bridge reads back before recording
(decisions only, everything sent, or never), with the reason that the risk is mishearing, not
authority: "readback for decisions only."

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Read back everything | Confirm every message and post | Slows conversation; the maintainer chose decisions only |
| No read-back | Record what was heard | A misheard "approve" is recorded as the maintainer's approval |

### Rationale

1. A read-back checks the transcription without changing the weight of the words.

### User Impact

A decision takes one extra "confirm"; a question or message does not.

---

## 4. Decision 3: The item is on screen with the spoken summary

### Decision

While Koinon Voice speaks about a decision item, the dashboard shows the full item and any
record it links, and shows the exact verdict during the read-back. The maintainer can decide
by voice or on the screen; both use the same maintainer action.

**Provenance:** Maintainer, 2026-10-23: "koinon-voice could also pull up the actual decision
record needing input on screen for maintainer review, to go along with the voice summary."

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Voice only | The summary is all the maintainer gets | A summary can omit what matters; the maintainer asked for the record on screen |

### Rationale

1. The maintainer asked for it.
2. A misheard verdict is visible as well as audible.

### User Impact

The maintainer hears the gist and can read the whole item before deciding.

---

## 5. Decision 4: A verdict names its user and computer, and each outcome is said

### Decision

- **Decided by:** Robert Capps. **Date:** 2026-11-16, board kv-36-gate-a, revision 1, approved.
- **Context:** Koinon Voice became a signed client of Koinon nodes (ADR 004): each verdict is
  signed with the key of one user and install and reaches the computer that holds the item
  through the links. The sprint record `docs/sprints/2026-11-14-signed-client/decision.md`
  proposed the changes below to this record ("Changes on the Koinon Voice side", R9 and R13).
  Gate A asked the maintainer to approve that record; the other option was to give feedback on
  it before the merge.
- **Decision:**
  1. A verdict records the user name of the key that signed it, on the computer that holds the
     item.
  2. The read-back of Decision 2 names that computer.
  3. Koinon Voice says each verdict's outcome: recorded; refused, with its reason; not run, for
     example when the computer is offline; or unknown, when the verdict was sent and its answer
     was lost.
- **Reason and scope:** the maintainer approved the sprint record. The read-back and the spoken
  outcome apply to verdicts given by voice; the dashboard shows the same outcomes.

### User Impact

The maintainer hears which computer a verdict goes to before confirming it, and hears whether
it was recorded.

---

## Conclusion

The maintainer's spoken words are the maintainer's dashboard actions. Decisions alone are read
back before they are recorded, with the item and the verdict on screen. Decisions 1 to 3 are
the maintainer's words of 2026-10-23. Decision 4, the maintainer's approval of 2026-11-16,
names the user and the computer of each verdict and says its outcome.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-25 | The agent-session alternative's reason stated as what Koinon records, not as a limit on peer messages (rwcii/koinon#487) |
| 2026-11-16 | Decision 4: a verdict names its user and computer, and each outcome is said (kv-36-gate-a, #38) |
