# Chunk 02 — Dashboard

Read `../decision.md` and `../definition-of-done.md` first. Chunk 01 must be merged.

## Outcome

1. `koinon-voice serve [--hosts FILE] [--listen 127.0.0.1:PORT] [--no-open]` starts a
   connector for each host and serves the dashboard on a loopback address (default
   `127.0.0.1:47680`). It refuses an address that is not loopback.
2. **Login.** As Koinon's dashboard does (Phase 0 `koinon-map.md`): `serve` prints a one-time
   login link and opens it unless `--no-open`. The token works once, within 60 seconds, and
   sets an HttpOnly, SameSite=Strict cookie that lasts while `serve` runs. Every request
   needs the Host to be the listen address; every POST needs `Origin` equal to `http://HOST`
   and the page's CSRF token. A new link comes from restarting `serve`.
3. **Page.** One HTML page, with no external resources, that shows each host by label with
   its state (connected, or disconnected with the reason and code) and its waiting items:
   title, kind, asker, revision, context and recommendation, as escaped text. Inbox messages
   to the maintainer are listed per host, read only.
4. **Updates.** A server-sent event stream tells the page when a host's state or data changes;
   the page fetches the data again. No item text goes through the event stream.
5. **Verdict.** Each decision item has approve and decline, each fyi item dismiss, with a note
   field. The form posts the host, reference and shown revision. On `stale_revision` or
   `not_found`, the page shows the item as it is now, or that it has left the board, with the
   message, and gives no verdict.
6. README and CHANGELOG describe `serve`, the login and the page.

## Method

Use `net/http`, `html/template` and `embed` from the standard library. Keep the HTTP handlers
apart from the connectors behind a small interface, so the tests use fake connectors.

## Tests

The definition of done's Dashboard and Secret tests, and the edge cases for a non-loopback
listen address and an item that leaves the board.

## Live check

Live check 2. The driver creates an ACTION FOR YOU item on the board with the commands and
steps. The maintainer runs `serve` against the local daemon and dismisses a synthetic fyi item
by click, and the driver records the `decision_history` entry with `via: mcp:koinon-voice` in
`../live-checks.md`.

## Done

The tests pass in CI on Linux and macOS, live check 2 is recorded, the gate in the definition
of done passes on the chunk's head, and the sprint's criteria 1 to 8 are met.
