# Sprint 2026-11-14 — Signed client of Koinon nodes — Decision

This sprint delivers #36: Koinon Voice as a signed client of Koinon nodes, built in step with
the Koinon pair's side of rwcii/koinon#693. This document first settles the Voice side of the
contract with the Koinon pair; then both sides build against it, and exchange commits at each
frozen checkpoint. Claude drives; Codex reviews. Gate A comes after the contract is settled and
the open points below are closed or deferred by name.

Each point is marked **[record]**, with its source, or **[proposal]**, which no record decides.

## Sources

- rwcii/koinon#693, "Signed maintainer actions across linked computers, with authorized keys
  and the user name on verdicts". Robert Capps, 2026-11-14, terminal, quoted there: "yes, carry
  maintainer traffic over the links" and "koinon-voice should create a keypair per
  user/install, and maintainer messages are signed before transmission, public keys can be
  registered with remote systems as authorized keys. this also gives us a way to tie auth to
  username. maintainer board decisions could gain identity".
- Board kv-693-ssh, approved by Robert Capps, 2026-11-14, revision 1: Koinon Voice replaces its
  SSH connectors with one connection to its own computer's daemon and reaches paired computers
  through the links; the SSH connectors stay until Koinon ships signed maintainer actions over
  links and Koinon Voice's sprint for them merges; then SSH and the `ssh` entries of
  `hosts.toml` are removed. Recorded in
  [ADR 004](../../architecture/adr/004-signed-client-of-koinon-nodes.md), Decision 1.
- The note with that verdict: "I question if we should have a separate local install for
  koinon to run voice. can't voice include the koinon instance?" The answer came as board
  kv-693-bundle (bundle the `koinon` binary, recommended; a separate install; embed Koinon),
  which was withdrawn, not decided, after Robert Capps answered in the terminal, 2026-11-14: "I
  would rather voice just contain the parts needed to communicate with full koinon nodes. I
  don't want it to run koinon itself." Recorded in ADR 004, Decision 2.
- Robert Capps, 2026-11-14, terminal: "as you progress your implementation work, coordinate
  with the koinon pair. keep in sync to deliver both parts. we can hold off on further modeling
  work, while you work on implementation of the client work." Then: "this should be a new
  sprint, we'll come back to the voice modeling work". So this is a new sprint, the client is
  built in it in step with the Koinon pair, and the voice-model work (#10, and #12's ratings)
  waits.
- Koinon PDR 009 (maintainer mode) and PDR 011 (linked computers), as of 2026-11-14.
- Koinon Voice ADR 001, PDRs 001 to 004, the connectors sprint
  (`docs/sprints/2026-10-27-connectors/`) and `internal/koinon/connector.go` at `cdb1e3a`;
  #15 (the SSH path passed on 2026-10-29).

## Problem

- **[record]** Koinon Voice holds one connector per Koinon daemon, through `ssh HOST` for a
  remote one, with one maintainer secret per host in the Keychain (ADR 001, Decisions 2 and 3).
- **[record]** Koinon links carry agent messages only. The decision board, the maintainer
  inbox and dashboard messages stay on each computer, and a maintainer sender is refused
  (Koinon PDR 011, Decision 2). #693 changes that.
- **[record]** A verdict records the client (`mcp:koinon-voice`, Koinon PDR 009, Decision 4),
  not which user made it. #693 adds the user name of the key.

## Requirements on Koinon

Each requirement has an ID, its state and the evidence that shows it met. The Koinon draft is
mapped to them in [Mapping to the Koinon draft](#mapping-to-the-koinon-draft).

### Actions

| ID | Requirement | State | Evidence |
| --- | --- | --- | --- |
| R1 | Read the board on each paired computer: `board_list` (paged), `decision_get`. | **[record]** called today (connector.go) | A read through the client connection returns another computer's items, with that computer named. |
| R2 | Record a verdict on another computer: `decision_verdict` with reference, revision, verdict and note; wire verdicts `approved`, `declined`, `dismissed`. | **[record]** called today; connectors sprint criterion 6 | The other computer's `decision_history` shows the verdict, the client and the user name. |
| R3 | Every verdict kind the board now has: task `completed`, `declined`, `feedback`; `feedback` on decision items. | **[proposal]** | As R2, for each kind. |
| R4 | Revision rule across the link: `stale_revision` with the current revision, and `not_found`, come back from the other computer. | **[record]** ADR 001, Decision 3 | A verdict at an old revision answers `stale_revision` with the current one; no verdict is recorded. |
| R5 | Read the maintainer inbox of another computer: `inbox`. | **[record]** called today | As R1. |
| R6 | Read rooms of another computer: `room_list`, `room_read`. | **[record]** called today | As R1. |
| R7 | `decision_history` on screen (PDR 002, Decision 3); `ack`, `send` and `room_post` as the maintainer for the voice loop (PDR 001, Decision 1; PDR 002, Decision 1); `peers` and `peer_status`. | **[proposal]** planned; not called today | As R1 and R2, per tool. |
| R8 | Peers of another computer for stalled sessions: activity state, its time, and the stall signal that the local maintainer view has, if any. Nothing that PDR 011, Decision 2, item 2 excludes (native IDs, folder paths, process data). | **[proposal]** | `peers` through the client shows these fields for another computer's sessions. |

### Identity and signatures

| ID | Requirement | State | Evidence |
| --- | --- | --- | --- |
| R9 | Each Koinon Voice install has a key pair per user; each maintainer action, reads included, is signed before it leaves; each computer verifies the signature and accepts only registered public keys, each with a user name; each verdict records the user name. | **[record]** #693 | An unsigned action, an action with a bad signature and an action with an unknown key are each refused with their code, and nothing is recorded; a verdict's history shows the user name. |
| R10 | The private key is in the macOS Keychain, as ADR 001, Decision 3 keeps secrets; other platforms stay open. | **[proposal]** | No private key in a file, log, argument or page. |
| R11 | The signature covers the action and its arguments, the target computer, the client, a key ID, a request ID and a creation time. | **[proposal]**, derived from R9 | An envelope with any covered field changed is refused. |
| R12 | The target computer is named by a stable identity, such as its pinned link key, not only by its name: the name is local to the receiver and can change (Koinon PDR 011, Decision 3). Koinon Voice keys its state (references, room IDs, cursors) by that identity and shows the name. | **[proposal]** | After a rename, an action and Voice's state still reach the same computer; two computers with the same reference stay apart (connectors sprint edge case). |

### Outcomes and retries

| ID | Requirement | State | Evidence |
| --- | --- | --- | --- |
| R13 | Each action's outcome reaches Voice: recorded there, refused there (with its code), queued here, or unknown. | **[proposal]** | Each outcome is shown in a test that makes it. |
| R14 | A stable request ID: a retry with the same ID returns the first result and never records a second action. | **[proposal]** | A verdict whose answer is dropped, then retried, is recorded once. |
| R15 | A read of an action's outcome by request ID, for reconciliation after a restart or a lost connection. | **[proposal]** | After a restart, Voice reads the outcome of an action it sent before. |
| R16 | Verdicts and inbox acks are refused at once when the target computer cannot be reached; `send` and `room_post` queue like agent messages, with their delivery state. No record decides this; PDR 002, Decision 2's read-back confirms intent before the action, and Voice then says the outcome. | **[proposal]** | A verdict to an offline computer answers "computer offline"; a `send` shows `queued`. |

### Updates and connection state

| ID | Requirement | State | Evidence |
| --- | --- | --- | --- |
| R17 | Change notices for each computer's maintainer inbox, board and rooms; a notice may carry only the resource, and Voice reads it again. | **[record]** connectors sprint, definition of done; **[proposal]** that each names its computer | A verdict on another computer sends a notice that names it. |
| R18 | Each computer's connection state, with its reason and time. | **[record]** ADR 001, Decision 2; criterion 4; **[proposal]** per computer through one connection | A computer that goes offline shows as disconnected, with the others connected. |
| R19 | A remote computer's restart or link loss does not end Voice's connection; that computer shows disconnected, then connected, and Voice catches up with a cursor for each computer and resource. The other computers' state stays. | **[proposal]** | A restart of one paired daemon leaves the others' items on screen, and the restarted one's items return with no gap. |
| R20 | Paged reads with `more` and `next_after`. | **[record]** connectors sprint | As now. |
| R21 | Latency: no record sets a number. The design stays notice-then-read, with a liveness read per connection when idle; through one connection, no polling for each remote computer. | **[record]** notice-then-read and the idle liveness read per connector (connectors sprint; `Options.Liveness` in connector.go); **[proposal]** no polling for each remote computer; no number | — |

### Where Voice connects

| ID | Requirement | State | Evidence |
| --- | --- | --- | --- |
| R22 | Voice contains only the parts that talk to full Koinon nodes and never runs Koinon itself. | **[record]** ADR 004, Decision 2 | Voice's install has no `koinon` binary and starts no daemon. |
| R23 | Where a Koinon daemon runs on Voice's computer, Voice has one connection to it and reaches the paired computers through it. | **[record]** ADR 004, Decision 1 | One connection shows every paired computer. |
| R24 | Where no Koinon daemon runs on Voice's computer, how Voice reaches a full node, and how it authenticates there with its registered key. | **Open**: R22 rules out starting or bundling a node; the endpoint is not decided | — |
| R25 | How Voice's connection to its own computer's daemon authenticates: the key, the local maintainer secret (ADR 001, Decision 3), or both, and which one signs an action for the local computer. | **Open**, asked of Koinon | — |
| R26 | How a public key is registered on each computer (#693: through that computer's maintainer login or secret), and whether a group join (Koinon PDR 011, Decision 5) carries registered keys to introduced computers. | **Open**, asked of Koinon | — |

## Changes on the Koinon Voice side

- **[record]** ADR 004 (draft) holds Decisions 1 and 2 above. ADR 001 is `modified`: its
  Decisions 2 and 3 point to ADR 004 and stay in effect until the transition in ADR 004,
  Decision 1 ends.
- **[record]** PDR 001, Decisions 1 and 3: the maintainer's direction of 2026-10-23 for
  remote daemons over SSH is changed by kv-693-ssh; Phase 2 reaches paired computers through
  the links. PDR 001, Decision 2 (audio stays local) does not change.
- **[proposal]** PDR 002: a verdict records the key's user name; the read-back names the
  target computer; Voice says each outcome (R13). These change PDR 002 when this sprint's Gate A
  approves them.
- **[record]** PDR 004, Decision 2 (external inference per host, off by default; board
  kv-pdr004-d2, 2026-10-26): the policy does not change. **[proposal]** The opt-in is kept by
  the computer's stable identity (R12), so a rename or a new pairing under a reused name never
  carries it to another computer.
- **[record]** PDR 003 does not change.
- **[proposal]** In this sprint, in step with Koinon's build: `internal/koinon` speaks to one
  daemon for all computers; `hosts.toml` gives way to the computer list from that daemon;
  `koinon-voice board` and `koinon-voice dashboard` show computers by name and keep state by
  identity; `koinon-voice key create` and `koinon-voice key show` make the key pair and print
  the public key to register.

## Acceptance criteria

**[proposal]** Gate A starts from these:

1. Every [record] requirement of R1 to R23 is met, with its evidence: first against the Koinon
   pair's branch commits or scratch-daemon builds at each checkpoint, then against the Koinon
   release that ships #693.
2. Each [proposal] requirement is met, or named as deferred in this sprint's plan.
3. The SSH connector is removed only after Koinon ships signed maintainer actions over links and
   Voice's client work merges (ADR 004, Decision 1). Voice's implementation does not wait for
   that release; the removal does.
4. R24 to R26 are settled with their sources before Gate A.

## Mapping to the Koinon draft

Filled when the Koinon pair sends its draft (a named commit on rwcii/koinon). Each requirement
maps to a section of the draft or is listed as a gap; each difference is settled by the pairs,
or put to the maintainer on the board when it is material or needs the maintainer's direction.

| ID | Koinon draft section | Status |
| --- | --- | --- |
| R1–R26 | — | waiting for the draft |

## Agreement

Recorded in a later commit, naming the Koinon draft commit and this document's checkpoint. The
agreement covers the settled requirements; requirements still open or deferred are named as
such, and the contract is build-ready only when none required is open.
