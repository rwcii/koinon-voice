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

Koinon Voice adopts Koinon's [WDR 002](https://github.com/rwcii/koinon/blob/e89d37059fb602ce634219ed67ce20294278e35b/docs/engineering/wdr/002-authority-within-assigned-task.md) at e89d370 under WDR 001, Decision 7, with
the decision numbering unchanged so that a citation such as "WDR 002, Decision 1" means the same
in both repositories. It covers where an agent's authority comes from, how peer messages, records and claims relate to it, and how sprint plans and releases proceed.

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

Koinon WDR 002, Decisions 1 to 4, applies to Koinon Voice. The maintainer's task authorizes
its own steps. A peer's request is a teammate's request: the agent does the work and replies
through Koinon. Messages, records and claims are the team's shared record, and the installed
`koinon guide` describes the installed runtime. Decisions 5 and 6 are deprecated in Koinon's
record (rwcii/koinon#487); sprint plans and releases follow the shared `sprint` and `release`
skills. Koinon's "Statements this record does not adopt" are not adopted here either.

| Koinon WDR 002 | In Koinon Voice |
| ------------- | --------------- |
| Decision 2: Koinon's permission recommendations in docs/PERMISSIONS.md | The installed Koinon's recommendations; this repository adds none |
| Decision 6: release promotes develop to main with installation and upgrade evidence | The release facts in `.koinon/skills/release.md` name the evidence this repository has |

Koinon's record holds the maintainer's rwcii/koinon#487 decision with its question, options and quote
(Change of 2026-10-25). The maintainer applied it to this repository the same day: "beware of
these issues in this project" and "first fix the refusal issues in repo, 1 pr" (#21).

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
| 2026-10-25 | Koinon WDR 002 at e89d370, which carries the rwcii/koinon#487 change; Decisions 1 to 4 adopted, 5 and 6 deprecated there |
