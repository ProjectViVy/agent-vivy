# Verification

Environment: Linux, Go from `/workspace/agent-vivy/work/bin`, PowerShell Core behind the cross-platform `powershell.exe` command, `GOMAXPROCS=2`, `GOFLAGS=-p=2`. Embedded UI assets copied into this isolated worktree from the integration checkout solely for Go compilation.

RED: `go test -p 2 ./internal/app -run '^TestMemoryLoopMemoryInjection$' -count=1` failed after 11.07 seconds with native diagnostic search succeeding but actual model query trace empty (`red.log`).

Default GREEN: `go test -p 2 ./internal/app -run '^(TestDefaultGenerationOmitsNativeRecall|TestMemoryLoopMemoryInjection|TestMemoryLoopRecallAfterProcessRestart|TestMemoryLoopRecallNegativeControls|TestMemoryLoopOrdinaryRecallAuthorityBoundary|TestMemoryLoopRecallDeadlineDegradesSafely|TestMemoryLoopCorrectionAndDeletionInModelInput)$' -count=1 -v` passed (`green-default.log`). The default absence regression passed and six recall-only groups correctly skipped. Those skips are composition selection, not native recall acceptance.

Actual DIVA gate: `powershell.exe -NoProfile -File scripts/test-diva-recall.ps1` executes the six groups under the official SDK-generated DIVA assembly with no skip permitted; all six groups passed, zero skips (`green-diva.log`).

`go test -p 2 ./sdk/internal/cmd/generate-default -count=1` compiled the generator. `just --show ci` and `just --show backend-ci` confirmed the required gate in both dependency lists. Implicit versus explicit default recipe generation produced byte-identical assembly. PowerShell syntax parsing and BOM-free JSON serialization checks passed. Go formatting and `git diff --check` passed.

No temporary conformance/source-hash refresh was needed. Full CI, final source evidence and artifact checks are owned by the integration lane, which had already observed all packages except these six wrongly selected App groups passing. This lane does not claim full CI or live-model evidence.
