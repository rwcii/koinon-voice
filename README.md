# Koinon Voice

Koinon Voice is a voice interface to [Koinon](https://github.com/rwcii/koinon), the
same-user daemon that carries messages between a maintainer's agent sessions. It is a local
bridge with one dashboard where the maintainer and the agents talk by voice, over the
Koinon daemon on the maintainer's machine and remote daemons reached over SSH, with all
speech processed locally ([PDR 001](docs/product/pdr/001-two-way-voice-bridge.md)). It is in
Phase 0, a spike that settles its engines and Koinon integration
([plan](docs/sprints/2026-10-23-phase0-spike/decision.md)).

Koinon Voice is a peer repository of Koinon. It is one Go module, built by the same agent
team under the same process and decision records
([WDR 001](docs/engineering/wdr/001-decision-records-govern-constraints.md)).

## Build

```sh
go build ./cmd/koinon-voice
go test -race -timeout 30m ./...
```

`koinon-voice version` prints the build version.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE).
