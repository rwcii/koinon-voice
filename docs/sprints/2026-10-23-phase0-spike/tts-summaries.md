# Synthesis summaries

Chunk 04's synthesis texts (definition of done, live check 3): Apple speech synthesis and
Kokoro speak each one, and the maintainer rates them blind with
`koinon-voice bench tts`. They are summaries of synthetic fixtures from chunk 03's
scorecard, chosen for their mix of names, numbers, times and lengths, each one correct on
the check. The engine that wrote each is in brackets and is not spoken.

<!-- The texts below are read by koinon-voice bench tts: one per line, as "NN. text". -->

01. Codex asks whether you want to merge pull request twelve, which adds the summarize command and passes checks on Linux and macOS. The options are to merge or not merge, and Codex recommends merging it. No deadline is specified. [SYN-01, OpenRouter Gemini]
02. Claude asks the maintainer to decide on a wake word engine for Phase 1. Options are openWakeWord, recognition plus phrase matching, or Porcupine. Claude recommends using openWakeWord with a custom Koinon model. Decision needed by Friday. [SYN-02, Qwen3-4B]
03. Claude, from koinon voice b nine, asks you to approve running the full race test suite before the release. It takes about twenty-five minutes and the machine will be slow while it runs, so you can approve now or hold off. Claude recommends approving so the release gets a full race run. [SYN-04, Claude Opus]
04. Codex asks you to decide whether to approve the dashboard login hotfix, which rereads the CSRF key to fix logins failing after a daemon restart. Codex recommends approval. It must merge by five p.m. UTC today to make the build for your afternoon demo. [SYN-06, OpenAI]
05. Claude, the agent named claude-koinon-voice-b9, asks you to decide whether to post a drafted comment on Koinon issue four seventy. The issue asks for a maintainer mode for the MCP server, and the draft supports storing a hashed secret in the daemon. Its recommendation is to post the comment as drafted. The item gives no deadline. [SYN-10, Claude Haiku]
06. Codex, the agent codex-koinon-voice-38, asks you to decide whether to raise the OpenRouter spending limit. The scorecard runs have used 36 of the 40 dollar monthly limit. If you keep it at 40 dollars, OpenRouter rows stop after about ten more items, while raising it to 80 dollars covers the remaining runs. Codex recommends raising it to 80 dollars, and no deadline is given. [SYN-17, Claude Sonnet]
