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
