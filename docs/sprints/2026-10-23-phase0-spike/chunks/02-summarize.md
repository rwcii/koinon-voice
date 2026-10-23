# Chunk 02 — Summarize

Read `../decision.md` and `../definition-of-done.md` first.

## Outcome

`koinon-voice summarize` turns decision items into spoken summaries with each configured
engine and writes a scorecard (criterion 1), using two of ADR 003's adapters. The Apple
adapter comes in chunk 03.

- `internal/summarize`: an `Engine` interface (`Summarize(ctx, Item) (Summary, error)`, where
  a `Summary` carries text, engine, local or external, input and output tokens, cost and
  timings), the prompt, and the scorecard.
- **Anthropic adapter** with the official Go SDK (`github.com/anthropics/anthropic-sdk-go`),
  streaming, so time to first token is measured. Models are configuration rows; the plan's
  candidates are `claude-haiku-5-5`, `claude-sonnet-5-5` and `claude-opus-5-5`.
- **OpenAI-compatible adapter**: one HTTP client with a base URL, for OpenAI, OpenRouter and
  any local server. Models are chosen when the scorecard runs and recorded with it.
- **Keys** from the macOS Keychain through `security find-generic-password -s
  koinon-voice-<provider> -w`, run as a subprocess so the module stays CGO-free. Linux and
  tests take no key and fail the external engines with a clear message.
- **Engines file**: `~/.config/koinon-voice/engines.toml` (outside the repository) lists rows
  of adapter, base URL, model and local or external; `testdata/engines.toml` is the example.
- **Items** from `--fixtures DIR`, or from `--item FILE`, a decision item exported as JSON.
  Reading items from a daemon waits for the read path that chunk 01's map records.
- **Fixtures**: at least 20 synthetic items in `internal/summarize/testdata/items/` with
  answer keys (definition of done, Fixtures).
- **Output**: one line per engine and item with the summary, correctness flags (by a
  deterministic check against the answer key where possible, else marked for the maintainer),
  time to first token, total time and cost; `--speak` plays each summary with macOS `say`
  for the blind rating, and `--blind` hides engine names until the ratings are entered.

The prompt asks for two or three sentences naming the asker, the ask, the options, the
recommendation and any deadline, without hashes, paths or markdown, with numbers as spoken,
and says not to invent a recommendation or deadline the item lacks.

## Tests

The definition of done's Summarize, Fixtures and edge-case tests, with `httptest` servers for
both adapters and a fake `security` command on `PATH` for the Keychain.

## Live check

Definition of done, live check 1, for the external engines and any local server the maintainer
installs. The evidence goes to `scorecard-summarize.md`.

## Done

The tests pass on Linux and macOS, `summarize` runs every configured engine on the fixtures,
and the scorecard is committed.
