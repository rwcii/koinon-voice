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
