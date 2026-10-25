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
Failed CI runs (WDR 004, Decision 6): the baseline is the previous release tag on main, or the
repository's first commit before the first release. List the runs of every workflow on all
branches from the baseline commit's time through the candidate's exact-head CI with the
repository Actions runs API (`created>=` the commit time, every page; split the time range when
a query reaches 1,000 results). For a run with `run_attempt` above 1, read each attempt 1..N
from its attempts endpoint. List each failed run and attempt in the promotion PR evidence with
its cause, fix or open issue.
