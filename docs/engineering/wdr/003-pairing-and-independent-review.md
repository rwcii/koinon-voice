---
wdr: "003"
title: "Pairing and Independent Review"
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
---

# WDR 003: Pairing and Independent Review

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice adopts Koinon's [WDR 003](https://github.com/rwcii/koinon/blob/e89d37059fb602ce634219ed67ce20294278e35b/docs/engineering/wdr/003-pairing-and-independent-review.md) at e89d370 under WDR 001, Decision 7, with
the decision numbering unchanged so that a citation such as "WDR 003, Decision 1" means the same
in both repositories. It covers the driver and observer pair, trading roles at a committed increment, review by the other agent family of a frozen commit, findings that carry evidence and a triage and one worktree per change.

The record is a draft because its source is: Koinon's record states which of its decisions the
maintainer made and which shipped without a recorded maintainer decision, and that status
carries over.

---

## Table of Contents

1. Problem Context
2. Decision 1: Adopt Koinon WDR 003
3. Conclusion
4. Document History

---

## 1. Problem Context

### The Problem

Koinon Voice uses Koinon's process (WDR 001, Decision 7). The shared skills cite Koinon's
workflow decisions by number, and a few of those decisions name Koinon's own CI jobs, files or
platform code.

### Requirements

1. Each of Koinon WDR 003's decisions has a stated scope here.
2. Each difference from Koinon is recorded.

### Scope

Covers what Koinon WDR 003 covers, for this repository. The provenance of each decision is in
Koinon's record and is not repeated here.

---

## 2. Decision 1: Adopt Koinon WDR 003

### Decision

Koinon WDR 003, Decisions 1 to 5, applies to Koinon Voice unchanged. The reviewer records its verdict as a comment on the pull request that names the full commit, and sets the `peer-review` commit status on that hash (WDR 004).

| Koinon WDR 003 | In Koinon Voice |
| ------------- | --------------- |
| Decision 3: pull requests on rwcii/koinon | Pull requests on rwcii/koinon-voice, under the same single GitHub account |

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Copy the record | Duplicate Koinon's text here | Two copies drift (WDR 001) |
| No local record | Cite Koinon only | The differences would have no home |

### Rationale

1. The maintainer asked for Koinon's process (WDR 001, Decision 7).

---

## Conclusion

Koinon Voice pairs, reviews and isolates work exactly as Koinon does.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-25 | Koinon WDR 003 at e89d370 (rwcii/koinon#487): Decisions 3 to 5 say what to do; Decision 4 is now findings with evidence and a triage |
