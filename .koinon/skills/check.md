# Repository facts — check

## Version
1
## Skill
check
## Sources
- AGENTS.md
- CONTRIBUTING.md
- .github/workflows/tests.yml
- docs/engineering/wdr/001-decision-records-govern-constraints.md
- docs/engineering/wdr/004-publication-and-verification.md

## Base branch
origin/develop
## Required checks
go vet ./...
gofmt -l .
go test -race -timeout 30m ./...
git diff --check origin/develop...HEAD
git diff --check
`gofmt -l .` prints nothing for a formatted tree. Scan added lines and read the final diff for private content, and inspect
each matching path or address. Confirm visible changes have an Unreleased CHANGELOG.md entry.
## Conditional checks
For shell edits: bash -n scripts/setup-repo.sh, sh -n .githooks/pre-commit,
sh -n .githooks/commit-msg, sh -n scripts/check-attribution.sh, sh -n helpers/apple/test.sh,
sh -n helpers/kokoro/test.sh, sh -n helpers/kokoro/fetch-misaki.sh and
sh -n scripts/build-kokoro-helper.sh.
For edits under helpers/apple, on macOS: swift build -c release --package-path helpers/apple
and helpers/apple/test.sh.
For edits under helpers/kokoro, on macOS: helpers/kokoro/test.sh, and with Xcode,
scripts/build-kokoro-helper.sh.
For edits under helpers/bench: python3 -m py_compile helpers/bench/bench_helper.py.
Runtime changes: four CGO_ENABLED=0 builds of ./cmd/koinon-voice (linux/amd64, linux/arm64,
darwin/amd64, darwin/arm64).
When adding a decision record or changing its status under docs/architecture/adr,
docs/product/pdr or docs/engineering/wdr, verify the series README.md index has the same
number, title and status (WDR 001).
## Required CI
Linux/macOS Go matrix on the exact PR head (WDR 004).
