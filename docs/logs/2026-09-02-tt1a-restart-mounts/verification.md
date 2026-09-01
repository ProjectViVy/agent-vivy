# Verification — TT-1a

| Check | Command | Result |
|---|---|---|
| gofmt | `gofmt -l internal/runtime` | clean |
| Build | `go build ./...` | ok |
| Vet | `go vet ./internal/runtime/...` | ok |
| Targeted tests | `go test ./internal/runtime/ -run 'TestServiceRecover…\|TestServiceResume…\|TestServiceJournal…\|TestServiceQuestionRecovery\|TestServiceRecover' -count=1` | ok |
| Full package | `go test ./internal/runtime/ -count=1` | ok — 68.2s |
| Kernel gate | `just ci` | background run, tail-checked below |

## Discrimination check

`TestServiceRecoverRestoresSkillMountedTools` was run with the
rebuildPendingQuestion mount seeding disabled (one-line revert): the run
never reaches completed — the hidden `echo_info` call is rejected because
no mount admits it. With the fix restored the run completes and the
post-restart `echo_info` tool.finished succeeds. The test is not
passing-by-accident.

## Test design notes

- The restarted engine declares `echo_info` as a hidden tool too, so even
  a wrongly-recovered selection could not admit the call — only the
  restored mount can.
- First run of the test failed at my own `waitForApprovalEvent` wait (an
  ask_user flow journals `user.question_required`, not
  `tool.approval_required`); fixed by following the established
  question-recovery pattern (`waitForPendingQuestion`, mount events
  precede the question suspend in journal order).
- One compile fix along the way: the journal iterator yields
  `domain.RunEvent` — the mount names live in the `tool.mounted` JSON
  payload (`payloadToolMounted.Tools`), not on the event struct.
