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
| R13 | Each action's outcome reaches Voice: recorded there, refused there (with its code), not dispatched (`computer_offline`), or unknown (dispatched, answer lost). | **[proposal]** | Each outcome is shown in a test that makes it; a reply dropped after the target recorded the write is reported unknown, not `computer_offline`. |
| R14 | A stable request ID: a retry with the same ID returns the first result and never records a second action, while the target keeps the receipt (7 days in the Koinon draft). | **[proposal]** | A verdict whose answer is dropped after it is recorded, then retried, is recorded once. |
| R15 | A read of an action's outcome by request ID, for reconciliation after a restart or a lost connection; a defined answer for an unknown or expired request ID, so Voice never sends a new write for an action it may have recorded. | **[proposal]** | After a restart, Voice reads the outcome of an action it sent before. |
| R16 | Signed actions, `send` and `room_post` included, are not queued: when the target cannot be reached they answer `computer_offline` at once, and Voice says so. The queue of `send` and `room_post` that Voice proposed is deferred to rwcii/koinon#695. | **[record]** Robert Capps, 2026-11-14, in claude-koinon@blackhole's terminal, relayed over the link: "No queue for now", and "file an issue to track any item we push off, so it's not lost." | An action to an offline computer answers `computer_offline` and is not recorded later. |

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
| R24 | Where no Koinon daemon runs on Voice's computer, Voice reaches another computer's Remote (Tailscale Serve) address on a route that takes only signed actions, and authenticates there with its registered key. | **[record]** Robert Capps, 2026-11-14, in claude-koinon@blackhole's terminal, relayed over the link: "Remote address, signed-only"; the wire protocol is open (G6) | With no daemon on Voice's computer, the same actions work through another computer's Remote address; the route answers nothing but signed envelopes. |
| R25 | Voice's connection to its own computer's daemon authenticates with a signed `session` in place of the maintainer secret, and every action, the local computer's included, is a signed envelope. | **[proposal]** Koinon draft, Decision 5.1 and 5.3; the session's exact form is open (G3) | The local connection opens with the key and no maintainer secret. |
| R26 | A public key is registered on each computer by that computer's own maintainer login or secret, with a signed enrollment statement; a group join carries no keys. | **[record]** #693 for registration through each computer's login or secret; **[proposal]** Koinon draft, Decisions 1.4 and 1.5, for the statement and the group join; the source of the target's pin is open (G7) | `koinon-voice` prints a statement that `koinon maintainer key add` accepts on each computer. |

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

Koinon draft: rwcii/koinon branch `docs/693-signed-maintainer`, commit `830345f`,
`docs/sprints/2026-11-14-signed-maintainer/decision.md` (D1 to D7 are its decisions). Gaps G1
to G11 went to the Koinon pair on 2026-11-14 from both Voice agents; each closes when the
revised draft answers it.

| ID | Koinon draft section | Status |
| --- | --- | --- |
| R1, R5, R6, R20 | D2 (`board_list`, `decision_get`, `inbox`, `room_list`, `room_read`, paged) | covered |
| R2, R3, R4 | D2 `decision_verdict` with every verdict the item's kind takes; `stale_revision`, `not_found` | covered, except G1: the snapshot digest's bytes, and that Voice computes it from what it shows |
| R7 | D2 (`decision_history`, `ack`, `send`, `room_post`, `peers`, `peer_status`) | covered |
| R8 | D2, the `peers` paragraph | covered |
| R9, R10 | D1 (Ed25519 per user and install; private key with the client); D3.2 refusals | covered |
| R11 | D3 (envelope `koinon-maintainer-1`, RFC 8785, Ed25519) | covered, except G2: the order of the time check and the receipt, and enroll's 5 or 10 minutes |
| R12 | D3 `target` is the pin; D5.4 lists pins and names | covered, except G4 (the list operation and its notice) and G7 (where the pin comes from) |
| R13 | D4.5 | gap G11: "not dispatched" and "answer lost" are one answer |
| R14, R15 | D4 (receipts, `request_conflict`, `outcome`) | gaps G9 (receipt scope and key re-enrollment) and G10 (expired or unknown request ID) |
| R16 | D4.5, settled point 1; rwcii/koinon#695 | settled |
| R17, R18, R19, R21 | D6, D5.4 | covered, except G5: the per-pin notice on the MCP wire |
| R22, R23 | D5 | covered |
| R24 | D5.2, settled point 2 | settled, except G6: the route's protocol and the tailnet |
| R25 | D5.1, D5.3 | gap G3: the signed `session` on `koinon mcp --maintainer` |
| R26 | D1.4, D1.5 | covered, except G7 |
| Records | Koinon draft, Records | gap G8: Voice's records are ADR 004, ADR 001 (modified), PDRs 001 and 002, and PDR 004, Decision 2 (opt-in kept by pin) |

## Agreement

Recorded in a later commit, naming the Koinon draft commit and this document's checkpoint. The
agreement covers the settled requirements; requirements still open or deferred are named as
such, and the contract is build-ready only when none required is open.
