# Sprint 2026-10-27 — Connectors and dashboard — Review

Reviewer: Codex. Driver: Claude.

## Plan (bc84acf)

Peer review, changes requested
([comment](https://github.com/rwcii/koinon-voice/pull/26#issuecomment-6449978834)):

1. **F1. A restart while subscribed ends the process with no request.** The definition of done
   said that a restart is seen only on a request. Codex, on its own scratch daemon with
   Koinon v0.5.0 (8abe1b9), saw `koinon mcp --maintainer` exit with status 1 within 5 seconds
   of a restart after `initialize` and a subscription, and cited `ServeMaintainer` and its
   change poll. Disposition: fixed. The driver reproduced it (exit 3 seconds after the
   restart while subscribed; no exit in 14 seconds while not subscribed). Its first run had
   the restart during the subscribe, which answered -32603 and left no subscription. The
   definition of done now records all three cases; the fake Koinon acts out each, the
   connector tests cover each, and the liveness read stays for the case with no exit.
2. **F2. One page of summaries is not the board.** `board_list` and the `decisions` resource
   give one page of summaries without context or recommendation, but chunk 02 shows the full
   text. Disposition: fixed. The connector reads every page of the board, the inbox and each
   room, and calls `decision_get` for each item whose revision it does not have. The fake
   serves pages; the tests cover more than one page and an edit that changes only the context
   or recommendation; live check 1 includes both.

Refusal review, pass
([comment](https://github.com/rwcii/koinon-voice/pull/26#issuecomment-6449979405)): no
unsourced restrictions; the board scope and Gate A were verified, and `decision.md` is
unchanged from 197a776.

## Plan (4374a58)

Bounded re-review of F1 and F2, both passed:
[peer review](https://github.com/rwcii/koinon-voice/pull/26#issuecomment-6450023730) and
[refusal review](https://github.com/rwcii/koinon-voice/pull/26#issuecomment-6450024357).

## Chunk 01 (922e3f7, PR #27)

Peer review, changes requested
([comment](https://github.com/rwcii/koinon-voice/pull/27#issuecomment-6450421214)); refusal
review, changes requested on the same room limit
([comment](https://github.com/rwcii/koinon-voice/pull/27#issuecomment-6450421711)). Codex
reproduced each with probe tests, which pass on the fix.

1. **F1. A blocked write held a request past its deadline.** `Client.Call` wrote before it
   waited on the deadline, so a process that stopped reading its input could hold a request,
   and with it the session's end and cleanup. Disposition: fixed. Each write runs apart from
   the caller; at the deadline the client stops with `ErrStalled`, the session ends, and
   closing the process's input ends the write. `TestClientBlockedWrite` uses a pipe that
   nobody reads.
2. **F2. Notices during the startup reads were dropped.** `start` discarded every notice after
   its reads, so a board change between the board read and the end of start stayed unread.
   Disposition: fixed. `start` discards only the subscriptions' own notices, before the reads;
   a notice during the reads stays for the loop. The liveness read now reads the board and the
   inbox too. `TestStartKeepsNoticesDuringReads` sends a board notice during the inbox read.
3. **F3 and R1. Rooms kept only their latest 50 posts.** The limit had no source, and the plan
   requires every page. Disposition: fixed. A room's state holds every post that Koinon
   retains; `TestConnectorKeepsEveryPost` reads 75 posts in pages of 20.

Bounded re-review at bb335f4, both passed:
[peer review](https://github.com/rwcii/koinon-voice/pull/27#issuecomment-6450503950) and
[refusal review](https://github.com/rwcii/koinon-voice/pull/27#issuecomment-6450504486). PR #27
merged as 21afebb.
