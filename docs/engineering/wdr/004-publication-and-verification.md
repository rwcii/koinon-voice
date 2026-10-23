---
wdr: "004"
title: "Publication and Verification"
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
  - ci
  - enforcement
---

# WDR 004: Publication and Verification

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Koinon Voice adopts Koinon's [WDR 004](https://github.com/rwcii/koinon/blob/7089386ce4d85758f991f5b56c8de0e13fe2e754/docs/engineering/wdr/004-publication-and-verification.md) at 7089386 under WDR 001, Decision 7, with
the decision numbering unchanged so that a citation such as "WDR 004, Decision 1" means the same
in both repositories. It covers the branch model, one open pull request per target branch on a head current with develop, required checks on Linux and macOS, auto-merge and signed, human-authored commits.

The record is a draft because its source is: Koinon's record states which of its decisions the
maintainer made and which shipped without a recorded maintainer decision, and that status
carries over.

---

## Table of Contents

1. Problem Context
2. Decision 1: Adopt Koinon WDR 004
3. Conclusion
4. Document History

---

## 1. Problem Context

### The Problem

Koinon Voice uses Koinon's process (WDR 001, Decision 7). The shared skills cite Koinon's
workflow decisions by number, and a few of those decisions name Koinon's own CI jobs, files or
platform code.

### Requirements

1. Each of Koinon WDR 004's decisions has a stated scope here.
2. Each difference from Koinon is recorded.

### Scope

Covers what Koinon WDR 004 covers, for this repository. The provenance of each decision is in
Koinon's record and is not repeated here.

---

## 2. Decision 1: Adopt Koinon WDR 004

### Decision

Koinon WDR 004, Decisions 1 to 5, applies to Koinon Voice. Branches are `feature/*`, `fix/*` or `chore/*` from `develop`, squashed into `develop` and promoted to `main` by a merge pull request. One pull request is open per target branch. Pull requests into `develop` set squash auto-merge. Commits are signed (`git commit -S -s`) under the maintainer's GitHub identity and no-reply address, with no co-author trailer, tool signature or model name, and Git prose calls Robert "the maintainer". The checks differ from Koinon's as below.

| Koinon WDR 004 | In Koinon Voice |
| ------------- | --------------- |
| Decision 3: required checks include the native service lifecycle jobs | Koinon Voice installs no service yet. `develop` requires `go (ubuntu-latest)`, `go (macos-latest)`, `attribution`, `peer-review` and `refusal-review`; `main` requires the two `go` checks and `attribution`. A native job is added by a change to this record when the product first needs one |
| Decision 3: the `go` job runs vet, gofmt, the race suite and four `CGO_ENABLED=0` builds of `./cmd/koinon` | The same steps for `./cmd/koinon-voice` |
| Decision 3: operating-system differences in `internal/platform` | The same, once the first one exists |
| Enforcement: `scripts/setup-repo.sh` and `.githooks/pre-commit` | The same files, with this repository's required checks |
| Decision 5, Enforcement: practice only | Also enforced. `scripts/check-attribution.sh` refuses a co-author trailer, a tool signature, a Claude session link or an Anthropic no-reply address in a message, and a commit identity naming Claude or Anthropic. The `.githooks/commit-msg` hook runs it on each local commit; the required `attribution` job runs it on each pull request's title, body and commits, since the squash commit takes the title and body. The project `.claude/settings.json` turns off Claude Code's own commit and pull request attribution and session link. Signing stays practice: `commit.gpgsign` in each clone |

**Provenance for the attribution enforcement:** Maintainer, 2026-10-23, raised by the
maintainer before the first sprint plan: "we sign all commits.  make sure the correct configs
are set for claude to supress Co-authored by, made with, and claude session link spam
injection in to the repo.  the claude user may not check code in at any point.  if you need
to modify the config bow, before we begin, do so." Reason: Koinon WDR 004, Decision 5 was
practice only, and agent tools add these lines by default. Scope: commit and pull request
text and commit identities in this repository; it does not restrict which agents work here.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Copy the record | Duplicate Koinon's text here | Two copies drift (WDR 001) |
| No local record | Cite Koinon only | The differences would have no home |

### Rationale

1. The maintainer asked for Koinon's process (WDR 001, Decision 7).

---

## Conclusion

Koinon Voice publishes like Koinon: one squash pull request at a time into `develop`, gated by Linux and macOS checks and commit-bound `peer-review` and `refusal-review` statuses, with auto-merge set and signed, human-authored commits.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
| 2026-10-23 | Attribution enforcement: hook, required check and project settings |
