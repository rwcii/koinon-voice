# Scorecard — Kokoro native

Chunk 01's evidence (definition of done, checks 12, 13, 16, 17 and 18, and the feasibility
steps of `chunks/01-kokoro-helper.md`). The model, the voices and the synthesized audio stay
under `~/.local/share/koinon-voice/` and are not in Git (definition of done, Publication
gates); this file holds measurements, phonemes, versions and hashes.

## Run

- Dates: 2026-10-29 (feasibility steps, outside the repository, in
  `~/.local/share/koinon-voice/kokoro-spike/`) and 2026-10-30 (the helper and checks 12, 13,
  16, 17 and 18), on the maintainer's Mac: Apple M2 Max, macOS 26.5 (build 25F71), Xcode 26.6
  (build 17F113) with Metal Toolchain 17F109, Go 1.27.3.
- Packages, as `helpers/kokoro/Package.resolved` pins them:
  - kokoro-ios 1.0.11, `4d6d1d8ff8cd012014180c9cd4cf0151e7682354` (MIT)
  - MisakiSwift 1.0.6, `6835a1ce4a8854075c89f18ff75c74b13ef58e15` (Apache-2.0); since Codex's
    review of PR #34, a local copy with `helpers/kokoro/patches/misaki-utf16-ranges.patch`
    (`helpers/kokoro/fetch-misaki.sh`). Checks 12, 13 and 17 ran before the patch.
  - mlx-swift 0.30.2, `f58bd2c2b3b84316da69182f436db4219aff30b9` (MIT)
  - MLXUtilsLibrary 0.0.6, swift-numerics 1.1.1 and ZIPFoundation 0.9.20, through kokoro-ios
- Model files, from `mlx-community/Kokoro-82M-bf16` at revision
  `a71e4d38b236d968966a2002c4c895dbd12b1c3c`, installed with `koinon-voice kokoro install`
  into `~/.local/share/koinon-voice/kokoro/` (12 s; each file mode 0600, directories 0700):

  | File | Bytes | SHA-256 |
  | --- | --- | --- |
  | `kokoro-v1_0.safetensors` | 327115152 | `4e9ecdf03b8b6cf906070390237feda473dc13327cb8d56a43deaa374c02acd8` |
  | `config.json` | 2351 | `5abb01e2403b072bf03d04fde160443e209d7a0dad49a423be15196b9b43c17f` |
  | `voices/af_heart.safetensors` | 522320 | `2c1c733b0e6576c810e268d3e440c21dea4e0f0131a3ba4cfc98d7fe6136d094` |

- The helper: `scripts/build-kokoro-helper.sh` (83 s) into
  `~/.local/share/koinon-voice/bin/kokoro/`, 47 MB with its frameworks and resource bundles.
  Its hello:

  ```json
  {"helper":"koinon-voice-kokoro","version":"0.1.0","capabilities":["synthesize"],
   "info":{"model":"kokoro-v1_0.safetensors",
           "model_sha256":"4e9ecdf03b8b6cf906070390237feda473dc13327cb8d56a43deaa374c02acd8",
           "models":"~/.local/share/koinon-voice/kokoro","sample_rate":24000,"voices":["af_heart"]}}
  ```

## Feasibility steps

1. **Build.** A command-line tool on kokoro-ios 1.0.11 built with `xcodebuild -scheme spike
   -configuration Release -destination 'platform=macOS' -skipMacroValidation build` once Xcode
   and its Metal Toolchain were installed. Plain `swift build` compiles but has no Metal
   library at run time.
2. **Phase 0's files.** `KokoroTTS(modelPath:)` loads `kokoro-v1_0.safetensors` above, and
   `MLX.loadArrays(url:)` loads `voices/af_heart.safetensors` directly (one array, `voice`,
   shape [510, 1, 256]). `voices.npz` of KokoroTestApp is not needed.
3. **Against Phase 0** (`~/.local/share/koinon-voice/bench/tts/01-kokoro.wav`), text 01 at
   speed 1.0 in one call: 15.30 s against 15.32 s; peak 0.71 against 0.72; clicks 8, 4 and 7
   in three runs against 3. Kokoro's output differs from run to run, so the clicks do too.
   The jumps fall at 3.8–3.9 s and 9.2 s, where Phase 0's fall at 3.8 s and 9.1 s.
4. **Stopping.** Text 01 at 1.0, each sentence in a new process: 900, 731 and 636 ms to
   synthesize sentences of 8.60, 4.88 and 2.23 s. SIGTERM or SIGKILL during the longest sentence ended
   the process in about 11 ms. A new process loaded the model and voice in 37 ms and
   synthesized its first sentence in 829 ms. The first synthesis after the build took 5.8 s
   (MLX's first setup on this Mac); later new processes did not.

## Check 12 — Native engine

During check 17's synthesis, `ps -axo pid,ppid,comm` listed `koinon-voice-kokoro` (its parent
the Go test) and no process with "python" in its name. The hello reports the model's
SHA-256 `4e9ecdf0…` above.

Start-up of a new helper process with the model in the file cache: hello in 178 ms (the
helper maps the model file to hash it); the first synthesis then waits for the helper's
warm-up, and its first audio came 805 ms after its request.

## Check 13 — Measured again

`koinon-voice bench tts --engine kokoro --rate=false --speed S --dir
~/.local/share/koinon-voice/bench/tts-native-S` on the six texts of `tts-summaries.md`. The
native engine sends one request per sentence; Phase 0's mlx-audio took each text in one
call. First audio is the first sentence's, as the helper measured it from its request.
Clicks are sample-to-sample jumps above 0.3 of full scale, counted in Phase 0's files the same
way.

| Text | Phase 0 first audio | Phase 0 audio | Phase 0 clicks | 1.0 first audio | 1.0 audio | 1.0 clicks | 1.1 first audio | 1.1 audio | 1.1 clicks |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 01 | 818 ms | 15.3 s | 3 | 1043 ms | 15.7 s | 44 | 394 ms | 14.8 s | 56 |
| 02 | 849 ms | 16.5 s | 81 | 708 ms | 17.2 s | 55 | 199 ms | 16.1 s | 69 |
| 03 | 612 ms | 17.4 s | 16 | 1177 ms | 18.0 s | 15 | 599 ms | 17.0 s | 18 |
| 04 | 633 ms | 17.8 s | 19 | 1179 ms | 18.9 s | 47 | 1648 ms | 18.1 s | 54 |
| 05 | 756 ms | 22.0 s | 20 | 2152 ms | 23.2 s | 38 | 1750 ms | 22.2 s | 30 |
| 06 | 934 ms | 25.2 s | 45 | 1094 ms | 27.2 s | 24 | 1161 ms | 25.9 s | 15 |
| Median or total | 787 ms | | 184 | 1136 ms | | 223 | 880 ms | | 242 |

Text 01 at 1.0 has 44 clicks, against 4 to 8 when the same text went in one call (step 3).
42 of them are in two bursts in sentence 2, "The options are to merge or not merge, and Codex
recommends merging it.", at about 0.9 s and 3.8 s into it: samples of up to 0.32 of full
scale that change sign from one sample to the next. That sentence alone gave 38, 43 and 43
clicks in three runs; its phonemes for "merge" and "merging" are `mˈɜɹʤ` and `mˈɜɹʤɪŋ`. Phase 0's
own files have 81 clicks in text 02 and 45 in text 06; the maintainer rated those 5 and 4.
Whether the bursts can be heard is the maintainer's to say in the blind rating (check 14,
chunk 02).

## Check 16 — Names

Phonemes from MisakiSwift 1.0.6, before and after the helper's fixed entry for "Koinon":

| Word | Misaki | The helper |
| --- | --- | --- |
| Koinon (capital) | `kOˈinˌɑn` ("ko-EE-non") | `kˈYnˌɑn` |
| koinon (lower case) | `kˈYnˌɑn` ("KOY-non") | `kˈYnˌɑn` |
| Codex | `kˈOdˌɛks` | `kˈOdˌɛks` |
| Claude | `klˈɔd` | `klˈɔd` |
| claude-koinon-voice-b9 | `klˈɔd—kˈYnˌɑn—vˈYs—bˈi`, or with `nˈIn` for the 9 in another sentence | unchanged |

The maintainer listened with `~/.local/share/koinon-voice/kokoro-spike/listen.sh`, which plays
each clip and records the answers (DECISIONS.md#kv-12-listen). The answers, 2026-10-29
19:54 UTC:

- The voice at speed 1.1: acceptable.
- "Koinon": clip B, the lower-case `kˈYnˌɑn`.
- The session name `claude-koinon-voice-b9`: acceptable.
- "Codex": acceptable.
- "Claude": acceptable, with the comment "Koinon was mispronounced". The clip was "Codex and
  Claude are waiting in Koinon.", with the capital `kOˈinˌɑn`.

The helper gives "Koinon" `kˈYnˌɑn` through Misaki's inline form `[Koinon](/kˈYnˌɑn/)`
(`helpers/kokoro`, `lexicon`); a word joined by hyphens, as in a session name, is left to
Misaki. Codex's review of PR #34 found that MisakiSwift 1.0.6 reads the position of an inline
form in UTF-16 units as a position in characters: after an emoji, the form made Misaki stop
("String index out of bounds"). With the patch, Misaki reads the position as a range of the
text, and "🙂 Koinon" gives `ˈɛks kˈYnˌɑn`. The same review found that a link inside a link
set phonemes: `[[approve](/x/)](/dɪklˈYn/)` gave `dɪklˈYn`; it now gives `əpɹˈuv`.
`koinon-voice-kokoro check-text` runs these sentences through MisakiSwift on each build. In the helper, "Codex and Claude are waiting in Koinon."
gives `kˈOdˌɛks ænd klˈɔd ɑɹ wˈATɪŋ ɪn kˈYnˌɑn.`

## Check 17 — Stability

One helper process, 100 runs of text 01's three sentences (300 `synthesize` requests) at
speed 1.1, through `internal/sidecar`'s `OpenAudio`: 70 s, slowest run 1.335 s, no crash, no
error, no dropped frame; the helper's resident memory afterwards was 503,136 KiB.

## Check 18 — Later platforms

- kokoro-ios README, "Requirements": "iOS 18.0+", "macOS 15.0+"; its `Package.swift`:
  `.iOS(.v18), .macOS(.v15)`. The definition of done notes that it does not run in the iOS
  simulator.
- MisakiSwift README, "Supported Platforms": "iOS 18.0+", "macOS 15.0+"; its `Package.swift`:
  `.iOS(.v18), .macOS(.v15)`.
- mlx-swift 0.30.2 `Package.swift`: `.macOS("14.0")`, `.iOS(.v17)`, `.tvOS(.v17)`, and its
  README lists example apps "that run on both iOS and macOS".

Swift packages declare iPadOS as `.iOS`. Nothing was built for iOS.
