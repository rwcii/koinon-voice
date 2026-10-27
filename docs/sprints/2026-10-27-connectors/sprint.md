# Sprint 2026-10-27 — Connectors and dashboard — Chunks

Each chunk is one pull request from `develop`, built with the `ship` skill after the chunks it
depends on have merged, with one pull request in flight at a time. Each chunk passes its part
of `definition-of-done.md` and updates the documents for the behavior it changes in the same
pull request.

| Chunk | Outcome | Criteria | Depends on |
| --- | --- | --- | --- |
| [01 Connector](chunks/01-connector.md) | `internal/keychain`, `internal/koinon` (MCP client and connector), `hosts.toml`, and `koinon-voice board`; live check 1. | 1, 2, 3, 4, 7 | — |
| [02 Dashboard](chunks/02-dashboard.md) | `koinon-voice serve`: the loopback page with login, host states, waiting items, updates and verdicts by click; live check 2. | 5, 6, 8 | 01 |

Gate A approved `decision.md` at 197a776. Maintainer, 2026-10-27, approved board item
kv-sprint1-gate-a, "Koinon Voice: Gate A. Approve the connectors sprint decision?", which
held the decision's text and named its two additions to option A (criterion 7's
configuration file, and the Build constraint for platforms without the Keychain), with no
note.
