---
adr: "003"
title: "Inference Adapters"
status: draft
created: "2026-10-23"
updated: "2026-10-23"
parent_adr: ""
authors:
  - "Robert Capps"
reviewers: []
supersedes: ""
superseded_by: ""
related:
  - "002"
tags:
  - architecture
  - integration
  - security
---

# ADR 003: Inference Adapters

**Date:** 2026-10-23
**Status:** Draft
**Authors:** Robert Capps

---

## Executive Summary

Text summarization (PDR 004) goes through three adapters behind one interface: Apple's
on-device model through the Swift helper (ADR 002); Claude through Anthropic's official Go
SDK; and an OpenAI-compatible HTTP client that covers OpenAI, OpenRouter and local model
servers (Ollama, LM Studio, MLX). Engines are configuration rows; API keys live in the macOS
Keychain; each engine is marked local or external.

This record is a proposal from the design session of 2026-10-23.

---

## Table of Contents

1. Problem Context
2. Decision 1: Three adapters, engines as configuration
3. Decision 2: Keys in the Keychain; external engines labeled
4. Conclusion
5. Document History

---

## 1. Problem Context

### The Problem

The maintainer wants to compare local models, Claude, OpenAI and OpenRouter (PDR 004).
A client per vendor multiplies code; one lowest-common-denominator client drops features that
matter for cost and quality.

### Requirements

1. Add or change a model without code.
2. Keep local open-weight models on the same path as hosted ones.
3. Never put a key in the repository or a config file in Git.
4. Know, for every summary, whether text left the machine and to whom.

### Scope

Covers summarization adapters. Speech engines are ADR 002. When external inference is allowed
is PDR 004, Decision 2.

---

## 2. Decision 1: Three adapters, engines as configuration

### Decision

The summarizer has three adapters: **Apple** (Foundation Models through the Swift helper),
**Anthropic** (the official Go SDK, keeping prompt caching, effort and refusal details), and
**OpenAI-compatible** (one HTTP client with a configurable base URL, for OpenAI, OpenRouter and
local servers). An engine is a configuration row: adapter, base URL, model, and local or
external.

**Provenance:** Maintainer, 2026-10-23: "for engines, we could also look at openrouter or
openai for models too." The adapter split is a proposal.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| A client per vendor | Separate code for each provider | Duplicates the same request shape |
| OpenAI-compatible for everything | Reach Claude through a compatibility layer | Drops prompt caching, effort and refusal details |

### Rationale

1. Two HTTP adapters cover every hosted and local server; Apple's model needs Swift.

---

## 3. Decision 2: Keys in the Keychain; external engines labeled

### Decision

API keys for Anthropic, OpenAI and OpenRouter are read from the macOS Keychain. Every summary
records its engine, and an external engine's summary is labeled on screen and in speech with
its provider (PDR 004, Decision 1). OpenRouter is used with its account settings that exclude
hosts which retain prompts or train on them, recorded with each result.

### Alternatives Considered

| Alternative | Description | Why Rejected |
| ----------- | ----------- | ------------ |
| Environment variables | Keys in the shell environment | Inherited by every child process, including engine helpers |

### Rationale

1. The Keychain scopes keys to the maintainer's login and keeps them off disk in plain text.

---

## Conclusion

Three adapters cover Apple's model, Claude and every OpenAI-compatible server, with engines as
configuration, keys in the Keychain and external use labeled.

---

## Document History

| Date       | Change           |
| ---------- | ---------------- |
| 2026-10-23 | Initial creation |
