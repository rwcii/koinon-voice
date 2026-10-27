---
adr: "001"
title: "Local Bridge and Koinon Connectors"
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
**Status:** Accepted
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice is one Go process on the maintainer's machine. It serves the unified dashboard on
loopback and holds one connector per Koinon daemon. Each connector runs Koinon's maintainer
mode, `koinon mcp --maintainer`, as a stdio MCP server: on this machine as a child process,
and on a remote host through `ssh HOST`. It presents that daemon's maintainer secret from the
Keychain, and its actions are recorded as the maintainer's, with the client `mcp:koinon-voice`.

This record is a proposal from the design session of 2026-10-23. It was revised on 2026-10-24
from Phase 0's map of Koinon v0.4.1 (`docs/sprints/2026-10-23-phase0-spike/koinon-map.md`) and
its audio results (`results.md` beside it), and on 2026-10-25 to the maintainer mode of Koinon
PDR 009, which the maintainer approved on rwcii/koinon#470 that day. Koinon v0.5.0 has the
maintainer mode, its change notices and its token rules.

**Provenance:** Maintainer, 2026-10-26, approved board item kv-adr001, "Koinon Voice: accept
ADR 001 (connectors use koinon mcp --maintainer)?", which recommended accepting the record as
written, with no note.

---

## Table of Contents

1. Problem Context
2. Decision 1: One local Go process serving a loopback dashboard
3. Decision 2: A connector per daemon; stdio over SSH for remote hosts
4. Decision 3: Act through Koinon's maintainer mode
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
ADR 002; inference adapters are ADR 003. The Koinon side of the maintainer mode is Koinon's
to decide (Koinon PDR 009, rwcii/koinon#470).

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

## 3. Decision 2: A connector per daemon; stdio over SSH for remote hosts

### Decision

Each Koinon daemon is a connector with a host label. A connector runs
`koinon mcp --maintainer --client koinon-voice --secret-stdin` and speaks MCP over its standard
input and output. The local connector starts it as a child process. A remote connector starts
it with `ssh HOST`, using the maintainer's SSH configuration, so the remote daemon stays on its
loopback address and no port is forwarded. The secret is written as the first line of the
process's standard input, so it travels inside the SSH channel and not on either host's
command line (Koinon PDR 009, Decision 3).

When the process exits, the SSH connection drops or Koinon ends the maintainer session (a
daemon restart ends it), the connector starts the process again. Items, peers and alerts carry
their host label in the unified view.

**Provenance:** The maintainer asked for local and remote daemons over SSH (PDR 001,
Decision 1). After reading Phase 0's map, the maintainer said "koinon will need an API
surfaced for this. maybe this needs to be an MCP" and asked for the spec to be filed with
Koinon (2026-10-23, rwcii/koinon#470). Koinon PDR 009 makes the maintainer mode a stdio MCP
server for local clients and clients over SSH; the maintainer approved it on 2026-10-25. One
connector per daemon and starting the process again were this record's proposals, which the
maintainer accepted with it on 2026-10-26. The SSH path has not been tried yet (#15).

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Remote daemons listen on the network | Expose each daemon's port | Breaks Koinon's loopback-only boundary |
| An SSH local forward per daemon | Forward a loopback port to the remote daemon and log in to its dashboard (this record's proposal until 2026-10-25) | The maintainer mode needs no port or login cookie; a forward was needed only for the dashboard |
| Run `koinon` commands over SSH per action | No persistent connection | No change notices; a process per action |

### Rationale

1. SSH is already how the maintainer reaches those hosts; no port opens on either end.
2. One long-lived SSH session per host carries the reads, the actions and the change notices.

### Trade-offs Accepted

- A dropped connection hides that host's items until it reconnects; the dashboard shows the
  host as disconnected.
- Each daemon restart, which every Koinon upgrade causes, ends the maintainer session, and
  the connector reconnects. Koinon v0.5.0 ends a subscribed maintainer-mode process with
  `maintainer_token_expired` a few seconds after the restart, and answers a process with no
  subscription so on its next request; the connector stays subscribed and also makes a read
  when it has been idle (the connectors sprint's definition of done).
- SSH must run without prompts: a key in the SSH agent, and a known host key. #15 tries it
  against a remote host.

---

## 4. Decision 3: Act through Koinon's maintainer mode

### Decision

Each connector reads what needs the maintainer, receives change notices and records the
maintainer's posts and verdicts through the maintainer mode's tools, as the built-in
`maintainer` session. Koinon Voice does not register as an agent session and is not listed in
`peers`.

- **Secret.** Each daemon has one maintainer secret, which Koinon stores only as a hash
  (Koinon PDR 009, Decision 1). Koinon Voice keeps each host's secret in the macOS Keychain,
  as it keeps API keys (ADR 003, Decision 2), and never writes it to a file, a log or the
  dashboard.
- **Reads and actions.** `board_list`, `decision_get` and `decision_history` read the decision
  board; `decision_verdict` records a verdict at the item's current revision; `inbox`, `ack`,
  `send` and the room tools carry messages and posts; `peers` and `peer_status` read the peers.
  A verdict on an item whose revision has changed is refused, and the bridge reads the item
  again and says so.
- **Notices.** The bridge subscribes to the maintainer's inbox, decision and room resources;
  a notice carries only the resource, and the bridge reads it again.
- **Client.** Koinon records each action's client as `mcp:koinon-voice` and shows it on the
  dashboard and in `decision_history` (Koinon PDR 009, Decision 4); agents see the same
  `maintainer` sender as for a dashboard action.

Koinon delivers PDR 009 in three PRs: the secret and the dashboard password login; the
maintainer mode with its tools and the client; then change notices. The bridge's Koinon
features can start on the second, reading on a timer until the third ships.

**Provenance:** The maintainer decided on rwcii/koinon#470 that "the maintainer should be able
to register a persistenet secret, or generate a password. this should be used to unlock the
dashboard, and allow a remote system interact with the MCP/API" and "we can hash the
secret/password, so only the valid secret would vlaidate" (2026-10-23). On 2026-10-25, asked
whether each maintainer action should record its client, the maintainer answered "yes, record
the client on #470". Asked whether to refuse the maintainer mode to processes that descend
from an agent session (option C), with the recommendation of no because the secret is already
the gate and ancestry adds no proof against a process that holds it, and then whether to
approve the draft as a whole, the maintainer answered "no on option C, PDR 009 approved".
That approves PDR 009 as drafted, with the client recorded and Koinon's three-PR delivery,
and leaves agent descendants able to start the maintainer mode with the secret. Keeping each
host's secret in the Keychain was this record's proposal, which the maintainer accepted with
it on 2026-10-26.

Phase 0 found that Koinon v0.4.1 offers the maintainer only the HTML dashboard: no
machine-readable read, no change notices, no stable action interface, no lasting credential,
and no client recorded on a verdict or post (rwcii/koinon#464). The maintainer mode's second
PR closes all but the notices, and its third PR closes those. Koinon v0.5.0 has all of it.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Scrape the dashboard | Drive the HTML dashboard with its login cookie, now also with the secret's password login | Undocumented and fragile; breaks with any dashboard change. Phase 0 showed it works from `curl`, including verdicts, which does not change the reason |
| Register as an agent session | Use the MCP tools as a peer | Koinon records an agent session's messages and posts as that agent's; only the `maintainer` session's actions are the maintainer's |
| The secret in a file | `--secret-file` with mode 0600 | A plain-text file on disk; the Keychain keeps it in the maintainer's login and off disk in plain text |

### Rationale

1. Spoken words carry the maintainer's weight only if they arrive as the maintainer's actions.
2. The recorded client tells a spoken verdict from a click, as #464 asked.

### Trade-offs Accepted

- Phase 1's Koinon features need Koinon v0.5.0 or later, which has PDR 009's maintainer mode
  and its notices.
- The secret separates the maintainer's clients from agents that use Koinon as intended; it
  does not stop a process running as the maintainer's OS user that sets out to bypass it
  (Koinon PDR 009, Decision 1).
- The Keychain is macOS only. Where a connector keeps the secret on another platform is not
  decided.

---

## Conclusion

One local Go process serves a loopback dashboard, leaves the audio to its helper, and reaches
each daemon through Koinon's maintainer mode on stdio, locally or over SSH, with that
daemon's secret from the Keychain. Its actions are the maintainer's, recorded with the client
`mcp:koinon-voice`. Koinon PDR 009 defines the maintainer mode, and Koinon v0.5.0 has it. The
maintainer accepted this record on 2026-10-26.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-24 | Revised from Phase 0: the helper captures audio, the login over SSH, and the gaps in Koinon v0.4.1 |
| 2026-10-25 | Revised to Koinon PDR 009: connectors run `koinon mcp --maintainer` on stdio, locally or over SSH, in place of SSH forwards and dashboard logins; the secret in the Keychain; the client recorded as `mcp:koinon-voice` |
| 2026-10-25 | The agent-session alternative's reason stated as what Koinon records, not as a limit on peer messages (rwcii/koinon#487) |
| 2026-10-26 | Accepted by the maintainer (board item kv-adr001); Koinon v0.5.0 has the maintainer mode |
| 2026-10-27 | Decision 2's trade-off states what Koinon v0.5.0 does when the maintainer session ends, which the record had left open |
