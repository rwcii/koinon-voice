# Sprint 2026-10-27 — Connectors and dashboard — Live checks

The live checks of `definition-of-done.md`. No secret is recorded.

## 1. Scratch daemon (chunk 01)

2026-10-27, on the maintainer's Mac, by Claude. Koinon v0.5.0 (8abe1b9).

A scratch daemon ran with `koinon serve --state-dir` on a temporary directory and
`127.0.0.1:47914`, with a secret from `koinon maintainer secret generate` and a synthetic
`codex` session registered through `/v1/sessions/register`, which created 105 decision items
(`p-1` to `p-105`, each with the context `ctx N`). One `board_list` page holds at most 100.

A driver program, built from this branch and not committed, ran one `internal/koinon`
connector against that daemon (`state_dir` and `address` set, the secret given in memory)
and logged each state. A shell read the driver's output, restarted the daemon when the
driver asked, and sent a message to the maintainer. The log, with seconds since the start:

| Time | Event |
| --- | --- |
| 0.13 s | Connected: 105 items, all 105 with their context from `decision_get`; counts `waiting` 105 |
| 0.63 s | `koinon decision edit --context` on `p-50` (context only): the new context shown |
| about 2.5 s | Daemon stopped and started again |
| 3.69 s | Disconnected, code `maintainer_token_expired` (the process exited while subscribed) |
| 4.84 s | Connected again, second start, 105 items |
| 4.86 s | Verdict on `p-1` at revision 1: recorded with `via: mcp:koinon-voice` |
| 4.86 s | Verdict on `p-50` at revision 1, after its edit: refused, `stale_revision` |
| 4.91 s | `p-1` left the board: 104 items |
| 5.33 s | A message to the maintainer: one inbox message shown |

The scratch daemon and its directory were removed afterwards.

The same day, `koinon-voice board --hosts FILE`, built from this branch, with one host
`local` and `secret = "maintainer-local"`, read the secret from the maintainer's Keychain and
connected to the maintainer's running daemon. It printed
`local: connected, 0 waiting, 0 open fyi, 2 in progress, 1 inbox messages, 0 rooms` and exited
with status 0. It only read; it gave no verdict.

## 2. The maintainer's daemon (chunk 02)

Not yet run.
