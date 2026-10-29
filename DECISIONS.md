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
