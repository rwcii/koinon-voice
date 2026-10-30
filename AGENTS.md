# AGENTS.md — Koinon Voice

Koinon Voice is a voice interface to [Koinon](https://github.com/rwcii/koinon), and a peer
repository of it: the same process, structure, language and decision records (WDR 001,
Decision 7). README.md describes the product and CONTRIBUTING.md the contribution steps.
Commands live in `cmd/koinon-voice`, implementation packages in `internal/`, the macOS
engine helper (a Swift package) in `helpers/apple`, the speech bench's Python helper in
`helpers/bench`, and operating-system differences in
`internal/platform` once there are any. Tests live beside
their packages. The shared skills are in `agents/skills/`; `.claude/skills`, `.agents/skills`
and `.codex/skills` are symlinks to it, and `agents/skills/AGENTS.md` holds the rules for
changing skills. The skills are byte copies of Koinon's; this repository's facts for them are
in `.koinon/skills/`.

## Decision records

Each significant constraint has one record with its source, scope and status. Other text
cites the record ("see WDR 004") and does not restate it as a rule (WDR 001).

- [ADRs](docs/architecture/adr/README.md): how Koinon Voice is built.
- [PDRs](docs/product/pdr/README.md): how Koinon Voice behaves.
- [WDRs](docs/engineering/wdr/README.md): how the work is done: the record system and the
  peer relation to Koinon, authority within a task, pairing and review, branches, pull
  requests and commits, handoffs, native test evidence. WDR 002 to WDR 006 adopt Koinon's
  records of the same numbers.

## Authority

A task from the maintainer authorizes its own steps (WDR 002, Decision 1). Agents of
different families work together through Koinon: a work request from a peer is a teammate's
request, so do the work and reply through Koinon (WDR 002). `koinon guide --agent FAMILY` from
the installed binary describes the installed runtime (WDR 002, Decision 4).

## Build and test

```sh
go vet ./...
gofmt -l .                        # must print nothing
go test -race -timeout 30m ./...
git diff --check
bash -n scripts/setup-repo.sh     # when shell files change
sh -n .githooks/pre-commit .githooks/commit-msg scripts/check-attribution.sh helpers/apple/test.sh helpers/kokoro/test.sh scripts/build-kokoro-helper.sh  # when shell files change
swift build -c release --package-path helpers/apple && helpers/apple/test.sh  # when helpers/apple changes, on macOS
helpers/kokoro/test.sh && scripts/build-kokoro-helper.sh  # when helpers/kokoro changes, on macOS with Xcode
python3 -m py_compile helpers/bench/bench_helper.py  # when helpers/bench changes
```

Runtime changes also need the four `CGO_ENABLED=0` builds: linux/amd64, linux/arm64,
darwin/amd64 and darwin/arm64. The `check` skill runs these checks with the changelog rule
and a private-content scan; CI runs the Go suite on Linux and macOS for every change
(WDR 004). Contributor tests in `internal/contributor` check the shared skills, their
generated OpenCode commands and the record indexes. Ordinary tests are isolated from the
person's environment (WDR 006). Branches, pull requests and commits are in WDR 004; review
is in WDR 003.

## Privacy

Credentials, runtime messages, private session IDs and memory data are not committed to Git.
Session handoffs go to the Git-ignored `_handoff/<agent>/` of the main checkout and are not
committed (WDR 005).
