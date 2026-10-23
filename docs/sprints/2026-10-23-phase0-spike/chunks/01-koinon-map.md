# Chunk 01 — Koinon map

Read `../decision.md` and `../definition-of-done.md` first.

## Outcome

`docs/sprints/2026-10-23-phase0-spike/koinon-map.md` records, for Koinon v0.4.1 (commit
7089386), each thing Koinon Voice needs (ADR 001, Decision 3) and how a non-browser client
can do it today, or that it cannot:

1. Read waiting decision items with every field, messages to the maintainer, room posts, and
   peers with their state.
2. Learn of a change: a new waiting item, a message to the maintainer, a room post.
3. Post to a room and send to a peer as the maintainer.
4. Record a verdict on a decision item as the maintainer.
5. Authenticate as the maintainer without a browser login, and what the dashboard login
   cookie, the per-port cookie name and the loopback address mean for a client over an SSH
   forward.

Each entry names the source (PROTOCOL.md section, dashboard route, MCP tool or command) and
the result of trying it. The chunk posts a short summary of the gaps as a comment on
rwcii/koinon#464 and links it from the map.

## Method

Read PROTOCOL.md, docs/USAGE.md and the dashboard handlers at 7089386. Try each path against a
scratch daemon started with `koinon serve --state-dir` on a temporary directory and an
ephemeral loopback port, never the maintainer's running daemon (definition of done, live
check 6). Try the SSH case with a local forward to that scratch daemon on the same machine.

## Tests

None in the suite; this chunk changes no code.

## Done

Every item above has a recorded source and result, and the #464 comment links the map.
