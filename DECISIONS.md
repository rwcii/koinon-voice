# Decisions

The maintainer's verdicts on the Koinon decision board, appended by Koinon's `decision_record`
tool. Each entry holds the card text that the maintainer saw when giving the verdict, and the
maintainer's note or feedback, with spelling and typing errors corrected. Entries are only
appended, never changed. Cite an entry as `DECISIONS.md#REFERENCE`; a link's target
is the reference in lower case.

<a id="kv-10-scoring"></a>

## kv-10-scoring

<!-- koinon-decision kv-10-scoring revision 1 approved 1793252248561 -->

**Approved**, revision 1, 2026-10-29 05:37:28 UTC.

- Title: Koinon Voice #10: score the retrained wake model on your 60 Phase 0 takes, or record new scoring takes?
- Where: rwcii/koinon-voice#10

Context:

> kv-10 adds a recording session for "Koinon" takes. Training takes and scoring takes must be different takes.
>
> Your 60 Phase 0 takes (20 at each distance, in `corpus/wake/near`, `mid` and `far`) were used only for scoring. They were not used to train any model.
>
> Two options:
> 1. **Use the Phase 0 takes for scoring.** The new session records only training takes, in new sets `corpus/wake-train/near`, `mid` and `far`. The retrained model gets scored on the same 60 takes and the same background audio as v4, so the two results compare directly.
> 2. **Record new scoring takes as well.** The session records training takes and scoring takes, and keeps them in separate sets. You record more takes, and the new score does not compare directly with v4's 29 of 60.

Recommendation:

> Option 1. You record only training takes (default 50 at each distance), and the new score compares directly with v4. If you want new scoring takes later, `koinon-voice corpus wake --count N` already records more of them.

Note:

> 1 for now. I can't provide new recordings right now. We can revisit this tomorrow.

<a id="kv-12-gate-a"></a>

## kv-12-gate-a

<!-- koinon-decision kv-12-gate-a revision 1 approved 1793274435414 -->

**Approved**, revision 1, 2026-10-29 11:47:15 UTC.

- Title: Koinon Voice: approve the decision of the Kokoro speech sprint (#12, Gate A)?
- Where: rwcii/koinon-voice, branch feature/kokoro-speech, commit 533b3dc, docs/sprints/2026-10-29-kokoro-speech/decision.md

Context:

> Issue #12: Koinon Voice speaks with Kokoro. You decided on 2026-10-26 that Kokoro at speed 1.1 is the voice (ADR 002, Decision 2), that all speech plays through the Swift helper with voice processing (Decision 3), and "B - NO Python" (Decision 4). Kokoro ran only in Phase 0's Python bench helper. You rated it 4.7 against Zoe at 3.2.
>
> The sprint decision has the problem, 7 acceptance criteria and the constraints. It has no design. Gate A is your approval of this content. After it, I write the definition of done and the chunks, Codex reviews them, and you approve the plan at Gate B.
>
> **Acceptance criteria, in short:**
>
> 1. Kokoro-82M runs natively on your Mac in a helper process. No Python at run time or at installation, text to phonemes included. The model stays outside the repository, with its source, revision and SHA-256 recorded.
> 2. Voice `af_heart` (the voice you rated) and speed 1.1 by default. Both can be changed in the engines file.
> 3. The audio plays through the Swift helper's voice-processing engine.
> 4. Speech plays sentence by sentence, and a stop request ends it.
> 5. `summarize --speak` and `helper --say` use Kokoro by default. Apple's voice stays as a choice. A missing model gives a message that says how to install it.
> 6. `bench tts` measures the native engine on Phase 0's six texts against Phase 0's Kokoro (787 ms to first audio). You rate it blind.
> 7. The runtime and the text-to-phoneme step must also be available on iOS and iPadOS, as their publishers document. This sprint builds for macOS only.
>
> **Scope:** this sprint does not include openWakeWord in a native runtime, the voice loop or an iOS app.
>
> **Constraint for you to confirm:** "No Python" means none at run time or at installation. Where the publisher gives a converted model (ONNX or Core ML), the sprint uses it.

Recommendation:

> Approve. The sprint is #12 only, and openWakeWord follows #10. To add openWakeWord's native runtime to this sprint, or to change the meaning of "No Python", decline and write it in the note.

Commands:

    git -C ~/dev/koinon-voice.worktrees/kokoro-speech show 533b3dc:docs/sprints/2026-10-29-kokoro-speech/decision.md

<a id="kv-12-voices"></a>

## kv-12-voices

<!-- koinon-decision kv-12-voices revision 1 feedback 1793276528395 -->

**Feedback**, revision 1, 2026-10-29 12:22:08 UTC.

- Title: Koinon Voice: add a blind rating of more Kokoro voices to the Kokoro sprint (#12)?
- Where: rwcii/koinon-voice, docs/sprints/2026-10-29-kokoro-speech/decision.md, criterion 6

Context:

> You asked in the terminal on 2026-10-29: "are there other voices for kokoro?"
>
> Kokoro v1.0 has 54 voices: 20 American English and 8 British English, and others in seven more languages. Its voice list (hexgrad/Kokoro-82M, VOICES.md) grades each voice on the quality of its training data. The English voices with the best grades are:
>
> - `af_heart` (American, female): target quality A. You rated it 4.7 in Phase 0.
> - `af_bella` (American, female): A-
> - `af_nicole` (American, female): B-
> - `bf_emma` (British, female): B-
> - `am_fenrir`, `am_michael`, `am_puck` (American, male): C+, the best male voices
>
> Most of the other English voices have the grade D. A voice is one voice file that the model reads, so the engines file can select any voice (criterion 2). The approved decision (Gate A) keeps `af_heart` as the default, and criterion 6 rates only that voice.
>
> **The change:** in the measurement step (criterion 6), you also rate `af_bella`, `af_nicole`, `bf_emma`, `am_michael` and `am_puck` blind at speed 1.1, on the same texts. The voice that you rate highest becomes the default.

Recommendation:

> Approve. It adds one listening session with no new code. To rate different voices, write their names in the note. Decline to keep `af_heart` with no comparison.

Feedback:

> Not at this time. Let's focus on getting this out and working with one voice. Open a tracking issue for this, but this is a post-GA release task.

<a id="kv-12-gate-b"></a>

## kv-12-gate-b

<!-- koinon-decision kv-12-gate-b revision 1 approved 1793278320545 -->

**Approved**, revision 1, 2026-10-29 12:52:00 UTC.

- Title: Koinon Voice: approve the reviewed plan of the Kokoro speech sprint (#12, Gate B)?
- Where: rwcii/koinon-voice, branch feature/kokoro-speech, commit 875d6c1, docs/sprints/2026-10-29-kokoro-speech/

Context:

> You approved the sprint decision at Gate A (533b3dc). You kept one voice, `af_heart`. Other voices are #32, after GA. This is the reviewed plan: `definition-of-done.md`, `sprint.md` and two chunks. Gate B approves it before I build the product.
>
> **The engine:** kokoro-ios (MIT) on MLX Swift, with MisakiSwift (Apache-2.0) to turn text into phonemes. The reasons:
> - It uses the same model file that you rated in Phase 0, with the same SHA-256 (`4e9ecdf0…`).
> - MisakiSwift is the Swift port of Misaki, the step that Phase 0 used.
> - FluidAudio (Core ML) is the fallback only. Its documents list a Core ML crash on macOS 26.4 to 26.5.x, and your Mac has 26.5.
>
> **Chunk 01, the Kokoro helper:** a separate process, `koinon-voice-kokoro`. A crash in the model therefore does not stop recognition or playback.
> - `koinon-voice kokoro install` downloads the model files and checks each file's SHA-256.
> - Go receives audio frames from a helper.
> - `bench tts` measures the native engine against Phase 0's Kokoro (787 ms to first audio).
> - The chunk starts with feasibility steps: build, voice file, output against your Phase 0 recording, and the time to stop. If a step fails, I stop and correct the plan.
>
> **Chunk 02, speak with Kokoro:**
> - The Apple helper plays Kokoro's audio with voice processing.
> - Each sentence plays while the next one is synthesized.
> - Ctrl-C stops speech. During synthesis, it ends the Kokoro process, because kokoro-ios cannot cancel a sentence once synthesis starts.
> - Kokoro becomes the default voice of `summarize --speak` and `helper --say`. Apple's voice stays as a choice.
>
> **Your part:**
> 1. Install Xcode (card kv-12-xcode). MLX can be built only with `xcodebuild`.
> 2. In chunk 01: listen to text 01 and the names "Koinon", "Codex" and "Claude".
> 3. In chunk 02: confirm that playback and stopping work, and rate the voice blind beside your Phase 0 recordings.
>
> **Review:** Codex review 1 found three problems:
> - No real mechanism to stop speech.
> - No audio transport between the processes.
> - An exclusion of GPL software with no source.
>
> All three are fixed. Review 2: peer-review and refusal-review both pass, with no findings (`review.md`).
>
> **Gate B does not approve:** openWakeWord in a native runtime, the voice loop, an iOS build, or other voices (#32).

Recommendation:

> Approve. Then install Xcode (kv-12-xcode). I start chunk 01 when `xcrun -f metal` finds the Metal compiler.

Commands:

    git -C ~/dev/koinon-voice.worktrees/kokoro-speech log --oneline 9a659bf..875d6c1

<a id="kv-12-listen"></a>

## kv-12-listen

<!-- koinon-decision kv-12-listen revision 1 feedback 1793297656981 -->

**Feedback**, revision 1, 2026-10-29 18:14:16 UTC.

- Title: ACTION FOR YOU: listen to native Kokoro and its names, then accept or correct the pronunciations (#12, chunk 01)
- Where: The maintainer's Mac, ~/.local/share/koinon-voice/kokoro-spike/listen/

Context:

> Chunk 01's feasibility steps passed. kokoro-ios builds with Xcode and runs Phase 0's model and voice natively, with no Python:
>
> - **Text 01 at speed 1.0:** 15.30 s, and 15.32 s in Phase 0.
> - **First sentence:** 829 ms in a new process. Phase 0's median time to first audio was 787 ms.
> - **Stop during synthesis:** the process ends in about 11 ms.
>
> Check 16 (names) is for you to hear. Misaki gives "Koinon" **two pronunciations**:
>
> - `Koinon` with a capital: `kOˈinˌɑn`, approximately "ko-EE-non" (texts 02 and 05).
> - `koinon` in lower case: `kˈYnˌɑn`, approximately "KOY-non" (session names such as "koinon voice b nine").
>
> Also, `b9` in `claude-koinon-voice-b9` becomes `bˈi`: the 9 is not spoken. "Codex" (`kˈOdˌɛks`) and "Claude" (`klˈɔd`) are always the same.
>
> **Listen** with the commands below:
>
> 1. Text 01 at 1.1.
> 2. "Koinon." with a capital.
> 3. "koinon." in lower case.
> 4. "Claude, the agent named claude-koinon-voice-b9, asks you to decide."
> 5. "Codex and Claude are waiting in Koinon."
>
> If you hear no sound, the cause can be the multichannel USB mixer (#8). In that case, use `open FILE` to play the file in QuickTime.

Recommendation:

> In the note, write how "Koinon" must sound: "KOY-non", "ko-EE-non", or a different sound. Also write whether "Codex", "Claude" and the voice at 1.1 are acceptable. I will then add the pronunciation that you choose to the helper's lexicon as a fixed entry. I will also make the helper speak digits in session names. Approve if all names are acceptable as they are.

Commands:

    afplay ~/.local/share/koinon-voice/kokoro-spike/listen/01-native-1.1.wav
    afplay ~/.local/share/koinon-voice/kokoro-spike/listen/name-koinon-capital.wav
    afplay ~/.local/share/koinon-voice/kokoro-spike/listen/name-koinon-lower.wav
    afplay ~/.local/share/koinon-voice/kokoro-spike/listen/name-session-b9.wav
    afplay ~/.local/share/koinon-voice/kokoro-spike/listen/name-codex-claude.wav

Feedback:

> I'm not transcribing things for you. If I run a command, it should ask for my input and record it.

## kv-12-listen

<!-- koinon-decision kv-12-listen revision 2 completed 1793303752526 -->

**Completed**, revision 2, 2026-10-29 19:55:52 UTC.

- Title: ACTION FOR YOU: run one command that plays the native Kokoro clips and records your answers (#12, chunk 01)
- Where: The maintainer's Mac, ~/.local/share/koinon-voice/kokoro-spike/listen/

Context:

> You said: "I'm not transcribing things for you. If I run a command, it should ask for my input and record it." (DECISIONS.md#kv-12-listen)
>
> The command below now does that. It plays each clip and asks you a question. You type one letter. It writes your answers to `~/.local/share/koinon-voice/kokoro-spike/listen/answers.md`. I read that file.
>
> It asks 4 questions:
>
> 1. Text 01 at speed 1.1: is the voice acceptable? (y or n)
> 2. "Koinon": it plays clip A, then clip B. Which one is right? (a, b, or n for neither; for n, type how it must sound)
> 3. The session name `claude-koinon-voice-b9`: is it acceptable? (y or n)
> 4. "Codex" and "Claude": is each one acceptable? (y or n)
>
> After questions 1, 3 and 4, you can type a comment, or press Enter for none.
>
> At each question: `r` plays the clip again. `o` opens the clip in QuickTime, if you hear no sound from `afplay` (#8).

Recommendation:

> Run the command, then mark this task completed. I then add the "Koinon" pronunciation that you chose to the helper's lexicon, and make the helper speak the digits in session names.

Commands:

    ~/.local/share/koinon-voice/kokoro-spike/listen.sh

Note:

> Recorded in /Users/robert/.local/share/koinon-voice/kokoro-spike/listen/answers.md
