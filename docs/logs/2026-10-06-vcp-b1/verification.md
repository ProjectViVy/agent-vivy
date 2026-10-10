# B1 verification

## Focused

```text
$ go test ./internal/runtime/ -run 'TestSteer|TestFollowUp|TestQueue' -v
TestSteerInjectsAtTurnBoundary            PASS
TestSteerOnIdleSessionIsUnavailable       PASS
TestFollowUpAdmitsAfterSettle             PASS
TestFollowUpOneAtATimeKeepsTail           PASS
TestQueueRebuildsFromNewestRunJournal     PASS
TestSteerDuringSuspendedRunDemotesToFollowUp  PASS

$ go test ./internal/app/ -run TestCodeFaceRPCMode -v
TestCodeFaceRPCModeGoldenTranscript              PASS
TestCodeFaceRPCModeQueuesFollowUpWhileRunning    PASS

$ go test ./sdk/codeclient/ -run TestClientAgainstRealVivyCode -v
PASS (steer on settled session → disposition started)
```

## Suite

```text
$ go test ./internal/...        all ok  (runtime 47.7s, rpc 17.7s, app 9.8s)
$ go test ./sdk/...             all ok
$ go build ./...                ok
$ gofmt -l internal/ sdk/       clean
```

Conformance re-pinned: `internal/` source digest
`9e024d18…` → `34c60318…` in
`sdk/internal/assembly/conformance_results.json` (5 sites);
`TestCheckedInProviderConformanceMatchesExecutedSuites` PASS (70.9s).

Go-host pack tests initially failed: generation staging uses
`git ls-files`, so untracked `queue.go`/`turn_queue.go`/`queue_test.go`
were missing from the staged tree → undefined symbols. Resolved by
staging the new files; `TestPackGoHost*` + `TestInspectGoHost*` PASS.

## Debug trace (manual)

Steer ordering on a gated run:
`turn.queued` → `tool.requested` → `turn.steered` → tool replay →
`model.delta` → `run.completed`. Verified end-to-end.
