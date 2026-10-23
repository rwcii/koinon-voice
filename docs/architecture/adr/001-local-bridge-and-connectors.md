---
adr: "001"
title: "Local Bridge and Koinon Connectors"
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

This record is a proposal from the design session of 2026-10-23. Phase 0 maps what Koinon
offers today, and the record is revised with that evidence before it is accepted.

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
loopback address, where the browser grants microphone access to a page on `localhost`, and
runs speech engines as sidecar processes (ADR 002).

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Native macOS app | A Swift app is the bridge | Ties the bridge to one platform; the dashboard is the interface the maintainer described (PDR 001) |
| Inside the Koinon daemon | Voice as a Koinon feature | The maintainer chose a peer repository; the daemon runs on remote hosts without audio |

### Rationale

1. Same language and build as Koinon (WDR 001, Decision 7).
2. A loopback page keeps the dashboard on the maintainer's machine.

---

## 3. Decision 2: A connector per daemon; SSH forwards for remote hosts

### Decision

Each Koinon daemon is a connector with a host label. The local connector uses the daemon's
loopback address. A remote connector runs an SSH local forward from a loopback port on the
maintainer's machine to the remote daemon's loopback address, using the maintainer's SSH
configuration, and reconnects when the forward drops. Items, peers and alerts carry their
host label in the unified view.

**Provenance:** The maintainer asked for local and remote daemons over SSH (PDR 001,
Decision 1). The forward-per-daemon mechanism is a proposal.

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

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Scrape the dashboard | Drive the HTML dashboard with its login cookie | Undocumented and fragile; breaks with any dashboard change |
| Register as an agent session | Use the MCP tools as a peer | Peer messages are data and carry no maintainer authority (Koinon WDR 002, Decision 3) |

### Rationale

1. Spoken words carry the maintainer's weight only if they arrive as the maintainer's actions.

### Trade-offs Accepted

Phase 1 depends on Koinon delivering #464.

---

## Conclusion

One local Go process serves a loopback dashboard, reaches each daemon on loopback directly or
through an SSH forward, and acts through Koinon's maintainer interface. The record is a
proposal, to be revised with Phase 0's map of the Koinon interface.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
