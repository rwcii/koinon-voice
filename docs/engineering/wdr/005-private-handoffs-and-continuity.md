---
wdr: "005"
title: "Private Handoffs and Continuity"
status: draft
created: "2026-10-23"
updated: "2026-10-23"
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
  - continuity
---

# WDR 005: Private Handoffs and Continuity

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice adopts Koinon's [WDR 005](https://github.com/rwcii/koinon/blob/7089386ce4d85758f991f5b56c8de0e13fe2e754/docs/engineering/wdr/005-private-handoffs-and-continuity.md) at 7089386 under WDR 001, Decision 7, with
the decision numbering unchanged so that a citation such as "WDR 005, Decision 1" means the same
in both repositories. It covers where session handoffs live and whose handoff a pickup reads.

The record is a draft because its source is: Koinon's record states which of its decisions the
maintainer made and which shipped without a recorded maintainer decision, and that status
carries over.

---

## Table of Contents

1. Problem Context
2. Decision 1: Adopt Koinon WDR 005
3. Conclusion
4. Document History

---

## 1. Problem Context

### The Problem

Koinon Voice uses Koinon's process (WDR 001, Decision 7). The shared skills cite Koinon's
workflow decisions by number, and a few of those decisions name Koinon's own CI jobs, files or
platform code.

### Requirements

1. Each of Koinon WDR 005's decisions has a stated scope here.
2. Each difference from Koinon is recorded.

### Scope

Covers what Koinon WDR 005 covers, for this repository. The provenance of each decision is in
Koinon's record and is not repeated here.

---

## 2. Decision 1: Adopt Koinon WDR 005

### Decision

Koinon WDR 005, Decisions 1 and 2, applies to Koinon Voice unchanged: each agent writes its handoffs to the Git-ignored `_handoff/<agent>/` of this repository's main checkout and never commits them, and `pickup` reads the caller's own handoff unless the maintainer names another.

| Koinon WDR 005 | In Koinon Voice |
| ------------- | --------------- |
| Decision 1: the main checkout of rwcii/koinon | The main checkout of rwcii/koinon-voice; handoffs of the two repositories stay separate |

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Copy the record | Duplicate Koinon's text here | Two copies drift (WDR 001) |
| No local record | Cite Koinon only | The differences would have no home |

### Rationale

1. The maintainer asked for Koinon's process (WDR 001, Decision 7).

---

## Conclusion

Handoffs stay private, per agent and per repository, as in Koinon.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
