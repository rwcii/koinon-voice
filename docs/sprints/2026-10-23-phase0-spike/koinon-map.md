# Koinon map — what a maintainer client can do with Koinon v0.4.1

Chunk 01 of the Phase 0 spike (criterion 6). Koinon v0.4.1, commit 7089386, read from source
and tried on 2026-10-23 against a scratch daemon on the maintainer's Mac: `koinon serve` with a
temporary state directory on 127.0.0.1:47791, never the maintainer's running daemon
(definition of done, live check 6). The synthetic agent session was registered through the
scratch daemon's own `/v1/sessions/register`.

## Summary

Everything the maintainer can do goes through the HTML dashboard, behind a one-time login
link, a cookie, and Host, Origin and CSRF checks. A non-browser client can drive it: every
read and act path below worked from `curl`. But it would be scraping HTML pages and posting
their forms, with no JSON, no change notices and no long-lived credential. ADR 001,
Decision 3 rejects that design. The `/v1` API that agent sessions use is not a maintainer
path: it takes the daemon's same-user secret and a session's caller key, and decision verdicts
are dashboard-only. The gaps are listed for rwcii/koinon#464 at the end.

## 1. Read

| Need | How, today | Source | Tried |
| ---- | ---------- | ------ | ----- |
| Waiting decision items, every field | `GET /dashboard/decisions` (HTML), with the item's reference, revision, title, context, recommendation and commands in the card and its action form | `internal/core/dashboard.go` (views), `dashboard_decisions.go` | Yes: 200, `text/html`, the item's `reference` and `revision` form fields present |
| Messages to the maintainer | `GET /dashboard/messages` (HTML) | `dashboard.go` views | Yes: the sent message appears |
| Room posts | `GET /dashboard/rooms` (HTML) | `dashboard.go` views | Read from source only |
| Peers and their state | `GET /dashboard/sessions` (HTML) | `dashboard.go` views | Yes: the login redirects there |
| Any view as a fragment | the same URL with `?fragment=1` returns only the list's HTML | `dashboard.go` `render` | Read from source |

There is no JSON form of any dashboard view. `/v1/decisions/list` and `/v1/decisions/get`
return JSON but only for a session caller (`caller` key) and filters such as `mine`; they are
the agents' API, authenticated with the daemon secret (`server.go`, bearer check).

## 2. Change notices

None for the maintainer. The dashboard page polls its own view: `web/static/app.js` fetches
`?fragment=1` on a timer while the page is visible and replaces the list. Agent sessions get
wakes and inbox notices; the maintainer session has no notice channel. A client today would
have to poll each view and diff the HTML.

## 3. Post and send as the maintainer

| Action | Route | Fields | Tried |
| ------ | ----- | ------ | ----- |
| Message a peer | `POST /dashboard/actions/send` | `to` (peer name, not `family:id`), `body`, `csrf` | Yes: 303 `notice=sent`; the message appears on the messages view. With `to=deepseek:synthetic-1` the answer was `notice=peer_not_found` |
| Post to a room | `POST /dashboard/actions/room-post` | room and body, `csrf` | Read from source only; same checks as send |

The daemon sends these as `maintainerKey`, so they carry the maintainer's weight (Koinon
PDR 005, Decision 3).

## 4. Record a verdict

`POST /dashboard/actions/decision-approve`, `decision-decline` or `decision-dismiss`, with
`reference`, `revision` (the revision the page showed), an optional `note` of up to 4 KiB and
`csrf` (`dashboard_decisions.go`). Tried:

- Without an `Origin` header: 403.
- With `Origin` but without the CSRF token: 403.
- With both: 303 to the board with `notice=decision_approved`. Read back through
  `koinon decision get`: state `in_progress`, verdict `approved` at revision 1, the note, and
  a snapshot of the item's title, context, recommendation and commands.

The verdict records no client: a voice approval and a click look the same.

## 5. Authentication

- **Login.** `koinon dashboard --no-open` (or `POST /v1/dashboard/links` with the daemon
  secret) prints `http://ADDRESS/dashboard/login?token=…`. The token works once, within 60
  seconds. Opening it sets an HttpOnly, SameSite=Strict cookie named
  `koinon_dashboard_<port>` on path `/dashboard/` and redirects to the sessions view.
- **Lifetime.** 24 hours idle, 7 days absolute (`dashboard_auth.go`). After that, a new link,
  which needs a command on the daemon's host.
- **Checks on every request.** The `Host` header must be a literal loopback address with a
  port; a state-changing request needs an `Origin` equal to `http://<Host>` and the CSRF
  token (form field `csrf` or header `X-Koinon-CSRF`) from a page of the same login.
- **Through a forwarded port.** A plain TCP forward from 127.0.0.1:47800 to the daemon stood in
  for an SSH local forward; an SSH forward to localhost was not tried, because it would have
  meant accepting this machine's own host key. The cookie from the direct port was refused on
  the forwarded port (401), because the cookie name carries the port. A new login link, its
  host and port rewritten to 127.0.0.1:47800, logged in (cookie `koinon_dashboard_47800`) and
  the decisions view answered 200. So each forwarded port needs its own login, and for a
  remote daemon the link comes from running `koinon dashboard --no-open` on that host, over
  SSH.

## Gaps for rwcii/koinon#464

Posted as [a comment on #464](https://github.com/rwcii/koinon/issues/464#issuecomment-6375563801).


1. **No machine-readable read.** Items, messages, rooms and peers exist for the maintainer only
   as HTML.
2. **No change notices** for the maintainer; only page polling.
3. **No stable action interface.** Posting and verdicts are HTML form endpoints that need a
   CSRF token scraped from a page; their fields can change with the dashboard.
4. **No long-lived maintainer credential.** A login needs a one-time link created on the
   daemon's host within 60 seconds, expires after 24 hours idle or 7 days, and is bound to the
   port the client reaches the daemon on.
5. **No client recorded on a verdict or post**, so a voice decision cannot be told from a click
   (#464 asks whether one should be).

## Consequence for the plan

Chunk 02 reads items from fixtures or exported JSON (its specification already says reading
from a daemon waits for this map). Nothing in Phase 0 needs more. Phase 1 needs at least gaps
1 to 3 closed in Koinon, or a decision to scrape, which ADR 001, Decision 3 rejects; chunk 06
carries this into ADR 001.
