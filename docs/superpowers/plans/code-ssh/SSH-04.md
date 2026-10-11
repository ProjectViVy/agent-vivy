# SSH-04 governed plugin transport implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Connect only the approved SSH binding and expose receipt-first reconnect without credential leakage.
**Architecture:** Optional public T2 plugin owns one OpenSSH stdio connection through the proven Host facade; wire remains data-only.
**Tech Stack:** Go 1.26.4, installed OpenSSH, public ToolWorld/Module contracts.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §4/7; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E2 / R1, R4-R5, R8. State and predecessors: [index](index.md).

## Global constraints and review focus

BLOCKED: H1/H2/H3/H5/H6 must provide the accepted producer/consumer signatures.
An approved exception needs implemented evidence, not only a decision. Do not
start production transport before this gate. No net.Dial, exec.Command,
x/crypto client, arbitrary SSH config, credential bytes or automatic install.
Review argv injection, inherited configuration widening and once-only Close.

## Prerequisites and contracts

Consume SSH-00's proved authority/profile/owner path and SSH-01 wire/A02.
Existing Spawn signature is in SSH-C1; current Spawn alone does not prove egress
authority. Produce SSHTransport implementing wire.Transport and Manager's
Invoke/Snapshot/Close. ApprovedTarget is server-validated immutable data. The
exact accepted Host scope/profile and shared-owner factory are inserted here
from SSH-00 before Ready; no substitute UI/model identity is allowed.

## Task 1: Plugin source boundary and fixed OpenSSH invocation

**Files:** NEW proposed `plugins/coding/ssh-workspace/{go.mod,vivy-module.yaml,module.go,descriptor.go,transport.go,transport_test.go}`.
Descriptor may advertise only the implemented supported contribution subset;
SSH-05/06/07 extend it after their evidence. No selectable success stub.

- [ ] Read plugin standards and scheduled Port phase. Establish Module ID,
  source pin, T2 classification, lifecycle owner, effective Grants and seven
  artifact record. Add `TestTransportRequiresApprovedTarget`,
  `TestSSHArgvCannotWidenProfile`, `TestDeniedTargetDoesNotSpawn`,
  `TestConstructAndStatusHaveNoEffects`, `TestTransportRejectsWorkerMismatch`.
  Fake Proc records actual argv/stdin; denied cases assert zero Spawn calls.
- [ ] In plugin module run `go test ./... -run 'Test(Transport|SSH|Denied|Construct)' -count=1`
  for red. Implement pure descriptor/Construct and exact fixed argv. Use approved
  host/user/port/auth/known-hosts only, batch mode, strict keys, no TTY, forwarding,
  local command, proxy or inherited unapproved config. Validate/quote each fixed
  remote-command atom, including paths with spaces; reject control characters.
  OpenSSH remote command is shell-parsed, so argv separation alone is not proof
  of remote quoting. Unknown config/options never enter this API.
- [ ] Call only accepted Host-governed Spawn after profile/egress/approval checks.
  Handshake verifies root/binding/protocol/build digest and effective limits.
  Missing ssh/worker/key/profile is unavailable with a safe actionable message;
  never falls back to local files. Auth bytes never enter wire/log/results.
- [ ] Run plugin `go test ./... -count=1`, `go vet ./...` and from root
  `go run ./sdk verify plugins/coding/ssh-workspace`. Verify imports contain no
  `agent-vivy/internal`, Eino or worker execution packages; record source-pin
  resolution. Commit as `feat: add governed SSH workspace transport`.

## Task 2: Serial requests, interruption and reconnect

**Files:** NEW proposed `manager.go`, `manager_test.go` and Story evidence log;
UPDATE transport implementation/tests only.

- [ ] Add `TestReplyIdentityMismatchUnavailable`, `TestDisconnectAfterSendUnknown`,
  `TestReconnectQueriesReceiptBeforeMutation`, `TestCloseOnceAndDrainsStderr`,
  `TestRenewCanRunDuringForegroundWait`, `TestStaleBindingCannotReconnect`.
  Model fake write/read breaks before send, after send and after remote settlement;
  count mutation frames. Two intentional operation IDs still send two commands.
- [ ] Run `go test ./... -run 'Test(Reply|Disconnect|Reconnect|Close|Renew|Stale)' -count=1`
  for red. Serialize RoundTrip framing, bound stderr, validate replies and retain
  pending identities. Distinguish no-send failure from ambiguous send; ambiguity
  returns unknown/query_receipt and never automatic retry. Fresh read is an
  explicit new request, not a hidden mutation retry.
- [ ] Reconnect only with the same accepted binding/profile/build; query each
  uncertain operation first. Do not resubmit even when a receipt is missing.
  Close stops admission, attempts worker close, settles confirmed evidence via
  the accepted authority and reaps local Proc once; uncertainty remains visible.
- [ ] Run `go test -race ./... -count=1`, `go vet ./...`, SDK verify and a real
  explicit test-target verified-key login through the Host path. Worker SSH-03
  is not a graph prerequisite for this transport Story: use wire fixtures here;
  complete real worker acceptance belongs to SSH-05. Record A05 transport slice.
  Commit as `feat: reconcile SSH transport interruption without replay`.

## Acceptance and stop conditions

Return exact accepted Host/factory signatures, argv/auth profile evidence,
transport interruption results and no-internal import evidence. Stop on any
unsupported Grant/facade, constructor duplication or unresolved source pin;
no raw capability fallback. Rollback closes owned connections and removes only
new plugin sources, without changing default Recipe or operator SSH files.
