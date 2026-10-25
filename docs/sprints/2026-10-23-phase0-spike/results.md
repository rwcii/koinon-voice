# Results

Chunk 06 (criterion 7): the Phase 0 evidence in one page. For each choice it gives the
candidates measured, a proposal and its reason. The proposals are for the maintainer's
decision; ADR 001 to ADR 003 and PDR 004, Decision 2 carry them as drafts. The evidence is
in `koinon-map.md`, `scorecard-summarize.md`, `scorecard-speech.md`, `scorecard-wake.md` and
`interruption.md`, all measured on the maintainer's Mac (Apple M2 Max, macOS 26.5).

| Choice | Proposal | Record |
| --- | --- | --- |
| Summarizer | Claude Haiku where the maintainer enables external inference for a host; Qwen3-4B on oMLX otherwise | PDR 004, Decision 2; ADR 003 |
| Recognition | Apple `SpeechTranscriber`, with Koinon names resolved after recognition | ADR 002 |
| Synthesis | Kokoro at speed 1.1 | ADR 002 |
| Wake word | openWakeWord, with a model that has yet to reach a usable miss rate; push-to-talk until it does | ADR 002 |
| Echo cancellation | Apple voice processing in the Swift helper, which plays all speech | ADR 002 |
| Engine processes | Sidecars, as proposed; how the Python engines ship is the maintainer's to decide | ADR 002 |
| Koinon connection | The maintainer interface of rwcii/koinon#464; Phase 1 waits for it | ADR 001 |

## Summarization

Measured: seven engines on 20 synthetic decision items, with correctness against an answer
key, time to first spoken word, cost, and the maintainer's blind rating of the first three
items (three ratings per engine). No real item was sent.

| Engine | Where | Correct | First spoken word (median) | Cost for 20 items | Mean rating |
| --- | --- | --- | --- | --- | --- |
| Claude Haiku | external | 20/20 | 1.39 s | $0.0019 | 3.0 |
| Claude Sonnet | external | 20/20 | 1.72 s | $0.038 | 3.3 |
| Claude Opus | external | 19/20 | 2.40 s | $0.093 | 4.3 |
| OpenAI `gpt-6-luna` | external | 18/20 | 2.92 s | $0.0020 | 4.7 |
| Gemini 3.8 Flash (OpenRouter) | external | 20/20 | 5.95 s | $0.050 | 4.7 |
| Qwen3-4B on oMLX | local | 14/20 | 1.09 s | none | 4.3 |
| Apple Foundation Models | local | 12/20 | 1.86 s | none | 3.7 |

**Proposal.** External inference is enabled per host by the maintainer and is off until then.
Where it is on, Claude Haiku summarizes; where it is off, Qwen3-4B on oMLX does. Every
external summary keeps its spoken and on-screen provider label.

**Reason.** A spoken summary that drops an option or invents a recommendation misleads the
maintainer about a decision (PDR 004's first requirement). The local models missed a fact in
6 and 8 of 20 items, mostly options dropped and recommendations invented; the external ones
in at most 2, which look like paraphrase. Haiku was correct on all 20, is
the second quickest to speak and costs about a hundredth of a cent per item. Its rating was
the lowest; its summaries spell agent names out ("Codex koinon voice thirty-eight"), which the
prompt can change (#13). The local fallback is Qwen3-4B, the quickest, and more correct than
Apple's model. This is the design session's proposal of local by default and external per
host, with a provider chosen; its spoken request to summarize one item externally stays
open.

**Not shown.** Three ratings per engine separate engines only coarsely. One local open-weight
model was tried; a 12B model did not fit oMLX's memory guard. After Phase 0, #9 fixed two
check errors (a denial after its claim, and "due to"), which made Gemini's SYN-12 correct, and
added a count of hashes, paths and Markdown read aloud; the recorded summaries were rescored
without a new run.

## Recognition

Measured: Apple `SpeechTranscriber` (with and without contextual strings),
`DictationTranscriber` with them, Parakeet TDT 0.6B v2, and Cactus Whistle with and without
keyword biasing, on the maintainer's reading of 60 Koinon phrases, clean and with coffee-shop
noise.

| Engine | WER clean, noisy | Koinon-term WER | "Koinon" heard (of 17) | Decision verbs heard (of 13) | Latency |
| --- | --- | --- | --- | --- | --- |
| Apple `SpeechTranscriber` | 11.4%, 14.3% | 28.7%, 29.9% | 0, 0 | 13, 11 | 87 ms |
| Apple `DictationTranscriber`, with the list | 13.0%, 15.3% | 32.2%, 28.7% | 9, 12 | 10, 11 | 210 ms |
| Parakeet | 11.1%, 12.7% | 29.9%, 33.3% | 0, 0 | 13, 12 | 40 ms |
| Whistle, with the list | 16.3%, 21.2% | 12.6%, 24.1% | 13, 12 | 8, 5 | 16 ms |

The decision verbs are "approve", "reject", "confirm" and "merge" in the phrase list; the
`DictationTranscriber` count is from the committed transcripts and was not in the scorecard.

**Proposal.** Apple `SpeechTranscriber` recognizes, in the Swift helper. Koinon's names (the
word "Koinon", peers, repositories) are matched against the names the connected daemons
report, after recognition. Measured after Phase 0 on the recorded transcripts (#11,
`scorecard-names.md`), matching takes `SpeechTranscriber`'s Koinon-term rate from 28.7% and
29.9% to 5.7% in both sets, with no wrong name in its 720 transcripts; the bare word "Koinon" also matches
ordinary words such as "canon" and "can in", which peer names do not.

**Reason.** A misheard verb is the costly error: the read-back of PDR 002, Decision 2 catches
it only if "confirm" is heard. The two engines given the keyword list heard Koinon's names
best: Whistle has the lowest Koinon-term rate, and `DictationTranscriber` heard "Koinon" in 9
and 12 of 17 and some full peer names ("claude koinon voice b 9"). But Whistle heard the
verbs in 8 and 5 of 13, and `DictationTranscriber` in 10 and 11, with a higher overall rate,
about the same Koinon-term rate as `SpeechTranscriber` (32.2% and 28.7% against 28.7% and
29.9%), and more than twice its latency. Apple's
`SpeechTranscriber` and Parakeet heard the verbs best and are within 0.3 and 1.6 points
of each other overall. Apple runs in the helper that already captures the microphone with
echo cancellation, without a Python runtime; Parakeet is the alternative if its 1.6 points in
noise matter more. No engine heard "Koinon" without a list, so the names need a step of their
own whichever engine is chosen.

## Synthesis

Measured: Apple speech synthesis (voice Zoe, Premium) and Kokoro-82M (voice `af_heart`) on
six summaries, rated blind.

**Proposal.** Kokoro at speed 1.1.

**Reason.** The maintainer rated Kokoro 4.7 against Apple's 3.2, higher on every text, and
said of Zoe "we need to stop using that one. its hard to understand, choppy"; the choppiness
is in Apple's own rendering. Kokoro starts later (787 ms to first audio against 489 ms) and
speaks about 15% longer at speed 1.0; of 1.1 and 1.2 the maintainer said "1.1 is the
smoothest speed up". Until Kokoro is wired in, Zoe stays, as the maintainer decided.

## Wake word

Measured: "Koinon" with openWakeWord (a model trained here, v4 of four) and with phrase
matching (Silero VAD, then Whistle with the list), on 20 of the maintainer's takes at each of
0.5, 1.5 and 2.7 m, and on 1.75 h of background audio (music with and without vocals, a
meeting, a podcast, coffee-shop sounds).

| Detector | Woke, near, mid, far (of 20) | False wakes |
| --- | --- | --- |
| openWakeWord | 11, 8, 10 | 1 (0.57 per hour) |
| Phrase matching | 17, 2, 5 | 0 |

**Proposal.** openWakeWord detects the wake word, with its model trained again, for example
with recorded positives of the maintainer's voice, until it reaches a miss rate the
maintainer accepts. Until then push-to-talk (PDR 003, Decision 1) is the reliable way in, and
armed listening with the wake word is offered as experimental.

**Reason.** Neither detector is good enough: openWakeWord missed half the takes and phrase
matching all but 7 of the 40 beyond 0.5 m. openWakeWord's misses do not grow with distance
and its scores are near 0 or 1, which points to the model rather than the audio: it woke on 65%
of held-out synthetic speech and on 48% of the maintainer's, and on none of macOS's voices
reading "Koinon". Phrase matching keeps a recognizer running on every sound and fails at
distance, and the bench does not show whether its misses come from the voice activity step
or the recognizer.

## Echo cancellation and interruption

Measured: the Swift helper speaking a 14.7 s summary through the speakers with Apple's voice
processing on, while listening; once with nobody speaking and once with the maintainer saying
"Stop, wait, I have a question."

**Proposal.** Apple voice processing (`AVAudioEngine` with voice processing on the input) in
the Swift helper, which plays all of the bridge's speech through the same engine, whatever
engine synthesized it. An interruption is any speech heard during playback; the bridge stops,
then acts on what follows, not on a cue word.

**Reason.** No bridge word reached the transcript in either run. But "Stop", said over the
bridge's speech, was suppressed as echo, and only "wait, I have a question" was heard, so a
cue word cannot be relied on. Echo cancellation needs the played audio as its reference, so
speech synthesized elsewhere (Kokoro) must still be played by the helper. Ordinary playback
through `AVAudioEngine` without voice processing was silent through the maintainer's
four-channel USB mixer, while playback with voice processing was heard, which is another
reason for one engine.

## Engine processes

Measured: the Swift helper and the Python bench helper's engines (Parakeet, Whistle, Kokoro,
openWakeWord, and phrase matching with Silero VAD) behind the same stdio protocol; no file
failed in the scored runs. The protocol's own cost was not measured separately; a whole
request for a phrase of about 4 s took 16 ms with Whistle, which bounds it.

**Proposal.** ADR 002, Decision 1 stands. The proposed engines need two runtimes: the Swift
helper, built on the maintainer's machine, and Python, for Kokoro and openWakeWord. ADR 002
rejected Python services; these are helpers, but they still need a Python environment. The
maintainer decides between a managed Python environment for those helpers and native
runtimes for them, which do not exist yet in this repository.

## Koinon connection

Measured: what Koinon v0.4.1 offers a non-browser maintainer client, tried against a scratch
daemon (`koinon-map.md`).

**Proposal.** ADR 001, Decision 3 stands: the bridge acts through a maintainer interface in
Koinon, and Phase 1's Koinon features wait for rwcii/koinon#464. A connector to a remote
daemon gets its login by running `koinon dashboard --no-open` on that host over SSH, until
#464 gives a lasting credential.

**Reason.** Everything a maintainer can do in v0.4.1 is an HTML dashboard page or form behind
a one-time login: no JSON, no change notices, no lasting credential, and no client recorded on
a verdict. A client can drive it, as `curl` did, but that is the scraping ADR 001 rejects.
A login works through a forwarded port, with the link's address rewritten; a TCP forward stood
in for SSH.

## Follow-up issues

Filed during this chunk:

- #8: ordinary playback may be silent on multichannel outputs.
- #9: the summarize check marks a denial after its claim as a claim, and does not count
  text that should not be read aloud.
- #10: retrain the "Koinon" wake word model to a usable miss rate.
- #11: resolve Koinon names after recognition, and measure it on the recorded transcripts
  (measured in `scorecard-names.md`).
- #12: speak with Kokoro, once ADR 002, Decision 4 is decided.
- #13: summaries that speak names as a person would, rated on more items.
- #14: score a larger local summarization model.
- #15: log in to a remote daemon through an SSH forward.

The gaps in Koinon itself are on rwcii/koinon#464.
