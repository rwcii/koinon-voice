# Corpus phrases

Chunk 04's phrase list (definition of done, live check 2). The maintainer reads every phrase
once in a quiet room (`clean`) and once with background noise (`noisy`), with
`koinon-voice corpus record`. The recordings stay under `~/.local/share/koinon-voice/corpus/`
and never enter Git (definition of done, Private data); this list is the reference text the
recognition engines are scored against.

Words in brackets are Koinon terms: peer names, repositories, Koinon's own words, numbers and
the decision verbs. Word error rate is reported overall and on these terms. The brackets are
not read aloud.

How to read them:

- A peer name is read as its parts, with letters spelled out: `codex-koinon-voice-af` is
  "codex koinon voice A F", and `claude-koinon-voice-b9` is "claude koinon voice B nine".
- A number is read the ordinary way: 38 is "thirty-eight", 412 is "four twelve" or "four
  hundred twelve".
- Each phrase is read once, at a normal pace, as if speaking to the bridge.

Scoring compares words after the same normalization on both sides: lower case, no
punctuation, hyphens as spaces, number words as digits, a letter-digit run split ("b9" is
"b 9") and spelled letters joined ("a f" is "af").

<!-- The phrases below are read by koinon-voice corpus and bench stt: one per line, as
"NN. text". -->

01. [Koinon], what's waiting?
02. [Koinon], read my inbox.
03. [Approve] it.
04. [Reject] it.
05. [Confirm].
06. Cancel that.
07. Read it again.
08. Skip this one.
09. [Approve] pull request [twelve].
10. [Reject] pull request [thirty-eight] and ask for a smaller change.
11. [Merge] pull request [five] on [koinon-voice].
12. Open issue [one] on [koinon-voice].
13. What did [codex-koinon-voice-af] ask?
14. Reply to [claude-koinon-voice-b9]: looks good.
15. Tell [deepseek-koinon-7c] to wait until Friday.
16. Send [codex-koinon-voice-38] a message: start chunk [four].
17. Who is online?
18. Which peers are waiting on me?
19. Is [codex-koinon-voice-af] still running?
20. What is [claude-koinon-voice-b9] working on?
21. [Koinon], how many items are waiting?
22. Read the oldest item first.
23. Summarize the decision on issue [four sixty-four].
24. What does [codex] recommend?
25. What are the options?
26. Is there a deadline?
27. [Approve] the first option.
28. Choose option [two].
29. Hold this until tomorrow morning.
30. Remind me in [thirty] minutes.
31. Post to the [phase zero] room: the spike is done.
32. What's new in the [voice] room?
33. Add [deepseek-koinon-7c] to the [release] room.
34. Create a room called [bench].
35. Retire the session [codex-koinon-voice-af].
36. Release the work claim on issue [one].
37. Who holds the claim on the speech bench?
38. Show me the failing checks.
39. Did the [macOS] tests pass?
40. Rerun the [Linux] job.
41. [Approve] the [peer review] and the [refusal review].
42. Request changes: the deadline test is missing.
43. [Merge] when the checks pass.
44. Don't [merge] yet.
45. [Confirm], [merge] pull request [five].
46. No, I said [reject].
47. Stop.
48. Go on.
49. Louder, please.
50. Slow down.
51. What time is it in [UTC]?
52. Read the summary for [rwcii] slash [koinon] issue [four sixty-six].
53. Switch to the [koinon] repository.
54. Is [oMLX] running?
55. Use the [Zoe] voice.
56. Ask [Opus] to summarize it instead.
57. [Koinon], I'm away for an hour.
58. Pause notifications until [three] o'clock.
59. Thanks, that's all.
60. [Koinon], goodnight.
