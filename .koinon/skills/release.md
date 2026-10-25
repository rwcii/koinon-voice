# Repository facts — release

## Version
1
## Skill
release
## Sources
- AGENTS.md
- CONTRIBUTING.md
- docs/engineering/wdr/002-authority-within-assigned-task.md
- docs/engineering/wdr/004-publication-and-verification.md

## Promotion path
Merge the PR from develop to main.
## Publish steps
Tag the verified main merge with the version as an annotated, signed tag whose
message is the release notes. No release workflow publishes binaries yet; adding one is its
own change.
## Release checks
Dated changelog, exact-head CI and independent review on the develop head being promoted,
and the four CGO-free builds of ./cmd/koinon-voice. Verify the candidate.
