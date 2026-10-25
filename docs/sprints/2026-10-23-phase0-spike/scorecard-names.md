# Scorecard — Names

A follow-up to Phase 0 (#11): Koinon's names resolved after recognition, as ADR 002,
Decision 2 proposes, measured on the transcripts committed in `scorecard-speech.md`. No new
recording was made and no audio was read.

## Run

- Command: `koinon-voice bench names --ordinary /usr/share/dict/words,internal/speech/testdata/near-names.txt`,
  on 2026-10-24, Go 1.27.3, macOS 26.5.
- Names: the keyword list the biased engines were given, listed below, standing in for the
  names the connected daemons will report.
- Matching (`internal/speech/names.go`): each run of transcript words, up to two words longer
  than a name, gets a rough English sound key (c, k and q alike, a run of vowels as one, a
  doubled letter as one), and becomes the name when the keys differ by at most one edit in
  six. "Koinon" must match exactly ("Quinan", "Coynan" and "coin in" all give `kanan`); a
  peer name allows two or three edits. The closest run wins, then the longer name.
- Held-out text: macOS's `/usr/share/dict/words` (235,976 words, SHA-256 prefix
  `be41ad97963bf8da`), each word as an utterance, and `near-names.txt`, 35 ordinary phrases
  written to sound near "Koinon" ("I can in a minute", "Fire the cannon").

## Reading the check

`WER` and `Koinon-term WER` are the speech scorecard's rates, recomputed from each transcript
before and after resolution. `Resolved` counts the runs replaced by a name; `Wrong` the
replacements whose name is not in the phrase; `Worse` the transcripts with more errors after
resolution than before. In the held-out text every change is wrong.

## Observations

- **Resolution removes most Koinon-term errors.** Apple's `SpeechTranscriber`, the proposed
  engine, goes from 28.7% to 5.7% on Koinon terms clean and from 29.9% to 5.7% noisy, and
  from 11.4% to 3.9% and 14.3% to 6.2% overall. Parakeet goes from 29.9% to 1.1% and 33.3% to
  8.0%. Both end below Whistle with its keyword list (12.6% and 24.1%), which heard the
  decision verbs worst, and resolution does not touch the verbs.
- **No transcript got a wrong name or more errors**, in 720.
- **The bare word "Koinon" sounds like ordinary words; the peer names do not.** 16 of the
  235,976 dictionary words become a name, all "Koinon" or "koinon voice" ("canon", "cannon",
  "canyon", "Canaan"); 13 of the 35 near-name phrases do, all "Koinon" ("can in", "keen on",
  "coin in", "Kenyon", "Conan"). No held-out text became a peer name. Where the bridge
  resolves the bare word, for example only where a name is expected, is a Phase 1 design
  question.
- **What is left** for Apple is mostly not a name: a letter or digit in a peer name ("b 9" as
  "benign"), "coin and" and "uncoin", "Quinn", and verbs ("approved first option",
  "rejectable").

**Not shown.** The matching was designed on these transcripts and its limits chosen by
comparing a few settings on them and on the dictionary; `near-names.txt` was written after.
The names are the seven of the phrase list; a daemon reports more, and more names give more
room for a wrong match. The held-out text is written words, not recognizer output.

## Names

Koinon, claude koinon voice b 9, codex koinon voice 38, codex koinon voice af, deepseek koinon 7 c, koinon voice, rwcii

## Totals

| Engine | Set | Files | WER before | WER after | Koinon-term WER before | Koinon-term WER after | Resolved | Wrong | Worse |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| apple | clean | 60 | 11.4% (35/307) | 3.9% (12/307) | 28.7% (25/87) | 5.7% (5/87) | 14 | 0 | 0 |
| apple-keywords | clean | 60 | 11.4% (35/307) | 3.9% (12/307) | 28.7% (25/87) | 5.7% (5/87) | 14 | 0 | 0 |
| apple-dictation-keywords | clean | 60 | 13.0% (40/307) | 10.4% (32/307) | 32.2% (28/87) | 23.0% (20/87) | 5 | 0 | 0 |
| parakeet | clean | 60 | 11.1% (34/307) | 2.0% (6/307) | 29.9% (26/87) | 1.1% (1/87) | 17 | 0 | 0 |
| whistle | clean | 60 | 25.4% (78/307) | 18.6% (57/307) | 37.9% (33/87) | 18.4% (16/87) | 13 | 0 | 0 |
| whistle-keywords | clean | 60 | 16.3% (50/307) | 15.6% (48/307) | 12.6% (11/87) | 11.5% (10/87) | 1 | 0 | 0 |
| apple | noisy | 60 | 14.3% (44/307) | 6.2% (19/307) | 29.9% (26/87) | 5.7% (5/87) | 14 | 0 | 0 |
| apple-keywords | noisy | 60 | 14.3% (44/307) | 6.2% (19/307) | 29.9% (26/87) | 5.7% (5/87) | 14 | 0 | 0 |
| apple-dictation-keywords | noisy | 60 | 15.3% (47/307) | 11.7% (36/307) | 28.7% (25/87) | 18.4% (16/87) | 6 | 0 | 0 |
| parakeet | noisy | 60 | 12.7% (39/307) | 4.6% (14/307) | 33.3% (29/87) | 8.0% (7/87) | 14 | 0 | 0 |
| whistle | noisy | 60 | 35.5% (109/307) | 30.6% (94/307) | 56.3% (49/87) | 39.1% (34/87) | 9 | 0 | 0 |
| whistle-keywords | noisy | 60 | 21.2% (65/307) | 20.8% (64/307) | 24.1% (21/87) | 23.0% (20/87) | 1 | 0 | 0 |

## Ordinary text

Lines that name nothing; each one resolution changed is listed.

| Source | Lines | Changed | Changes |
| --- | --- | --- | --- |
| words | 235976 | 16 | Cainian: cainian → koinon; Canaan: canaan → koinon; cannon: cannon → koinon; canon: canon → koinon; canonlike: canonlike → koinon voice; canun: canun → koinon; canyon: canyon → koinon; coneen: coneen → koinon; conin: conin → koinon; Cunan: cunan → koinon; Cunoniaceae: cunoniaceae → koinon voice; cyanean: cyanean → koinon; cyanin: cyanin → koinon; kainyn: kainyn → koinon; kanoon: kanoon → koinon; quinin: quinin → koinon |
| near-names.txt | 35 | 13 | I can in a minute.: can in → koinon; I'm keen on that one.: keen on → koinon; That is canon now.: canon → koinon; Fire the cannon.: cannon → koinon; Conan is on tonight.: conan → koinon; The canyon is deep.: canyon → koinon; Ask Kenyon to review it.: kenyon → koinon; Cone in hand.: cone in → koinon; Cannon fire at noon.: cannon → koinon; The canon law.: canon → koinon; Queen in the chess game.: queen in → koinon; Is the coin in the slot?: coin in → koinon; Kenan called.: kenan → koinon |

## Changed transcripts

Errors before and after, over reference words; a wrong match is marked.

| Phrase | Set | Engine | Errors | Term errors | Heard | Resolved |
| --- | --- | --- | --- | --- | --- | --- |
| 01 | clean | apple | 1 to 0 | 1 to 0 | quinnan → koinon | koinon whats waiting |
| 01 | clean | apple-keywords | 1 to 0 | 1 to 0 | quinnan → koinon | koinon whats waiting |
| 01 | clean | parakeet | 2 to 0 | 1 to 0 | coin in → koinon | koinon whats waiting |
| 01 | noisy | apple | 1 to 0 | 1 to 0 | coinan → koinon | koinon whats waiting |
| 01 | noisy | apple-keywords | 1 to 0 | 1 to 0 | coinan → koinon | koinon whats waiting |
| 01 | noisy | parakeet | 1 to 0 | 1 to 0 | coynan → koinon | koinon whats waiting |
| 01 | noisy | whistle | 1 to 0 | 1 to 0 | coinen → koinon | koinon whats waiting |
| 02 | clean | apple | 1 to 0 | 1 to 0 | quinin → koinon | koinon read my inbox |
| 02 | clean | apple-keywords | 1 to 0 | 1 to 0 | quinin → koinon | koinon read my inbox |
| 02 | clean | parakeet | 1 to 0 | 1 to 0 | quinin → koinon | koinon read my inbox |
| 02 | clean | whistle | 1 to 0 | 1 to 0 | coinin → koinon | koinon read my inbox |
| 02 | noisy | apple | 1 to 0 | 1 to 0 | coinan → koinon | koinon read my inbox |
| 02 | noisy | apple-keywords | 1 to 0 | 1 to 0 | coinan → koinon | koinon read my inbox |
| 02 | noisy | parakeet | 1 to 0 | 1 to 0 | coynan → koinon | koinon read my inbox |
| 11 | clean | apple | 1 to 0 | 1 to 0 | quentin voice → koinon voice | merge pull request 5 on koinon voice |
| 11 | clean | apple-keywords | 1 to 0 | 1 to 0 | quentin voice → koinon voice | merge pull request 5 on koinon voice |
| 11 | clean | apple-dictation-keywords | 2 to 0 | 2 to 0 | coin invoice → koinon voice | merge pull request 5 on koinon voice |
| 11 | clean | parakeet | 2 to 0 | 2 to 0 | coin invoice → koinon voice | merge pull request 5 on koinon voice |
| 11 | noisy | apple | 2 to 0 | 2 to 0 | coin invoice → koinon voice | merge pull request 5 on koinon voice |
| 11 | noisy | apple-keywords | 2 to 0 | 2 to 0 | coin invoice → koinon voice | merge pull request 5 on koinon voice |
| 11 | noisy | apple-dictation-keywords | 3 to 1 | 2 to 0 | coin invoice → koinon voice | merge request 5 on koinon voice |
| 11 | noisy | parakeet | 2 to 0 | 2 to 0 | coin invoice → koinon voice | merge pull request 5 on koinon voice |
| 12 | clean | apple | 2 to 0 | 2 to 0 | quinn invoice → koinon voice | open issue 1 on koinon voice |
| 12 | clean | apple-keywords | 2 to 0 | 2 to 0 | quinn invoice → koinon voice | open issue 1 on koinon voice |
| 12 | clean | parakeet | 2 to 0 | 2 to 0 | coininvoice → koinon voice | open issue 1 on koinon voice |
| 12 | clean | whistle | 3 to 1 | 1 to 0 | coin in voice → koinon voice | open issue 1 and koinon voice |
| 12 | clean | whistle-keywords | 3 to 1 | 1 to 0 | coin in voice → koinon voice | open issue 1 and koinon voice |
| 13 | clean | apple | 1 to 0 | 1 to 0 | codex quinin voice af → codex koinon voice af | what did codex koinon voice af ask |
| 13 | clean | apple-keywords | 1 to 0 | 1 to 0 | codex quinin voice af → codex koinon voice af | what did codex koinon voice af ask |
| 13 | clean | apple-dictation-keywords | 3 to 2 | 3 to 2 | quinnen → koinon | what did kodex koinon invoice af ask |
| 13 | clean | parakeet | 3 to 0 | 3 to 0 | kodaks coin invoice af → codex koinon voice af | what did codex koinon voice af ask |
| 13 | clean | whistle | 3 to 1 | 2 to 0 | codex coin in voice af → codex koinon voice af | what did codex koinon voice af passed |
| 13 | noisy | apple | 2 to 0 | 2 to 0 | codex quinn invoice af → codex koinon voice af | what did codex koinon voice af ask |
| 13 | noisy | apple-keywords | 2 to 0 | 2 to 0 | codex quinn invoice af → codex koinon voice af | what did codex koinon voice af ask |
| 13 | noisy | apple-dictation-keywords | 5 to 3 | 4 to 2 | quinn invoice → koinon voice | what did kodak koinon voice afs |
| 13 | noisy | parakeet | 3 to 0 | 3 to 0 | kodaks coin invoice af → codex koinon voice af | what did codex koinon voice af ask |
| 13 | noisy | whistle | 6 to 5 | 4 to 3 | queen in → koinon | what didnt produce koinon boys airbask |
| 14 | clean | apple | 3 to 2 | 3 to 2 | quinnan voice → koinon voice | reply to claude koinon voice benign looks good |
| 14 | clean | apple-keywords | 3 to 2 | 3 to 2 | quinnan voice → koinon voice | reply to claude koinon voice benign looks good |
| 14 | clean | parakeet | 1 to 0 | 1 to 0 | claude coynan voice b 9 → claude koinon voice b 9 | reply to claude koinon voice b 9 looks good |
| 14 | clean | whistle | 4 to 2 | 2 to 1 | coin in voice → koinon voice | reply the clod koinon voice b 9 looks good |
| 14 | noisy | apple | 1 to 0 | 1 to 0 | claude coinan voice b 9 → claude koinon voice b 9 | reply to claude koinon voice b 9 looks good |
| 14 | noisy | apple-keywords | 1 to 0 | 1 to 0 | claude coinan voice b 9 → claude koinon voice b 9 | reply to claude koinon voice b 9 looks good |
| 14 | noisy | parakeet | 3 to 2 | 3 to 2 | coynan voice → koinon voice | reply to claude koinon voice benine looks good |
| 15 | clean | apple | 3 to 0 | 2 to 0 | deep sea coinin 7 c → deepseek koinon 7 c | tell deepseek koinon 7 c to wait until friday |
| 15 | clean | apple-keywords | 3 to 0 | 2 to 0 | deep sea coinin 7 c → deepseek koinon 7 c | tell deepseek koinon 7 c to wait until friday |
| 15 | clean | parakeet | 3 to 0 | 2 to 0 | deep sea coinin 7 c → deepseek koinon 7 c | tell deepseek koinon 7 c to wait until friday |
| 15 | clean | whistle | 5 to 3 | 3 to 2 | coin in → koinon | tell deep sea koinon 7 sea to wait until friday |
| 15 | noisy | apple | 4 to 0 | 2 to 0 | deep sea coin in 7 c → deepseek koinon 7 c | tell deepseek koinon 7 c to wait until friday |
| 15 | noisy | apple-keywords | 4 to 0 | 2 to 0 | deep sea coin in 7 c → deepseek koinon 7 c | tell deepseek koinon 7 c to wait until friday |
| 15 | noisy | apple-dictation-keywords | 2 to 0 | 1 to 0 | deep sea koinon 7 c → deepseek koinon 7 c | tell deepseek koinon 7 c to wait until friday |
| 15 | noisy | parakeet | 2 to 1 | 1 to 0 | deepseek coinin 7 c → deepseek koinon 7 c | tell deepseek koinon 7 c to wait till friday |
| 15 | noisy | whistle | 4 to 2 | 3 to 0 | deepsy coin and → deepseek koinon 7 c | tell deepseek koinon 7 c 7 sea to wait until friday |
| 16 | clean | apple | 2 to 0 | 2 to 0 | codex coin and voice 38 → codex koinon voice 38 | send codex koinon voice 38 a message start chunk 4 |
| 16 | clean | apple-keywords | 2 to 0 | 2 to 0 | codex coin and voice 38 → codex koinon voice 38 | send codex koinon voice 38 a message start chunk 4 |
| 16 | clean | parakeet | 2 to 0 | 2 to 0 | codex coininvoice 38 → codex koinon voice 38 | send codex koinon voice 38 a message start chunk 4 |
| 16 | clean | whistle | 3 to 1 | 2 to 0 | codex coin in voice 38 → codex koinon voice 38 | send codex koinon voice 38 to message start chunk 4 |
| 16 | noisy | apple | 2 to 0 | 2 to 0 | codex quinn in voice 38 → codex koinon voice 38 | send codex koinon voice 38 a message start chunk 4 |
| 16 | noisy | apple-keywords | 2 to 0 | 2 to 0 | codex quinn in voice 38 → codex koinon voice 38 | send codex koinon voice 38 a message start chunk 4 |
| 16 | noisy | parakeet | 2 to 0 | 2 to 0 | codex coininvoice 38 → codex koinon voice 38 | send codex koinon voice 38 a message start chunk 4 |
| 19 | clean | apple | 2 to 0 | 2 to 0 | codex coin and voice af → codex koinon voice af | is codex koinon voice af still running |
| 19 | clean | apple-keywords | 2 to 0 | 2 to 0 | codex coin and voice af → codex koinon voice af | is codex koinon voice af still running |
| 19 | clean | apple-dictation-keywords | 1 to 0 | 1 to 0 | kodex koinon voice af → codex koinon voice af | is codex koinon voice af still running |
| 19 | clean | parakeet | 2 to 0 | 2 to 0 | codex coin and voice af → codex koinon voice af | is codex koinon voice af still running |
| 19 | clean | whistle | 3 to 1 | 2 to 0 | codex coin in voice af → codex koinon voice af | his codex koinon voice af still running |
| 19 | noisy | apple | 2 to 0 | 2 to 0 | codex coin and voice af → codex koinon voice af | is codex koinon voice af still running |
| 19 | noisy | apple-keywords | 2 to 0 | 2 to 0 | codex coin and voice af → codex koinon voice af | is codex koinon voice af still running |
| 19 | noisy | apple-dictation-keywords | 1 to 0 | 1 to 0 | kodex koinon voice af → codex koinon voice af | is codex koinon voice af still running |
| 19 | noisy | parakeet | 2 to 0 | 2 to 0 | codex coin invoice af → codex koinon voice af | is codex koinon voice af still running |
| 20 | clean | apple | 1 to 0 | 1 to 0 | claude coinan voice b 9 → claude koinon voice b 9 | what is claude koinon voice b 9 working on |
| 20 | clean | apple-keywords | 1 to 0 | 1 to 0 | claude coinan voice b 9 → claude koinon voice b 9 | what is claude koinon voice b 9 working on |
| 20 | clean | parakeet | 1 to 0 | 1 to 0 | claude coynan voice b 9 → claude koinon voice b 9 | what is claude koinon voice b 9 working on |
| 20 | clean | whistle | 1 to 0 | 1 to 0 | claude coinden voice b 9 → claude koinon voice b 9 | what is claude koinon voice b 9 working on |
| 20 | noisy | apple | 1 to 0 | 1 to 0 | claude quinan voice b 9 → claude koinon voice b 9 | what is claude koinon voice b 9 working on |
| 20 | noisy | apple-keywords | 1 to 0 | 1 to 0 | claude quinan voice b 9 → claude koinon voice b 9 | what is claude koinon voice b 9 working on |
| 20 | noisy | parakeet | 2 to 0 | 2 to 0 | claude coininvoice b 9 → claude koinon voice b 9 | what is claude koinon voice b 9 working on |
| 20 | noisy | whistle | 3 to 1 | 3 to 1 | quinn in voice → koinon voice | what is claude koinon voice be 9 working on |
| 21 | clean | apple | 1 to 0 | 1 to 0 | quinan → koinon | koinon how many items are waiting |
| 21 | clean | apple-keywords | 1 to 0 | 1 to 0 | quinan → koinon | koinon how many items are waiting |
| 21 | clean | parakeet | 1 to 0 | 1 to 0 | koinen → koinon | koinon how many items are waiting |
| 21 | clean | whistle | 1 to 0 | 1 to 0 | coinin → koinon | koinon how many items are waiting |
| 21 | noisy | apple | 1 to 0 | 1 to 0 | coinan → koinon | koinon how many items are waiting |
| 21 | noisy | apple-keywords | 1 to 0 | 1 to 0 | coinan → koinon | koinon how many items are waiting |
| 33 | clean | apple | 4 to 0 | 2 to 0 | deep sea coin in 7 c → deepseek koinon 7 c | add deepseek koinon 7 c to the release room |
| 33 | clean | apple-keywords | 4 to 0 | 2 to 0 | deep sea coin in 7 c → deepseek koinon 7 c | add deepseek koinon 7 c to the release room |
| 33 | clean | parakeet | 1 to 0 | 1 to 0 | deepseek coinin 7 c → deepseek koinon 7 c | add deepseek koinon 7 c to the release room |
| 33 | clean | whistle | 5 to 3 | 3 to 2 | coin in → koinon | add deep sea koinon 7 sea to the release room |
| 33 | noisy | apple | 6 to 2 | 2 to 0 | deep sea coin and 7 c → deepseek koinon 7 c | and deepseek koinon 7 c into the release room |
| 33 | noisy | apple-keywords | 6 to 2 | 2 to 0 | deep sea coin and 7 c → deepseek koinon 7 c | and deepseek koinon 7 c into the release room |
| 33 | noisy | apple-dictation-keywords | 2 to 0 | 1 to 0 | deep sea koinon 7 c → deepseek koinon 7 c | add deepseek koinon 7 c to the release room |
| 33 | noisy | parakeet | 4 to 0 | 2 to 0 | deep sea coin in 7 c → deepseek koinon 7 c | add deepseek koinon 7 c to the release room |
| 33 | noisy | whistle | 8 to 5 | 3 to 0 | deep sea coin and → deepseek koinon 7 c | and deepseek koinon 7 c 7 sea which the release rambled |
| 35 | clean | apple | 2 to 0 | 2 to 0 | codex coin invoice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | clean | apple-keywords | 2 to 0 | 2 to 0 | codex coin invoice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | clean | apple-dictation-keywords | 3 to 0 | 3 to 0 | kodex coin invoice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | clean | parakeet | 2 to 0 | 2 to 0 | codex coin invoice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | clean | whistle | 2 to 0 | 2 to 0 | codex coin in voice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | noisy | apple | 2 to 0 | 2 to 0 | codex coin invoice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | noisy | apple-keywords | 2 to 0 | 2 to 0 | codex coin invoice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | noisy | apple-dictation-keywords | 5 to 3 | 4 to 2 | coin invoice → koinon voice | retired the session kodex koinon voice |
| 35 | noisy | parakeet | 2 to 0 | 2 to 0 | codex coin invoice af → codex koinon voice af | retire the session codex koinon voice af |
| 35 | noisy | whistle | 5 to 3 | 3 to 1 | coin and voice → koinon voice | we tire the session codex koinon voice am |
| 52 | clean | apple-dictation-keywords | 2 to 1 | 1 to 0 | rwci → rwcii | read the summary for rwcii koinon issue 466 |
| 52 | clean | parakeet | 1 to 0 | 1 to 0 | coinin → koinon | read the summary for rwcii slash koinon issue 466 |
| 52 | clean | whistle | 2 to 1 | 2 to 1 | coinin → koinon | read the summary for rgbcis slash koinon issue 466 |
| 52 | noisy | apple | 1 to 0 | 1 to 0 | coinin → koinon | read the summary for rwcii slash koinon issue 466 |
| 52 | noisy | apple-keywords | 1 to 0 | 1 to 0 | coinin → koinon | read the summary for rwcii slash koinon issue 466 |
| 52 | noisy | parakeet | 1 to 0 | 1 to 0 | coinin → koinon | read the summary for rwcii slash koinon issue 466 |
| 52 | noisy | whistle | 2 to 0 | 2 to 0 | rwci → rwcii; coinin → koinon | read the summary for rwcii slash koinon issue 466 |
| 52 | noisy | whistle-keywords | 1 to 0 | 1 to 0 | rwci → rwcii | read the summary for rwcii slash koinon issue 466 |
| 53 | clean | parakeet | 2 to 0 | 1 to 0 | coin in → koinon | switch to the koinon repository |
| 53 | noisy | parakeet | 2 to 0 | 1 to 0 | coin in → koinon | switch to the koinon repository |
| 53 | noisy | whistle | 2 to 0 | 1 to 0 | coin in → koinon | switch to the koinon repository |
| 57 | clean | apple | 1 to 0 | 1 to 0 | coinan → koinon | koinon im away for an hour |
| 57 | clean | apple-keywords | 1 to 0 | 1 to 0 | coinan → koinon | koinon im away for an hour |
| 57 | clean | parakeet | 1 to 0 | 1 to 0 | coynan → koinon | koinon im away for an hour |
| 57 | clean | whistle | 2 to 1 | 1 to 0 | coinin → koinon | koinon am away for an hour |
| 57 | noisy | parakeet | 1 to 0 | 1 to 0 | coinin → koinon | koinon im away for an hour |
| 60 | clean | parakeet | 1 to 0 | 1 to 0 | quinin → koinon | koinon goodnight |
| 60 | noisy | apple | 1 to 0 | 1 to 0 | coinan → koinon | koinon goodnight |
| 60 | noisy | apple-keywords | 1 to 0 | 1 to 0 | coinan → koinon | koinon goodnight |
