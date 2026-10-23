---
wdr: "006"
title: "Native Integration Evidence"
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
  - testing
---

# WDR 006: Native Integration Evidence

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice adopts Koinon's [WDR 006](https://github.com/rwcii/koinon/blob/7089386ce4d85758f991f5b56c8de0e13fe2e754/docs/engineering/wdr/006-native-integration-evidence.md) at 7089386 under WDR 001, Decision 7, with
the decision numbering unchanged so that a citation such as "WDR 006, Decision 1" means the same
in both repositories. It covers the isolated ordinary test suite, guarded native tests on disposable CI runners, and named live checks of an installed build.

The record is a draft because its source is: Koinon's record states which of its decisions the
maintainer made and which shipped without a recorded maintainer decision, and that status
carries over.

---

## Table of Contents

1. Problem Context
2. Decision 1: Adopt Koinon WDR 006
3. Conclusion
4. Document History

---

## 1. Problem Context

### The Problem

Koinon Voice uses Koinon's process (WDR 001, Decision 7). The shared skills cite Koinon's
workflow decisions by number, and a few of those decisions name Koinon's own CI jobs, files or
platform code.

### Requirements

1. Each of Koinon WDR 006's decisions has a stated scope here.
2. Each difference from Koinon is recorded.

### Scope

Covers what Koinon WDR 006 covers, for this repository. The provenance of each decision is in
Koinon's record and is not repeated here.

---

## 2. Decision 1: Adopt Koinon WDR 006

### Decision

Koinon WDR 006, Decisions 2 and 3, applies to Koinon Voice: the ordinary suite is isolated from the person's environment (synthetic peers, temporary state and configuration, ephemeral loopback ports, private tmux servers), and evidence CI cannot produce is gathered as named live checks that the maintainer runs or assigns. Decision 1 applies when Koinon Voice first drives a real service manager or other host facility from its tests.

| Koinon WDR 006 | In Koinon Voice |
| ------------- | --------------- |
| Decision 1: `internal/nativetest` and `native-go-lifecycle.yml` | None yet; added with the first need, by a change to this record |
| Decision 3: the #283 live checks on a Mac | Koinon Voice has none yet; each is named in its issue or sprint definition of done |

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Copy the record | Duplicate Koinon's text here | Two copies drift (WDR 001) |
| No local record | Cite Koinon only | The differences would have no home |

### Rationale

1. The maintainer asked for Koinon's process (WDR 001, Decision 7).

---

## Conclusion

Koinon Voice keeps its ordinary tests away from the person's environment and gathers evidence CI cannot produce as named live checks, as Koinon does.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
