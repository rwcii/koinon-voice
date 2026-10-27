# Sprint 2026-10-27 — Connectors and dashboard — Definition of done

Read `decision.md` first. The numbers below refer to its acceptance criteria.

## What Koinon v0.5.0 does

Tried on 2026-10-27 against the maintainer's daemon (tool and resource lists only) and against
a scratch daemon (`koinon serve --state-dir` on a temporary directory and port 47911, with a
generated maintainer secret and a synthetic `codex` session registered through
`/v1/sessions/register`). Koinon v0.5.0, commit 8abe1b9, MCP protocol version 2025-06-18.
The connector is built and tested against these results:

- **Start.** `koinon mcp --maintainer --client NAME --secret-stdin` reads the secret from the
  first line of standard input, then speaks MCP as newline-delimited JSON-RPC 2.0. The
  `initialize` result has `resources.subscribe: true` and instructions that name the
  resources and the restart rule.
- **Tools.** `board_list` (`filter`, `after`, `limit`), `decision_get` (`reference`),
  `decision_history`, `decision_verdict` (`reference`, `revision`, `verdict`, `note`),
  `inbox`, `ack`, `send`, `room_list`, `room_read`, `room_post`, `room_create`, `room_add`,
  `room_remove`, `peers`, `peer_status`. A tool result is one text block that holds Koinon's
  JSON answer; a refusal has `isError: true` and a `code`.
- **Resources.** `koinon://maintainer/inbox`, `koinon://maintainer/decisions` and
  `koinon://maintainer/rooms/{room_id}`. `resources/subscribe` answers `{}` and then sends one
  `notifications/resources/updated` for the URI at once. A new item sends one for
  `decisions`, and so does a verdict. A notice carries only the URI.
  `resources/templates/list` answers -32601.
- **Verdict.** `decision_verdict` answers with `via: "mcp:koinon-voice"`, and
  `decision_history` shows the same `via`. A verdict at an old revision answers
  `stale_revision` with `details.current_revision`.
- **Wrong or empty secret.** The process exits with status 1 before `initialize`. Its
  standard error is one JSON line with `code: "maintainer_secret_invalid"`.
- **Daemon down at start.** Exit status 1, standard error `code: "daemon_error"`, detail
  "daemon unavailable".
- **Daemon restart, subscribed.** With a subscription in place, the process exits with
  status 1 a few seconds after the restart (3 seconds in the driver's run, under 5 in
  Codex's), with no request from the client. Koinon's `ServeMaintainer` ends the token when
  its long poll for changes sees it end.
- **Daemon restart, not subscribed.** The process does not exit by itself. The next tool call
  answers `maintainer_token_expired` (`isError: true`), and then the process exits with
  status 1 and the same code on standard error. In one run the restart came while
  `resources/subscribe` was in flight: it answered JSON-RPC error -32603 with
  `data.code: "daemon_unavailable"`, no subscription was in place, and the process stayed
  until the next tool call.

The connector is always subscribed, so it sees most restarts as an exit. It treats a failed
subscribe as a disconnection, and it makes one read at a fixed interval when no other request
has run (the liveness read), for the case where no exit comes.
- **Pages.** `board_list` and the `decisions` resource give one page of item summaries, with
  `more` and `next_after`; `inbox` and `room_read` page the same way. A summary has the
  reference, title, kind, state, revision and order, not the context or recommendation;
  `decision_get` gives the full item.

## Design that the chunks build

- `internal/keychain`: the Keychain reader that `internal/summarize` has now, moved so that
  summarize and the connectors share it. The service name stays `koinon-voice-<name>`.
- `internal/koinon`: an MCP client on a reader and a writer (requests, answers, notices,
  Koinon's refusal codes), and a connector that starts the process, writes the secret,
  initializes, subscribes, reads, gives verdicts and starts again.
- `hosts.toml` in the maintainer's configuration directory (`~/.config/koinon-voice/`, beside
  `engines.toml`). Each `[[host]]` has `label`, `ssh` (empty for this machine), `koinon` (the
  binary; a full path for a remote host), optional `state_dir` and `address`, and `secret`,
  the Keychain name (`maintainer-local` is the service `koinon-voice-maintainer-local`).
- A remote host runs `ssh -o BatchMode=yes -T -- DEST 'KOINON' 'mcp' '--maintainer' ...`,
  with each remote word quoted for the remote shell. The `--` ends ssh's options before the
  destination; after the destination it would be a word of the remote command.
- `koinon-voice board` prints each host's state and waiting items, for the chunk 01 checks.
- `koinon-voice serve` serves the dashboard (chunk 02).

## Tests (synthetic, in the Go suite)

- **Fake Koinon.** A Go program, built by the test, acts as `koinon mcp --maintainer` from a
  script of the behavior above: it checks the first line against a test secret, answers
  `initialize`, tools and subscriptions, serves lists in pages of a set size, sends notices
  on a signal, and can refuse the secret, report the daemon down, act out a restart (exit
  while subscribed; `maintainer_token_expired` on the next call while not subscribed; -32603
  on a subscribe in flight), or write junk.
- **Client (1, 3).** Requests and answers match by ID with notices between them; a refusal
  becomes an error with Koinon's code; an answer after the deadline, a closed stream, junk
  and a line over the size limit each fail the request and leave no goroutine behind.
- **Connector (1, 3, 4).** The connector writes the secret as the first line and on no command
  line; it subscribes to inbox, decisions and each room, reads again after a notice, and
  shows the new state. It reads every page of the board, the inbox and each room (the fake
  serves more than one page), and calls `decision_get` for each listed item whose revision
  it does not have, so an edit that changes only the context or the recommendation shows
  after its notice. An exit while subscribed with no request in flight (the fake's restart
  case), an exit after `maintainer_token_expired` when not subscribed, a -32603
  `daemon_unavailable` on subscribe, junk and a failed liveness read each make it
  disconnected, then start the process again with
  a delay that grows to a limit. A refused secret shows the code
  `maintainer_secret_invalid` and waits for the longest delay.
- **SSH (1).** A fake `ssh` on `PATH` records its arguments and runs the fake Koinon. The
  test checks `BatchMode=yes`, `-T`, `--` before the destination, the quoted remote words (a
  path with a space and a quote), and that the secret is not in the arguments.
- **Secret (2).** A sentinel secret appears in no argument, error, log line, `board` output,
  page or event stream in any test, including the refused-secret and junk cases.
- **Configuration (7).** A valid file, an unknown field, a duplicate label, no hosts, a remote
  host without a full `koinon` path, and a missing file each give the expected result or a
  message that names the file and the field.
- **Keychain.** The moved reader keeps its tests: off macOS it reports that the Keychain is
  available on macOS only, and a missing or empty item names the service.
- **Dashboard (5, 6).** With `httptest` and fake connectors: the login link works once and
  only for 60 seconds and sets an HttpOnly, SameSite=Strict cookie; a request without the
  cookie gets the login page; a request with a Host that is not the loopback address and
  port gets 403; a POST without `Origin` equal to `http://HOST`, or without the CSRF token,
  gets 403 and gives no verdict. The page shows each host's state and items, labeled by
  host, with each item's context and recommendation from `decision_get`, and an item whose
  context changed shows the new text after the update. Item text from agents is escaped (a `<script>` in a title, context or note shows as
  text). A verdict goes to that host's connector with the shown revision and note;
  `stale_revision` shows the item again with a message. An event stream tells the page to
  reload its data when a connector's state changes.
- **Isolation (WDR 006, Decision 2).** No test reads the Keychain, starts `ssh` or `koinon`
  from `PATH`, or reads the person's configuration; each uses temporary directories, the fake
  programs and loopback listeners on port 0.

## Live checks (recorded in `live-checks.md`)

1. **Scratch daemon (1, 3, 4).** The driver runs `koinon-voice board` against a scratch
   daemon with a generated secret and more waiting items than one `board_list` page holds,
   edits one item's context, restarts the daemon while subscribed, and records that every
   item and the edit show, the reconnection, and the `via` of a verdict.
2. **The maintainer's daemon (5, 6, 8).** The maintainer runs `koinon-voice serve` against the
   local daemon, opens the page, and records a verdict on a synthetic fyi item by click.
   `decision_history` shows `mcp:koinon-voice`. This is an ACTION FOR YOU item on the board.
3. **SSH (1).** On #15, when the maintainer names a host. It is not a gate of this sprint.

## Edge cases that must have tests

- A host whose `koinon` is missing, or whose `ssh` destination cannot be reached: the host is
  disconnected with the reason, and the other hosts work.
- Two hosts with items of the same reference: each verdict goes to its own host.
- An item that leaves the board while the page shows it: the verdict answers `not_found`, and
  the page says so.
- `serve` asked for a non-loopback listen address: refused.
- A child process that does not exit when its input closes: killed with its process group.

## Gate

The `check` skill with the four `CGO_ENABLED=0` builds, the Go workflow on Linux and macOS,
the `attribution` check, and the other family's `peer-review` and `refusal-review` statuses on
each chunk's head. Live check 1 is recorded in chunk 01 and live check 2 in chunk 02 before
the sprint closes.
