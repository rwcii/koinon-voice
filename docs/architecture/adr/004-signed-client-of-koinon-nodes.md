---
adr: "004"
title: "Signed Client of Koinon Nodes"
status: accepted
created: "2026-11-14"
updated: "2026-11-16"
parent_adr: ""
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related:
  - "001"
tags:
  - architecture
  - integration
  - security
  - auth
---

# ADR 004: Signed Client of Koinon Nodes

**Date:** 2026-11-14
**Status:** Accepted
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice reaches every paired Koinon computer through one connection to its own computer's
Koinon daemon, as a client that signs the maintainer's actions with a key pair per user and
install (rwcii/koinon#693). It contains only the parts that talk to full Koinon nodes and never runs
Koinon itself. The per-host SSH connectors of ADR 001, Decision 2 stayed until Koinon shipped
signed maintainer actions over links and Koinon Voice's sprint for them merged; #36 removes
them.

Decisions 1 and 2 are the maintainer's. Decision 3, how the client signs, connects and
registers its key, was settled with the Koinon pair in
`docs/sprints/2026-11-14-signed-client/decision.md` (#36) and approved by the maintainer at
that sprint's Gate A (kv-36-gate-a).

---

## Table of Contents

1. Problem Context
2. Decision 1: One connection through the links, SSH until signed actions ship (kv-693-ssh)
3. Decision 2: A client of full nodes, which never runs Koinon
4. Decision 3: Signed envelopes, one key per user and install, two ways to a node (kv-36-gate-a)
5. Conclusion
6. Document History

---

## 1. Problem Context

### The Problem

ADR 001 gives Koinon Voice one connector per Koinon daemon, through `ssh HOST` for a remote
one, with one maintainer secret per host. Koinon now pairs the maintainer's computers (Koinon
PDR 011), and #693 carries maintainer traffic over those links, signed by a per-user key.

### Requirements

1. Reach every paired computer's board, inbox and rooms as the maintainer (PDR 001, Decision 1;
   PDR 002, Decision 1).
2. Sign each maintainer action with the key of one user and install, and record that user on
   verdicts (#693).
3. Build CGO-free for the same four targets as Koinon (WDR 004).

### Scope

Covers where Koinon Voice connects and what it runs. The Koinon side is Koinon's to decide on
#693. The requirements that Koinon Voice puts on it are in the sprint decision named above.

---

## 2. Decision 1: One connection through the links, SSH until signed actions ship (kv-693-ssh)

- **Decided by:** Robert Capps. **Date:** 2026-11-14, board kv-693-ssh, revision 1, approved.
- **Context:** #693 records the maintainer's words "yes, carry maintainer traffic over the
  links" and the per-user key pair; its Follow-on section says that Koinon Voice replaces its
  per-host SSH maintainer connectors with one connection to its own computer's daemon. The
  maintainer's quoted words do not name SSH, and PDR 001, Decision 1 holds the maintainer's
  direction of 2026-10-23 for "a remote koinon session over SSH", so the card asked. Options:
  replace SSH and keep it until the signed path ships (recommended); keep SSH beside the links
  for computers that are not paired; remove SSH at once.
- **Decision:** Koinon Voice connects only to its own computer's Koinon daemon and reaches
  paired computers through the links. The SSH connectors of ADR 001, Decision 2 stay until
  Koinon ships signed maintainer actions over links and Koinon Voice's sprint for them merges;
  then the SSH connector and the `ssh` entries of `hosts.toml` are removed.
- **Done:** #36 removes the SSH connector, the secret connection of ADR 001, Decision 3 and the
  `ssh` and `secret` entries of `hosts.toml`. `hosts.toml` takes only signed node rows.
- **Reason and scope:** one connection and one key replace a secret per host. A computer that
  is not paired is then not reachable from Koinon Voice.
- **Review when:** a computer that cannot be paired must be reached from Koinon Voice.

### When the removal merges (kv-36-ssh-merge)

- **Decided by:** Robert Capps. **Date:** 2026-11-16, board kv-36-ssh-merge, revision 1,
  approved.
- **Context:** the maintainer asked to finish #36 ("take on 36. finish it off."). The removal
  was ready while #693 was on Koinon `develop` only (latest release v0.11.2), and the
  maintainer's `hosts.toml` still had a secret row and an `ssh` row. Options: merge after a
  Koinon release with #693 is installed and `hosts.toml` is moved to signed rows (recommended);
  merge at once, with a Koinon build from `develop` on each computer until the release.
- **Decision:** the removal merges after a Koinon release with #693 is installed and the
  maintainer's `hosts.toml` is moved to signed rows, and Koinon Voice works on them. A task
  card gives the maintainer the commands for that move when the release exists.
- **Reason and scope:** Decision 1 above; a merge before the release stops the maintainer's
  board and dashboard, because v0.11.2 has no signed actions. The Koinon release is not
  decided here.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| SSH beside the links | Keep SSH for computers that are not paired | Two connector paths to test and keep; not chosen on kv-693-ssh |
| Remove SSH at once | Drop SSH before Koinon ships | Koinon Voice has no remote path until then; not chosen on kv-693-ssh |
| Merge the removal at once | Merge before a Koinon release, with Koinon from `develop` on each computer | Not chosen on kv-36-ssh-merge |

---

## 3. Decision 2: A client of full nodes, which never runs Koinon

- **Decided by:** Robert Capps. **Date:** 2026-11-14, terminal.
- **Context:** with the kv-693-ssh verdict, the maintainer wrote: "I question if we should have
  a separate local install for koinon to run voice. can't voice include the koinon instance?"
  Board kv-693-bundle then offered three options: bundle the `koinon` binary and start it when
  no daemon runs (recommended); a separate Koinon install; embed Koinon in Koinon Voice's
  process. Before a verdict, the maintainer answered in the terminal: "I would rather voice just
  contain the parts needed to communicate with full koinon nodes. I don't want it to run koinon
  itself." The card was withdrawn with that answer.
- **Decision:** Koinon Voice contains only the parts that talk to full Koinon nodes. It does
  not bundle, start or embed a Koinon daemon.
- **Reason and scope:** the maintainer's words. How Koinon Voice reaches a full node from a
  computer where no Koinon daemon runs is not decided; it is settled with the Koinon pair on
  #693 and #36.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Bundle the `koinon` binary | Start it when no daemon runs; use the running one otherwise | The maintainer does not want Koinon Voice to run Koinon |
| Embed Koinon | Koinon in Koinon Voice's process | As above; Koinon's packages are also all under `internal/`, which another module cannot import |

---

## 4. Decision 3: Signed envelopes, one key per user and install, two ways to a node (kv-36-gate-a)

- **Decided by:** Robert Capps. **Date:** 2026-11-16, board kv-36-gate-a, revision 1, approved.
- **Context:** Decisions 1 and 2 left how the client authenticates, reaches a node from a
  computer without one, and registers its key to the sprint record
  `docs/sprints/2026-11-14-signed-client/decision.md`, settled with the Koinon pair against the
  Koinon contract that the maintainer approved as LINK-693-SIGNED, revision 10. Gate A asked the
  maintainer to approve that record and merge PR #37; the other option was to give feedback on
  it first.
- **Decision:**
  1. Each install has an Ed25519 key pair for its user. The private key is in the macOS
     Keychain (R10). `koinon-voice key create` makes it and `koinon-voice key show` prints the
     public key.
  2. Each maintainer action, reads included, leaves as a signed `koinon-maintainer-1` envelope.
     The signature covers the action and its arguments, the target computer, the client, the
     key ID, a request ID and the creation time (R9, R11).
  3. Where a Koinon daemon runs on Koinon Voice's computer, Koinon Voice connects to it with
     `koinon mcp --maintainer --signed`, opened by a signed `session` (R25). Where none runs, it
     uses another computer's Remote address on the route `/v1/maintainer/signed`, which takes
     only signed envelopes after a signed session (R24).
  4. The key is registered on each computer by that computer's own maintainer:
     `koinon-voice key enroll --target PIN` writes a signed enrollment statement, and
     `koinon maintainer key add` on that computer accepts it (R26).
  5. A computer is named by its pinned link key. Koinon Voice keys its state by that pin and
     shows the computer's name (R12).
  6. A write is signed once and sent again only as the same envelope. When its answer is lost,
     Koinon Voice asks for its `outcome` under the same request (R13 to R15).
  7. Koinon Voice keeps one signed `changes` wait for each computer it shows, and reads a topic
     again when its version changes (R17, R21).
- **Reason and scope:** the maintainer approved the sprint record, which maps these points to
  the Koinon contract. The sprint record holds each requirement with its evidence and the live
  checks.
- **Review when:** Koinon changes the `koinon-maintainer-1` contract in a section that the
  sprint record maps.

---

## Conclusion

Koinon Voice becomes a signed client of full Koinon nodes, reaching paired computers through
the links, and it never runs Koinon. #36 removes the SSH connectors and the secret per host,
as Decision 1 set out. Decisions 1 and 2 are the maintainer's of 2026-11-14;
Decision 3, the mechanism, follows the Koinon design on #693 and was approved at Gate A on
2026-11-16.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-11-14 | Initial creation, with Decisions 1 (kv-693-ssh) and 2 (the maintainer's terminal answer) |
| 2026-11-16 | Decision 3 from the sprint record approved at Gate A (kv-36-gate-a); status accepted (#38) |
| 2026-11-16 | Decision 1 done: the SSH connector and the secret connection removed (#36) |
| 2026-11-16 | Decision 1: the removal merges after a Koinon release with #693 is installed (kv-36-ssh-merge) |
