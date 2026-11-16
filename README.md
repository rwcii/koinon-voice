# Koinon Voice

Koinon Voice is a voice interface to [Koinon](https://github.com/rwcii/koinon), the
same-user daemon that carries messages between a maintainer's agent sessions. It is a local
bridge with one dashboard where the maintainer and the agents talk by voice, over the
Koinon nodes it reaches as a signed maintainer client, with all speech processed locally
([PDR 001](docs/product/pdr/001-two-way-voice-bridge.md)). Phase 0
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

`koinon-voice --help` lists every command with its options, as `koinon --help` does. A
command line koinon-voice cannot use prints one JSON error object on standard error, as
Koinon's do, and exits 1. `koinon-voice version` prints the build as JSON, with the field
names of `koinon version`.

## Kokoro voice

Koinon Voice speaks with Kokoro-82M, voice `af_heart`
([ADR 002](docs/architecture/adr/002-local-speech-engine-sidecars.md), Decision 2), natively and without Python
(Decision 4). The helper `koinon-voice-kokoro` runs it on MLX Swift through kokoro-ios, with
MisakiSwift to turn text into phonemes. MisakiSwift is version 1.0.6 with the patches in
`helpers/kokoro/patches/`; `helpers/kokoro/fetch-misaki.sh` clones it, checks its commit and
applies them, and the test and build scripts run it. It opens no audio device: it sends 16-bit mono PCM at
24 kHz to the bridge, one sentence at a time.

MLX's Metal shaders build only with Xcode, so the helper needs Xcode and its Metal Toolchain
(`xcodebuild -downloadComponent MetalToolchain`). Build and install it, then install the model:

```sh
scripts/build-kokoro-helper.sh   # into ~/.local/share/koinon-voice/bin/kokoro/
koinon-voice kokoro install      # into ~/.local/share/koinon-voice/kokoro/
koinon-voice kokoro status
```

The helper's frameworks and resource bundles sit beside it in `bin/kokoro/`; run it by that
path, not through a symlink. Before it installs the helper, the build script runs
`koinon-voice-kokoro check-text`: fixed sentences through MisakiSwift, with the phonemes each
must give. A failure keeps the helper installed before. `kokoro install` downloads the model, its config and the
`af_heart` voice from `mlx-community/Kokoro-82M-bf16` at the revision that Phase 0 rated,
checks each file's size and SHA-256 against the values in `internal/kokoro`, and keeps a file
only when they match. `kokoro status` checks the files again and exits 1 when one is missing
or changed. Both take `--dir DIR`.

`koinon-voice summarize --speak` and `koinon-voice helper --say TEXT` speak with Kokoro by
default. Each text starts a new `koinon-voice-kokoro`, which synthesizes it sentence by
sentence; the Apple helper (`helper` in the engines file) plays each sentence through its
voice-processing engine (ADR 002, Decision 3) while the next one is synthesized. Ctrl-C stops
the playback and the synthesis. When the helper or a model file is missing, the command names
it with the command that installs it, and exits 1 before any audio.

The engines file's `[speech]` table chooses the voice:

```toml
[speech]
engine = "kokoro"   # the default; "apple" speaks with Apple's voice
voice = "af_heart"  # a Kokoro voice, or an AVSpeechSynthesisVoice identifier for "apple"
speed = 1.1         # Kokoro only
```

With `engine = "apple"` and no `voice` in `[speech]`, the top-level `voice` applies, as before.
`helper --say` takes `--engine kokoro|apple`, `--voice` and `--speed` for one text, and
`--out FILE` to write the speech to a file in place of the output device.

## Connect to Koinon

Koinon Voice reaches Koinon only as a signed client of each Koinon node
([ADR 004](docs/architecture/adr/004-signed-client-of-koinon-nodes.md), rwcii/koinon#693): one
key pair per user and install signs each maintainer action, and each Koinon computer accepts
only keys enrolled on it.

### Maintainer key

```sh
koinon-voice key create --user NAME   # one Ed25519 key for this install, in the Keychain
koinon-voice key show                 # its key ID, public key and user
koinon-voice key enroll --target PIN  # a signed enrollment statement for one computer
```

`key create` refuses when this install has a key, and never replaces one. The private key is
kept in the Keychain item `koinon-voice-maintainer-key`, written through `security` on
standard input, and is printed nowhere. For `key enroll`, read the target's pin on the target
itself: `koinon link status` prints it as `Fingerprint`. `--client` sets the client label
(default `koinon-voice`) and `--out FILE` writes the statement to a new file. Each statement
makes one authorization on its computer; `key enroll` records it for that pin in
`~/.local/share/koinon-voice/enrollments.json`, and every signed action to the computer carries
it. To enroll again after a key is removed there, sign a new statement. On the target,
within 10 minutes, run `koinon maintainer key add --statement FILE` (a Koinon release with
#693).

### Signed nodes

List the nodes in `~/.config/koinon-voice/hosts.toml`. Each row is a signed node with
`node_pin`, the node's pin read on the node with `koinon link status`. Without `serve` it is
the koinon on this computer: `koinon` is the binary (default `koinon` on `PATH`), and
`state_dir` and `address` select a daemon other than the default one. With `serve` it is
another computer's Remote address, `https://HOST`:

```toml
[[host]]
label = "studio"
node_pin = "<64 hex digits from koinon link status on studio>"

[[host]]
label = "blackhole"
node_pin = "<64 hex digits from koinon link status on blackhole>"
serve = "https://blackhole.example.ts.net"
```

Two rows with the same `node_pin` are refused, and so is a row with `ssh` or `secret`: those
rows of the maintainer-secret connection were removed. Voice tries a signed node only when
`enrollments.json` holds an enrollment for its pin made with this install's key, user and
client; only the node's answer confirms it. Without the key, or without an enrollment for the
node, the row shows why and nothing is sent.

`koinon-voice board` connects to every row and prints one line of JSON: the state of each
computer, the items that wait for the maintainer in full, the maintainer's inbox and the rooms,
the pending verdicts through nodes that no row names under `unresolved`, and `ok`, which is
false, with exit status 1, when a computer is not connected or not read in full. `--watch`
prints a new line on each change.

`board` and `dashboard` show a signed row as its node and each computer paired with it, by
name, with its pin. They read each computer's board, inbox and rooms once, then only what its
signed `changes` wait names. A computer that goes offline or is unpaired stays shown,
disconnected, with what was last read.

On the dashboard, a verdict on a signed computer's item is signed only for the item as the page
showed it: when the item changed since, nothing is sent and the page shows it as it is now. Each
item offers the verdicts of its kind (decision: approved, declined, feedback; task: completed,
declined, feedback; fyi: acknowledged). The same form sent again sends nothing and shows its
first result. A verdict is written to `~/.local/share/koinon-voice/pending.json` before it is
sent and removed when its outcome is known. When an answer is lost, the computer shows the
verdict as waiting for its outcome, and Koinon Voice asks the computer for it under the
original signed request, also after a restart; it never signs the verdict again. An outcome that
can no longer be asked, such as after the key was enrolled again there, stays shown until you
remove its local record; read the item's record on that computer. A pending verdict through a
node that no row names is listed with its node and computer pins, and nothing is sent for it.

## Dashboard

`koinon-voice dashboard` connects to every row and serves the dashboard on
`http://127.0.0.1:17680/` (`--listen` takes another loopback address). It prints a login link
and opens it unless `--no-open`; the link works once, within 60 seconds, and the session lasts
while `koinon-voice dashboard` runs. Start it again for a new link.

The page shows each computer by name with its pin and state, the items that wait for the
maintainer with their full text, and the maintainer's unacknowledged inbox messages. It updates
when Koinon reports a change. Give a verdict on an item, with an optional note, from its
buttons, as "Signed nodes" describes.

As Koinon's own dashboard does, the page answers only requests that name its loopback
address in `Host`, takes a form only with its own `Origin` and the session's form token, and
shows agent text as text.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE).
