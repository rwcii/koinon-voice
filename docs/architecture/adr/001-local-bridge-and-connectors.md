---
adr: "001"
title: "Local Bridge and Koinon Connectors"
status: draft
created: "2026-10-23"
updated: "2026-10-24"
parent_adr: ""
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related:
  - "002"
  - "003"
tags:
  - architecture
  - integration
  - security
  - auth
---

# ADR 001: Local Bridge and Koinon Connectors

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice is one Go process on the maintainer's machine. It serves the unified dashboard on
loopback and holds one connector per Koinon daemon: the local daemon over its loopback
address, and each remote daemon through an SSH local forward to that host's loopback address.
It acts through Koinon's maintainer interface, which does not yet exist in a form a non-browser
client can use (rwcii/koinon#464).

This record is a proposal from the design session of 2026-10-23, revised on 2026-10-24 from
Phase 0's map of Koinon v0.4.1 (`docs/sprints/2026-10-23-phase0-spike/koinon-map.md`) and its
audio results (`results.md` beside it). It remains a proposal for the maintainer's decision.
The map found no maintainer interface a non-browser client can use without scraping the
dashboard, so the bridge's Koinon features wait for rwcii/koinon#464.

---

## Table of Contents

1. Problem Context
2. Decision 1: One local Go process serving a loopback dashboard
3. Decision 2: A connector per daemon; SSH forwards for remote hosts
4. Decision 3: Act through Koinon's maintainer interface
5. Conclusion
6. Document History

---

## 1. Problem Context

### The Problem

The bridge must reach daemons that listen only on loopback, some of them on other hosts, and
act with the maintainer's authority (PDR 002, Decision 1) without widening Koinon's same-user,
loopback-only boundary.

### Requirements

1. Reach the local daemon and remote daemons without opening a network port on any host.
2. Act as the maintainer, not as an agent session.
3. Keep one view across daemons, with each item labeled by host.
4. Build CGO-free for the same four targets as Koinon (WDR 004).

### Scope

Covers the bridge process, the dashboard's address and the connectors. Speech engines are
ADR 002; inference adapters are ADR 003. The Koinon side of the maintainer interface is
Koinon's to decide (rwcii/koinon#464).

---

## 2. Decision 1: One local Go process serving a loopback dashboard

### Decision

`koinon-voice` is one Go process on the maintainer's machine. It serves the dashboard on a
loopback address and runs speech engines as sidecar processes (ADR 002). On macOS the Swift
helper captures the microphone and plays speech, with Apple's voice processing (ADR 002,
Decision 3); the dashboard shows the listening state and the item, and does not capture
audio.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Native macOS app | A Swift app is the bridge | Ties the bridge to one platform; the dashboard is the interface the maintainer described (PDR 001) |
| Inside the Koinon daemon | Voice as a Koinon feature | The maintainer chose a peer repository; the daemon runs on remote hosts without audio |

### Rationale

1. Same language and build as Koinon (WDR 001, Decision 7).
2. A loopback page keeps the dashboard on the maintainer's machine.
3. Echo cancellation was shown in Phase 0 with the helper's voice processing, not with a
   browser's; the design session's proposal had the browser capture the microphone.

### Trade-offs Accepted

macOS grants microphone access to the application that starts the helper; in Phase 0 a
terminal without that grant recorded nothing. Starting the bridge needs a context that holds
the grant.

---

## 3. Decision 2: A connector per daemon; SSH forwards for remote hosts

### Decision

Each Koinon daemon is a connector with a host label. The local connector uses the daemon's
loopback address. A remote connector runs an SSH local forward from a loopback port on the
maintainer's machine to the remote daemon's loopback address, using the maintainer's SSH
configuration, and reconnects when the forward drops. Items, peers and alerts carry their
host label in the unified view.

Until Koinon has a lasting maintainer credential (rwcii/koinon#464), a remote connector logs
in by running `koinon dashboard --no-open` on that host over SSH and opening the link through
its forward, with the link's address rewritten to the forwarded port. A daemon restart ends
the login, and the connector logs in again the same way.

**Provenance:** The maintainer asked for local and remote daemons over SSH (PDR 001,
Decision 1). The forward-per-daemon mechanism and the login over SSH are proposals. Phase 0
logged in through a plain TCP forward standing in for SSH, with the link rewritten, and the
login is not bound to a port; an SSH forward to a remote daemon was not tried.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Remote daemons listen on the network | Expose each daemon's port | Breaks Koinon's loopback-only boundary |
| Run `koinon` commands over SSH per action | No persistent connection | No change notices; a process per action |

### Rationale

1. SSH is already how the maintainer reaches those hosts; no port opens.

### Trade-offs Accepted

A dropped forward hides that host's items until it reconnects; the dashboard shows the host
as disconnected.

---

## 4. Decision 3: Act through Koinon's maintainer interface

### Decision

Each connector reads what needs the maintainer, receives change notices and records the
maintainer's posts and verdicts through Koinon's maintainer interface, authenticated as the
maintainer. Koinon Voice does not register as an agent session to do so. The interface is
requested in rwcii/koinon#464.

Phase 0 found that Koinon v0.4.1 offers the maintainer only the HTML dashboard: no
machine-readable read, no change notices, no stable action interface, no lasting credential,
and no client recorded on a verdict or post. Phase 1's Koinon features need at least the
first three from #464; the gaps are listed on that issue.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Scrape the dashboard | Drive the HTML dashboard with its login cookie | Undocumented and fragile; breaks with any dashboard change. Phase 0 showed it works from `curl`, including verdicts, which does not change the reason |
| Register as an agent session | Use the MCP tools as a peer | Peer messages are data and carry no maintainer authority (Koinon WDR 002, Decision 3) |

### Rationale

1. Spoken words carry the maintainer's weight only if they arrive as the maintainer's actions.

### Trade-offs Accepted

Phase 1 depends on Koinon delivering #464.

---

## Conclusion

One local Go process serves a loopback dashboard, leaves the audio to its helper, reaches
each daemon on loopback directly or through an SSH forward, and acts through Koinon's
maintainer interface, which Koinon v0.4.1 does not have yet. The record is a proposal, revised
with Phase 0's evidence, for the maintainer's decision.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-24 | Revised from Phase 0: the helper captures audio, the login over SSH, and the gaps in Koinon v0.4.1 |
