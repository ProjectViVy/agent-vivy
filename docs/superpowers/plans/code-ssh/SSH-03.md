# SSH-03 worker process and lifecycle implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Execute bounded remote jobs with observable output, receipts and confirmed-or-unknown cleanup.
**Architecture:** One stdio worker and connection-owned process manager; no agent loop, listener, service daemon or independent approval system.
**Tech Stack:** Go/Linux process groups, pinned OS primitives, fake clock and subprocess fault tests.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §5-6; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E1 / R3-R5. State and predecessors: [index](index.md).

## Global constraints and review focus

Commands are subject to an approved worker profile and existing Host policy,
never wire-granted authority. Reject required OS isolation when unavailable.
Own process groups and verify process birth before signaling. Closing SSH is
not proof of cancellation; daemonized descendants may remain unknown. No jobs-
survive-disconnect promise. Review lease/cancel availability during long commands.

## Prerequisites and contracts

Consume SSH-02 workspace/receipts/dispatcher and A03, transitively SSH-C1 wire.
Produce process.Manager/Profile/Clock/CloseResult and executable stdio entry;
closed process.execute/jobs.*/lease.renew/close. One connection owns one binding.
Manager retains exact operation linkage and updates the original receipt when
job terminal state is confirmed; cancel has its own Host-issued operation ID.

## Task 1: Processes, output cursors and cancellation

**Files:** NEW proposed `extensions/ssh-worker/process/{manager,jobs,output,signal_linux}.go`,
`process/{manager,output,signal_linux}_test.go`; UPDATE worker dispatcher.

- [ ] Write `TestCommandExitAndStreams`, `TestForegroundReturnsQueryableJob`,
  `TestCommandCwdRaceCannotEscape`,
  `TestOutputBoundAndCursorGap`, `TestCancelConfirmsOwnedProcess`,
  `TestRecycledPIDNotSignaled`, `TestIsolationRequiredRefused`,
  `TestDuplicateCommandDoesNotRunTwice`. Use harmless fixture subprocesses,
  separate stdout/stderr, nonzero exit, flooding output and a detached child.
- [ ] Run `go test ./process ./worker -count=1` for red. Persist intent/start,
  spawn executable/argv with explicit cwd/env/profile and a process group;
  use Workspace.OpenCwd's owned directory handle and verify a concurrent cwd
  symlink replacement cannot redirect launch. Keep the handle live through Start;
  Linux launch may resolve the trusted worker proc-fd reference, never a newly
  joined project path. Include this race in the process fixture; unavailable
  secure cwd resolution refuses execution.
  never interpolate arbitrary wire data into a shell. Record job ID and birth
  identity. Return started job handle promptly; foreground waiting happens via
  client polling. No long request blocks renew/cancel/receipt queries.
- [ ] Drain both streams continuously into bounded owned output storage with
  stream-specific cursors. Report output gap/limit rather than silently claiming
  full output. Output storage is outside the project; quotas stop growth.
- [ ] Cancel verifies ownership/birth, sends TERM then bounded KILL escalation,
  waits/drains and persists outcome. Return cancelled only when confirmed;
  stale PID, failed signal/wait or uncertain descendant cleanup returns unknown.
  The worker's fixed profile defines timeout/cleanup bounds; test them with
  injected timers instead of long sleeps. No automatic command resubmission.
- [ ] Run `go test -race ./process ./worker ./receipts -count=1` and `go vet ./...`.
  Commit as `feat: add bounded SSH worker jobs and cancellation`.

## Task 2: Stdio executable, handshake and owned lease

**Files:** NEW proposed `cmd/vivy-ssh-worker/main.go`,
`worker/{server,lease}.go`, `worker/{server,lease}_test.go` and subprocess fixtures.

- [ ] Write `TestHandshakeFreezesBinding`, `TestLeaseExpiryStopsAdmission`,
  `TestEOFCleansOwnedJobs`, `TestCloseIdempotent`, `TestCrashRecoveryUnknown`.
  Use a fake Clock for 10s/30s renew/expiry defaults. Inject EOF, malformed frame,
  startup state failure, mismatched worker digest and worker termination.
- [ ] Run `go test ./worker ./process -count=1` for red. Implement explicit
  operator root/state/profile arguments, build digest, strict hello, actual
  methods/limits/isolation capability and binding equality on every request.
  Serve one framed request at a time while process/output/lease work remains
  independently cancellable. Diagnostics use bounded stderr, never stdout.
- [ ] EOF/lease expiry/close stops admission, invokes Manager.Close, persists
  confirmed/unknown jobs and releases root/state handles once. A killed worker
  must recover uncertain receipts on the next explicit start, never replay them.
- [ ] Run `go test -race ./... -count=1`, `go vet ./...`,
  `go build ./cmd/vivy-ssh-worker` in the worker module. Cross-compile with
  `GOOS=linux GOARCH=amd64 go build -o <fresh-output> ./cmd/vivy-ssh-worker` and
  arm64 equivalent. Cross-build success is not execution evidence for that arch.
  Commit as `feat: serve SSH worker stdio with owned lease cleanup`.

## Acceptance and handoff

Return A04, exact startup/profile contract, actual methods/capabilities, Linux
job/fault logs and build artifacts/digests for SSH-04/05. No installer or remote
machine is modified. Stop on unconfirmed containment rather than hiding it.
Rollback closes owned workers and fixture descendants; keep uncertain operator
job evidence and receipts for reconciliation.
