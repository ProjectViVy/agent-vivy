# Verification

Base: `a12748a7114bb54fd0bdf663bfb1f01fcec90802`. Isolated branch:
`codex/d14-warming-accounting`. Own sibling Laputa is clean and detached at
`9acc999301d5d575da2cea902a1c6d213763f381`. Native Go is
`C:/Program Files/Go/bin/go.exe`; commands use `GOFLAGS=-p=2` and a checkout-local
`.workspace/go-tmp` for TEMP/TMP/GOTMPDIR. The existing external Go build cache
requires approved shell escalation; ordinary sandbox access was denied.

## RED evidence

- Original implementation paid after budget exhaustion, emitted no mandatory
  maintenance lifecycle, silently completed after mandatory append failures,
  and reported idle before a cancelled paid call drained.
- Native Generate discarded mandatory End failures; Stream returned EOF after
  an End failure. Replay let maintenance credit foreground completion.
- Strengthened request/usage/finish append-failure tests showed that foreground
  work remained blocked instead of failing promptly.
- The steering regression failed because the replacement leg inherited its
  predecessor's closed warmer.
- Concurrent trajectory regression falsely marked the foreground call
  interrupted when maintenance began.

All corresponding cases are GREEN in the final focused run.

## Final commands and outcomes

```powershell
go test -tags=vivy_headless ./internal/runtime -run 'TestTrajectory|TestSessionTrajectory|TestCacheWarm|TestObserver|TestObserved|TestUsageAccumulator|TestBudget|TestRecoveredLedger|TestModelCallPayloadSchemas|TestSteer|TestQueue|TestCancel|TestServiceApproval' -count=1 -timeout=6m -json
go vet -tags=vivy_headless ./internal/runtime
git diff --check
```

Final code: 88 tests/subtests PASS, zero failures/skips; package runtime 23.928s.
Vet PASS with no output. Diff whitespace check PASS. The selection includes
real Service/SQLite/checkpoint paths, three-mode warming and the new same-Run
steering and concurrent-trajectory regressions. Actual maintenance payloads
validate against all three schemas and project to source-attributed usage rows.

One intermediate selection hit the existing approval test's strict ordering
expectation: Eino began the next model request before the consumer projected
`tool.started`. The immediate isolated rerun and both later expanded selections
passed. The approval test and production ordering contract were not changed.
A redundant extra Go cancellation wrapper triggered vet's lostcancel check;
it was removed in favor of the original Run owner plus fresh native ADK handles.

## Unrun acceptance lanes

Per the integration owner's scope, no heavy/full gate was launched. Linux
`just wiring-ci`, `just kernel-ci`, race and affected platform/Face gates must be
run at the combined candidate. Native race was not run (no GCC available).
No live provider, browser, Postgres, Docker, packaged executable, full-product
`just ci`, release or human real-model acceptance is claimed. No production
state was accessed. Temporary raw test logs and the checkout-local Go temporary
directory were removed after recording the results; no scratch file is part of
the deliverable.
