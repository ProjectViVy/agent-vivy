# SDD ledger — plan: docs/plans/toolkits/index.md

## Scope and starting point
- Branch `TOOLKITS`, clean at `f7b032ad`.
- Execute remaining acceptance work from TK-04–TK-13 and reconcile already-proven TK-01–03 / TK-10–11 evidence.
- Preserve the user's explicit no-Windows-smoke constraint. Linux Docker is available; `just`, `pwsh`, and `powershell` are not installed on the host.
- No upstream architecture gap is currently established. Eino v0.9.13 and EinoExt MCP v0.0.9 provide the required MCP discovery/schema adapter; timeout coordination is Vivy-owned.

## Execution order
1. MCP cold discovery budget and live Toolkit-captured consent/provenance run.
2. TK-04/05 fault recovery, admissions, drain, resume, revocation, and fixture-owned MemoryLoop stall diagnosis.
3. TK-06–09 owner integration gaps and settings/publication races.
4. TK-10–12 UI action/error/recovery/viewport acceptance gaps.
5. TK-13 matrix reconciliation, Linux-only repo gate, artifact review, logs/status/TODO/defer reconciliation.

## Task 1: MCP discovery deadline chain
- RED: add failing tests proving per-world discovery timeout and separate MCP discovery timeout.
- GREEN: 30-second MCP-only discovery across ToolHost and MCPHost; other worlds and calls retain existing timeout budgets.
- Verify focused tests and warm-cache official stdio smoke; no cache clearing or external network dependency in unit tests.

Task 1: complete (commit 1ceeacbc, tests: focused ToolHost/MCPHost package tests; Runtime MCP+Toolkit selection; App Toolkit/MCP composition tests; all passed. Live isolated cold-cache server-everything discovery/call: passed in 10.89s. git diff --check passed.)
Ruling: the prior MCP status test required resolved EnvFrom/AuthEnv values to remain in the displayed error. Changed that contract to redact configured secret values while preserving bounded path diagnostics, because AGENTS.md and the Toolkit architecture prohibit credential values in logs/status. Cost if wrong: some diagnostic output loses the exact credential text, but the secret value is not recoverable from the error.

## Task 2: first captured Toolkit MCP execution
- RED: both the focused manager test and the App Service test initially exposed that an installed-disabled MCP candidate was absent from the active View, so consent lookup rejected its exact declaration.
- GREEN: consent resolves the exact installed immutable artifact, checks capability/release/declaration identity, and commits against both epoch and config revision. The real Service path proves the allowed consent reaches the MCP owner, captured run snapshot, approval and Journal.
- Verification: `internal/bundles` passed; `internal/storage/...` passed against SQLite and a disposable local PostgreSQL 16 container after the config-revision CAS; focused App Service test passed; `git diff --check` passed.
- Commit: `c294f52c fix(toolkits): consent installed MCP declarations`.

## Task 3: MemoryLoop suite timing / crash fixture
- C05's intentional real-lease wait is 29.88 seconds after process crash; isolated `TestMemoryLoopCrashC05EffectsDoneBeforeWatermark` passed in 57.886 seconds.
- Isolated `TestMemoryLoopAutomaticReflection` passed in 10.19 seconds.
- Full `go test ./internal/app -count=1 -timeout=10m` failed at the 10-minute timeout during `TestMemoryLoopAutomaticReflection`; the stack showed its cognition-status RPC and roughly 724k goroutines. This is cross-test/app-suite state outside Toolkit code, not a Toolkit failure or upstream architecture gap. No Windows command or smoke test was run.
- Next: rerun the focused Runtime MCP/Toolkit and App Toolkit selections, record the gate limitation, then continue storage/lifecycle/admission/UX acceptance triage. Do not add the unrelated suite issue to DEFER.MD.
