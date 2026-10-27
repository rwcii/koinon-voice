# Sprint 2026-10-27 — Connectors and dashboard — Decision

This sprint builds the base of Phase 1 (PDR 001, Decision 3): the connectors of ADR 001 and a
loopback dashboard that shows what needs the maintainer and records the maintainer's verdicts
by click. Speech is the next sprint. Claude drives the plan; Codex reviews.

**Provenance:** Maintainer, 2026-10-27, approved board item kv-sprint1-scope, "Koinon Voice:
what does the next sprint deliver?", with the note "a". Option A was "Connector and dashboard
first, without voice": the connector (local child process and `ssh HOST`, the same code), the
Keychain secret, restart after a daemon restart, change notices, and `koinon-voice serve` with
a loopback dashboard that shows the waiting items by host and records a verdict by click as
`mcp:koinon-voice`; the SSH test stays on #15, and the voice loop is the next sprint. The other
options were all of Phase 1 in one sprint, and a terminal command without a dashboard. The
recommendation was A.

## Problem

Phase 0 settled the engines and mapped Koinon; ADR 001 is accepted, and Koinon v0.5.0 has the
maintainer mode it needs. Koinon Voice has no connection to Koinon yet:

- No process holds a connection to a daemon, as the maintainer, with the secret from the
  Keychain (ADR 001, Decision 3).
- Nothing reads the decision board, the maintainer's inbox or the rooms, or follows their
  change notices.
- Nothing records a verdict as the maintainer with the client `mcp:koinon-voice`.
- There is no dashboard to show items from more than one daemon, each with its host label
  (ADR 001, Decision 2).

## Acceptance criteria

1. **Connector.** A connector starts `koinon mcp --maintainer --client koinon-voice
   --secret-stdin` for one daemon, as a child process for the local daemon or through
   `ssh HOST` for a remote one, writes the secret as the first line of its standard input,
   and speaks MCP on its standard input and output (ADR 001, Decision 2).
2. **Secret.** Each host's secret comes from the macOS Keychain and appears in no file, log,
   command line, error or dashboard page (ADR 001, Decision 3).
3. **Reads and notices.** The connector reads the waiting decision items, the maintainer's
   inbox and the rooms, subscribes to their resources, and reads a resource again when its
   change notice arrives.
4. **Restart.** When the process exits, the SSH connection drops or a request answers
   `maintainer_token_expired`, the connector starts the process again and reads its state
   again; while it is down, the host shows as disconnected (ADR 001, Decision 2).
5. **Dashboard.** `koinon-voice serve` serves a page on a loopback address that shows each
   host's connection state and its waiting items, labeled by host, and updates when a notice
   arrives.
6. **Verdict.** From the page, the maintainer approves or declines a decision item, or
   dismisses an fyi item, with an optional note, at the revision the page showed. Koinon
   records it with the client `mcp:koinon-voice`. A verdict on a changed item is refused, and
   the page shows the item again and says so (ADR 001, Decision 3).
7. **Configuration.** The hosts, each with its label, its `ssh` destination or local
   address, and its Keychain item, come from a configuration file in the maintainer's
   configuration directory.
8. **Evidence.** The maintainer runs the bridge against the local daemon and records a
   verdict by click; the history shows `mcp:koinon-voice`. The SSH path is tested on #15
   when the maintainer names a host.

## Constraints

- **Boundary.** The dashboard and the connectors stay on the maintainer's machine; no port
  opens on any host (ADR 001, Decisions 1 and 2).
- **Koinon.** Koinon Voice uses the maintainer mode of Koinon v0.5.0 or later and does not
  register as an agent session (ADR 001, Decision 3). It proposes Koinon changes through
  Koinon issues and does not change rwcii/koinon from this repository.
- **Tests.** The Go suite uses a synthetic `koinon mcp` process, temporary state, and no
  daemon, Keychain, SSH or network beyond loopback, isolated from the person's environment
  (WDR 006, Decision 2).
- **Build.** The Go module stays CGO-free for the four targets (WDR 004). The Keychain is
  macOS only (ADR 001, Decision 3); on other platforms the build succeeds and the bridge
  says where it has no secret store.
- **Process.** Gates A and B (WDR 002, Decision 5); one open pull request per target branch
  (WDR 004, Decision 2).

## Issues

This sprint delivers the connectors of ADR 001 and the dashboard base of Phase 1. It does not
deliver #15, which needs a remote host, or the voice loop.
