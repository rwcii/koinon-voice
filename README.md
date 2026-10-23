# Koinon Voice

Koinon Voice is a voice interface to [Koinon](https://github.com/rwcii/koinon), the
same-user daemon that carries messages between a maintainer's agent sessions. Its product
behavior is decided in [PDRs](docs/product/pdr/README.md); none is decided yet.

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
