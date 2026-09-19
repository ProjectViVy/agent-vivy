# Verification — approval resume integrity and timed auto-approval

All commands were run from the repository root on 2026-09-19 (Windows,
`pwsh`). Nothing here touches `data/vivy.db`, `data/demo/`, or
`data/workspaces/`.

## 1. The regression test reproduces the reported failure on the old code

The new `TestServiceResumedRunDispatchesToolsAfterApproval`
(`internal/runtime/approval_test.go`) scripts three turns: an effectful
`write_note` (approval interrupt), then a read-only `echo_info` call **after
the resume**, then a final answer. With the fix disabled
(`if false && (!isTarget || !hasData)`, a temporary edit that was reverted):

```text
WARN run failed err="engine event error: [NodeRunError] failed to stream tool
call call-echo-after-approval: runtime: approval for echo_info is stale:
approved tool checkpoint state is unavailable"
--- FAIL: TestServiceResumedRunDispatchesToolsAfterApproval (30.26s)
    run ... never reached status completed (last status failed)
```

That is the production message from `run_9b5ae7e437a91008` verbatim
(`tool.proposal_stale … "approved tool checkpoint state is unavailable"`),
reproduced deterministically at unit level. With the fix in place the same
test passes.

## 2. Focused Go tests

```text
go build ./internal/runtime/            # exit 0
go build ./...                          # exit 0
go test ./internal/runtime/ -run 'TestServiceResumedRunDispatchesToolsAfterApproval|TestToolAdapterTieredApprovalForClassifiedTools|TestServiceApprovalApproveFlow' -count=1
  ok  agent-vivy/internal/runtime  3.932s
go test ./internal/runtime/ -run 'Approval|Sweep|ShellApproval|ToolAdapter' -count=1
  ok  agent-vivy/internal/runtime  35.250s
go test ./internal/runtime/ -run 'TestServiceSweep|TestAutoApprovesOnTimeout' -count=1
  ok  agent-vivy/internal/runtime  1.849s
go test ./internal/app/ -run 'TestApplySettingsOverlay' -count=1
  ok  agent-vivy/internal/app  0.747s
go test ./internal/app/settings/ -run 'TestSaveAndLoadApprovalTimeout|TestSaveAndLoadSandboxPreset' -count=1
  ok  agent-vivy/internal/app/settings  0.192s
go test ./internal/rpc/ -run 'TestSettingsApprovalTimeoutOverlay' -count=1
  ok  agent-vivy/internal/rpc  1.045s
```

New cases and what they pin:

- `TestServiceResumedRunDispatchesToolsAfterApproval` — an unrelated call after
  an approval executes, no `tool.proposal_stale`, exactly one terminal event,
  and the approval row stays `approved`.
- `TestToolAdapterDenyTableRefusalEmitsPolicyEvent` — a deny-table hit returns
  a refusal result with no error, the tool is never invoked, and a
  `policy.evaluated` deny event is recorded. `TestToolAdapterTieredApprovalForClassifiedTools`
  was updated from "deny table ⇒ `ErrPolicyDenied`" to the refusal contract.
- `TestServiceSweepAutoApprovesSmartApprovalOnTimeout` — a smart-preset
  approval past its deadline is approved by the sweep with actor `system` and
  an `auto-approved …` reason, the tool executes, and the run completes.
- `TestServiceSweepExpiresSmartApprovalWhenAutoApproveDisabled` — with the
  window disabled the same approval expires, `tool.approval_expired` is
  journaled, and the run fails with `human_timeout`.
- `TestAutoApprovesOnTimeoutPresetGate` — only `workspace_write` + `ask`
  qualifies.
- `TestApplySettingsOverlayApprovalTimeout` — the overlay wins, explicit `0`
  disables, and a window at/above the expiration is refused.
- `TestSaveAndLoadApprovalTimeout` / `TestSettingsApprovalTimeoutOverlay` —
  round-trip through `settings.yaml`, `settings/get`, and `settings/update`,
  including explicit `0`, "absent keeps the saved value", and out-of-range
  rejection.

## 3. UI tests

```text
cd ui; npx vitest run src/components/settings/ApprovalTimeoutCard.test.tsx
  ✓ 4 tests passed
cd ui; npx vitest run src/components/settings/ApprovalTimeoutCard.test.tsx src/i18n/completeness.test.ts
  Test Files  1 failed | 1 passed (2)   # first run: the test's own input
                                        # dispatch; fixed with the native
                                        # value setter, then green above
  completeness.test.ts: 24 passed       # zh/en key parity holds
```

## 4. UI contract sync (found by the first gate run)

The first `just ci` failed `ui-core` with

```text
src/components/settings/ApprovalTimeoutCard.test.tsx(43,27): error TS2551:
  Property 'config_approval_expiration_seconds' does not exist ...   # own typo, fixed
src/lib/ui-build-provenance.test.ts(24,7): error TS2322: Type 'false' is not assignable to type 'true'.
src/lib/ui-sdk-face-compat.test.ts(41,7): error TS2322: Type 'false' is not assignable to type 'true'.
```

A temporary probe (`const probe: FaceClientStore = useVivyStore`, since
deleted) gave the real cause: `FaceClientStore.setState` makes the store check
contravariant, so the host's settings view and the SDK's `FaceSandboxSettings`
must declare the same members:

```text
Type 'FaceSandboxSettings' is missing the following properties from type
'SandboxSettingsView': approval_timeout_seconds, config_approval_timeout_seconds,
approval_expiration_seconds
```

Fixed by declaring the fields in `sdk/ui/src/module.ts` (the Face contract is
the declared host view, exactly as the `catalog` field documents). The `file:`
dependency needed `cd ui; pnpm install --frozen-lockfile --force` to refresh
the hard-linked SDK copy; `pnpm typecheck` then exits 0.

`cd sdk/ui; npx tsc --noEmit` still fails on four **pre-existing** mock drifts
(`catalog`, `chooseWorkspace`, `browseWorkspace`, `setSessionWorkspace`) that
predate this lane and are not covered by `just ci`; recorded as
`APR-SDK-MOCK-DRIFT` in `docs/TODO.md` §0.1. None of them is a sandbox-settings
field, so this lane's contract change is not implicated.

## 5. Full gate

```text
just ci        # fmt-check ui-ci vet test headless-compile plugin-ci
```

**Green (exit 0)** after two fixes found by the gate itself:

1. `i18n-check` rejected the new Web keys as unclassified; the twelve
   `settings.approvalTimeout.*` keys were added to
   `scripts/i18n-cross-face-contract.json` in sort order
   (`node scripts/check-i18n-cross-face.js` → `PASS: 13 shared semantic units`).
2. `test` failed `agent-vivy/sdk/internal/conformance`
   (`TestCheckedInProviderConformanceMatchesExecutedSuites`), because an
   `internal/` edit moves the canonical source digest every internal-rooted
   Provider is pinned to. The new digest
   `caa93f07c74cb17d6f6b9deedd117b4bbca03d12d98e8421afa3090b85636ca4`
   (taken from the failure's own `actual:` payload) replaced the previous
   `5e386f…d3da` in all five entries of
   `sdk/internal/assembly/conformance_results.json`; the conformance suite then
   passed in 188–217 s inside the green run. This is the documented
   `PROVIDER-PROFILE-DIGEST-PIN` step, not a workaround.

Notable stage results inside the green run: `ui-ci` typecheck clean, 368 UI
tests passed, `PASS: en=1441 keys / 145 placeholders; zh=1441 keys / 145
placeholders; runtime copy audit clean`, `go test` green across every package
(including `sdk/internal` 827 s and `sdk/internal/conformance` 217 s),
`headless-compile` green, `plugin-ci` green for all ten plugin/face modules.

## 6. Real-path smoke (live socket, scratch data)

The backend was started against a scratch config
(`VIVY_CONFIG` → scratch `data_dir`, `server.addr: 127.0.0.1:18787`, smart
preset, `timeout_seconds: 300`) so no production Journal was touched, and a
Node client spoke the real JSON-RPC WebSocket (`/rpc/bootstrap` token +
`ws://…/rpc?token=…`) against it:

```text
PASS  config fallback window: 300
PASS  config fallback field: 300
PASS  hard expiration: 300
PASS  update accepted: true
PASS  overlay window: 45
PASS  explicit zero accepted: undefined
PASS  explicit zero in force: 0
PASS  negative rejected: true
```

The scratch `settings.yaml` written by the live process carries the key in the
sandbox block (`approval_timeout_seconds: 0` after the final write), the
server answered `101` on the WebSocket upgrade, and the scratch data directory
was removed afterwards. That covers the wire serialization, the settings
document, validation, and the live apply seam end to end.

## 7. Second cut: every per-call refusal is a tool result

### 7.1 The reported failure is reproduced deterministically

A temporary end-to-end test (scripted model + real engine + real sqlite + real
bash backend) issued exactly the production turn — `pwd; ls -a` and
`cd ../../../../../.. 2>/dev/null && pwd && ls` in one assistant message — on
the pre-fix code. It was deleted after use; the copy is
`.workspace/diag/zz_diag_traversal_test.go` (gitignored scratch):

```text
classifier: class=2 findings=[deny-table: absolute or host path is outside the run workspace ...]
WARN run failed err="engine event error: [NodeRunError] failed to stream tool call
  call-01-traversal: tool argument \"command\": path traversal or UNC paths are not allowed
  ------------------------ node path: [node_1, ToolNode]"
run status = failed
```

`class=2` is `InvocationDenied`: the bash deny table already refuses that
script, so the argument guard was preempting a refusal path that the previous
cut had just made non-fatal. The message matches `run_d32ebd41a4203750` in
`gateway.out.log` verbatim.

### 7.2 Permanent regression test

`TestServiceBashTraversalRefusedWithoutFailingRun`
(`internal/runtime/command_backend_bash_test.go`) pins the incident: the run
reaches `RunCompleted`, no `run.failed` is journaled, the `call-traversal`
`tool.finished` payload carries `did not run` + `path traversal` and no shell
`exit_code`, and the sibling call's `vivy_sibling_marker` really executed.

### 7.3 Tests updated to the refusal contract

`TestToolAdapterRefusesInvalidSchemaBeforeInvocation` (renamed),
`TestToolAdapterPlanModeRefusesEffectfulToolBeforeApproval` (renamed),
`TestToolAdapterApprovalPolicyNeverRefusesEffectful` (renamed),
`TestToolAdapterRechecksPolicyAfterPublicMiddlewareRewrite`,
`TestToolAdapterTieredApprovalForClassifiedTools` (the `safe+never` case),
`TestToolAdapterInfoAndRun` (malformed args), and
`TestServicePlanModeDoesNotOpenApprovalOrMutate` now assert "refused, never
runs, run continues" instead of a returned `ErrPolicyDenied` /
`ErrPlanModeToolDenied` / argument error.

```text
go test ./internal/runtime/ -count=1
  ok  agent-vivy/internal/runtime  223.871s
```

### 7.4 Full gate

`internal/` changed, so the pinned source digest in
`sdk/internal/assembly/conformance_results.json` was refreshed from the
conformance failure's own `actual:` value (five entries, one digest), and
`just ci` was re-run from the repository root:

```text
just ci                                     # exit 0
  fmt-check                                 # clean
  ui-ci        typecheck + 45 test files + i18n completeness/cross-face
  vet                                         # clean
  test         ok internal/runtime 319.866s   ok internal/rpc 173.476s
               ok internal/app 84.239s        ok sdk/internal 705.363s
               ok sdk/internal/assembly 22.152s
               ok sdk/internal/conformance 316.622s (digest pin verified)
  headless-compile                            # ok
  plugin-ci                                   # all plugins/* and faces/* modules ok
```

`internal/workflow` is another lane's untracked work-in-progress and is
excluded by the `justfile` itself (it does not compile yet); the same
exclusion was used when running `go test ./internal/... ./cmd/...` directly.

## 8. Not verified here

- Live provider behaviour (no network/credentials in unit tests). The
  approval→resume→continuation path is covered by the real engine, real
  sqlite, and the real sweeper in `internal/runtime`, including the
  deterministic reproduction above.
- The browser walkthrough at `http://127.0.0.1:3015` in `acceptance.md`
  remains a human step: the component test and `pnpm typecheck` cover
  rendering and the save payload, and the smoke above covers the live
  socket, but no browser was driven in this lane.