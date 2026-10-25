---
wdr: "001"
title: "Decision Records Govern Constraints"
status: accepted
created: "2026-10-23"
updated: "2026-10-25"
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related:
  - "WDR 002"
  - "WDR 003"
  - "WDR 004"
  - "WDR 005"
  - "WDR 006"
tags:
  - workflow
  - agentic
  - enforcement
---

# WDR 001: Decision Records Govern Constraints

**Date:** 2026-10-23
**Status:** Accepted
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice is a peer repository of [rwcii/koinon](https://github.com/rwcii/koinon), built by
the same agent team under the same development process. This record adopts Koinon's record
system: Koinon's [WDR 001](https://github.com/rwcii/koinon/blob/e89d37059fb602ce634219ed67ce20294278e35b/docs/engineering/wdr/001-decision-records-govern-constraints.md)
at e89d370, Decisions 1 to 6, applies here with the same numbering. Decision 7 records the
maintainer's choice to run this repository as Koinon's peer.

A decision is stated once, in its record, as "Koinon Voice currently does X because Y". Other
text cites the record instead of restating it as a general rule.

---

## Table of Contents

1. Problem Context
2. Decisions 1 to 6: Koinon's record system
3. Decision 7: A peer repository of Koinon
4. Conclusion
5. Document History

---

## 1. Problem Context

### The Problem

Koinon is written almost entirely by agents, and its WDR 001 records how unsourced constraints
spread through skills, plans and code until later agents applied rules the maintainer never set.
Koinon Voice starts with the same agents and the same shared skills, which cite records by type
and number. Without its own records, those citations would point at nothing, and the process
the maintainer asked for would exist only as copied prose.

### Requirements

1. The process, structure, language and record format match Koinon's.
2. Each significant constraint has one record with its source, scope and status.
3. The shared skills' citations (WDR 001 to WDR 006) resolve in this repository.
4. A deliberate difference from Koinon is visible in a record, not silent.

### Scope

Covers the record system and this repository's relation to Koinon. Product behavior is a PDR
and architecture an ADR; both series start empty. The other workflow records are WDR 002 to
WDR 006.

---

## 2. Decisions 1 to 6: Koinon's record system

### Decision

Koinon WDR 001 at e89d370, Decisions 1 to 6, applies to Koinon Voice with these substitutions:

| Koinon WDR 001 | In Koinon Voice |
| -------------- | --------------- |
| Decision 1: vendor the ADR, PDR and WDR processes from rwcii/engineering-standards at 425b2ef | The same vendored guides, copied from Koinon at e89d370 with "Koinon" read as "Koinon Voice" |
| Decision 3: carriers (PROTOCOL.md, USAGE, INSTALL, `koinon guide`) cite records | This repository's README, CONTRIBUTING, skills facts and future docs cite records |
| Decision 6: `develop` requires `refusal-review` | The same status, set by `scripts/setup-repo.sh` (WDR 004) |

Koinon's records are adopted without their limit text: where an adopted Koinon decision states
a limit, caveat or control on what an agent does that the maintainer did not ask for, it does
not apply here, and the rules the maintainer gave in words stay (WDR 002 records the
maintainer's decision of 2026-10-25).

Decisions 2 (significance first, route by subject) and 5 (a record covers the case it
answered) apply unchanged. Decision 4 is deprecated in Koinon's record (rwcii/koinon#487). Koinon's constraint inventory (C001 to C086) and its
provenance belong to Koinon; this repository cites them through Koinon's records and adds none.

A later change to Koinon's WDR 001 does not change this record. Koinon Voice takes it by a
change to this record that names the new Koinon commit.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Copy Koinon's WDRs in full | Duplicate each record's text and provenance | Two copies of the same decision drift; Koinon's provenance is Koinon's history |
| Cite Koinon without local records | Point AGENTS.md at Koinon's WDRs | The shared skills cite WDR numbers that would not resolve here, and differences have no home |
| Track Koinon's head | Follow whatever Koinon's records say today | A Koinon change would change this repository's rules without a change here |

### Rationale

1. One statement per decision: Koinon's text stays the source; this record states the scope.
2. Pinning a commit makes each adoption of a later Koinon change visible as a change here.

---

## 3. Decision 7: A peer repository of Koinon

### Decision

Koinon Voice is a peer repository of rwcii/koinon, published as rwcii/koinon-voice. It uses
Koinon's development process (WDR 002 to WDR 006), its repository structure (`cmd/`,
`internal/`, tests beside their packages, `agents/skills/` with discovery symlinks,
`.koinon/skills/` facts, `docs/` record series and `docs/sprints/`), its language (Go, one
module) and its decision record format. The shared skills are byte copies of Koinon's
`agents/skills/*/SKILL.md` and `agents/skills/AGENTS.md` at e89d370, the skills of rwcii/koinon#491 and #492
that say what to do (rwcii/koinon#487); this repository's facts are in `.koinon/skills/`.

**Provenance:** Maintainer, 2026-10-23, in this repository's first session, raised by the
maintainer, not in answer to a question: "connect to koinon and then let's set this directory
up as a peer repo to rwcii/koinon - let's use the same dev process, same structure, langauges,
etc.  we use the same decsision records format, etc.  this will land on rwcii as
rwcii/koinon-voice".

- Questions then asked, with the options offered: what the product is (a voice interface to
  Koinon; a voice agent family that joins Koinon as a peer; leave it blank), and whether the
  repository is public like Koinon or private. Answers: "Voice interface to Koinon" and
  "Public (Recommended)".
- Reason: the maintainer's message above. Scope: this repository's process, structure,
  language and record format, and its visibility. It decides no product behavior beyond the
  one-line description; product decisions are PDRs.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Package inside rwcii/koinon | Build voice as a Koinon package | The maintainer asked for a peer repository |
| Independent process | Choose a process for this repository alone | The maintainer asked for the same process |

### Rationale

1. The maintainer asked for it.
2. The same agents move between both repositories; one process means one set of habits.

---

## Conclusion

Koinon Voice runs Koinon's record system and process, pinned to Koinon at e89d370, with each
difference stated in the record that adopts it. The peer-repository choice, the product line
and public visibility are the maintainer's, from 2026-10-23.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-25 | Skills taken from Koinon at 0cf3979 (rwcii/koinon#491); Koinon's records adopted without their limit text (rwcii/koinon#487) |
| 2026-10-25 | Koinon's records, process guides and skills taken at e89d370, after rwcii/koinon#492 and #494; Decision 4 deprecated there |
