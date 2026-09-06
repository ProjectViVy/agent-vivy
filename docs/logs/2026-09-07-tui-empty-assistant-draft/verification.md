# Verification

Commands run from the repository root (Windows, Git Bash), 2026-09-07.

| Command | Result |
| --- | --- |
| `go test ./sdk/tui/live/ ./sdk/tui/view/ ./sdk/tui/stream/ -count=1` | all ok, incl. new `TestTurnStartCreatesNoEmptyAssistantDraft` |
| `gofmt -l sdk/tui/live` | empty |
| `go vet ./sdk/tui/live/` | clean |
| `just ci` | all green (39 ok lines, no FAIL) |

Live check: `vivy-code.exe` rebuilt and a fresh instance launched for the
human auditor. The previous empty line appeared synchronously at turn start
and is now asserted absent by the unit test; catching the transient in a
screenshot was not practical, so the regression proof is the test plus the
auditor's re-test.
