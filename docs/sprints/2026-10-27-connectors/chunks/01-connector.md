# Chunk 01 — Connector

Read `../decision.md` and `../definition-of-done.md` first.

## Outcome

1. `internal/keychain` holds the Keychain reader from `internal/summarize/keychain.go`, with
   its tests; `internal/summarize` uses it and behaves as before.
2. `internal/koinon` has:
   - an MCP client over a reader and a writer: `initialize`, `tools/call`,
     `resources/subscribe` and `resources/read`, answers matched by ID, notices delivered on a
     channel, a deadline per request, a bound on line size, and Koinon's refusal `code` (and
     `details`) as a typed error;
   - a connector for one host: it builds the local or `ssh` command, writes the secret as the
     first line, initializes, subscribes to inbox and decisions, lists the rooms and subscribes
     to each, reads the waiting items, the inbox and the rooms, and reads a resource again
     after its notice. It makes the liveness read after an idle interval. It publishes each
     state change (connected with its data, or disconnected with a reason and code). It starts
     the process again after an exit, a refusal of the token, a -32603 `daemon_unavailable`,
     junk or a failed liveness read, with a delay that grows to a limit;
   - `Verdict(reference, revision, verdict, note)` on a connector, which returns Koinon's
     answer or its refusal code (`stale_revision` with the current revision, `not_found`).
3. `hosts.toml` loads with the checks in the definition of done.
4. `koinon-voice board [--hosts FILE] [--watch]` connects every host, prints each host's
   state and waiting items (reference, kind, revision, title, asker), and with `--watch`
   prints again on each change until interrupted. It never prints the secret.
5. ADR 001, Decision 2's trade-off on what a running maintainer-mode process reports when its
   session ends is replaced with the behavior in the definition of done (a correction within
   the accepted record: Koinon answered the question that the record left open). README and
   CHANGELOG describe `board` and `hosts.toml`.

## Method

Build the fake Koinon and the fake `ssh` first, from the behavior in the definition of done,
then the client, then the connector. Keep the process start and kill in the existing
`internal/sidecar` pattern (process group, kill on stop). The connector does not acknowledge
the inbox or post; this sprint has no action other than the verdict.

## Tests

The definition of done's Fake Koinon, Client, Connector, SSH, Secret, Configuration and
Keychain tests, and the edge cases for a missing `koinon`, an unreachable destination, two
hosts with the same reference, and a child that does not exit.

## Live check

Live check 1, recorded in `../live-checks.md` with the Koinon version and commands, and with
the scratch daemon's secret not recorded.

## Done

The tests pass in CI on Linux and macOS, live check 1 is recorded, and the gate in the
definition of done passes on the chunk's head.
