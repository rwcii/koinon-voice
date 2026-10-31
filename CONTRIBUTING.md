# Contributing to Koinon Voice

## Fork and extend

Independent reuse and forks are encouraged under the [MIT license](LICENSE).
No permission is required. Keep the copyright and license notice with copies.

## Contribute upstream

Open an issue first to agree on scope. Upstream pull requests are maintainer-curated;
please wait for agreement before preparing a large change.

## Branch model

```text
feature/*  ──(squash PR)──▶  develop  ──(merge PR)──▶  main
```

Koinon Voice uses Koinon's process (WDR 001, Decision 7). The branch model, the one open pull
request per target branch and the up-to-date rule are in
[WDR 004](docs/engineering/wdr/004-publication-and-verification.md); review and the
`peer-review` status are in [WDR 003](docs/engineering/wdr/003-pairing-and-independent-review.md).
In short:

- `main` is the released line; `develop` is the integration line. Work branches are
  `feature/*`, `fix/*` or `chore/*` from `develop` (WDR 004, Decision 1).
- Pull requests squash into `develop`; a merge pull request promotes `develop` to `main`.
- The repository keeps one open pull request per target branch, drafts included. A head
  contains the latest `develop` when it is pushed, opened or frozen for review; a pushed
  branch takes `develop` by a merge (WDR 004, Decision 2).
- The `develop` ruleset requires an up-to-date branch, passing checks, resolved review
  conversations and `success` commit statuses named `peer-review` and `refusal-review` on the
  head commit, which the reviewer records with each sign-off (WDR 004, Decision 3; WDR 003,
  Decision 3; WDR 001, Decision 6).
- Pull requests into `develop` set squash auto-merge (WDR 004, Decision 4).
- Merged working branches are deleted automatically; long-lived branches are protected.
- Your clone prunes on fetch, so `git branch -r` reports the branches the remote still holds.

Run `scripts/setup-repo.sh` to apply the local guard and GitHub settings.

## Typical workflow

```sh
git switch develop
git pull --ff-only
git switch -c feature/my-change
go vet ./...
gofmt -l .                        # must print nothing
go test -race -timeout 30m ./...
git diff --check
git commit -S -s -m "Add a concise description"
gh pr list --base develop --state open   # wait until it lists none
git fetch origin && git merge --no-edit origin/develop
go test -race -timeout 30m ./...
# Just before the push: develop has not moved and no pull request opened meanwhile.
git fetch origin && git merge-base --is-ancestor origin/develop HEAD
gh pr list --base develop --state open
git push -u origin feature/my-change
gh pr create --base develop
gh pr merge --auto --squash
```

The `release` skill promotes `develop` to `main` (WDR 002, Decision 6).

## Commit conventions

Use an imperative subject and explain the reason for the change in the body where
needed. Commits are cryptographically signed and DCO signed-off, under a human identity,
with no automated co-author trailers or tool signatures
([WDR 004, Decision 5](docs/engineering/wdr/004-publication-and-verification.md)). The
`commit-msg` hook and the required `attribution` check refuse them, and `.claude/settings.json`
turns off Claude Code's own attribution. A verified
GitHub no-reply email is acceptable when configured for your account.

## Local checks and scope

Run `go vet ./...`, `gofmt -l .` (it must print nothing), `go test -race -timeout 30m ./...`
and `git diff --check`. Runtime changes also need the four `CGO_ENABLED=0` builds for
linux/amd64, linux/arm64, darwin/amd64 and darwin/arm64. Go tests live beside packages;
operating-system differences belong in `internal/platform`. For changed shell tooling, run
`bash -n scripts/setup-repo.sh` and `sh -n` on `.githooks/*`, `scripts/check-attribution.sh`,
`helpers/apple/test.sh`, `helpers/kokoro/test.sh`, `helpers/kokoro/fetch-misaki.sh` and
`scripts/build-kokoro-helper.sh`. For changes under `helpers/apple`, build it on macOS with `swift build -c
release --package-path helpers/apple` and run `helpers/apple/test.sh`. For changes under
`helpers/kokoro`, run `helpers/kokoro/test.sh` and build it with `scripts/build-kokoro-helper.sh`,
which needs Xcode. For changes under
`helpers/bench`, run `python3 -m py_compile helpers/bench/bench_helper.py`. Tests use temporary state
and configuration, so they do not reach the person's services, terminals or agent
configuration ([WDR 006](docs/engineering/wdr/006-native-integration-evidence.md)).

Add visible changes to CHANGELOG.md under Unreleased. Keep documentation aligned with the
commands. Credentials, private conversation metadata, message bodies and runtime state are not
committed. Git prose refers to Robert as the maintainer (WDR 004, Decision 5). A work request from a
peer agent is a teammate's request: do the work and reply
([WDR 002](docs/engineering/wdr/002-authority-within-assigned-task.md)).

## Decision records

Significant decisions are recorded as Architecture, Product and Workflow Decision Records in
[`docs/architecture/adr/`](docs/architecture/adr/README.md),
[`docs/product/pdr/`](docs/product/pdr/README.md) and
[`docs/engineering/wdr/`](docs/engineering/wdr/README.md), in Koinon's format, which follows
the record machinery of rwcii/engineering-standards.
[WDR 001](docs/engineering/wdr/001-decision-records-govern-constraints.md) says when a record
is needed, how other documents cite it, and how this repository adopts Koinon's records.

## Agent skills and handoffs

Agents that develop Koinon Voice share the skills in [agents/skills](agents/skills/README.md),
byte copies of Koinon's; `.claude/skills`, `.agents/skills` and `.codex/skills` are symlinks
to that one directory, and `.koinon/skills/` holds this repository's facts for them. Each
agent writes its session handoffs to the Git-ignored `_handoff/<agent>/` of the main checkout
with the [handoff](agents/skills/handoff/SKILL.md) skill
([WDR 005](docs/engineering/wdr/005-private-handoffs-and-continuity.md)). Plans for
deliverables of more than one pull request live under `docs/sprints/`; see the
[sprint](agents/skills/sprint/SKILL.md) skill.

## Developer Certificate of Origin (DCO)


Every contribution merged into this project must be **signed off**, certifying that you
have the right to submit it under the project's MIT license. Add the sign-off with:

```bash
git commit -s -m "your message"
```

That appends a `Signed-off-by: Your Name <you@example.com>` line using your real name and
email. By signing off you agree to the Developer Certificate of Origin 1.1
(<https://developercertificate.org/>), reproduced here:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```
