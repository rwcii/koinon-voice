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
| R17 | Voice learns of changes to each computer's maintainer inbox, board and rooms, and reads the changed topics again. | **[record]** notice-then-read (connectors sprint, definition of done); **[proposal]** the signed mechanism: one signed `changes` long wait per target that Voice shows (`wait_seconds` up to 30), with topic versions; Voice signs each next wait, and the node makes no reads of its own (Koinon contract D6.1, D6.2) | A verdict on another computer bumps that target's `decisions` version, and Voice reads it again. |
| R18 | Each computer's connection state, with its reason and time. | **[record]** ADR 001, Decision 2; criterion 4; **[proposal]** per computer through one connection | A computer that goes offline shows as disconnected, with the others connected. |
| R19 | A remote computer's restart or link loss does not end Voice's connection; that target answers `computer_offline`, Voice watches the node's `computers` topic for its return, then reads each topic again from its stored version. The other computers' state stays. | **[proposal]** Koinon contract D6.3 to D6.5 | A restart of one paired daemon leaves the others' items on screen, and the restarted one's items return with no gap. |
| R20 | Paged reads with `more` and `next_after`. | **[record]** connectors sprint | As now. |
| R21 | Latency: no record sets a number. Voice keeps one concurrent signed `changes` long wait for each target it shows, not a list read at an interval. | **[record]** notice-then-read and the idle liveness read per connector (connectors sprint; `Options.Liveness` in connector.go) for the current connectors; **[proposal]** the long waits for the signed client; no number | — |

### Where Voice connects

| ID | Requirement | State | Evidence |
| --- | --- | --- | --- |
| R22 | Voice contains only the parts that talk to full Koinon nodes and never runs Koinon itself. | **[record]** ADR 004, Decision 2 | Voice's install has no `koinon` binary and starts no daemon. |
| R23 | Where a Koinon daemon runs on Voice's computer, Voice has one connection to it and reaches the paired computers through it. | **[record]** ADR 004, Decision 1 | One connection shows every paired computer. |
| R24 | Where no Koinon daemon runs on Voice's computer, Voice reaches another computer's Remote (Tailscale Serve) address on a route that takes only signed actions, and authenticates there with its registered key. | **[record]** Robert Capps, 2026-11-14, in claude-koinon@blackhole's terminal, relayed over the link: "Remote address, signed-only"; the protocol is Koinon contract D5.2 at `15ec0ff` (session token, then one envelope per request; the client on the tailnet) | With no daemon on Voice's computer, the same actions work through another computer's Remote address; the route answers nothing but signed envelopes. |
| R25 | Voice's connection to its own computer's daemon authenticates with a signed `session` in place of the maintainer secret, and every action, the local computer's included, is a signed envelope. | **[proposal]** Koinon contract D5.1 at `15ec0ff`: `koinon mcp --maintainer --signed`, the signed `session` as the first line of standard input, then the `signed` tool | The local connection opens with the key and no maintainer secret. |
| R26 | A public key is registered on each computer by that computer's own maintainer login or secret, with a signed enrollment statement; a group join carries no keys. | **[record]** #693 for registration through each computer's login or secret; **[proposal]** Koinon contract D1.4, D1.5 and D3.5 at `15ec0ff` for the statement (with `public_key`), the group join and the pin, which is read on the target from `koinon link status` | `koinon-voice` prints a statement that `koinon maintainer key add` accepts on each computer. |

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

1. Every [record] requirement of R1 to R26, and the recorded part of each mixed row, is met,
   with its evidence: first against the Koinon
   pair's branch commits or scratch-daemon builds at each checkpoint, then against the Koinon
   release that ships #693.
2. Each [proposal] requirement is met, or named as deferred in this sprint's plan.
3. The SSH connector is removed only after Koinon ships signed maintainer actions over links and
   Voice's client work merges (ADR 004, Decision 1). Voice's implementation does not wait for
   that release; the removal does.
4. R24 to R26 follow Koinon contract D5.2, D5.1 and D1.4 at `15ec0ff`; a change to them there is mapped here before Gate A.

## Mapping to the Koinon draft

Koinon contract: rwcii/koinon branch `docs/693-signed-maintainer`, frozen commit `4e324d5`,
`docs/sprints/2026-11-14-signed-maintainer/decision.md` (D1 to D7 are its decisions); on the
Koinon board as card LINK-693-SIGNED, which the maintainer approved at revision 10 on
2026-11-15 (Koinon PDR 011, Decision 6; rwcii/koinon PR #698). Gaps G1 to G19
went to the Koinon pair on 2026-11-14 from both Voice agents, and each is closed. G20, the
Serve route's wire details, is closed at `dfb2ed7`, a commit on `4e324d5`:

| Gap | Closed by | Head |
| --- | --- | --- |
| G1 snapshot bytes; the client computes the digest | D2.2 | `6fac345` |
| G2 time check against the receipt; enroll's 10 minutes | D3.3, D3.5 | `6fac345` |
| G3 signed `session` on `koinon mcp --maintainer --signed` | D5.1 | `6fac345` |
| G4 the `computers` operation and topic | D2.4, D6.3 | `086b9f5` |
| G5, G16 notices: client-signed `changes` waits, no node reads | D5.3, D6.1, D6.2 | `086b9f5` |
| G6 the Serve route's protocol; the client on the tailnet | D5.2 | `6fac345` |
| G7 the pin read on the target (`koinon link status`) | D1.4 | `6fac345` |
| G8 Voice's records | Records | `6fac345` |
| G9 receipts per authorization | D1.3, D4.1 | `6fac345` |
| G10 receipts' 7 days; `unknown_request` | D4.3 | `6fac345` |
| G11 `computer_offline` against `outcome_unknown` | D4.5 | `6fac345` |
| G12 test vectors; v1 values; key ID without padding | D3.1, D3.2 | `a66c4f1` |
| G13 `public_key` in the enrollment statement | D3.5 | `8dc1c25` |
| G14 outcome's finality, serialized with writes | D3.3, D4.3, Acceptance 3 | `15ec0ff` |
| G15 `commands` as an array in the snapshot only | D2.2 | `ea8b97a` |
| G17 computer state for the Serve route | D6.3, D6.4 | `086b9f5` |
| G18 a write's outcome after the key is enrolled again | D1.3, D1.7, D3.3, D4.3: each envelope carries `auth`, the hex SHA-256 of its enrollment statement's signed bytes, and runs only under that authorization; a statement is added once (`statement_used`). This replaced the clock floor of `d578d0e`, `072870b` and `fa95824` (G18b, G18c) | `05c531c`; outcome refused under a gone authorization at `4e324d5` |
| G19 answer shapes of `outcome`, `changes` (20-digit string versions), `computers` and refusals | Answer shapes | `cbd8e21` |
| G20 the Serve route's session answer (flat, with `idle_seconds` and `absolute_seconds`), `maintainer_token_expired` with HTTP 409 before the envelope is read, `invalid_request` with 400, and the sizes: 512 KiB a request, 1 MiB an answer and a link frame | Answer shapes, D5.2 | `dfb2ed7`, on `4e324d5`; Voice's Serve form (`internal/koinon/signedhttp.go`) |
| G21 the MCP form answered one `tools/call` at a time, so an open `changes` wait held every other call of the session for up to 30 seconds (found in the live checks below). Koinon runs up to 128 calls at once; a call past them is answered at once with `too_many_calls` (`details.limit` 128) and is not sent to the daemon, which Voice treats as not run | D5.3, D6.2; claude-koinon, 2026-11-15 | Koinon #693 chunk 2 |

| ID | Koinon contract section | Status |
| --- | --- | --- |
| R1, R5, R6, R20 | D2 (`board_list`, `decision_get`, `inbox`, `room_list`, `room_read`, paged) | covered |
| R2, R3, R4 | D2, D2.1, D2.2 (`decision_verdict` with the snapshot digest; every verdict the kind takes; `stale_revision`, `not_found`, `snapshot_changed`) | covered |
| R7 | D2 (`decision_history`, `ack`, `send`, `room_post`, `peers`, `peer_status`) | covered |
| R8 | D2.3 | covered |
| R9, R10 | D1; D3.3 refusals | covered |
| R11 | D3 (envelope `koinon-maintainer-1`, RFC 8785, Ed25519; vectors) | covered; Voice's encoder passes the vectors (`internal/signed`) |
| R12 | D3 `target` is the pin; D1.4; D2.4 | covered |
| R13, R14, R15 | D4 (receipts per authorization, `request_conflict`, `outcome` with `now` under the write's own `auth`, `computer_offline` against `outcome_unknown`) | covered; Voice's `Reconcile` (`internal/koinon/signedclient.go`) |
| R16 | D4.5, settled point 1; rwcii/koinon#695 | settled |
| R17, R18, R19, R21 | D6 (client-signed `changes` waits; `computers` topic; versions are strings) | covered; Voice's `Watcher` (`internal/koinon/signedwatch.go`) |
| R22, R23 | D5 | covered |
| R24 | D5.2, settled point 2; Answer shapes at `dfb2ed7` (G20) | covered; Voice's Serve form (`internal/koinon/signedhttp.go`) |
| R25 | D5.1 | covered |
| R26 | D1.4, D1.5, D1.7, D3.5 | covered; `koinon-voice key enroll` records each computer's authorization |
| Records | Records | covered |

## Agreement

The Voice spec (this document, mapping R1 to R26) matches the Koinon contract at `4e324d5`
(rwcii/koinon, `docs/sprints/2026-11-14-signed-maintainer/decision.md`). Agreement, 2026-11-14,
over Koinon messages:

| Agent | Agreement |
| --- | --- |
| claude-koinon-voice (driver) | agrees with `bf7c290` against `4e324d5` |
| codex-koinon-voice (observer) | agrees with `bf7c29096638d26b5c2479325ceb5515d15ff993` against `4e324d56e938904c4f4be838603cc35433c75dcd`, after reading the changes from `15ec0ff` and reproducing the new vectors; the agreement is to the design and mapping, not to runtime completion |
| claude-koinon@blackhole (Koinon driver) | agreed with `d33ca6e` against `15ec0ff`, with one finding fixed in `eb178a7`; it drove the changes to `4e324d5` with the Voice pair |
| codex-koinon@blackhole (Koinon observer) | agrees with the contract at `4e324d56e938904c4f4be838603cc35433c75dcd`, vectors verified independently |

The maintainer approved the Koinon contract on the Koinon board as card LINK-693-SIGNED
(revision 10) on 2026-11-15. A change to the contract is mapped here before the Voice side
builds on it.

## Live checks

Koinon #693 chunk 1 (rwcii/koinon `88d3870`, PR #699: actions on the node's own pin), a
scratch daemon on mac-studio, 2026-11-15, against this branch at `ae98f02`:

| Check | Result |
| --- | --- |
| `koinon-voice key enroll`, then `koinon maintainer key add` and `key list`; the same statement again | authorized; `statement_used` |
| Session; `board_list`, `decision_get`, `inbox`, `room_list`, `peers`, `computers`, `changes` without versions | answered in the contract's shapes |
| `koinon-voice board` on a signed row | connected, the node's items and inbox, `ok` true |
| `decision_verdict` with the digest from `decision_get` | recorded, `via` `key:koinon-voice`, `signer` with user, client and key ID; in `decision_history` |
| The same envelope again; `outcome`; `outcome` of a request never sent | the identical answer; `recorded` with it; `unknown_request` with `now` |
| A verdict on text the node no longer shows | `snapshot_changed` |
| Reconciling a write that was never sent | `unknown_request` in the window, the same envelope sent once, recorded |
| `changes` with held versions, a message sent during the wait | `changed` `["inbox"]` |
| `ack`, `send` to a peer; `room_post` to an unknown room | recorded; `invalid_request` |
| A verdict through the dashboard's path (`SignedSource.Verdict`, pending-verdicts file) and the same form again | recorded; a repeat with nothing sent; the file empty |
| `koinon maintainer key remove` | the next call `maintainer_token_expired` and the process exits; a new session `signature_refused` (the authorization was removed); `board` not ok |
| A verdict while a `changes` wait is open | answered after 30 seconds: G21 |

On the chunk-2 commit that runs signed calls at once (rwcii/koinon `59e6478`, branch
`feat/693-signed-chunk2`, G21), with this branch at `71043af`, every check above gave the same
result, and a call made while a `changes` wait was open answered in under a millisecond; a
verdict through the dashboard's path answered without waiting for the open wait.

The forward to paired computers (the rest of chunk 2) and chunk 3 (the Serve route) are
checked when Koinon sends them.
