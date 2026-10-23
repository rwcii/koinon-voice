# Product Decision Records (PDRs)

PDRs document **how Koinon Voice behaves** — what it does for the maintainer and the agent
sessions it serves. For process rules, the template, naming, and lifecycle, see
[PDR_PROCESS.md](PDR_PROCESS.md).
[WDR 001](../../engineering/wdr/001-decision-records-govern-constraints.md) records why
Koinon Voice keeps these records and how a constraint becomes one.

## How PDRs Relate to ADRs

ADRs document **how the system is built** (architecture, patterns, infrastructure). PDRs
document **how the product behaves**. The two often come in pairs: an ADR decides the
mechanism, a PDR decides the behavior that mechanism serves. Cross-reference via
`related_adrs` / `related_pdrs` in frontmatter.

## PDRs

| Number | Title | Status |
| ------ | ----- | ------ |
| 001    | [Two-Way Voice Bridge](001-two-way-voice-bridge.md) | Draft |
| 002    | [Spoken Maintainer Decisions](002-spoken-maintainer-decisions.md) | Draft |
| 003    | [Listening States and Alerts](003-listening-states-and-alerts.md) | Draft |
| 004    | [Spoken Summaries and Inference](004-spoken-summaries-and-inference.md) | Draft |
