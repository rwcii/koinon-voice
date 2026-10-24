# Scorecard — Speech

Chunk 04's live checks 2 and 3 (definition of done): every recognition engine on the
maintainer's own reading of the 60 phrases in `corpus-phrases.md`, clean and with background
noise, and Apple speech synthesis and Kokoro on the six texts in `tts-summaries.md`, rated
blind by the maintainer. The recordings and the synthesized audio stay under
`~/.local/share/koinon-voice/` and are not in Git (definition of done, Private data); this file
holds scores, transcripts of the committed phrases, versions and hashes.

## Run

- Date: 2026-10-23, on the maintainer's Mac (Apple M2 Max, macOS 26.5, build 25F71), Go
  1.27.3, one request at a time; each engine's first request loads its model and is not
  scored.
- Corpus: read by the maintainer with `koinon-voice corpus record`, through the EMEET
  SmartCam Nova 4K's microphone (channel 1 of 2), 16 kHz mono 16-bit. The first takes, through
  the MacBook Pro microphone and then a 14-channel USB mixer (silent until one channel was
  chosen, then peaking at 5–6%), were replaced, and the clean set was then recorded again from
  the start. `clean`: 60 files, 245 s, manifest SHA-256 prefix `2bf760cf1256f545`. `noisy`:
  60 files, 244 s, with background noise playing, manifest SHA-256 prefix
  `b233dcdc5bae5ef0`. Clean phrase 31 was recorded again with a longer window after its
  first take cut off "the spike is done".
- Apple: the helper `koinon-voice-apple` 0.1.0, Speech framework 3525.2.2, `SpeechTranscriber`
  and `DictationTranscriber` (preset `shortDictation`) for `en-US`, on the device. Apple gives
  no version for a locale's speech assets; their status was `supported`.
- Bench helper (`helpers/bench`), in a Python 3.11.17 virtual environment outside the
  repository, with `NEEDLE_TELEMETRY=0`, `DO_NOT_TRACK=1` and `HF_HUB_DISABLE_TELEMETRY=1`:
  - Parakeet: `mlx-community/parakeet-tdt-0.6b-v2` at revision
    `8ae155301e23d820d82aa60d24817c900e69e487`, `model.safetensors` SHA-256 prefix
    `b958c37a6baa6874`, through `mlx-audio` 0.6.1 on `mlx` 0.32.4.
  - Whistle: `cactus-needle` 3.2.8, `whistle.cact` 2.0.0 (16.9 MB, SHA-256 prefix
    `b6e02f048568ac5d`) from `Cactus-Compute/whistle` at revision
    `741118687e1f008f06d1351a402a731274789790`, engine library `libneedle.dylib` 3.3.3
    (SHA-256 prefix `0137354250a85fa5`), language fixed to English.
  - Kokoro: `mlx-community/Kokoro-82M-bf16` at revision
    `a71e4d38b236d968966a2002c4c895dbd12b1c3c`, `kokoro-v1_0.safetensors` SHA-256 prefix
    `4e9ecdf03b8b6cf9`, voice `af_heart` (prefix `2c1c733b0e6576c8`), speed 1.0, through
    `mlx-audio` 0.6.1 with `misaki[en]` 0.9.4 for text processing.
- Keyword biasing: `whistle-keywords`, `apple-keywords` and `apple-dictation-keywords` get the
  same list, seeded from the phrase list's peer and repository names: Koinon, claude koinon
  voice b 9, codex koinon voice 38, codex koinon voice af, deepseek koinon 7 c, koinon voice,
  rwcii. Parakeet through `mlx-audio` takes no list.
- Latency is the bench's time from request to transcript for one whole file, with the model
  loaded; real-time factor is that over the file's length.

## Reading the check

Word error rate is substitutions, deletions and insertions over the reference words, after
the normalization the phrase list describes. The Koinon-term rate counts only errors on the
bracketed words, including a word inserted inside a term, and not words inserted around one.
Spellings that are the same spoken word are equal: British and American spellings
("summarise"), an ordinal written as digits ("1st"), "good night" and "goodnight", and an
issue number split by a comma ("four, 64"); these were found in the first run and the bench
was run again.

## Observations

- **No engine hears "Koinon" without a list.** Apple, Parakeet and Whistle got it in 0 of 17
  places in each set ("Quinan", "Coinan", "Coynan", "coin in"). With the list, Whistle got it
  in 13 of 17 clean and 12 of 17 noisy, and Apple's `DictationTranscriber` in 9 and 12.
  `SpeechTranscriber` ignores contextual strings: `apple-keywords` is word for word the
  same as `apple`, with the context set before and at the analyzer's creation.
- **Parakeet is the most accurate overall**: 11.1% clean and 12.7% noisy, at 40 ms per
  phrase. Apple's `SpeechTranscriber` is close (11.4% and 14.3%, 87 ms). Both miss every
  Koinon name, so their Koinon-term rate stays near 30%.
- **Whistle with the list is the best on Koinon terms** (12.6% clean, 24.1% noisy) but worse
  on ordinary words (16.3%, 21.2%), and it is the fastest (16 ms). It also hears the decision
  verbs worst: "approve", "reject", "confirm" and "merge" in 8 of 13 clean and 5 of 13 noisy
  ("I prove it", "Rejected", "Confirmed me"), against 13 and 11 or 12 for Apple and Parakeet.
  For a bridge that acts on "approve", that is a safety concern, not only an error rate.
- **Noise costs Whistle the most**: its unbiased rate goes from 25.4% to 35.5%; Parakeet's
  from 11.1% to 12.7%.
- `DictationTranscriber` takes the list but is no more accurate overall (13.0%, 15.3%) and is
  slower (about 210 ms).
- No engine failed a file. Letters in peer names come back joined or split ("AF", "a. f.")
  and are scored the same.

## Recognition

### Engines

| Engine | Helper | Info | Note |
| --- | --- | --- | --- |
| apple | koinon-voice-apple 0.1.0 | {"foundation_models":"1.5.2","model":"available","os":"Version 26.5 (Build 25F71)","speech":"3525.2.2","transcriber_available":true,"voice":"com.apple.voice.premium.en-US.Ava"} | SpeechAnalyzer, en-US |
| apple-keywords | koinon-voice-apple 0.1.0 | {"foundation_models":"1.5.2","model":"available","os":"Version 26.5 (Build 25F71)","speech":"3525.2.2","transcriber_available":true,"voice":"com.apple.voice.premium.en-US.Ava"} | SpeechAnalyzer, en-US, contextual strings: the Whistle biasing list |
| apple-dictation-keywords | koinon-voice-apple 0.1.0 | {"foundation_models":"1.5.2","model":"available","os":"Version 26.5 (Build 25F71)","speech":"3525.2.2","transcriber_available":true,"voice":"com.apple.voice.premium.en-US.Ava"} | SpeechAnalyzer with DictationTranscriber (short dictation), en-US, contextual strings: the Whistle biasing list |
| parakeet | koinon-voice-bench-parakeet 0.1.0 | {"engine": "parakeet", "model": "mlx-community/parakeet-tdt-0.6b-v2", "package": "mlx-audio", "package_version": "0.6.1", "python": "3.11.17"} | Parakeet TDT 0.6B v2 through mlx-audio |
| whistle | koinon-voice-bench-whistle 0.1.0 | {"engine": "whistle", "model": "Cactus-Compute/whistle", "package": "cactus-needle", "package_version": "3.2.8", "python": "3.11.17"} | Cactus Whistle, no biasing |
| whistle-keywords | koinon-voice-bench-whistle 0.1.0 | {"engine": "whistle", "model": "Cactus-Compute/whistle", "package": "cactus-needle", "package_version": "3.2.8", "python": "3.11.17"} | Cactus Whistle, biased to: Koinon, claude koinon voice b 9, codex koinon voice 38, codex koinon voice af, deepseek koinon 7 c, koinon voice, rwcii |

### Totals

| Engine | Set | Files | Failed | WER | Koinon-term WER | Median latency | Median RTF |
| --- | --- | --- | --- | --- | --- | --- | --- |
| apple | clean | 60 | 0 | 11.4% (35/307) | 28.7% (25/87) | 87 ms | 0.024 |
| apple | noisy | 60 | 0 | 14.3% (44/307) | 29.9% (26/87) | 89 ms | 0.023 |
| apple-keywords | clean | 60 | 0 | 11.4% (35/307) | 28.7% (25/87) | 87 ms | 0.023 |
| apple-keywords | noisy | 60 | 0 | 14.3% (44/307) | 29.9% (26/87) | 90 ms | 0.024 |
| apple-dictation-keywords | clean | 60 | 0 | 13.0% (40/307) | 32.2% (28/87) | 208 ms | 0.058 |
| apple-dictation-keywords | noisy | 60 | 0 | 15.3% (47/307) | 28.7% (25/87) | 214 ms | 0.058 |
| parakeet | clean | 60 | 0 | 11.1% (34/307) | 29.9% (26/87) | 40 ms | 0.011 |
| parakeet | noisy | 60 | 0 | 12.7% (39/307) | 33.3% (29/87) | 40 ms | 0.010 |
| whistle | clean | 60 | 0 | 25.4% (78/307) | 37.9% (33/87) | 16 ms | 0.004 |
| whistle | noisy | 60 | 0 | 35.5% (109/307) | 56.3% (49/87) | 16 ms | 0.004 |
| whistle-keywords | clean | 60 | 0 | 16.3% (50/307) | 12.6% (11/87) | 16 ms | 0.004 |
| whistle-keywords | noisy | 60 | 0 | 21.2% (65/307) | 24.1% (21/87) | 16 ms | 0.004 |

### Transcripts

Errors are word errors over reference words, then the same on Koinon terms.

| Phrase | Set | Engine | Errors | Term errors | Latency | Transcript |
| --- | --- | --- | --- | --- | --- | --- |
| 01 | clean | apple | 1/3 | 1/1 | 89 ms | Quinnan, what's waiting? |
| 01 | clean | apple-keywords | 1/3 | 1/1 | 86 ms | Quinnan, what's waiting? |
| 01 | clean | apple-dictation-keywords | 1/3 | 1/1 | 206 ms | Quinn, what's waiting? |
| 01 | clean | parakeet | 2/3 | 1/1 | 36 ms | Coin in, what's waiting? |
| 01 | clean | whistle | 1/3 | 1/1 | 17 ms | Cornin, what's waiting? |
| 01 | clean | whistle-keywords | 1/3 | 1/1 | 17 ms | Cornin, what's waiting? |
| 01 | noisy | apple | 1/3 | 1/1 | 86 ms | Coinan, what's waiting? |
| 01 | noisy | apple-keywords | 1/3 | 1/1 | 84 ms | Coinan, what's waiting? |
| 01 | noisy | apple-dictation-keywords | 0/3 | 0/1 | 225 ms | Koinon what's waiting? |
| 01 | noisy | parakeet | 1/3 | 1/1 | 37 ms | Coynan, what's waiting? |
| 01 | noisy | whistle | 1/3 | 1/1 | 16 ms | Coinen, what's waiting? |
| 01 | noisy | whistle-keywords | 0/3 | 0/1 | 15 ms | Koinon what's waiting? |
| 02 | clean | apple | 1/4 | 1/1 | 90 ms | Quinin, read my inbox. |
| 02 | clean | apple-keywords | 1/4 | 1/1 | 89 ms | Quinin, read my inbox. |
| 02 | clean | apple-dictation-keywords | 1/4 | 1/1 | 200 ms | Quinn read my inbox |
| 02 | clean | parakeet | 1/4 | 1/1 | 38 ms | Quinin, read my inbox. |
| 02 | clean | whistle | 1/4 | 1/1 | 15 ms | Coinin, read my inbox. |
| 02 | clean | whistle-keywords | 0/4 | 0/1 | 14 ms | Koinon, read my inbox. |
| 02 | noisy | apple | 1/4 | 1/1 | 86 ms | Coinan, read my inbox. |
| 02 | noisy | apple-keywords | 1/4 | 1/1 | 88 ms | Coinan, read my inbox. |
| 02 | noisy | apple-dictation-keywords | 1/4 | 1/1 | 214 ms | Coen read my inbox |
| 02 | noisy | parakeet | 1/4 | 1/1 | 38 ms | Coynan, read my inbox. |
| 02 | noisy | whistle | 2/4 | 1/1 | 14 ms | Coin and read my inbox. |
| 02 | noisy | whistle-keywords | 0/4 | 0/1 | 16 ms | Koinon, read my inbox. |
| 03 | clean | apple | 0/2 | 0/1 | 78 ms | Approve it. |
| 03 | clean | apple-keywords | 0/2 | 0/1 | 74 ms | Approve it. |
| 03 | clean | apple-dictation-keywords | 0/2 | 0/1 | 187 ms | Approve it |
| 03 | clean | parakeet | 0/2 | 0/1 | 34 ms | Approve it. |
| 03 | clean | whistle | 2/2 | 1/1 | 12 ms | I prove it. |
| 03 | clean | whistle-keywords | 2/2 | 1/1 | 12 ms | I prove it. |
| 03 | noisy | apple | 0/2 | 0/1 | 71 ms | Approve it. |
| 03 | noisy | apple-keywords | 0/2 | 0/1 | 74 ms | Approve it. |
| 03 | noisy | apple-dictation-keywords | 0/2 | 0/1 | 187 ms | Approve it |
| 03 | noisy | parakeet | 0/2 | 0/1 | 35 ms | Approve it. |
| 03 | noisy | whistle | 2/2 | 1/1 | 12 ms | I prove it. |
| 03 | noisy | whistle-keywords | 2/2 | 1/1 | 13 ms | I prove it. |
| 04 | clean | apple | 0/2 | 0/1 | 77 ms | Reject it. |
| 04 | clean | apple-keywords | 0/2 | 0/1 | 71 ms | Reject it. |
| 04 | clean | apple-dictation-keywords | 2/2 | 1/1 | 188 ms | Rejected |
| 04 | clean | parakeet | 0/2 | 0/1 | 33 ms | Reject it. |
| 04 | clean | whistle | 2/2 | 1/1 | 10 ms | Rejected. |
| 04 | clean | whistle-keywords | 2/2 | 1/1 | 9 ms | Rejected. |
| 04 | noisy | apple | 0/2 | 0/1 | 73 ms | Reject it. |
| 04 | noisy | apple-keywords | 0/2 | 0/1 | 91 ms | Reject it. |
| 04 | noisy | apple-dictation-keywords | 2/2 | 1/1 | 189 ms | Rejected |
| 04 | noisy | parakeet | 0/2 | 0/1 | 35 ms | Reject it. |
| 04 | noisy | whistle | 2/2 | 1/1 | 10 ms | Rejected. |
| 04 | noisy | whistle-keywords | 2/2 | 1/1 | 10 ms | Rejected. |
| 05 | clean | apple | 0/1 | 0/1 | 78 ms | Confirm |
| 05 | clean | apple-keywords | 0/1 | 0/1 | 77 ms | Confirm |
| 05 | clean | apple-dictation-keywords | 0/1 | 0/1 | 179 ms | Confirm |
| 05 | clean | parakeet | 0/1 | 0/1 | 33 ms | Confirm. |
| 05 | clean | whistle | 0/1 | 0/1 | 8 ms | Confirm. |
| 05 | clean | whistle-keywords | 0/1 | 0/1 | 7 ms | Confirm. |
| 05 | noisy | apple | 0/1 | 0/1 | 73 ms | Confirm |
| 05 | noisy | apple-keywords | 0/1 | 0/1 | 76 ms | Confirm |
| 05 | noisy | apple-dictation-keywords | 0/1 | 0/1 | 193 ms | Confirm |
| 05 | noisy | parakeet | 0/1 | 0/1 | 34 ms | Confirm. |
| 05 | noisy | whistle | 2/1 | 1/1 | 10 ms | Confirmed me. |
| 05 | noisy | whistle-keywords | 2/1 | 1/1 | 11 ms | Confirmed me. |
| 06 | clean | apple | 0/2 | 0/0 | 76 ms | Cancel that. |
| 06 | clean | apple-keywords | 0/2 | 0/0 | 72 ms | Cancel that. |
| 06 | clean | apple-dictation-keywords | 0/2 | 0/0 | 190 ms | Cancel that |
| 06 | clean | parakeet | 0/2 | 0/0 | 36 ms | Cancel that. |
| 06 | clean | whistle | 2/2 | 0/0 | 12 ms | CANSOL BET. |
| 06 | clean | whistle-keywords | 2/2 | 0/0 | 13 ms | CANSOL BET. |
| 06 | noisy | apple | 0/2 | 0/0 | 70 ms | Cancel that. |
| 06 | noisy | apple-keywords | 0/2 | 0/0 | 72 ms | Cancel that. |
| 06 | noisy | apple-dictation-keywords | 0/2 | 0/0 | 184 ms | Cancel that |
| 06 | noisy | parakeet | 0/2 | 0/0 | 34 ms | Cancel that. |
| 06 | noisy | whistle | 0/2 | 0/0 | 21 ms | Cancel that. |
| 06 | noisy | whistle-keywords | 0/2 | 0/0 | 9 ms | Cancel that. |
| 07 | clean | apple | 0/3 | 0/0 | 73 ms | Read it again. |
| 07 | clean | apple-keywords | 0/3 | 0/0 | 74 ms | Read it again. |
| 07 | clean | apple-dictation-keywords | 0/3 | 0/0 | 185 ms | Read it again |
| 07 | clean | parakeet | 0/3 | 0/0 | 34 ms | Read it again? |
| 07 | clean | whistle | 0/3 | 0/0 | 9 ms | Read it again. |
| 07 | clean | whistle-keywords | 0/3 | 0/0 | 10 ms | Read it again. |
| 07 | noisy | apple | 0/3 | 0/0 | 71 ms | Read it again. |
| 07 | noisy | apple-keywords | 0/3 | 0/0 | 77 ms | Read it again. |
| 07 | noisy | apple-dictation-keywords | 0/3 | 0/0 | 185 ms | Read it again |
| 07 | noisy | parakeet | 0/3 | 0/0 | 36 ms | Read it again. |
| 07 | noisy | whistle | 2/3 | 0/0 | 18 ms | Really again. |
| 07 | noisy | whistle-keywords | 2/3 | 0/0 | 11 ms | Really again. |
| 08 | clean | apple | 0/3 | 0/0 | 74 ms | Skip this one. |
| 08 | clean | apple-keywords | 0/3 | 0/0 | 72 ms | Skip this one. |
| 08 | clean | apple-dictation-keywords | 0/3 | 0/0 | 181 ms | Skip this one |
| 08 | clean | parakeet | 0/3 | 0/0 | 33 ms | Skip this one. |
| 08 | clean | whistle | 0/3 | 0/0 | 12 ms | Skip this one. |
| 08 | clean | whistle-keywords | 0/3 | 0/0 | 11 ms | Skip this one. |
| 08 | noisy | apple | 0/3 | 0/0 | 70 ms | Skip this one. |
| 08 | noisy | apple-keywords | 0/3 | 0/0 | 75 ms | Skip this one. |
| 08 | noisy | apple-dictation-keywords | 0/3 | 0/0 | 183 ms | Skip this one |
| 08 | noisy | parakeet | 0/3 | 0/0 | 34 ms | Skip this one. |
| 08 | noisy | whistle | 0/3 | 0/0 | 12 ms | Skip this one. |
| 08 | noisy | whistle-keywords | 0/3 | 0/0 | 11 ms | Skip this one. |
| 09 | clean | apple | 0/4 | 0/2 | 82 ms | Approve pull request 12. |
| 09 | clean | apple-keywords | 0/4 | 0/2 | 82 ms | Approve pull request 12. |
| 09 | clean | apple-dictation-keywords | 1/4 | 0/2 | 200 ms | Approve pool request 12 |
| 09 | clean | parakeet | 0/4 | 0/2 | 39 ms | Approve pull request twelve. |
| 09 | clean | whistle | 3/4 | 1/2 | 17 ms | A proof pull requests twelve. |
| 09 | clean | whistle-keywords | 3/4 | 1/2 | 16 ms | A proof pull requests twelve. |
| 09 | noisy | apple | 1/4 | 0/2 | 81 ms | Approve pool request 12. |
| 09 | noisy | apple-keywords | 1/4 | 0/2 | 82 ms | Approve pool request 12. |
| 09 | noisy | apple-dictation-keywords | 2/4 | 1/2 | 265 ms | Approve pool request file |
| 09 | noisy | parakeet | 0/4 | 0/2 | 40 ms | Approve pull request twelve. |
| 09 | noisy | whistle | 4/4 | 2/2 | 15 ms | A proof pull requests file. |
| 09 | noisy | whistle-keywords | 4/4 | 2/2 | 14 ms | A proof pull requests file. |
| 10 | clean | apple | 0/10 | 0/2 | 119 ms | Reject, pull request 38, and ask for a smaller change. |
| 10 | clean | apple-keywords | 0/10 | 0/2 | 121 ms | Reject, pull request 38, and ask for a smaller change. |
| 10 | clean | apple-dictation-keywords | 1/10 | 0/2 | 270 ms | Reject request 38 and ask for a smaller change |
| 10 | clean | parakeet | 0/10 | 0/2 | 53 ms | Reject pull request thirty-eight and ask for a smaller change. |
| 10 | clean | whistle | 5/10 | 0/2 | 28 ms | Reject pulver quests thirty-eight and asphor smaller change. |
| 10 | clean | whistle-keywords | 5/10 | 0/2 | 28 ms | Reject pulver quests thirty-eight and asphor smaller change. |
| 10 | noisy | apple | 2/10 | 1/2 | 122 ms | Rejectable request 38 and ask for a smaller change. |
| 10 | noisy | apple-keywords | 2/10 | 1/2 | 124 ms | Rejectable request 38 and ask for a smaller change. |
| 10 | noisy | apple-dictation-keywords | 1/10 | 0/2 | 433 ms | Reject request 38 and ask for a smaller change |
| 10 | noisy | parakeet | 0/10 | 0/2 | 56 ms | Reject pull request thirty-eight and ask for a smaller change. |
| 10 | noisy | whistle | 4/10 | 1/2 | 28 ms | Rejected Corequest 38 and asked for a smaller change. |
| 10 | noisy | whistle-keywords | 4/10 | 1/2 | 28 ms | Rejected Corequest 38 and asked for a smaller change. |
| 11 | clean | apple | 1/7 | 1/4 | 110 ms | Merge pull request five on Quentin voice. |
| 11 | clean | apple-keywords | 1/7 | 1/4 | 107 ms | Merge pull request five on Quentin voice. |
| 11 | clean | apple-dictation-keywords | 2/7 | 2/4 | 280 ms | Merge pull request five on coin invoice |
| 11 | clean | parakeet | 2/7 | 2/4 | 45 ms | Merge pull request five on coin invoice |
| 11 | clean | whistle | 4/7 | 1/4 | 23 ms | Merge Puller requests five unpointed voice. |
| 11 | clean | whistle-keywords | 4/7 | 1/4 | 23 ms | Merge Puller requests five unpointed voice. |
| 11 | noisy | apple | 2/7 | 2/4 | 102 ms | Merge pull request five on coin invoice. |
| 11 | noisy | apple-keywords | 2/7 | 2/4 | 104 ms | Merge pull request five on coin invoice. |
| 11 | noisy | apple-dictation-keywords | 3/7 | 2/4 | 242 ms | Merge request five on coin invoice |
| 11 | noisy | parakeet | 2/7 | 2/4 | 43 ms | Merge pull request five on coin invoice |
| 11 | noisy | whistle | 5/7 | 2/4 | 19 ms | Mergical requests five unpointed voice. |
| 11 | noisy | whistle-keywords | 5/7 | 2/4 | 21 ms | Mergical requests five unpointed voice. |
| 12 | clean | apple | 2/6 | 2/3 | 97 ms | Open issue one on Quinn invoice. |
| 12 | clean | apple-keywords | 2/6 | 2/3 | 97 ms | Open issue one on Quinn invoice. |
| 12 | clean | apple-dictation-keywords | 3/6 | 2/3 | 266 ms | Open issue one unwanted invoice |
| 12 | clean | parakeet | 2/6 | 2/3 | 44 ms | Open issue one on Coininvoice. |
| 12 | clean | whistle | 3/6 | 1/3 | 17 ms | Open issue one and coin in voice. |
| 12 | clean | whistle-keywords | 3/6 | 1/3 | 17 ms | Open issue one and coin in voice. |
| 12 | noisy | apple | 2/6 | 1/3 | 96 ms | Open issue one, uncoin voice. |
| 12 | noisy | apple-keywords | 2/6 | 1/3 | 98 ms | Open issue one, uncoin voice. |
| 12 | noisy | apple-dictation-keywords | 1/6 | 0/3 | 317 ms | Open issue one in koinon voice |
| 12 | noisy | parakeet | 2/6 | 2/3 | 42 ms | Open issue one on CoinVoice. |
| 12 | noisy | whistle | 3/6 | 2/3 | 16 ms | Open issue one and coin boys. |
| 12 | noisy | whistle-keywords | 3/6 | 2/3 | 16 ms | Open issue one and coin boys. |
| 13 | clean | apple | 1/7 | 1/4 | 115 ms | What did Codex Quinin voice a. f. ask? |
| 13 | clean | apple-keywords | 1/7 | 1/4 | 117 ms | What did Codex Quinin voice a. f. ask? |
| 13 | clean | apple-dictation-keywords | 3/7 | 3/4 | 317 ms | What did Kodex Quinnen invoice AF ask |
| 13 | clean | parakeet | 3/7 | 3/4 | 46 ms | What did Kodak's coin invoice Af ask? |
| 13 | clean | whistle | 3/7 | 2/4 | 24 ms | What did Codex coin in voice AF passed? |
| 13 | clean | whistle-keywords | 1/7 | 0/4 | 24 ms | What did Codex Koinon Voice Af has? |
| 13 | noisy | apple | 2/7 | 2/4 | 111 ms | What did Codex Quinn invoice AF ask? |
| 13 | noisy | apple-keywords | 2/7 | 2/4 | 115 ms | What did Codex Quinn invoice AF ask? |
| 13 | noisy | apple-dictation-keywords | 5/7 | 4/4 | 289 ms | What did Kodak Quinn invoice AFS? |
| 13 | noisy | parakeet | 3/7 | 3/4 | 45 ms | What did Kodak's coin invoice Af ask? |
| 13 | noisy | whistle | 6/7 | 4/4 | 24 ms | What didn't produce queen in boys' airbask? |
| 13 | noisy | whistle-keywords | 5/7 | 4/4 | 26 ms | What did Kurdak's queen and boys airbask? |
| 14 | clean | apple | 3/9 | 3/5 | 118 ms | Reply to Claude Quinnan voice benign. looks good. |
| 14 | clean | apple-keywords | 3/9 | 3/5 | 118 ms | Reply to Claude Quinnan voice benign. looks good. |
| 14 | clean | apple-dictation-keywords | 0/9 | 0/5 | 249 ms | Reply to claude koinon voice b 9 looks good |
| 14 | clean | parakeet | 1/9 | 1/5 | 55 ms | Reply to Claude Coynan Voice b nine looks good. |
| 14 | clean | whistle | 4/9 | 2/5 | 30 ms | Reply the clod coin in voice B9 looks good. |
| 14 | clean | whistle-keywords | 1/9 | 0/5 | 28 ms | Reply the Claude Koinon Voice B 9 looks good. |
| 14 | noisy | apple | 1/9 | 1/5 | 112 ms | Reply to Claude Coinan voice B9, looks good. |
| 14 | noisy | apple-keywords | 1/9 | 1/5 | 118 ms | Reply to Claude Coinan voice B9, looks good. |
| 14 | noisy | apple-dictation-keywords | 0/9 | 0/5 | 238 ms | Reply to claude koinon voice b 9 looks good |
| 14 | noisy | parakeet | 3/9 | 3/5 | 55 ms | Reply to Claude Coynan Voice benine: Looks good |
| 14 | noisy | whistle | 6/9 | 4/5 | 24 ms | Replied a clod-pointed voice benign looks good. |
| 14 | noisy | whistle-keywords | 1/9 | 0/5 | 29 ms | Reply the Claude Koinon Voice B 9 looks good. |
| 15 | clean | apple | 3/9 | 2/4 | 115 ms | Tell Deep Sea Coinin 7 C to wait until Friday. |
| 15 | clean | apple-keywords | 3/9 | 2/4 | 110 ms | Tell Deep Sea Coinin 7 C to wait until Friday. |
| 15 | clean | apple-dictation-keywords | 0/9 | 0/4 | 246 ms | Tell DeepSeek Koinon 7C to wait until Friday |
| 15 | clean | parakeet | 3/9 | 2/4 | 55 ms | Tell Deep Sea Coinin Seven C to wait until Friday |
| 15 | clean | whistle | 5/9 | 3/4 | 28 ms | Tell Deep Sea coin in Seven Sea to wait until Friday. |
| 15 | clean | whistle-keywords | 0/9 | 0/4 | 29 ms | Tell Deepseek Koinon 7 C to wait until Friday. |
| 15 | noisy | apple | 4/9 | 2/4 | 115 ms | Tell Deep Sea coin in 7C to wait until Friday. |
| 15 | noisy | apple-keywords | 4/9 | 2/4 | 116 ms | Tell Deep Sea coin in 7C to wait until Friday. |
| 15 | noisy | apple-dictation-keywords | 2/9 | 1/4 | 251 ms | Tell deep sea Koinon 7C to wait until Friday |
| 15 | noisy | parakeet | 2/9 | 1/4 | 55 ms | Tell Deepseek Coinin seven C to wait till Friday |
| 15 | noisy | whistle | 4/9 | 3/4 | 25 ms | Tell Deepsy Coin and Seven Sea to wait until Friday. |
| 15 | noisy | whistle-keywords | 0/9 | 0/4 | 28 ms | Tell Deepseek Koinon 7 C to wait until Friday. |
| 16 | clean | apple | 2/10 | 2/5 | 125 ms | Send codex coin and voice 38 a message, start chunk four. |
| 16 | clean | apple-keywords | 2/10 | 2/5 | 126 ms | Send codex coin and voice 38 a message, start chunk four. |
| 16 | clean | apple-dictation-keywords | 2/10 | 1/5 | 255 ms | Send codex koinon voice 38 a message start trunk for |
| 16 | clean | parakeet | 2/10 | 2/5 | 56 ms | Send Codex Coininvoice thirty eight a message start chunk four |
| 16 | clean | whistle | 3/10 | 2/5 | 33 ms | Send Codex coin in Voice 38 to message: Start Chunk 4. |
| 16 | clean | whistle-keywords | 1/10 | 0/5 | 33 ms | Send Codex Koinon Voice 38 to message: Start Chunk 4. |
| 16 | noisy | apple | 2/10 | 2/5 | 126 ms | Send Codex Quinn in voice 38 a message, start chunk four. |
| 16 | noisy | apple-keywords | 2/10 | 2/5 | 133 ms | Send Codex Quinn in voice 38 a message, start chunk four. |
| 16 | noisy | apple-dictation-keywords | 1/10 | 1/5 | 281 ms | Send codex koinon voice 38 a message start chunk for |
| 16 | noisy | parakeet | 2/10 | 2/5 | 58 ms | Send codex coininvoice thirty eight a message: start chunk four. |
| 16 | noisy | whistle | 5/10 | 3/5 | 35 ms | Sand Podex coined in voice 38 a message, "Start Chunk For". |
| 16 | noisy | whistle-keywords | 0/10 | 0/5 | 32 ms | Send Codex Koinon Voice 38 a message, Start Chunk 4. |
| 17 | clean | apple | 0/3 | 0/0 | 79 ms | Who is online? |
| 17 | clean | apple-keywords | 0/3 | 0/0 | 76 ms | Who is online? |
| 17 | clean | apple-dictation-keywords | 0/3 | 0/0 | 182 ms | Who is online? |
| 17 | clean | parakeet | 0/3 | 0/0 | 34 ms | Who is online? |
| 17 | clean | whistle | 0/3 | 0/0 | 10 ms | Who is online? |
| 17 | clean | whistle-keywords | 0/3 | 0/0 | 10 ms | Who is online? |
| 17 | noisy | apple | 0/3 | 0/0 | 75 ms | Who is online? |
| 17 | noisy | apple-keywords | 0/3 | 0/0 | 76 ms | Who is online? |
| 17 | noisy | apple-dictation-keywords | 0/3 | 0/0 | 187 ms | Who is online? |
| 17 | noisy | parakeet | 0/3 | 0/0 | 34 ms | Who is online? |
| 17 | noisy | whistle | 0/3 | 0/0 | 12 ms | Who is online? |
| 17 | noisy | whistle-keywords | 0/3 | 0/0 | 11 ms | Who is online? |
| 18 | clean | apple | 0/6 | 0/0 | 86 ms | Which peers are waiting on me? |
| 18 | clean | apple-keywords | 0/6 | 0/0 | 86 ms | Which peers are waiting on me? |
| 18 | clean | apple-dictation-keywords | 0/6 | 0/0 | 205 ms | Which peers are waiting on me |
| 18 | clean | parakeet | 0/6 | 0/0 | 42 ms | Which peers are waiting on me? |
| 18 | clean | whistle | 0/6 | 0/0 | 16 ms | Which peers are waiting on me? |
| 18 | clean | whistle-keywords | 0/6 | 0/0 | 16 ms | Which peers are waiting on me? |
| 18 | noisy | apple | 0/6 | 0/0 | 94 ms | Which peers are waiting on me? |
| 18 | noisy | apple-keywords | 0/6 | 0/0 | 96 ms | Which peers are waiting on me? |
| 18 | noisy | apple-dictation-keywords | 0/6 | 0/0 | 213 ms | Which peers are waiting on me |
| 18 | noisy | parakeet | 0/6 | 0/0 | 41 ms | Which peers are waiting on me? |
| 18 | noisy | whistle | 1/6 | 0/0 | 20 ms | Which peers are waiting on making? |
| 18 | noisy | whistle-keywords | 1/6 | 0/0 | 18 ms | Which peers are waiting on making? |
| 19 | clean | apple | 2/7 | 2/4 | 101 ms | Is Codex Coin and Voice AF still running? |
| 19 | clean | apple-keywords | 2/7 | 2/4 | 101 ms | Is Codex Coin and Voice AF still running? |
| 19 | clean | apple-dictation-keywords | 1/7 | 1/4 | 236 ms | Is Kodex koinon voice AF still running? |
| 19 | clean | parakeet | 2/7 | 2/4 | 43 ms | Is Codex Coin and Voice Af still running? |
| 19 | clean | whistle | 3/7 | 2/4 | 24 ms | His Codex coin in voice AF still running. |
| 19 | clean | whistle-keywords | 1/7 | 0/4 | 24 ms | His Codex Koinon Voice Af still running. |
| 19 | noisy | apple | 2/7 | 2/4 | 113 ms | Is Codex Coin and Voice AF still running? |
| 19 | noisy | apple-keywords | 2/7 | 2/4 | 108 ms | Is Codex Coin and Voice AF still running? |
| 19 | noisy | apple-dictation-keywords | 1/7 | 1/4 | 279 ms | Is Kodex koinon voice AF still running? |
| 19 | noisy | parakeet | 2/7 | 2/4 | 44 ms | Is Codex Coin Invoice Af still running? |
| 19 | noisy | whistle | 7/7 | 4/4 | 30 ms | His Codex coined in voice A. I've stolen him. |
| 19 | noisy | whistle-keywords | 0/7 | 0/4 | 26 ms | Is Codex Koinon Voice Af still running? |
| 20 | clean | apple | 1/9 | 1/5 | 108 ms | What is Claude Coinan voice B9 working on? |
| 20 | clean | apple-keywords | 1/9 | 1/5 | 113 ms | What is Claude Coinan voice B9 working on? |
| 20 | clean | apple-dictation-keywords | 2/9 | 2/5 | 241 ms | What is Claude koinon voice benign working on? |
| 20 | clean | parakeet | 1/9 | 1/5 | 51 ms | What is Claude Coynan Voice B nine working on? |
| 20 | clean | whistle | 1/9 | 1/5 | 27 ms | What is Claude Coinden Voice B9 working on? |
| 20 | clean | whistle-keywords | 0/9 | 0/5 | 27 ms | What is Claude Koinon Voice B 9 working on? |
| 20 | noisy | apple | 1/9 | 1/5 | 115 ms | What is Claude Quinan voice B9 working on? |
| 20 | noisy | apple-keywords | 1/9 | 1/5 | 133 ms | What is Claude Quinan voice B9 working on? |
| 20 | noisy | apple-dictation-keywords | 0/9 | 0/5 | 249 ms | What is claude koinon voice b 9 working on? |
| 20 | noisy | parakeet | 2/9 | 2/5 | 54 ms | What is Claude Coininvoice b9 working on |
| 20 | noisy | whistle | 3/9 | 3/5 | 26 ms | What is Claude Quinn in voice be nine working on? |
| 20 | noisy | whistle-keywords | 0/9 | 0/5 | 28 ms | What is Claude Koinon Voice B 9 working on? |
| 21 | clean | apple | 1/6 | 1/1 | 92 ms | Quinan, how many items are waiting? |
| 21 | clean | apple-keywords | 1/6 | 1/1 | 96 ms | Quinan, how many items are waiting? |
| 21 | clean | apple-dictation-keywords | 1/6 | 1/1 | 208 ms | Quinn, how many items are waiting? |
| 21 | clean | parakeet | 1/6 | 1/1 | 41 ms | Koinen: How many items are waiting? |
| 21 | clean | whistle | 1/6 | 1/1 | 19 ms | Coinin, how many items are waiting? |
| 21 | clean | whistle-keywords | 0/6 | 0/1 | 19 ms | Koinon, how many items are waiting? |
| 21 | noisy | apple | 1/6 | 1/1 | 98 ms | Coinan, how many items are waiting? |
| 21 | noisy | apple-keywords | 1/6 | 1/1 | 115 ms | Coinan, how many items are waiting? |
| 21 | noisy | apple-dictation-keywords | 0/6 | 0/1 | 281 ms | Koinon how many items are waiting? |
| 21 | noisy | parakeet | 1/6 | 1/1 | 43 ms | Coinance: How many items are waiting? |
| 21 | noisy | whistle | 1/6 | 1/1 | 19 ms | Coinince, how many items are waiting? |
| 21 | noisy | whistle-keywords | 1/6 | 1/1 | 20 ms | Coinince, how many items are waiting? |
| 22 | clean | apple | 0/5 | 0/0 | 87 ms | Read the oldest item first. |
| 22 | clean | apple-keywords | 0/5 | 0/0 | 87 ms | Read the oldest item first. |
| 22 | clean | apple-dictation-keywords | 0/5 | 0/0 | 198 ms | Read the oldest item 1st |
| 22 | clean | parakeet | 0/5 | 0/0 | 39 ms | Read the oldest item first. |
| 22 | clean | whistle | 0/5 | 0/0 | 15 ms | Read the oldest item first. |
| 22 | clean | whistle-keywords | 0/5 | 0/0 | 14 ms | Read the oldest item first. |
| 22 | noisy | apple | 1/5 | 0/0 | 85 ms | Read the oldest items first. |
| 22 | noisy | apple-keywords | 1/5 | 0/0 | 94 ms | Read the oldest items first. |
| 22 | noisy | apple-dictation-keywords | 1/5 | 0/0 | 207 ms | Read the oldest items first |
| 22 | noisy | parakeet | 1/5 | 0/0 | 40 ms | Read the oldest items first. |
| 22 | noisy | whistle | 1/5 | 0/0 | 14 ms | Read the oldest items first. |
| 22 | noisy | whistle-keywords | 1/5 | 0/0 | 14 ms | Read the oldest items first. |
| 23 | clean | apple | 0/6 | 0/1 | 105 ms | Summarize the decision on issue four, 64. |
| 23 | clean | apple-keywords | 0/6 | 0/1 | 107 ms | Summarize the decision on issue four, 64. |
| 23 | clean | apple-dictation-keywords | 2/6 | 1/1 | 229 ms | Summarize the decision on issue for 64 |
| 23 | clean | parakeet | 0/6 | 0/1 | 46 ms | Summarise the decision on issue four, sixty four. |
| 23 | clean | whistle | 2/6 | 0/1 | 22 ms | Summer has the decision on issue four sixty-four. |
| 23 | clean | whistle-keywords | 2/6 | 0/1 | 22 ms | Summer has the decision on issue four sixty-four. |
| 23 | noisy | apple | 0/6 | 0/1 | 105 ms | Summarize the decision on issue 464. |
| 23 | noisy | apple-keywords | 0/6 | 0/1 | 112 ms | Summarize the decision on issue 464. |
| 23 | noisy | apple-dictation-keywords | 2/6 | 1/1 | 209 ms | Summarize the decision on issue for 64 |
| 23 | noisy | parakeet | 0/6 | 0/1 | 44 ms | Summarise the decision on issue four sixty-four. |
| 23 | noisy | whistle | 2/6 | 0/1 | 22 ms | Summer is the decision on issue 464. |
| 23 | noisy | whistle-keywords | 2/6 | 0/1 | 21 ms | Summer is the decision on issue 464. |
| 24 | clean | apple | 0/4 | 0/1 | 90 ms | What does codex recommend? |
| 24 | clean | apple-keywords | 0/4 | 0/1 | 87 ms | What does codex recommend? |
| 24 | clean | apple-dictation-keywords | 1/4 | 1/1 | 193 ms | What does Kodex recommend? |
| 24 | clean | parakeet | 0/4 | 0/1 | 38 ms | What does Codex recommend? |
| 24 | clean | whistle | 0/4 | 0/1 | 13 ms | What does codex recommend? |
| 24 | clean | whistle-keywords | 0/4 | 0/1 | 13 ms | What does codex recommend? |
| 24 | noisy | apple | 0/4 | 0/1 | 94 ms | What does codex recommend? |
| 24 | noisy | apple-keywords | 0/4 | 0/1 | 94 ms | What does codex recommend? |
| 24 | noisy | apple-dictation-keywords | 1/4 | 1/1 | 314 ms | What does FedEx recommend? |
| 24 | noisy | parakeet | 0/4 | 0/1 | 37 ms | What does Codex recommend? |
| 24 | noisy | whistle | 1/4 | 0/1 | 15 ms | What does Codex recommend you? |
| 24 | noisy | whistle-keywords | 1/4 | 0/1 | 14 ms | What does Codex recommend you? |
| 25 | clean | apple | 0/4 | 0/0 | 82 ms | What are the options? |
| 25 | clean | apple-keywords | 0/4 | 0/0 | 80 ms | What are the options? |
| 25 | clean | apple-dictation-keywords | 0/4 | 0/0 | 207 ms | What are the options? |
| 25 | clean | parakeet | 0/4 | 0/0 | 34 ms | What are the options? |
| 25 | clean | whistle | 0/4 | 0/0 | 11 ms | What are the options? |
| 25 | clean | whistle-keywords | 0/4 | 0/0 | 11 ms | What are the options? |
| 25 | noisy | apple | 0/4 | 0/0 | 81 ms | What are the options? |
| 25 | noisy | apple-keywords | 0/4 | 0/0 | 86 ms | What are the options? |
| 25 | noisy | apple-dictation-keywords | 0/4 | 0/0 | 191 ms | What are the options? |
| 25 | noisy | parakeet | 0/4 | 0/0 | 35 ms | What are the options? |
| 25 | noisy | whistle | 0/4 | 0/0 | 11 ms | What are the options? |
| 25 | noisy | whistle-keywords | 0/4 | 0/0 | 10 ms | What are the options? |
| 26 | clean | apple | 0/4 | 0/0 | 82 ms | Is there a deadline? |
| 26 | clean | apple-keywords | 0/4 | 0/0 | 83 ms | Is there a deadline? |
| 26 | clean | apple-dictation-keywords | 0/4 | 0/0 | 204 ms | Is there a deadline? |
| 26 | clean | parakeet | 0/4 | 0/0 | 36 ms | Is there a deadline? |
| 26 | clean | whistle | 0/4 | 0/0 | 11 ms | Is there a deadline? |
| 26 | clean | whistle-keywords | 0/4 | 0/0 | 11 ms | Is there a deadline? |
| 26 | noisy | apple | 0/4 | 0/0 | 82 ms | Is there a deadline? |
| 26 | noisy | apple-keywords | 0/4 | 0/0 | 83 ms | Is there a deadline? |
| 26 | noisy | apple-dictation-keywords | 0/4 | 0/0 | 189 ms | Is there a deadline? |
| 26 | noisy | parakeet | 0/4 | 0/0 | 38 ms | Is there a deadline? |
| 26 | noisy | whistle | 0/4 | 0/0 | 10 ms | Is there a deadline? |
| 26 | noisy | whistle-keywords | 0/4 | 0/0 | 11 ms | Is there a deadline? |
| 27 | clean | apple | 0/4 | 0/1 | 84 ms | Approve the first option. |
| 27 | clean | apple-keywords | 0/4 | 0/1 | 81 ms | Approve the first option. |
| 27 | clean | apple-dictation-keywords | 1/4 | 1/1 | 201 ms | Approved the first option |
| 27 | clean | parakeet | 0/4 | 0/1 | 36 ms | Approve the first option. |
| 27 | clean | whistle | 1/4 | 1/1 | 13 ms | approved the first option. |
| 27 | clean | whistle-keywords | 1/4 | 1/1 | 13 ms | approved the first option. |
| 27 | noisy | apple | 2/4 | 1/1 | 83 ms | Approved 1st option. |
| 27 | noisy | apple-keywords | 2/4 | 1/1 | 87 ms | Approved 1st option. |
| 27 | noisy | apple-dictation-keywords | 2/4 | 1/1 | 197 ms | Approved first option |
| 27 | noisy | parakeet | 2/4 | 1/1 | 35 ms | I proved the first option. |
| 27 | noisy | whistle | 2/4 | 1/1 | 14 ms | I proved the first option. |
| 27 | noisy | whistle-keywords | 2/4 | 1/1 | 13 ms | I proved the first option. |
| 28 | clean | apple | 0/3 | 0/1 | 77 ms | Choose option two. |
| 28 | clean | apple-keywords | 0/3 | 0/1 | 79 ms | Choose option two. |
| 28 | clean | apple-dictation-keywords | 0/3 | 0/1 | 185 ms | Choose option two |
| 28 | clean | parakeet | 0/3 | 0/1 | 34 ms | Choose option two. |
| 28 | clean | whistle | 2/3 | 0/1 | 11 ms | She was option two. |
| 28 | clean | whistle-keywords | 2/3 | 0/1 | 10 ms | She was option two. |
| 28 | noisy | apple | 0/3 | 0/1 | 75 ms | Choose option 2. |
| 28 | noisy | apple-keywords | 0/3 | 0/1 | 77 ms | Choose option 2. |
| 28 | noisy | apple-dictation-keywords | 0/3 | 0/1 | 184 ms | Choose option two |
| 28 | noisy | parakeet | 0/3 | 0/1 | 35 ms | Choose option two. |
| 28 | noisy | whistle | 0/3 | 0/1 | 11 ms | Choose option two. |
| 28 | noisy | whistle-keywords | 0/3 | 0/1 | 11 ms | Choose option two. |
| 29 | clean | apple | 0/5 | 0/0 | 84 ms | Hold this until tomorrow morning. |
| 29 | clean | apple-keywords | 0/5 | 0/0 | 81 ms | Hold this until tomorrow morning. |
| 29 | clean | apple-dictation-keywords | 0/5 | 0/0 | 224 ms | Hold this until tomorrow morning |
| 29 | clean | parakeet | 0/5 | 0/0 | 41 ms | Hold this until tomorrow morning. |
| 29 | clean | whistle | 1/5 | 0/0 | 16 ms | Pulled this until tomorrow morning. |
| 29 | clean | whistle-keywords | 1/5 | 0/0 | 14 ms | Pulled this until tomorrow morning. |
| 29 | noisy | apple | 0/5 | 0/0 | 83 ms | Hold this until tomorrow morning. |
| 29 | noisy | apple-keywords | 0/5 | 0/0 | 81 ms | Hold this until tomorrow morning. |
| 29 | noisy | apple-dictation-keywords | 0/5 | 0/0 | 206 ms | Hold this until tomorrow morning |
| 29 | noisy | parakeet | 0/5 | 0/0 | 45 ms | Hold this until tomorrow morning. |
| 29 | noisy | whistle | 0/5 | 0/0 | 16 ms | Hold this until tomorrow morning. |
| 29 | noisy | whistle-keywords | 0/5 | 0/0 | 16 ms | Hold this until tomorrow morning. |
| 30 | clean | apple | 0/5 | 0/1 | 87 ms | Remind me in 30 minutes. |
| 30 | clean | apple-keywords | 0/5 | 0/1 | 85 ms | Remind me in 30 minutes. |
| 30 | clean | apple-dictation-keywords | 0/5 | 0/1 | 209 ms | Remind me in 30 minutes |
| 30 | clean | parakeet | 0/5 | 0/1 | 40 ms | Remind me in thirty minutes. |
| 30 | clean | whistle | 1/5 | 0/1 | 15 ms | Remind me in thirty months. |
| 30 | clean | whistle-keywords | 1/5 | 0/1 | 14 ms | Remind me in thirty months. |
| 30 | noisy | apple | 0/5 | 0/1 | 84 ms | Remind me in 30 minutes. |
| 30 | noisy | apple-keywords | 0/5 | 0/1 | 82 ms | Remind me in 30 minutes. |
| 30 | noisy | apple-dictation-keywords | 0/5 | 0/1 | 202 ms | Remind me in 30 minutes |
| 30 | noisy | parakeet | 0/5 | 0/1 | 41 ms | Remind me in thirty minutes. |
| 30 | noisy | whistle | 0/5 | 0/1 | 14 ms | Remind me in 30 minutes. |
| 30 | noisy | whistle-keywords | 0/5 | 0/1 | 15 ms | Remind me in 30 minutes. |
| 31 | clean | apple | 1/10 | 0/2 | 112 ms | Post the phase 0 room. The spike is done. |
| 31 | clean | apple-keywords | 1/10 | 0/2 | 126 ms | Post the phase 0 room. The spike is done. |
| 31 | clean | apple-dictation-keywords | 1/10 | 0/2 | 278 ms | Post the phase 0 room the spike is done |
| 31 | clean | parakeet | 1/10 | 0/2 | 107 ms | Post the Phase Zero Room, The Spike is Done. |
| 31 | clean | whistle | 1/10 | 0/2 | 25 ms | Post the phase zero room, the spike is done. |
| 31 | clean | whistle-keywords | 1/10 | 0/2 | 26 ms | Post the phase zero room, the spike is done. |
| 31 | noisy | apple | 1/10 | 0/2 | 112 ms | Post the phase 0 room. The spike is done. |
| 31 | noisy | apple-keywords | 1/10 | 0/2 | 114 ms | Post the phase 0 room. The spike is done. |
| 31 | noisy | apple-dictation-keywords | 1/10 | 0/2 | 525 ms | Post the phase 0 room the spike is done |
| 31 | noisy | parakeet | 0/10 | 0/2 | 53 ms | Post to the phase zero room. The spike is done. |
| 31 | noisy | whistle | 3/10 | 0/2 | 24 ms | Post the phase zero room to spike it done. |
| 31 | noisy | whistle-keywords | 3/10 | 0/2 | 27 ms | Post the phase zero room to spike it done. |
| 32 | clean | apple | 0/6 | 0/1 | 87 ms | What's new in the voice room? |
| 32 | clean | apple-keywords | 0/6 | 0/1 | 91 ms | What's new in the voice room? |
| 32 | clean | apple-dictation-keywords | 0/6 | 0/1 | 236 ms | What's new in the voice room? |
| 32 | clean | parakeet | 0/6 | 0/1 | 43 ms | What's New in the Voice Room? |
| 32 | clean | whistle | 0/6 | 0/1 | 17 ms | What's new in the voice room? |
| 32 | clean | whistle-keywords | 0/6 | 0/1 | 18 ms | What's new in the voice room? |
| 32 | noisy | apple | 0/6 | 0/1 | 89 ms | What's new in the voice room? |
| 32 | noisy | apple-keywords | 0/6 | 0/1 | 90 ms | What's new in the voice room? |
| 32 | noisy | apple-dictation-keywords | 0/6 | 0/1 | 221 ms | What's new in the voice room? |
| 32 | noisy | parakeet | 0/6 | 0/1 | 41 ms | What's new in the voice room? |
| 32 | noisy | whistle | 0/6 | 0/1 | 19 ms | What's new in the voice room? |
| 32 | noisy | whistle-keywords | 0/6 | 0/1 | 21 ms | What's new in the voice room? |
| 33 | clean | apple | 4/9 | 2/5 | 116 ms | Add Deep Sea coin in 7 C to the release room. |
| 33 | clean | apple-keywords | 4/9 | 2/5 | 118 ms | Add Deep Sea coin in 7 C to the release room. |
| 33 | clean | apple-dictation-keywords | 0/9 | 0/5 | 261 ms | Add DeepSeek Koinon 7C to the release room |
| 33 | clean | parakeet | 1/9 | 1/5 | 55 ms | Add Deepseek Coinin seven C to the release room. |
| 33 | clean | whistle | 5/9 | 3/5 | 27 ms | Add Deep Sea coin in Seven Sea to the release room. |
| 33 | clean | whistle-keywords | 0/9 | 0/5 | 26 ms | Add Deepseek Koinon 7 C to the release room. |
| 33 | noisy | apple | 6/9 | 2/5 | 114 ms | And deep sea coin and 7 C into the release room. |
| 33 | noisy | apple-keywords | 6/9 | 2/5 | 114 ms | And deep sea coin and 7 C into the release room. |
| 33 | noisy | apple-dictation-keywords | 2/9 | 1/5 | 259 ms | Add deep sea Koinon 7C to the release room |
| 33 | noisy | parakeet | 4/9 | 2/5 | 53 ms | Add Deep Sea Coin in 7C to the release room. |
| 33 | noisy | whistle | 8/9 | 3/5 | 30 ms | And deep sea coin and seven sea, which the release rambled. |
| 33 | noisy | whistle-keywords | 3/9 | 0/5 | 28 ms | and Deepseek Koinon 7 C, which the release ran. |
| 34 | clean | apple | 1/5 | 0/1 | 87 ms | Greet a room called bench. |
| 34 | clean | apple-keywords | 1/5 | 0/1 | 86 ms | Greet a room called bench. |
| 34 | clean | apple-dictation-keywords | 0/5 | 0/1 | 210 ms | Create a room called bench |
| 34 | clean | parakeet | 0/5 | 0/1 | 41 ms | Create a room called Bench |
| 34 | clean | whistle | 1/5 | 0/1 | 16 ms | Creed a room called Bench. |
| 34 | clean | whistle-keywords | 1/5 | 0/1 | 15 ms | Creed a room called Bench. |
| 34 | noisy | apple | 0/5 | 0/1 | 86 ms | Create a room called Bench. |
| 34 | noisy | apple-keywords | 0/5 | 0/1 | 87 ms | Create a room called Bench. |
| 34 | noisy | apple-dictation-keywords | 0/5 | 0/1 | 208 ms | Create a room called bench |
| 34 | noisy | parakeet | 0/5 | 0/1 | 42 ms | Create a room called Bench. |
| 34 | noisy | whistle | 0/5 | 0/1 | 14 ms | Create a room called Bench. |
| 34 | noisy | whistle-keywords | 0/5 | 0/1 | 14 ms | Create a room called Bench. |
| 35 | clean | apple | 2/7 | 2/4 | 117 ms | Retire the session codex coin invoice af. |
| 35 | clean | apple-keywords | 2/7 | 2/4 | 112 ms | Retire the session codex coin invoice af. |
| 35 | clean | apple-dictation-keywords | 3/7 | 3/4 | 231 ms | Retire the session, Kodex coin invoice AF |
| 35 | clean | parakeet | 2/7 | 2/4 | 47 ms | Retire the session Codex Coin Invoice Af |
| 35 | clean | whistle | 2/7 | 2/4 | 25 ms | Retire the session Codex coin in voice "AF". |
| 35 | clean | whistle-keywords | 0/7 | 0/4 | 24 ms | Retire the session Codex Koinon Voice Af. |
| 35 | noisy | apple | 2/7 | 2/4 | 121 ms | Retire the session Codex Coin Invoice AF. |
| 35 | noisy | apple-keywords | 2/7 | 2/4 | 123 ms | Retire the session Codex Coin Invoice AF. |
| 35 | noisy | apple-dictation-keywords | 5/7 | 4/4 | 235 ms | Retired the session Kodex coin invoice |
| 35 | noisy | parakeet | 2/7 | 2/4 | 47 ms | Retire the session Codex Coin Invoice AF. |
| 35 | noisy | whistle | 5/7 | 3/4 | 30 ms | We tire the session Codex Coin and Voice A.M. |
| 35 | noisy | whistle-keywords | 0/7 | 0/4 | 24 ms | Retire the session, Codex Koinon Voice Af. |
| 36 | clean | apple | 0/7 | 0/1 | 103 ms | Release the work claim on issue one. |
| 36 | clean | apple-keywords | 0/7 | 0/1 | 103 ms | Release the work claim on issue one. |
| 36 | clean | apple-dictation-keywords | 0/7 | 0/1 | 220 ms | Release the work claim on issue one |
| 36 | clean | parakeet | 0/7 | 0/1 | 44 ms | Release the work claim on issue one. |
| 36 | clean | whistle | 0/7 | 0/1 | 19 ms | Release the work claim on issue one. |
| 36 | clean | whistle-keywords | 0/7 | 0/1 | 19 ms | Release the work claim on issue one. |
| 36 | noisy | apple | 0/7 | 0/1 | 95 ms | Release the work claim on issue one. |
| 36 | noisy | apple-keywords | 0/7 | 0/1 | 95 ms | Release the work claim on issue one. |
| 36 | noisy | apple-dictation-keywords | 0/7 | 0/1 | 222 ms | Release the work claim on issue one |
| 36 | noisy | parakeet | 0/7 | 0/1 | 41 ms | Release the work claim on issue one. |
| 36 | noisy | whistle | 2/7 | 0/1 | 18 ms | Released to work claim on issue one. |
| 36 | noisy | whistle-keywords | 2/7 | 0/1 | 18 ms | Released to work claim on issue one. |
| 37 | clean | apple | 1/8 | 0/0 | 115 ms | Who holds the claim on speech bench? |
| 37 | clean | apple-keywords | 1/8 | 0/0 | 109 ms | Who holds the claim on speech bench? |
| 37 | clean | apple-dictation-keywords | 1/8 | 0/0 | 241 ms | Who holds the claim on speech bench? |
| 37 | clean | parakeet | 3/8 | 0/0 | 51 ms | Who holds the claim on speechbench? |
| 37 | clean | whistle | 1/8 | 0/0 | 48 ms | Who holds the claim on speech bench? |
| 37 | clean | whistle-keywords | 1/8 | 0/0 | 21 ms | Who holds the claim on speech bench? |
| 37 | noisy | apple | 0/8 | 0/0 | 99 ms | Who holds the claim on the speech bench? |
| 37 | noisy | apple-keywords | 0/8 | 0/0 | 100 ms | Who holds the claim on the speech bench? |
| 37 | noisy | apple-dictation-keywords | 0/8 | 0/0 | 226 ms | Who holds the claim on the speech bench? |
| 37 | noisy | parakeet | 0/8 | 0/0 | 41 ms | Who holds the claim on the speech bench? |
| 37 | noisy | whistle | 0/8 | 0/0 | 20 ms | Who holds the claim on the speech bench? |
| 37 | noisy | whistle-keywords | 0/8 | 0/0 | 19 ms | Who holds the claim on the speech bench? |
| 38 | clean | apple | 1/5 | 0/0 | 84 ms | Show me the feeling checks. |
| 38 | clean | apple-keywords | 1/5 | 0/0 | 86 ms | Show me the feeling checks. |
| 38 | clean | apple-dictation-keywords | 1/5 | 0/0 | 199 ms | Show me the feeling checks |
| 38 | clean | parakeet | 0/5 | 0/0 | 39 ms | Show me the failing checks. |
| 38 | clean | whistle | 1/5 | 0/0 | 40 ms | Show me the feeling checks. |
| 38 | clean | whistle-keywords | 1/5 | 0/0 | 16 ms | Show me the feeling checks. |
| 38 | noisy | apple | 1/5 | 0/0 | 90 ms | Show me the feeling checks. |
| 38 | noisy | apple-keywords | 1/5 | 0/0 | 88 ms | Show me the feeling checks. |
| 38 | noisy | apple-dictation-keywords | 2/5 | 0/0 | 207 ms | Show me the feeling chicks |
| 38 | noisy | parakeet | 1/5 | 0/0 | 40 ms | Show me the feeling checks. |
| 38 | noisy | whistle | 1/5 | 0/0 | 16 ms | Show me the feeling checks. |
| 38 | noisy | whistle-keywords | 1/5 | 0/0 | 16 ms | Show me the feeling checks. |
| 39 | clean | apple | 0/5 | 0/1 | 86 ms | Did the macOS tests pass? |
| 39 | clean | apple-keywords | 0/5 | 0/1 | 85 ms | Did the macOS tests pass? |
| 39 | clean | apple-dictation-keywords | 1/5 | 0/1 | 204 ms | Did the macOS test pass? |
| 39 | clean | parakeet | 0/5 | 0/1 | 39 ms | Did the macOS tests pass? |
| 39 | clean | whistle | 0/5 | 0/1 | 18 ms | Did the macOS tests pass? |
| 39 | clean | whistle-keywords | 0/5 | 0/1 | 17 ms | Did the macOS tests pass? |
| 39 | noisy | apple | 1/5 | 0/1 | 85 ms | Did the macOS test pass? |
| 39 | noisy | apple-keywords | 1/5 | 0/1 | 88 ms | Did the macOS test pass? |
| 39 | noisy | apple-dictation-keywords | 1/5 | 0/1 | 205 ms | Did the macOS test pass? |
| 39 | noisy | parakeet | 1/5 | 0/1 | 56 ms | Did the macOS test pass? |
| 39 | noisy | whistle | 1/5 | 0/1 | 15 ms | Did the macOS test pass? |
| 39 | noisy | whistle-keywords | 1/5 | 0/1 | 17 ms | Did the macOS test pass? |
| 40 | clean | apple | 0/4 | 0/1 | 87 ms | Rerun the Linux job. |
| 40 | clean | apple-keywords | 0/4 | 0/1 | 88 ms | Rerun the Linux job. |
| 40 | clean | apple-dictation-keywords | 1/4 | 1/1 | 189 ms | Rerun the Lennox job |
| 40 | clean | parakeet | 0/4 | 0/1 | 36 ms | Rerun the Linux job. |
| 40 | clean | whistle | 2/4 | 0/1 | 15 ms | Ri run the Linux job. |
| 40 | clean | whistle-keywords | 2/4 | 0/1 | 14 ms | Ri run the Linux job. |
| 40 | noisy | apple | 0/4 | 0/1 | 87 ms | Rerun the Linux job. |
| 40 | noisy | apple-keywords | 0/4 | 0/1 | 87 ms | Rerun the Linux job. |
| 40 | noisy | apple-dictation-keywords | 1/4 | 1/1 | 223 ms | Rerun the Lenox job |
| 40 | noisy | parakeet | 0/4 | 0/1 | 39 ms | Rerun the Linux job. |
| 40 | noisy | whistle | 2/4 | 0/1 | 12 ms | We run the Linux job. |
| 40 | noisy | whistle-keywords | 2/4 | 0/1 | 14 ms | We run the Linux job. |
| 41 | clean | apple | 1/8 | 0/5 | 98 ms | Approve the peer review and refusal review. |
| 41 | clean | apple-keywords | 1/8 | 0/5 | 99 ms | Approve the peer review and refusal review. |
| 41 | clean | apple-dictation-keywords | 4/8 | 3/5 | 328 ms | Approved the peer review and refuse audio |
| 41 | clean | parakeet | 1/8 | 0/5 | 53 ms | Approve the peer review and refusal review. |
| 41 | clean | whistle | 2/8 | 1/5 | 26 ms | Approved the peer review and refusal review. |
| 41 | clean | whistle-keywords | 2/8 | 1/5 | 25 ms | Approved the peer review and refusal review. |
| 41 | noisy | apple | 0/8 | 0/5 | 113 ms | Approve the peer review and the refusal review. |
| 41 | noisy | apple-keywords | 0/8 | 0/5 | 110 ms | Approve the peer review and the refusal review. |
| 41 | noisy | apple-dictation-keywords | 0/8 | 0/5 | 239 ms | Approve the peer review and the refusal review |
| 41 | noisy | parakeet | 0/8 | 0/5 | 51 ms | Approve the peer review and the refusal review. |
| 41 | noisy | whistle | 0/8 | 0/5 | 24 ms | Approve the peer review and the refusal review. |
| 41 | noisy | whistle-keywords | 0/8 | 0/5 | 24 ms | Approve the peer review and the refusal review. |
| 42 | clean | apple | 0/7 | 0/0 | 113 ms | Request changes, the deadline test is missing. |
| 42 | clean | apple-keywords | 0/7 | 0/0 | 106 ms | Request changes, the deadline test is missing. |
| 42 | clean | apple-dictation-keywords | 0/7 | 0/0 | 247 ms | Request changes the deadline test is missing |
| 42 | clean | parakeet | 0/7 | 0/0 | 45 ms | Request changes, the deadline test is missing. |
| 42 | clean | whistle | 0/7 | 0/0 | 18 ms | Request changes the deadline test is missing. |
| 42 | clean | whistle-keywords | 0/7 | 0/0 | 17 ms | Request changes the deadline test is missing. |
| 42 | noisy | apple | 0/7 | 0/0 | 105 ms | Request changes. The deadline test is missing. |
| 42 | noisy | apple-keywords | 0/7 | 0/0 | 101 ms | Request changes. The deadline test is missing. |
| 42 | noisy | apple-dictation-keywords | 0/7 | 0/0 | 232 ms | Request changes the deadline test is missing |
| 42 | noisy | parakeet | 1/7 | 0/0 | 42 ms | If request changes, the deadline test is missing. |
| 42 | noisy | whistle | 1/7 | 0/0 | 19 ms | With request changes, the deadline test is missing. |
| 42 | noisy | whistle-keywords | 1/7 | 0/0 | 19 ms | With request changes, the deadline test is missing. |
| 43 | clean | apple | 0/5 | 0/1 | 85 ms | Merge when the checks pass. |
| 43 | clean | apple-keywords | 0/5 | 0/1 | 88 ms | Merge when the checks pass. |
| 43 | clean | apple-dictation-keywords | 0/5 | 0/1 | 200 ms | Merge when the checks pass |
| 43 | clean | parakeet | 0/5 | 0/1 | 40 ms | Merge when the checks pass. |
| 43 | clean | whistle | 0/5 | 0/1 | 16 ms | Merge when the checks pass. |
| 43 | clean | whistle-keywords | 0/5 | 0/1 | 16 ms | Merge when the checks pass. |
| 43 | noisy | apple | 0/5 | 0/1 | 89 ms | Merge when the checks pass. |
| 43 | noisy | apple-keywords | 0/5 | 0/1 | 85 ms | Merge when the checks pass. |
| 43 | noisy | apple-dictation-keywords | 0/5 | 0/1 | 249 ms | Merge when the checks pass |
| 43 | noisy | parakeet | 0/5 | 0/1 | 40 ms | Merge when the checks pass. |
| 43 | noisy | whistle | 1/5 | 1/1 | 17 ms | Marriage when the checks pass. |
| 43 | noisy | whistle-keywords | 1/5 | 1/1 | 17 ms | Marriage when the checks pass. |
| 44 | clean | apple | 0/3 | 0/1 | 80 ms | Don't merge yet. |
| 44 | clean | apple-keywords | 0/3 | 0/1 | 81 ms | Don't merge yet. |
| 44 | clean | apple-dictation-keywords | 0/3 | 0/1 | 181 ms | Don't merge yet |
| 44 | clean | parakeet | 0/3 | 0/1 | 35 ms | Don't merge yet. |
| 44 | clean | whistle | 1/3 | 0/1 | 11 ms | Don't merge it. |
| 44 | clean | whistle-keywords | 1/3 | 0/1 | 13 ms | Don't merge it. |
| 44 | noisy | apple | 1/3 | 0/1 | 80 ms | Don't merge in. |
| 44 | noisy | apple-keywords | 1/3 | 0/1 | 78 ms | Don't merge in. |
| 44 | noisy | apple-dictation-keywords | 0/3 | 0/1 | 188 ms | Don't merge yet |
| 44 | noisy | parakeet | 0/3 | 0/1 | 37 ms | Don't merge yet. |
| 44 | noisy | whistle | 1/3 | 0/1 | 15 ms | Don't merge it. |
| 44 | noisy | whistle-keywords | 1/3 | 0/1 | 15 ms | Don't merge it. |
| 45 | clean | apple | 0/5 | 0/3 | 90 ms | Confirm, merge pull request five. |
| 45 | clean | apple-keywords | 0/5 | 0/3 | 90 ms | Confirm, merge pull request five. |
| 45 | clean | apple-dictation-keywords | 0/5 | 0/3 | 220 ms | Confirm merge pull request five |
| 45 | clean | parakeet | 1/5 | 1/3 | 41 ms | Confirm: Merge pull request V. |
| 45 | clean | whistle | 0/5 | 0/3 | 16 ms | Confirm, merge pull request five. |
| 45 | clean | whistle-keywords | 0/5 | 0/3 | 17 ms | Confirm, merge pull request five. |
| 45 | noisy | apple | 1/5 | 0/3 | 93 ms | Confirm merge poll request five. |
| 45 | noisy | apple-keywords | 1/5 | 0/3 | 97 ms | Confirm merge poll request five. |
| 45 | noisy | apple-dictation-keywords | 1/5 | 0/3 | 222 ms | Confirm merge pole request five |
| 45 | noisy | parakeet | 0/5 | 0/3 | 41 ms | Confirm, merge pull request 5. |
| 45 | noisy | whistle | 0/5 | 0/3 | 17 ms | Confirm, merge pull request five. |
| 45 | noisy | whistle-keywords | 0/5 | 0/3 | 17 ms | Confirm, merge pull request five. |
| 46 | clean | apple | 0/4 | 0/1 | 85 ms | No, I said reject. |
| 46 | clean | apple-keywords | 0/4 | 0/1 | 80 ms | No, I said reject. |
| 46 | clean | apple-dictation-keywords | 0/4 | 0/1 | 194 ms | No, I said reject |
| 46 | clean | parakeet | 0/4 | 0/1 | 39 ms | No, I said reject. |
| 46 | clean | whistle | 0/4 | 0/1 | 20 ms | "No," I said reject. |
| 46 | clean | whistle-keywords | 0/4 | 0/1 | 20 ms | "No," I said reject. |
| 46 | noisy | apple | 0/4 | 0/1 | 82 ms | No, I said reject. |
| 46 | noisy | apple-keywords | 0/4 | 0/1 | 83 ms | No, I said reject. |
| 46 | noisy | apple-dictation-keywords | 0/4 | 0/1 | 186 ms | No, I said reject |
| 46 | noisy | parakeet | 0/4 | 0/1 | 36 ms | No, I said reject. |
| 46 | noisy | whistle | 0/4 | 0/1 | 14 ms | No, I said reject. |
| 46 | noisy | whistle-keywords | 0/4 | 0/1 | 14 ms | No, I said reject. |
| 47 | clean | apple | 0/1 | 0/0 | 79 ms | Stop. |
| 47 | clean | apple-keywords | 0/1 | 0/0 | 69 ms | Stop. |
| 47 | clean | apple-dictation-keywords | 0/1 | 0/0 | 185 ms | Stop |
| 47 | clean | parakeet | 0/1 | 0/0 | 33 ms | Stop. |
| 47 | clean | whistle | 0/1 | 0/0 | 8 ms | Stop. |
| 47 | clean | whistle-keywords | 0/1 | 0/0 | 7 ms | Stop. |
| 47 | noisy | apple | 0/1 | 0/0 | 72 ms | Stop. |
| 47 | noisy | apple-keywords | 0/1 | 0/0 | 68 ms | Stop. |
| 47 | noisy | apple-dictation-keywords | 0/1 | 0/0 | 179 ms | Stop |
| 47 | noisy | parakeet | 0/1 | 0/0 | 36 ms | Stop. |
| 47 | noisy | whistle | 0/1 | 0/0 | 8 ms | Stop. |
| 47 | noisy | whistle-keywords | 0/1 | 0/0 | 8 ms | Stop. |
| 48 | clean | apple | 0/2 | 0/0 | 70 ms | Go on. |
| 48 | clean | apple-keywords | 0/2 | 0/0 | 71 ms | Go on. |
| 48 | clean | apple-dictation-keywords | 0/2 | 0/0 | 186 ms | Go on |
| 48 | clean | parakeet | 0/2 | 0/0 | 33 ms | Go on. |
| 48 | clean | whistle | 0/2 | 0/0 | 9 ms | Go on. |
| 48 | clean | whistle-keywords | 0/2 | 0/0 | 9 ms | Go on. |
| 48 | noisy | apple | 0/2 | 0/0 | 70 ms | Go on. |
| 48 | noisy | apple-keywords | 0/2 | 0/0 | 72 ms | Go on. |
| 48 | noisy | apple-dictation-keywords | 0/2 | 0/0 | 192 ms | Go on |
| 48 | noisy | parakeet | 0/2 | 0/0 | 36 ms | Go on. |
| 48 | noisy | whistle | 0/2 | 0/0 | 12 ms | Go on. |
| 48 | noisy | whistle-keywords | 0/2 | 0/0 | 12 ms | Go on. |
| 49 | clean | apple | 0/2 | 0/0 | 77 ms | Louder please. |
| 49 | clean | apple-keywords | 0/2 | 0/0 | 76 ms | Louder please. |
| 49 | clean | apple-dictation-keywords | 0/2 | 0/0 | 189 ms | Louder please |
| 49 | clean | parakeet | 0/2 | 0/0 | 37 ms | Louder, please. |
| 49 | clean | whistle | 0/2 | 0/0 | 12 ms | Louder, please. |
| 49 | clean | whistle-keywords | 0/2 | 0/0 | 12 ms | Louder, please. |
| 49 | noisy | apple | 0/2 | 0/0 | 75 ms | Louder, please. |
| 49 | noisy | apple-keywords | 0/2 | 0/0 | 77 ms | Louder, please. |
| 49 | noisy | apple-dictation-keywords | 0/2 | 0/0 | 183 ms | Louder please |
| 49 | noisy | parakeet | 0/2 | 0/0 | 35 ms | Louder, please. |
| 49 | noisy | whistle | 0/2 | 0/0 | 12 ms | Louder, please. |
| 49 | noisy | whistle-keywords | 0/2 | 0/0 | 12 ms | Louder, please. |
| 50 | clean | apple | 0/2 | 0/0 | 75 ms | Slow down. |
| 50 | clean | apple-keywords | 0/2 | 0/0 | 74 ms | Slow down. |
| 50 | clean | apple-dictation-keywords | 0/2 | 0/0 | 185 ms | Slow down |
| 50 | clean | parakeet | 0/2 | 0/0 | 34 ms | Slow down. |
| 50 | clean | whistle | 0/2 | 0/0 | 10 ms | Slow down. |
| 50 | clean | whistle-keywords | 0/2 | 0/0 | 10 ms | Slow down. |
| 50 | noisy | apple | 0/2 | 0/0 | 76 ms | Slow down. |
| 50 | noisy | apple-keywords | 0/2 | 0/0 | 71 ms | Slow down. |
| 50 | noisy | apple-dictation-keywords | 0/2 | 0/0 | 187 ms | Slow down |
| 50 | noisy | parakeet | 0/2 | 0/0 | 35 ms | Slow down. |
| 50 | noisy | whistle | 0/2 | 0/0 | 10 ms | Slow down. |
| 50 | noisy | whistle-keywords | 0/2 | 0/0 | 11 ms | Slow down. |
| 51 | clean | apple | 0/6 | 0/1 | 89 ms | What time is it in UTC? |
| 51 | clean | apple-keywords | 0/6 | 0/1 | 90 ms | What time is it in UTC? |
| 51 | clean | apple-dictation-keywords | 0/6 | 0/1 | 208 ms | What time is it in UTC? |
| 51 | clean | parakeet | 0/6 | 0/1 | 45 ms | What time is it in Utc? |
| 51 | clean | whistle | 0/6 | 0/1 | 17 ms | What time is it in UTC? |
| 51 | clean | whistle-keywords | 0/6 | 0/1 | 16 ms | What time is it in UTC? |
| 51 | noisy | apple | 0/6 | 0/1 | 90 ms | What time is it in UTC? |
| 51 | noisy | apple-keywords | 0/6 | 0/1 | 88 ms | What time is it in UTC? |
| 51 | noisy | apple-dictation-keywords | 0/6 | 0/1 | 209 ms | What time is it in UTC? |
| 51 | noisy | parakeet | 0/6 | 0/1 | 38 ms | What time is it in UTC? |
| 51 | noisy | whistle | 0/6 | 0/1 | 14 ms | What time is it in UTC? |
| 51 | noisy | whistle-keywords | 0/6 | 0/1 | 15 ms | What time is it in UTC? |
| 52 | clean | apple | 2/9 | 1/3 | 141 ms | Read the summary for RWCII/coin, issue 466. |
| 52 | clean | apple-keywords | 2/9 | 1/3 | 125 ms | Read the summary for RWCII/coin, issue 466. |
| 52 | clean | apple-dictation-keywords | 2/9 | 1/3 | 614 ms | Read the summary for RWCI/Koinon issue 466 |
| 52 | clean | parakeet | 1/9 | 1/3 | 59 ms | Read the summary for Rwcii slash coinin issue four sixty six |
| 52 | clean | whistle | 2/9 | 2/3 | 38 ms | Read the summary for RGBCI's slash coinin issue four-sixty-six. |
| 52 | clean | whistle-keywords | 1/9 | 1/3 | 38 ms | Read the summary for RGBCI's "Slash Koinon" issue four, sixty-six. |
| 52 | noisy | apple | 1/9 | 1/3 | 137 ms | Read the summary for RWCII slash coinin, issue 466. |
| 52 | noisy | apple-keywords | 1/9 | 1/3 | 132 ms | Read the summary for RWCII slash coinin, issue 466. |
| 52 | noisy | apple-dictation-keywords | 2/9 | 1/3 | 680 ms | Read the summary for our WCI/Koinon issue 466 |
| 52 | noisy | parakeet | 1/9 | 1/3 | 53 ms | Read the summary for RWCII slash coinin issue 466. |
| 52 | noisy | whistle | 2/9 | 2/3 | 28 ms | Read the summary for RWCI slash coinin issue 466. |
| 52 | noisy | whistle-keywords | 1/9 | 1/3 | 29 ms | Read the summary for RWCI Slash Koinon issue 466. |
| 53 | clean | apple | 2/5 | 1/1 | 97 ms | Switch to the coin and repository. |
| 53 | clean | apple-keywords | 2/5 | 1/1 | 111 ms | Switch to the coin and repository. |
| 53 | clean | apple-dictation-keywords | 0/5 | 0/1 | 213 ms | Switch to the Koinon repository |
| 53 | clean | parakeet | 2/5 | 1/1 | 41 ms | Switch to the coin in repository. |
| 53 | clean | whistle | 2/5 | 1/1 | 21 ms | Switch to the coin and repository. |
| 53 | clean | whistle-keywords | 0/5 | 0/1 | 20 ms | Switch to the Koinon repository. |
| 53 | noisy | apple | 2/5 | 1/1 | 92 ms | Switch to the coin and repository. |
| 53 | noisy | apple-keywords | 2/5 | 1/1 | 96 ms | Switch to the coin and repository. |
| 53 | noisy | apple-dictation-keywords | 0/5 | 0/1 | 209 ms | Switch to the Koinon repository |
| 53 | noisy | parakeet | 2/5 | 1/1 | 38 ms | Switch to the coin in repository. |
| 53 | noisy | whistle | 2/5 | 1/1 | 20 ms | Switch to the coin in repository. |
| 53 | noisy | whistle-keywords | 0/5 | 0/1 | 17 ms | Switch to the Koinon repository. |
| 54 | clean | apple | 0/3 | 0/1 | 80 ms | Is OMLX running? |
| 54 | clean | apple-keywords | 0/3 | 0/1 | 83 ms | Is OMLX running? |
| 54 | clean | apple-dictation-keywords | 1/3 | 1/1 | 211 ms | Is OLX running? |
| 54 | clean | parakeet | 0/3 | 0/1 | 36 ms | Is Omlx running? |
| 54 | clean | whistle | 1/3 | 0/1 | 13 ms | His OMLX running? |
| 54 | clean | whistle-keywords | 1/3 | 0/1 | 15 ms | His OMLX running? |
| 54 | noisy | apple | 0/3 | 0/1 | 79 ms | Is OMLX running? |
| 54 | noisy | apple-keywords | 0/3 | 0/1 | 85 ms | Is OMLX running? |
| 54 | noisy | apple-dictation-keywords | 1/3 | 1/1 | 193 ms | Is OLX running? |
| 54 | noisy | parakeet | 0/3 | 0/1 | 34 ms | Is OMLX running? |
| 54 | noisy | whistle | 2/3 | 1/1 | 15 ms | Is OM Alex running? |
| 54 | noisy | whistle-keywords | 2/3 | 1/1 | 13 ms | Is OM Alex running? |
| 55 | clean | apple | 0/4 | 0/1 | 81 ms | Use the Zoe voice. |
| 55 | clean | apple-keywords | 0/4 | 0/1 | 85 ms | Use the Zoe voice. |
| 55 | clean | apple-dictation-keywords | 0/4 | 0/1 | 200 ms | Use the Zoe voice |
| 55 | clean | parakeet | 0/4 | 0/1 | 38 ms | Use the Zoe voice. |
| 55 | clean | whistle | 0/4 | 0/1 | 13 ms | Use the Zoe voice. |
| 55 | clean | whistle-keywords | 0/4 | 0/1 | 13 ms | Use the Zoe voice. |
| 55 | noisy | apple | 0/4 | 0/1 | 86 ms | Use the Zoe voice. |
| 55 | noisy | apple-keywords | 0/4 | 0/1 | 88 ms | Use the Zoe voice. |
| 55 | noisy | apple-dictation-keywords | 0/4 | 0/1 | 214 ms | Use the Zoe voice |
| 55 | noisy | parakeet | 0/4 | 0/1 | 37 ms | Use the Zoe voice. |
| 55 | noisy | whistle | 0/4 | 0/1 | 14 ms | Use the Zoe voice. |
| 55 | noisy | whistle-keywords | 0/4 | 0/1 | 12 ms | Use the Zoe voice. |
| 56 | clean | apple | 0/6 | 0/1 | 105 ms | Ask Opus to summarize it instead. |
| 56 | clean | apple-keywords | 0/6 | 0/1 | 105 ms | Ask Opus to summarize it instead. |
| 56 | clean | apple-dictation-keywords | 0/6 | 0/1 | 212 ms | Ask opus to summarize it instead |
| 56 | clean | parakeet | 0/6 | 0/1 | 41 ms | Ask Opus to summarize it instead. |
| 56 | clean | whistle | 2/6 | 1/1 | 17 ms | Hask opens to summarize it instead. |
| 56 | clean | whistle-keywords | 2/6 | 1/1 | 18 ms | Hask opens to summarize it instead. |
| 56 | noisy | apple | 0/6 | 0/1 | 100 ms | ask Opus to summarize it instead. |
| 56 | noisy | apple-keywords | 0/6 | 0/1 | 102 ms | ask Opus to summarize it instead. |
| 56 | noisy | apple-dictation-keywords | 3/6 | 1/1 | 220 ms | Pasco is the summarize it instead |
| 56 | noisy | parakeet | 0/6 | 0/1 | 44 ms | Ask Opus to summarize it instead. |
| 56 | noisy | whistle | 5/6 | 1/1 | 20 ms | Hask opens the summer as it instead. |
| 56 | noisy | whistle-keywords | 5/6 | 1/1 | 19 ms | Hask opens the summer as it instead. |
| 57 | clean | apple | 1/6 | 1/1 | 96 ms | Coinan, I'm away for an hour. |
| 57 | clean | apple-keywords | 1/6 | 1/1 | 97 ms | Coinan, I'm away for an hour. |
| 57 | clean | apple-dictation-keywords | 0/6 | 0/1 | 232 ms | Koinon I'm away for an hour |
| 57 | clean | parakeet | 1/6 | 1/1 | 41 ms | Coynan, I'm away for an hour. |
| 57 | clean | whistle | 2/6 | 1/1 | 15 ms | Coinin am away for an hour. |
| 57 | clean | whistle-keywords | 1/6 | 0/1 | 17 ms | Koinon am away for an hour. |
| 57 | noisy | apple | 2/6 | 1/1 | 93 ms | Coin and I'm away for an hour. |
| 57 | noisy | apple-keywords | 2/6 | 1/1 | 93 ms | Coin and I'm away for an hour. |
| 57 | noisy | apple-dictation-keywords | 3/6 | 1/1 | 315 ms | For an hour |
| 57 | noisy | parakeet | 1/6 | 1/1 | 43 ms | Coinin', I'm away for an hour. |
| 57 | noisy | whistle | 3/6 | 1/1 | 16 ms | Quite in time away for an hour. |
| 57 | noisy | whistle-keywords | 3/6 | 1/1 | 16 ms | Quite in time away for an hour. |
| 58 | clean | apple | 0/5 | 0/1 | 86 ms | Pause notifications until 3 o'clock. |
| 58 | clean | apple-keywords | 0/5 | 0/1 | 86 ms | Pause notifications until 3 o'clock. |
| 58 | clean | apple-dictation-keywords | 0/5 | 0/1 | 257 ms | Pause notifications until 3 o'clock |
| 58 | clean | parakeet | 0/5 | 0/1 | 42 ms | Pause notifications until three o'clock. |
| 58 | clean | whistle | 2/5 | 0/1 | 18 ms | Austin indications until three o'clock. |
| 58 | clean | whistle-keywords | 2/5 | 0/1 | 18 ms | Austin indications until three o'clock. |
| 58 | noisy | apple | 0/5 | 0/1 | 91 ms | Pause notifications until 3 o'clock. |
| 58 | noisy | apple-keywords | 0/5 | 0/1 | 95 ms | Pause notifications until 3 o'clock. |
| 58 | noisy | apple-dictation-keywords | 0/5 | 0/1 | 212 ms | Pause notifications until 3 o'clock |
| 58 | noisy | parakeet | 0/5 | 0/1 | 41 ms | Pause notifications until 3 o'clock. |
| 58 | noisy | whistle | 1/5 | 0/1 | 16 ms | Post notifications until 3 o'clock. |
| 58 | noisy | whistle-keywords | 1/5 | 0/1 | 18 ms | Post notifications until 3 o'clock. |
| 59 | clean | apple | 0/3 | 0/0 | 81 ms | Thanks, that's all. |
| 59 | clean | apple-keywords | 0/3 | 0/0 | 83 ms | Thanks, that's all. |
| 59 | clean | apple-dictation-keywords | 0/3 | 0/0 | 181 ms | Thanks, that's all |
| 59 | clean | parakeet | 0/3 | 0/0 | 36 ms | Thanks, that's all. |
| 59 | clean | whistle | 0/3 | 0/0 | 14 ms | Thanks, that's all. |
| 59 | clean | whistle-keywords | 0/3 | 0/0 | 13 ms | Thanks, that's all. |
| 59 | noisy | apple | 0/3 | 0/0 | 82 ms | Thanks, that's all. |
| 59 | noisy | apple-keywords | 0/3 | 0/0 | 87 ms | Thanks, that's all. |
| 59 | noisy | apple-dictation-keywords | 0/3 | 0/0 | 184 ms | Thanks, that's all |
| 59 | noisy | parakeet | 0/3 | 0/0 | 36 ms | Thanks. That's all. |
| 59 | noisy | whistle | 0/3 | 0/0 | 13 ms | Thanks, that's all. |
| 59 | noisy | whistle-keywords | 0/3 | 0/0 | 14 ms | Thanks, that's all. |
| 60 | clean | apple | 1/2 | 1/1 | 78 ms | Quinn, good night. |
| 60 | clean | apple-keywords | 1/2 | 1/1 | 79 ms | Quinn, good night. |
| 60 | clean | apple-dictation-keywords | 1/2 | 1/1 | 206 ms | Quinn, good night |
| 60 | clean | parakeet | 1/2 | 1/1 | 38 ms | Quinin, good night. |
| 60 | clean | whistle | 1/2 | 1/1 | 12 ms | Quan Goodnight. |
| 60 | clean | whistle-keywords | 1/2 | 1/1 | 12 ms | Queen Goodnight. |
| 60 | noisy | apple | 1/2 | 1/1 | 80 ms | Coinan, good night. |
| 60 | noisy | apple-keywords | 1/2 | 1/1 | 92 ms | Coinan, good night. |
| 60 | noisy | apple-dictation-keywords | 0/2 | 0/1 | 230 ms | Koinon good night |
| 60 | noisy | parakeet | 2/2 | 1/1 | 36 ms | Coin and good night. |
| 60 | noisy | whistle | 3/2 | 1/1 | 16 ms | It's quite nice. Good night. |
| 60 | noisy | whistle-keywords | 0/2 | 0/1 | 16 ms | Koinon, good night. |

## Synthesis

Apple's voice is the one the maintainer chose in chunk 03, `com.apple.voice.premium.en-US.Zoe`;
the `voice` in the Apple helper's hello is its own default, not the one the bench passed.
Each engine writes its audio to a file (Apple 22.05 kHz, Kokoro 24 kHz), which `afplay`
plays to the output device; first audio is the helper's time from its request to its first
audio buffer. The six texts were played in a shuffled order per text with the engines'
names hidden, and the maintainer rated each 1 to 5 right after hearing it.

Observations:

- **The maintainer prefers Kokoro**: a mean of 4.7 against Apple's 3.2, and higher on every
  text. In the maintainer's words, about the voice of texts 1A, 2B and 3A (Kokoro): "voice
  was fantastic quality, though a bit slow"; about the other (Apple's Zoe): "the alternate is
  bad, we need to stop using that one. its hard to understand, choppy." The maintainer also
  noted that Zoe had sounded choppy live in chunk 03, less so than here.
- **The choppiness is Apple's voice, not the bench's.** Text 01 rendered by Zoe has 304
  sample-to-sample jumps above 0.3 of full scale (clicks), against 3 for Kokoro, and macOS's
  own `say -v "Zoe (Premium)"` renders the same text with the same 304.
- **Kokoro is slower to start and to speak**: a median of 818 ms to first audio against
  536 ms, and about 15% longer audio for the same text (15.3 s against 13.2 s for text 01).
  Its speed setting was left at 1.0.
- **Speed.** After the rating, text 01 was played with Kokoro at speeds 1.0, 1.1 and 1.2
  (15.3 s, 14.5 s and 13.2 s of audio). The maintainer: "1.1 is the smoothest speed up, 1.2
  is acceptable, but pauses get very short". Apple's Zoe stays the speaking voice until
  Kokoro is wired in, as the maintainer decided.

### Engines

| Engine | Helper | Info | Note |
| --- | --- | --- | --- |
| apple | koinon-voice-apple 0.1.0 | {"foundation_models":"1.5.2","model":"available","os":"Version 26.5 (Build 25F71)","speech":"3525.2.2","transcriber_available":true,"voice":"com.apple.voice.premium.en-US.Ava"} | AVSpeechSynthesizer, com.apple.voice.premium.en-US.Zoe |
| kokoro | koinon-voice-bench-kokoro 0.1.0 | {"engine": "kokoro", "model": "mlx-community/Kokoro-82M-bf16", "package": "mlx-audio", "package_version": "0.6.1", "python": "3.11.17"} | Kokoro-82M through mlx-audio, voice af_heart |

### Totals

| Engine | Texts | Failed | Median first audio | Mean rating |
| --- | --- | --- | --- | --- |
| apple | 6 | 0 | 536 ms | 3.2 (6) |
| kokoro | 6 | 0 | 818 ms | 4.7 (6) |

### Renditions

| Text | Engine | First audio | Audio | Rating |
| --- | --- | --- | --- | --- |
| 01 | kokoro | 818 ms | 15.3 s | 5 |
| 01 | apple | 536 ms | 13.2 s | 3 |
| 02 | apple | 442 ms | 13.9 s | 4 |
| 02 | kokoro | 849 ms | 16.5 s | 5 |
| 03 | kokoro | 612 ms | 17.4 s | 5 |
| 03 | apple | 755 ms | 14.7 s | 3 |
| 04 | apple | 412 ms | 15.7 s | 3 |
| 04 | kokoro | 633 ms | 17.8 s | 5 |
| 05 | kokoro | 756 ms | 22.0 s | 4 |
| 05 | apple | 739 ms | 19.3 s | 3 |
| 06 | apple | 424 ms | 22.2 s | 3 |
| 06 | kokoro | 934 ms | 25.2 s | 4 |
