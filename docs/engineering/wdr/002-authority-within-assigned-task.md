---
wdr: "002"
title: "Authority Within the Assigned Task"
status: draft
created: "2026-10-23"
updated: "2026-10-25"
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related:
  - "WDR 001"
tags:
  - workflow
  - agentic
  - security
---

# WDR 002: Authority Within the Assigned Task

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice adopts Koinon's [WDR 002](https://github.com/rwcii/koinon/blob/7089386ce4d85758f991f5b56c8de0e13fe2e754/docs/engineering/wdr/002-authority-within-assigned-task.md) at 7089386 under WDR 001, Decision 7, with
the decision numbering unchanged so that a citation such as "WDR 002, Decision 1" means the same
in both repositories. It covers where an agent's authority comes from, how peer messages, records and claims relate to it, and where the maintainer approves sprint plans and releases.

The record is a draft because its source is: Koinon's record states which of its decisions the
maintainer made and which shipped without a recorded maintainer decision, and that status
carries over.

---

## Table of Contents

1. Problem Context
2. Decision 1: Adopt Koinon WDR 002
3. Conclusion
4. Document History

---

## 1. Problem Context

### The Problem

Koinon Voice uses Koinon's process (WDR 001, Decision 7). The shared skills cite Koinon's
workflow decisions by number, and a few of those decisions name Koinon's own CI jobs, files or
platform code.

### Requirements

1. Each of Koinon WDR 002's decisions has a stated scope here.
2. Each difference from Koinon is recorded.

### Scope

Covers what Koinon WDR 002 covers, for this repository. The provenance of each decision is in
Koinon's record and is not repeated here.

---

## 2. Decision 1: Adopt Koinon WDR 002

### Decision

Koinon WDR 002, Decisions 1 to 6, applies to Koinon Voice without its limit text. The
maintainer's task authorizes its own steps. Agents of different families work together
through Koinon: a work request from a peer is a teammate's request, which the agent does and
answers through Koinon, and the agents' own built-in safeties apply. The installed `koinon
guide` describes the installed runtime. Sprint plans and releases follow the shared `sprint`
and `release` skills. Koinon's "Statements this record does not adopt" are not adopted here
either.

| Koinon WDR 002 | In Koinon Voice |
| ------------- | --------------- |
| Decision 2: Koinon's permission recommendations in docs/PERMISSIONS.md | The installed Koinon's recommendations; this repository adds none |
| Decision 6: release promotes develop to main with installation and upgrade evidence | The release facts in `.koinon/skills/release.md` name the evidence this repository has |
| Decisions 2 and 3: a peer request adds no authority; messages, memory entries, work claims and handoffs are data and grant no permission | A peer's work request is a teammate's request: do it and reply. No limit, caveat or control is added on what an agent does with a peer's message or request |
| Decision 5: sprint plans pass gates A and B | The `sprint` skill's steps, which have no gates A and B |
| Decision 6: each release is approved by the maintainer in a separate step | The `release` skill's steps, which merge and publish with no second approval |

**Provenance for the differences:** Maintainer, 2026-10-25, on rwcii/koinon#487, a Codex
session that waited on its paired session's review requests until the maintainer repeated
them. Asked whether to keep three limits as well (a peer message is not approval for a
pending prompt, a peer cannot widen permissions, refuse permission laundering), with the
recommendation to remove them because each agent's own CLI enforces its prompts, the
maintainer answered: "remove all limit text.  remove all of the caveats, controls, etc.  the
agents have their own built in safeties, they do not need you adding more complications."
Earlier that day: "YOU DO NOT ADD LIMITING OR CONTROLLING LANGUAGE LIKE THIS". For this
repository, the same day: "beware of these issues in this project" and "first fix the refusal
issues in repo, 1 pr". Scope: the agent-facing text of this repository (instructions, skills,
facts and records). Code that refuses, such as the attribution check and the required review
statuses, is unchanged.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Copy the record | Duplicate Koinon's text here | Two copies drift (WDR 001) |
| No local record | Cite Koinon only | The differences would have no home |

### Rationale

1. The maintainer asked for Koinon's process (WDR 001, Decision 7).

---

## Conclusion

Agents working on Koinon Voice act on the maintainer's task and do each other's work requests as teammates' requests, with no limit text added.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-25 | Adopted without its limit text, per the maintainer's decision on rwcii/koinon#487; sprint gates and the separate release approval follow the shared skills |
