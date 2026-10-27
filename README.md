# Koinon Voice

Koinon Voice is a voice interface to [Koinon](https://github.com/rwcii/koinon), the
same-user daemon that carries messages between a maintainer's agent sessions. It is a local
bridge with one dashboard where the maintainer and the agents talk by voice, over the
Koinon daemon on the maintainer's machine and remote daemons reached over SSH, with all
speech processed locally ([PDR 001](docs/product/pdr/001-two-way-voice-bridge.md)). Phase 0
settled its engines and Koinon integration
([results](docs/sprints/2026-10-23-phase0-spike/results.md)); the connectors sprint builds the
connection to each daemon and the dashboard
([plan](docs/sprints/2026-10-27-connectors/decision.md)).

Koinon Voice is a peer repository of Koinon. It is one Go module, built by the same agent
team under the same process and decision records
([WDR 001](docs/engineering/wdr/001-decision-records-govern-constraints.md)).

## Build

```sh
go build ./cmd/koinon-voice
go test -race -timeout 30m ./...
```

`koinon-voice version` prints the build version.

## Connect to Koinon

Koinon Voice reaches each Koinon daemon through Koinon's maintainer mode,
`koinon mcp --maintainer`, on stdio: as a child process on this machine, or through `ssh` on
a remote host, with no port forwarded
([ADR 001](docs/architecture/adr/001-local-bridge-and-connectors.md)). It needs Koinon v0.5.0
or later and each daemon's maintainer secret (`koinon maintainer secret set` or `generate`).

Keep each secret in the macOS Keychain, with the service `koinon-voice-<name>`:

```sh
security add-generic-password -a "$USER" -s koinon-voice-maintainer-local -w
```

List the daemons in `~/.config/koinon-voice/hosts.toml`:

```toml
[[host]]
label = "local"
secret = "maintainer-local"        # the Keychain service koinon-voice-maintainer-local

[[host]]
label = "devbox"
ssh = "maintainer@devbox"              # any ssh destination; ssh runs with BatchMode=yes
koinon = "/home/maintainer/.local/bin/koinon"   # the full path on the remote host
secret = "maintainer-devbox"
```

`state_dir` and `address` select a daemon other than the default one. A remote host needs a
key in the SSH agent and a known host key, because ssh asks nothing.

`koinon-voice board` connects to every host and prints its state and the items that wait for
the maintainer; `--watch` prints again on each change. The secret is written as the first
line of the process's standard input and appears in no argument, log or output.

## Dashboard

`koinon-voice dashboard` connects to every host and serves the dashboard on
`http://127.0.0.1:17680/` (`--listen` takes another loopback address). It prints a login link
and opens it unless `--no-open`; the link works once, within 60 seconds, and the session lasts
while `koinon-voice dashboard` runs. Start it again for a new link.

The page shows each host by label with its state, the items that wait for the maintainer
(decision and fyi) with their full text, and the maintainer's unacknowledged inbox messages.
It updates when Koinon reports a change. Approve or decline a decision item, or dismiss an
fyi item, with an optional note: Koinon records the verdict at the revision the page showed,
with the client `mcp:koinon-voice`. When the item changed after the page showed it, Koinon
refuses the verdict, and the page shows the item as it is now.

As Koinon's own dashboard does, the page answers only requests that name its loopback
address in `Host`, takes a form only with its own `Origin` and the session's form token, and
shows agent text as text.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE).
