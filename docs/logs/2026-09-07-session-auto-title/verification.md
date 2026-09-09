# Verification

Commands run from the repository root (Windows, Git Bash), 2026-09-07.

| Command | Result |
| --- | --- |
| `go test ./internal/runtime/ -run TestAutoTitle -count=1 -v` | 5/5 PASS incl. new `TestAutoTitleFiresAfterFailedRun` |
| `go test ./internal/runtime/ ./sdk/tui/live/ ./sdk/tui/view/ -count=1` | all ok |
| `gofmt -l internal/runtime sdk/tui/live` | empty (after one struct-alignment fix) |
| `go vet ./internal/runtime/ ./sdk/tui/live/` | clean |
| `just ci` | all green (fmt-check, ui-ci, vet, test, headless-compile, plugin-ci) |

## Live acceptance (real product path)

Built `vivy-code.exe` (`just vivy-code`) and launched a fresh instance:

1. Boot: tab title bare `VIVY CODE`, rail shows the `untitled session`
   placeholder — sessions are now born untitled.
2. Sent `Introduce yourself in one sentence` (plain chat, no tools). The run completed.
3. Seconds after the run ended (kernel LLM titler + 4s-paced sidebar
   re-check): the rail header showed the generated title
   `Introduce yourself in one sentence` and the Windows Terminal tab title became
   `VIVY CODE · Introduce yourself in one sentence` — the F7 title sync consumed the same
   value with no further interaction.

Instance logs for the smoke instance contain no auto-title skip/failure
lines; the title arrived through the intended path.

Limitation: the failing-run titling path is covered by the runtime unit
test (empty scripted model); producing a real provider failure in a live
session was not practical here.
