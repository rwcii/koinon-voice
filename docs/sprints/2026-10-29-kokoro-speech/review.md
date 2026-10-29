# Sprint 2026-10-29 — Speak with Kokoro — Review

## Plan review 1

Codex, `peer-review` and `refusal-review` of the plan at
`3e3a1ae6b09f0dd525078691d58e32080c730c7e` against `develop` 9a659bf, 2026-10-29: CHANGES
REQUESTED for both. Codex reproduced macOS 26.5 and the Command Line Tools selection, read the
native package pins and MisakiSwift's bundled resources, and confirmed FluidAudio's
26.4 to 26.5 BNNS issue in its documents. It did not build or run the model or read private
audio. It checked the Gate A and voice-feedback entries against the board history.

| Finding | Severity | Disposition |
| --- | --- | --- |
| F1. Stopping is promised with no native mechanism or test. kokoro-ios's `generateAudio` (`KokoroTTS.swift` at 4d6d1d8, line 168) is synchronous and returns a whole sentence's samples, with no cancellation; no native check stops a real synthesis; no user stop is named. | high (peer) | Fixed. Confirmed the signature at 4d6d1d8. The definition of done now has "Stopping": a stop cancels `play` and ends the Kokoro helper's process group during synthesis, and the next text starts a new helper; pipelining is between sentences, not within one; Ctrl-C is the user's stop. Feasibility step 4 in chunk 01 measures the times stopping depends on; check 4 and the new native check 17a test it. |
| F2. The shared transport is an omitted dependency: `helpers/apple/Package.swift` exports no product, `internal/sidecar` discards audio frames and `Stream` has no audio API, and the Apple helper discards incoming audio. | medium (peer) | Fixed. Confirmed in `helper.go` and `Server.swift`. The definition of done now has "Audio frames in Go", with receiving in chunk 01 (outcome 3) and sending and the Apple helper's routing in chunk 02 (outcome 1), and check 10a for order, unknown, finished and cancelled requests, two requests, a blocked receiver and a helper exit. Chunk 01 exports `SidecarProtocol` as a library product. |
| R1. "Excluded: GPL" has no source in a decision or in Gate A. | medium (refusal) | Fixed. The row now says "Not chosen" with the technical reason: espeak-ng is a different text-to-phoneme step from the Misaki step that Phase 0 rated. |
| Note. `/usr/bin/xcodebuild` exists as a stub. | — | Fixed: the definition of done says the stub exists and no working Xcode toolchain is present. |

While fixing F1, the driver removed a time limit ("within 1 s") from check 17a that no
source gave; the check records the time instead.
