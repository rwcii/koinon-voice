# Scorecard — Wake

Chunk 05's live check 4 (definition of done): each wake detector on the maintainer's takes of
"Koinon" at three distances, and false wakes on at least one hour of background recordings.
The recordings and the wake model stay under `~/.local/share/koinon-voice/` and are not in Git
(definition of done, Private data); this file holds scores, versions, training settings and
hashes.

## Run

- Date: 2026-10-24, on the maintainer's Mac (Apple M2 Max, macOS 26.5, build 25F71), Go
  1.27.3, with `koinon-voice bench wake` at its defaults (threshold 0.5). Each detector's first
  request also loads its model; it is scored like the others, and its Latency includes the
  load.
- Takes: recorded by the maintainer with `koinon-voice corpus wake`, through the EMEET
  SmartCam Nova 4K's microphone (channel 1 of 2), 16 kHz mono 16-bit, about 3 s each, "Koinon"
  said once per take, in a quiet room. Twenty takes per distance: `near` (about 0.5 m, 61.4 s,
  manifest SHA-256 prefix `b8e14dff7b3359d5`), `mid` (about 1.5 m, 61.5 s, `80fcac5a06aba7c3`)
  and `far` (about 2.7 m, the maintainer's 9 feet, 61.8 s, `79a6437d1af3a423`).
- Background: recorded with `koinon-voice corpus background` through the same microphone, the
  source playing in the room and nobody saying "Koinon", 1.75 h in all. `music`: two
  recordings of the maintainer's office music, 01 instrumental and 02 with vocals (2,519.4 s,
  `aadf73900e774256`). `meeting`: one of a meeting recording played in the room (1,259.3 s,
  `8d98184618bd2f3d`), recorded after peer review (Decisions 4). `podcast`: one of a podcast
  the maintainer recorded (1,259.7 s, `1b54442c4d1e4ec9`). `coffee`: one of coffee shop sounds
  from a YouTube recording (1,259.4 s, `985e591961000754`). Each asked for 1,200 s and ran
  about 59 s longer.
- Bench helper (`helpers/bench`), in chunk 04's Python 3.11.17 virtual environment:
  - openWakeWord 0.6.0, installed from the training checkout below, with onnxruntime 1.31.0
    and the feature models from release v0.5.1 (`melspectrogram.onnx` SHA-256 prefix
    `ba2b0e0f8b7b8753`, `embedding_model.onnx` `70d164290c1d095d`), on 80 ms frames.
  - Phrase matching: Silero VAD 6.2.3 segments, each widened by 200 ms, transcribed by Cactus
    Whistle (`cactus-needle` 3.2.8, the model of scorecard-speech.md) with chunk 04's biasing
    list; it wakes on the word "koinon" in a transcript. Whistle with the list was chunk 04's
    best engine on Koinon terms.
- A take wakes if the detector fires anywhere in it; firings closer than 2 s count once. The
  Latency column is the bench's processing time per file, not a time to wake.
- openWakeWord gets 2 s of silence, not scored, before each file. Its `reset()` refills the
  feature buffer from unseeded random noise, which reached the first 1.3 s of a file's
  scores: two runs without the lead-in differed by up to 0.2 on a take and woke on 12 and then
  13 near takes. With it, two runs gave the same scores.

## The "Koinon" model

The model is v4 of four trained on 2026-10-24 with openWakeWord's synthetic-sample procedure
(`openwakeword/train.py`), each from a configuration `koinon.yml`: a DNN of layer size 32,
50,000 steps, batches of 1,024 ACAV100M negatives, 50 generated negatives and 50 generated
positives. Each run's automatic schedule doubled the negative weight twice, because the
false-positive rate stayed above its target after the first two sequences.

| Model | Positive phrases | Custom negatives | Clips (train, test) | Augmentation rounds | Negative weight, target FP/h | openWakeWord validation: accuracy, recall, FP/h | Held-out synthetic positives woken | `koinon.onnx`, `koinon.onnx.data` SHA-256 prefixes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| v1 | koinon, coin on | 11 (below) | 10,000, 2,000 | 1 | 1,500, 0.2 | 0.652, 0.310, 4.51 | 90 of 200 | `02b04e32a900dd11`, `83d5b793fc423457` |
| v2 | koinon, coin on | 11 | 50,000, 2,000 | 2 | 1,500, 0.2 | 0.530, 0.060, 0.53 | 0 of 200 | `a1973e8eeb3cdf06`, `8c04039e37fe4abd` |
| v3 | koinon, coin on | 11 | v2's | 2 | 500, 1.0 | 0.584, 0.169, 0.88 | 17 of 200 | `95841941784ef6d2`, `f3a4f69b220080f4` |
| v4 | koinon | 5 (below) | 50,000, 2,000, new | 2 | 500, 1.0 | 0.824, 0.650, 1.15 | 192 of 200 | `9060e913e5bad05b`, `73944ab6dc18de1c` |

- Custom negatives for v1 to v3: coin, coins, coin in, quinine, quinoa, canon, cannon, corn
  on, join on, come on, koi pond. v4 kept quinine, quinoa, canon, cannon and koi pond: with
  "coin on" both a target and close to six negatives, v1 to v3 did not learn the word. v2's
  larger set was the experimenter's choice after v1's smoke test; v3 and v4 followed the
  maintainer's answers below.
- The smoke tests ran each model on 200 held-out generated positives and negatives and on
  macOS `say` voices. No model fired on a negative (largest score 0.14). No model woke on
  `say` reading the spelling "Koinon" (0 of 10 for each), which those voices pronounce
  otherwise; spelled "Koy-nahn", "Coy non", "coin-on" and "Coin on", v4 woke on 8 of 12
  (Samantha 4, Fred 3, Daniel 1). The pretrained `hey_jarvis` model woke on 5 of 5 `say`
  positives and 0 of 5 negatives through the same harness.
- Sources: openWakeWord at `368c03716d1e92591906a84949bc477f3a834455`;
  piper-sample-generator 3.2.0 at `2971426a55072f7d22fec416ca7800df8bd23207` with piper-tts
  1.3.0 and the generator model `en_US-libritts_r-medium.pt` (SHA-256 prefix
  `e95ee53770bf598c`), up to 800 speakers; the feature models above; ACAV100M features
  (`openwakeword_features_ACAV100M_2000_hrs_16bit.npy`, 2,000 h) and the validation features
  (`validation_set_features.npy`, 11.3 h) for negatives and the false-positive rate; 270 MIT
  room impulse responses and 1,500 AudioSet `bal_train` clips (4.13 h), both at 16 kHz, for
  augmentation. Python 3.11.17, torch 2.14.1, torchaudio 2.11.0, onnx 1.23.2, onnxscript
  0.7.2, audiomentations 0.33.0, torch-audiomentations 0.12.0, scipy 1.16.3.
- Workarounds, all outside the repository, for a procedure written for older releases and
  Linux:
  1. `PYTORCH_JIT=0`: the JIT graph fuser fails on Apple GPUs after a few batches.
  2. "koinon" is not in CMUdict and openWakeWord's fallback phonemizer model is no longer
     downloadable (HTTP 403), so the pronunciation K OY1 N AA0 N, espeak-ng's, is added to
     the dictionary that builds the confusable negatives.
  3. A shim adapts `train.py`'s `from generate_samples import generate_samples` to the
     package API of piper-sample-generator 3.2.0.
  4. torchaudio 2.11 loads audio only through TorchCodec, which needs FFmpeg, and has no
     `info()`: a startup shim gives `torchaudio.load` (with `frame_offset` and `num_frames`)
     and `torchaudio.info` from soundfile.
  5. scipy is held at 1.16.3, because `acoustics` needs `sph_harm`.
  6. The generator writes 22.05 kHz clips and augmentation needs 16 kHz: a step between
     generation and augmentation resamples them.
  7. macOS starts DataLoader workers with spawn, which cannot pickle the batch generator, so
     `train.py` loads training batches in the main process (`num_workers=0`).
  8. The export to TFLite fails (no `onnx_tf`); only ONNX is needed. The ONNX export uses
     opset 18 (conversion to 13 fails on LayerNormalization) and keeps the weights in
     `koinon.onnx.data` beside the model.

## Decisions

The maintainer's choices during the run, with the questions put to them; the options are
quoted as offered, the recommended one first.

1. **v3's weighting**, 2026-10-24, after v2's smoke test. Asked: "Both Koinon wake models came
   out weak: v1 missed every macOS voice, and v2, trained on 5x the data, now also misses
   held-out synthetic clips (recall 6%). How should I proceed?" Options: "Relax FP weighting
   (Recommended)", retraining on v2's features with a looser false-wake target and a lower
   maximum negative weight; "Drop near-confusables", removing "corn on", "join on", "come on"
   and "coin in" from the negatives; "Add macOS voices", adding `say` positives, which departs
   from the synthetic procedure the chunk names; and "Stop, record result", scoring the best
   model as it was. Answer: "Relax FP weighting (Recommended)". Scope: v3's two settings only.
2. **v4's phrases**, 2026-10-24, after v3's smoke test. Asked: "v3 (relaxed weighting) is still
   weak: openWakeWord validation recall 17% at 0.88 false wakes/h; smoke test 0/10
   macOS-voice positives (best 0.39), 17/200 held-out Piper positives, 0 false wakes.
   Weighting still doubled twice, so the model can't separate 'Koinon' from ordinary speech
   well. Next?" Options: "Drop near-confusables (Recommended)", removing "corn on", "join on",
   "come on", "coin in", "coin" and "coins" from the negatives and "coin on" from the targets,
   with new clips; "Stop, record result"; and "Add macOS voices". Answer: "Drop
   near-confusables (Recommended)". Scope: v4's phrases and clips; the weighting stayed v3's.
3. **Music, instrumental and vocals**, 2026-10-24. The maintainer asked "should music be
   instrumental or vocals?"; the experimenter recommended vocals, because sung words are what
   can trip a wake detector. Answer: "let's do an instrmental run and a vocals run, so you
   have both".
4. **The meeting recording**, 2026-10-24, after peer review F4 found that criterion 4's
   meeting recording was missing. Asked: "Criterion 4 asks for false wakes against 'music, a
   meeting recording and speech that is not addressed to the bridge'. We have music
   (instrumental and vocals), your podcast and coffee-shop sounds, but no meeting recording.
   How should chunk 05 handle that?" Options: "Record a meeting (Recommended)", 20 minutes of
   a meeting recording played in the room as a new `meeting` source; "Podcast stands in",
   recording a decision that the podcast substitutes for it; and "Change the criterion".
   Answer: "Record a meeting (Recommended)".

The sources music, podcast and coffee shop sounds were chosen by the maintainer in an earlier
session; its handoff records the choice but not the question or the answer. The length of
each recording, 20 minutes, is the experimenter's reading of the one-hour minimum, given
when the maintainer said, of the coffee shop sounds, "stop when you reach what you need for
this sound source".

## Observations

- **Neither detector is good enough for a wake word.** openWakeWord with v4 woke on 29 of 60
  takes (11, 8 and 10 by distance) with 0.57 false wakes per hour; phrase matching on 24 of
  60 (17, 2 and 5) with none.
- **openWakeWord does not get worse with distance**, but misses half the takes at each: its
  scores are near 1 or near 0, with few between: only 3 of its 31 misses scored above 0.2
  (0.39 to 0.44), so a threshold of 0.35 would wake on 32 takes. Its recall on the
  maintainer's voice, 48%, is below its 65% on held-out generated speech, and
  it missed every `say` voice reading "Koinon": it likely learned the generator's
  pronunciation more than the word.
- **Phrase matching is the best at 0.5 m and fails beyond it.** It woke on 17 of 20 near
  takes and 7 of 40 farther ones. The bench does not record whether Silero VAD found no
  segment in a quieter take or Whistle's transcript lacked the word.
- **False wakes are rare for both.** openWakeWord's one, at 9.6 min of the coffee shop
  recording, scored 0.578; the highest score in the other recordings was 0.047, in the
  meeting. Neither detector woke on the vocals or the meeting, which hold the most words.
- No file failed.

## Detectors

| Detector | Helper | Info | Note |
| --- | --- | --- | --- |
| openwakeword | koinon-voice-bench-openwakeword 0.1.0 | {"engine": "openwakeword", "model": "openWakeWord with the model in the request", "package": "openwakeword", "package_version": "0.6.0", "python": "3.11.17"} | openWakeWord, model ~/.local/share/koinon-voice/wake-train/model/koinon.onnx (SHA-256 prefix 9060e913e5bad05b; weights koinon.onnx.data 73944ab6dc18de1c), threshold 0.50 |
| phrase | koinon-voice-bench-phrase 0.1.0 | {"engine": "phrase", "model": "Silero VAD, then Cactus-Compute/whistle", "package": "cactus-needle", "package_version": "3.2.8", "python": "3.11.17"} | Silero VAD, then Cactus Whistle biased to: Koinon, claude koinon voice b 9, codex koinon voice 38, codex koinon voice af, deepseek koinon 7 c, koinon voice, rwcii; wakes on the word koinon |

## Misses

Takes of "Koinon" the detector did not wake on; a failed file counts as a miss.

| Detector | Set | Takes | Failed | Woke | Misses |
| --- | --- | --- | --- | --- | --- |
| openwakeword | wake/near | 20 | 0 | 11 | 9 |
| openwakeword | wake/mid | 20 | 0 | 8 | 12 |
| openwakeword | wake/far | 20 | 0 | 10 | 10 |
| phrase | wake/near | 20 | 0 | 17 | 3 |
| phrase | wake/mid | 20 | 0 | 2 | 18 |
| phrase | wake/far | 20 | 0 | 5 | 15 |

## False wakes

Wakes on background audio, with firings closer than 2 s counted once. The rate is over the hours scored; a failed recording's hours are not in it.

| Detector | Set | Recordings | Failed | Hours scored | Hours failed | False wakes | Per hour |
| --- | --- | --- | --- | --- | --- | --- | --- |
| openwakeword | background/music | 2 | 0 | 0.70 | 0.00 | 0 | 0.00 |
| openwakeword | background/meeting | 1 | 0 | 0.35 | 0.00 | 0 | 0.00 |
| openwakeword | background/podcast | 1 | 0 | 0.35 | 0.00 | 0 | 0.00 |
| openwakeword | background/coffee | 1 | 0 | 0.35 | 0.00 | 1 | 2.86 |
| openwakeword | all | 5 | 0 | 1.75 | 0.00 | 1 | 0.57 |
| phrase | background/music | 2 | 0 | 0.70 | 0.00 | 0 | 0.00 |
| phrase | background/meeting | 1 | 0 | 0.35 | 0.00 | 0 | 0.00 |
| phrase | background/podcast | 1 | 0 | 0.35 | 0.00 | 0 | 0.00 |
| phrase | background/coffee | 1 | 0 | 0.35 | 0.00 | 0 | 0.00 |
| phrase | all | 5 | 0 | 1.75 | 0.00 | 0 | 0.00 |

## Runs

| Detector | Set | File | Wakes | Firings | Max score | Latency |
| --- | --- | --- | --- | --- | --- | --- |
| openwakeword | wake/near | 01 | 0 |  | 0.389 | 1535 ms |
| openwakeword | wake/near | 02 | 0 |  | 0.017 | 156 ms |
| openwakeword | wake/near | 03 | 0 |  | 0.427 | 154 ms |
| openwakeword | wake/near | 04 | 0 |  | 0.168 | 157 ms |
| openwakeword | wake/near | 05 | 0 |  | 0.174 | 153 ms |
| openwakeword | wake/near | 06 | 1 | 1.12 1.20 1.28 1.36 1.44 1.52 1.60 | 0.985 | 156 ms |
| openwakeword | wake/near | 07 | 1 | 1.36 1.44 1.52 1.60 1.68 1.76 1.84 | 0.984 | 160 ms |
| openwakeword | wake/near | 08 | 1 | 1.04 1.12 1.20 1.28 1.36 | 0.978 | 157 ms |
| openwakeword | wake/near | 09 | 1 | 1.20 1.28 1.44 1.52 | 0.941 | 154 ms |
| openwakeword | wake/near | 10 | 0 |  | 0.041 | 159 ms |
| openwakeword | wake/near | 11 | 0 |  | 0.011 | 160 ms |
| openwakeword | wake/near | 12 | 1 | 1.52 1.60 1.68 1.84 1.92 | 0.948 | 152 ms |
| openwakeword | wake/near | 13 | 1 | 1.28 1.36 1.44 1.52 1.68 | 0.970 | 160 ms |
| openwakeword | wake/near | 14 | 1 | 1.20 1.28 1.36 1.44 1.52 1.60 | 0.978 | 160 ms |
| openwakeword | wake/near | 15 | 1 | 1.28 1.36 1.52 | 0.977 | 157 ms |
| openwakeword | wake/near | 16 | 1 | 1.04 1.12 | 0.843 | 153 ms |
| openwakeword | wake/near | 17 | 0 |  | 0.001 | 154 ms |
| openwakeword | wake/near | 18 | 1 | 1.20 1.28 1.36 1.44 1.52 | 0.968 | 155 ms |
| openwakeword | wake/near | 19 | 1 | 1.20 1.28 1.36 1.44 1.52 1.60 1.68 | 0.984 | 156 ms |
| openwakeword | wake/near | 20 | 0 |  | 0.048 | 159 ms |
| openwakeword | wake/mid | 01 | 0 |  | 0.013 | 155 ms |
| openwakeword | wake/mid | 02 | 1 | 1.12 1.20 1.28 1.36 1.44 | 0.983 | 158 ms |
| openwakeword | wake/mid | 03 | 0 |  | 0.008 | 155 ms |
| openwakeword | wake/mid | 04 | 1 | 1.28 1.36 | 0.872 | 158 ms |
| openwakeword | wake/mid | 05 | 0 |  | 0.001 | 157 ms |
| openwakeword | wake/mid | 06 | 0 |  | 0.001 | 156 ms |
| openwakeword | wake/mid | 07 | 0 |  | 0.001 | 159 ms |
| openwakeword | wake/mid | 08 | 1 | 1.28 | 0.550 | 157 ms |
| openwakeword | wake/mid | 09 | 1 | 1.04 1.12 1.20 1.36 | 0.777 | 156 ms |
| openwakeword | wake/mid | 10 | 0 |  | 0.061 | 154 ms |
| openwakeword | wake/mid | 11 | 1 | 1.60 1.68 1.76 | 0.973 | 154 ms |
| openwakeword | wake/mid | 12 | 1 | 1.20 | 0.653 | 157 ms |
| openwakeword | wake/mid | 13 | 0 |  | 0.442 | 156 ms |
| openwakeword | wake/mid | 14 | 1 | 1.20 1.28 1.36 1.52 | 0.976 | 156 ms |
| openwakeword | wake/mid | 15 | 1 | 1.20 | 0.539 | 157 ms |
| openwakeword | wake/mid | 16 | 0 |  | 0.001 | 159 ms |
| openwakeword | wake/mid | 17 | 0 |  | 0.085 | 155 ms |
| openwakeword | wake/mid | 18 | 0 |  | 0.079 | 156 ms |
| openwakeword | wake/mid | 19 | 0 |  | 0.005 | 158 ms |
| openwakeword | wake/mid | 20 | 0 |  | 0.001 | 155 ms |
| openwakeword | wake/far | 01 | 1 | 1.28 1.36 1.44 1.52 1.60 | 0.963 | 156 ms |
| openwakeword | wake/far | 02 | 0 |  | 0.001 | 155 ms |
| openwakeword | wake/far | 03 | 0 |  | 0.024 | 154 ms |
| openwakeword | wake/far | 04 | 1 | 1.20 | 0.903 | 156 ms |
| openwakeword | wake/far | 05 | 0 |  | 0.002 | 155 ms |
| openwakeword | wake/far | 06 | 0 |  | 0.055 | 155 ms |
| openwakeword | wake/far | 07 | 1 | 1.20 1.28 1.36 1.44 1.52 1.60 | 0.983 | 156 ms |
| openwakeword | wake/far | 08 | 1 | 1.28 | 0.627 | 153 ms |
| openwakeword | wake/far | 09 | 0 |  | 0.003 | 155 ms |
| openwakeword | wake/far | 10 | 0 |  | 0.001 | 155 ms |
| openwakeword | wake/far | 11 | 1 | 1.28 1.36 1.44 1.52 1.60 1.68 | 0.979 | 159 ms |
| openwakeword | wake/far | 12 | 1 | 1.28 1.36 | 0.771 | 158 ms |
| openwakeword | wake/far | 13 | 1 | 1.68 1.76 1.84 1.92 2.00 | 0.981 | 153 ms |
| openwakeword | wake/far | 14 | 1 | 1.52 1.60 1.68 1.76 1.84 1.92 2.00 | 0.984 | 157 ms |
| openwakeword | wake/far | 15 | 0 |  | 0.144 | 156 ms |
| openwakeword | wake/far | 16 | 1 | 1.28 1.36 | 0.675 | 157 ms |
| openwakeword | wake/far | 17 | 0 |  | 0.003 | 159 ms |
| openwakeword | wake/far | 18 | 0 |  | 0.001 | 156 ms |
| openwakeword | wake/far | 19 | 0 |  | 0.005 | 155 ms |
| openwakeword | wake/far | 20 | 1 | 2.08 2.16 2.24 | 0.907 | 157 ms |
| openwakeword | background/music | 01 | 0 |  | 0.007 | 31574 ms |
| openwakeword | background/music | 02 | 0 |  | 0.009 | 31626 ms |
| openwakeword | background/meeting | 01 | 0 |  | 0.047 | 32274 ms |
| openwakeword | background/podcast | 01 | 0 |  | 0.012 | 32069 ms |
| openwakeword | background/coffee | 01 | 1 | 577.12 | 0.578 | 31033 ms |
| phrase | wake/near | 01 | 1 | 0.71 | 1.000 | 1397 ms |
| phrase | wake/near | 02 | 1 | 0.80 | 1.000 | 20 ms |
| phrase | wake/near | 03 | 1 | 0.83 | 1.000 | 18 ms |
| phrase | wake/near | 04 | 1 | 0.64 | 1.000 | 18 ms |
| phrase | wake/near | 05 | 1 | 0.77 | 1.000 | 18 ms |
| phrase | wake/near | 06 | 1 | 0.67 | 1.000 | 17 ms |
| phrase | wake/near | 07 | 0 |  | 0.000 | 19 ms |
| phrase | wake/near | 08 | 1 | 0.58 | 1.000 | 19 ms |
| phrase | wake/near | 09 | 1 | 0.71 | 1.000 | 17 ms |
| phrase | wake/near | 10 | 1 | 0.87 | 1.000 | 18 ms |
| phrase | wake/near | 11 | 1 | 0.55 | 1.000 | 19 ms |
| phrase | wake/near | 12 | 1 | 1.06 | 1.000 | 17 ms |
| phrase | wake/near | 13 | 1 | 0.83 | 1.000 | 17 ms |
| phrase | wake/near | 14 | 0 |  | 0.000 | 18 ms |
| phrase | wake/near | 15 | 1 | 0.64 | 1.000 | 18 ms |
| phrase | wake/near | 16 | 1 | 0.55 | 1.000 | 17 ms |
| phrase | wake/near | 17 | 0 |  | 0.000 | 17 ms |
| phrase | wake/near | 18 | 1 | 0.64 | 1.000 | 18 ms |
| phrase | wake/near | 19 | 1 | 0.74 | 1.000 | 18 ms |
| phrase | wake/near | 20 | 1 | 0.51 | 1.000 | 17 ms |
| phrase | wake/mid | 01 | 0 |  | 0.000 | 17 ms |
| phrase | wake/mid | 02 | 1 | 0.64 | 1.000 | 17 ms |
| phrase | wake/mid | 03 | 0 |  | 0.000 | 16 ms |
| phrase | wake/mid | 04 | 0 |  | 0.000 | 17 ms |
| phrase | wake/mid | 05 | 0 |  | 0.000 | 17 ms |
| phrase | wake/mid | 06 | 0 |  | 0.000 | 10 ms |
| phrase | wake/mid | 07 | 0 |  | 0.000 | 17 ms |
| phrase | wake/mid | 08 | 0 |  | 0.000 | 18 ms |
| phrase | wake/mid | 09 | 0 |  | 0.000 | 18 ms |
| phrase | wake/mid | 10 | 1 | 0.80 | 1.000 | 18 ms |
| phrase | wake/mid | 11 | 0 |  | 0.000 | 18 ms |
| phrase | wake/mid | 12 | 0 |  | 0.000 | 19 ms |
| phrase | wake/mid | 13 | 0 |  | 0.000 | 20 ms |
| phrase | wake/mid | 14 | 0 |  | 0.000 | 18 ms |
| phrase | wake/mid | 15 | 0 |  | 0.000 | 17 ms |
| phrase | wake/mid | 16 | 0 |  | 0.000 | 11 ms |
| phrase | wake/mid | 17 | 0 |  | 0.000 | 18 ms |
| phrase | wake/mid | 18 | 0 |  | 0.000 | 21 ms |
| phrase | wake/mid | 19 | 0 |  | 0.000 | 18 ms |
| phrase | wake/mid | 20 | 0 |  | 0.000 | 18 ms |
| phrase | wake/far | 01 | 0 |  | 0.000 | 18 ms |
| phrase | wake/far | 02 | 0 |  | 0.000 | 16 ms |
| phrase | wake/far | 03 | 1 | 0.74 | 1.000 | 17 ms |
| phrase | wake/far | 04 | 0 |  | 0.000 | 21 ms |
| phrase | wake/far | 05 | 0 |  | 0.000 | 10 ms |
| phrase | wake/far | 06 | 1 | 0.77 | 1.000 | 18 ms |
| phrase | wake/far | 07 | 0 |  | 0.000 | 19 ms |
| phrase | wake/far | 08 | 0 |  | 0.000 | 18 ms |
| phrase | wake/far | 09 | 0 |  | 0.000 | 11 ms |
| phrase | wake/far | 10 | 0 |  | 0.000 | 11 ms |
| phrase | wake/far | 11 | 0 |  | 0.000 | 18 ms |
| phrase | wake/far | 12 | 0 |  | 0.000 | 17 ms |
| phrase | wake/far | 13 | 1 | 1.25 | 1.000 | 20 ms |
| phrase | wake/far | 14 | 0 |  | 0.000 | 18 ms |
| phrase | wake/far | 15 | 1 | 1.25 | 1.000 | 17 ms |
| phrase | wake/far | 16 | 0 |  | 0.000 | 19 ms |
| phrase | wake/far | 17 | 0 |  | 0.000 | 18 ms |
| phrase | wake/far | 18 | 0 |  | 0.000 | 19 ms |
| phrase | wake/far | 19 | 1 | 0.93 | 1.000 | 18 ms |
| phrase | wake/far | 20 | 0 |  | 0.000 | 17 ms |
| phrase | background/music | 01 | 0 |  | 0.000 | 4209 ms |
| phrase | background/music | 02 | 0 |  | 0.000 | 4998 ms |
| phrase | background/meeting | 01 | 0 |  | 0.000 | 12307 ms |
| phrase | background/podcast | 01 | 0 |  | 0.000 | 14481 ms |
| phrase | background/coffee | 01 | 0 |  | 0.000 | 4339 ms |
