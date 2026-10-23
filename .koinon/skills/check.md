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
`gofmt -l .` must print nothing. Scan added lines and read the final diff for private content; inspect every matching path or
address individually. Confirm visible changes have an Unreleased CHANGELOG.md entry.
## Conditional checks
For shell edits: bash -n scripts/setup-repo.sh, sh -n .githooks/pre-commit,
sh -n .githooks/commit-msg and sh -n scripts/check-attribution.sh.
Runtime changes: four CGO_ENABLED=0 builds of ./cmd/koinon-voice (linux/amd64, linux/arm64,
darwin/amd64, darwin/arm64).
When adding a decision record or changing its status under docs/architecture/adr,
docs/product/pdr or docs/engineering/wdr, verify the series README.md index has the same
number, title and status (WDR 001).
## Required CI
Linux/macOS Go matrix on the exact PR head (WDR 004).
