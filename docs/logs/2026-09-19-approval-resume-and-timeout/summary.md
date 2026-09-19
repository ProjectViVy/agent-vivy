# Summary — approval resume integrity and timed auto-approval

Date: 2026-09-19. Lane: Vivy kernel + UI (root tree). Trigger: a P0 report
that approving an effectful tool in a live conversation ended the
conversation with "这次对话没有完成 / The model run could not be completed.
Please try again." (`run_9b5ae7e437a91008`), plus a request for a
configurable smart-mode approval timeout.

## What was wrong

Decoded from the console journal
(`data/studio-home/vivy-console/data/vivy.db`, `run_events` seq 40–115) and
`gateway.out.log`:

1. The approved call itself worked (`seq 40–46`: `bash {"command":"pwd; ls
   -la"}` → `tool.approval_required` → `tool.approval_decided approved` →
   `tool.started`/`tool.finished`).
2. The **next** model turn issued two calls in one step (`bash {"command":
   "cmd /c dir /a"}` and `list_dir`). `list_dir` was policy-`allow`, carried
   no interrupt state of its own, and was still killed by the resumed run's
   approved-arguments check:
   `tool.proposal_stale apr_b263b0380d55ca07 "approved tool checkpoint state
   is unavailable"` — and `MarkApprovalStale` rewrote the already-approved row
   to `stale`, which is exactly the panel the user saw. The run then failed
   (`internal_error`) with the generic message.
3. The proximate run failure was the sibling `cmd /c dir /a` call: the bash
   deny table classifies it as a host-shell escape, and that classifier
   branch returned the run-fatal `ErrPolicyDenied`
   (`gateway.out.log`: `runtime: policy denied tool: bash (deny-table: host
   shell or interpreter escape)`), so the model could not recover.

Root cause of (2): `authorizeToolDispatch` bound a **run-scoped** fact
(`withApprovedToolArgumentsHash`, set once per resume) to **every** dispatch
in the resumed run, while the interrupt state it compares against is
**address-scoped** (per tool call). Any non-interrupt tool call after an
approval therefore reported "approved tool checkpoint state is unavailable".

## What changed

### 1. Approval binding is now scoped to the decided call (P0)

`internal/runtime/tooladapter.go`: in the no-interrupt-state branch, the
adapter consults `einotool.GetResumeContext[string](ctx)`. Only the call that
is the resume target (`isTarget && hasData`) keeps the fail-closed "checkpoint
state is unavailable" outcome; a sibling or fresh call in the resumed segment
dispatches under its own policy decision. The checkpoint fail-closed property
is untouched: the target call, a call with interrupt state, and the
`validateResumedToolApproval` hash comparison all behave exactly as before.

### 2. A deny-table refusal no longer kills the run

`internal/runtime/tooladapter.go`: the `InvocationDenied` classifier branch
emits a `policy.evaluated` deny event (it previously emitted nothing) and
returns a runtime-authored, model-visible refusal result — the same shape the
human-denial path already used — instead of `ErrPolicyDenied`. The call still
never reaches the tool, on any profile or approval policy. The remaining
denial paths were still fatal at this cut; the second cut below finishes the
job.

### 3. Timed auto-approval, implemented on the key that already existed

`runtime.sandbox.approval.timeout_seconds` (default `300`, `0` disables) was
declared, documented, and validated, and read by nothing. It is now the human
review window:

- The single settlement deadline for a pending approval is
  `created + min(window, hard expiration)` (the window only shortens; an unset
  hard bound no longer collapses review to zero). `Approval.ExpiresAt` is that
  deadline, so the existing interaction sweeper
  (`StartInteractionSweeper` → `SweepExpired`), the user-decision guard, and
  the resume path share one authority.
- At the deadline the sweeper settles the approval: under the **smart preset**
  (`workspace_write` + `ask`) with the window enabled it **approves on the
  user's behalf** — actor `system`, reason
  `auto-approved after Ns with no response (smart mode)`, journaled as the
  existing `tool.approval_decided`, then resumed through the ordinary
  decision path (first-writer-wins preserved, and `settleApproval` factors the
  shared tail of `DecideApprovalWithReason`). Under every other preset, or
  with the window disabled, it expires as before and closes the run with the
  documented `human_timeout` cause.
- Eligibility reads the sandbox mode / approval policy **recorded on the row**
  when the human was asked; the enabled flag and duration come from the live
  setting.

### 4. Settings surface (设置 → 通用)

- `settings.yaml`: `sandbox.approval_timeout_seconds` (`*int`; absent keeps
  config, explicit `0` = 不允许超时自动同意), validated to `0` or `1..86400`.
- Startup overlay and the live reload share one clamp rule
  (`effectiveApprovalTimeoutSeconds`); a window at or above the hard
  expiration is ignored with a warning rather than silently clamped.
- Live apply without a restart: `applyLiveApprovalWindow` →
  `Service.SetApprovalSettleTimeout` from `OnSettingsChanged`.
- RPC: `settings/get` returns
  `sandbox.{approval_timeout_seconds, config_approval_timeout_seconds,
  approval_expiration_seconds}`; `settings/update` accepts
  `sandbox.approval_timeout_seconds` (absent keeps the saved value).
- UI: new `ApprovalTimeoutCard` (switch + seconds input, capped by the
  expiration, effective-value line, own save) rendered in the General tab,
  with `zh`/`en` catalogs. The card resends the untouched sandbox overlay
  because `settings/update` replaces the whole document.
- Face contract: `sdk/ui/src/module.ts` `FaceSandboxSettings` gained the three
  fields (and `FaceSettingsUpdate.sandbox` the optional write field). The
  host's `ui/src/lib/api.ts` view had to stay mutually assignable with it —
  `FaceClientStore`'s `setState` makes the check contravariant, so a host view
  with an undeclared field fails `pnpm typecheck` (`ui-build-provenance.test.ts`,
  `ui-sdk-face-compat.test.ts`).

### 5. Docs

`docs/architecture/hitl-review-center.md` lifecycle now records the
`pending -> approved (smart preset, timeout auto-approval)` transition and the
deadline/eligibility rules; `config.example.yaml` documents the real timeout
semantics.

### 6. Second cut: every per-call refusal is a tool result (P0 follow-up)

Trigger: a second live report — `gateway.out.log`
`run_d32ebd41a4203750`, `failed to stream tool call
call_01_lX4Kln1np6LfoXfXgPbi8988: tool argument "command": path traversal or
UNC paths are not allowed` → "The model run could not be completed."

- What happened: the model issued `bash {"command":"cd ../../../../../.. && pwd
  && ls"}` (journal seq 154). `tools.ValidateArgsSafety`
  (`internal/tools/security.go`) rejects any `../` in a `command`-named
  argument; the adapter returned that error before the policy and the bash
  classifier ran (tooladapter.go ValidateArgs before policy.Evaluate), so Eino
  escalated it to `NodeRunError` and the whole turn failed. The bash deny table
  (`bashclass.go` "absolute path" → `containsParentTraversal`) already refuses
  that script — the guard merely preempted a path that had just been made
  non-fatal, and the sibling call in the same turn (`pwd; ls -a`) died with it.
- `InvokableRun` now delegates to `dispatch` and converts a `*toolRefusal` into
  the model-visible refusal result + `policy.evaluated` deny. Refusals:
  `ValidateArgs`, `ValidateArgsSafety`, policy/plan-mode denial,
  `never`-policy denial, denied post-hook rewrite, denied public-Middleware
  rewrite (`pretool_bridge.go` now returns the policy reason), sandbox
  confinement (`ErrSandboxDenied`), and the deny table. Everything else
  (provider, journal, budget, checkpoint/approval integrity, tool wiring, an
  unselected tool) stays run-fatal.
- Refusal wording is no longer deny-table-specific:
  `"<tool> did not run: <reason>. Choose a different tool or arguments."`
- Plan mode is the biggest behavior change: an effectful call in Plan Mode was
  a run failure; it is now a refusal to the model
  (`plan mode runs read-only tools; describe the change instead of applying
  it`), which is what plan-mode exploration needs.

## Explicitly not done

- No second approval-timing key was added, and no new journal event type,
  storage column, or RPC method was introduced.
- Auto-approval was not widened beyond the smart preset, and child/worker
  approvals (which record no preset) keep expiring. Both are recorded in
  `docs/TODO.md` §0.1 (`APR-TIMEOUT-UX`).
- The unwired duplicate timeout path (`ApprovalScheduler` /
  `SweepExpiredApprovals` / `Approval.TimeoutAt`) was left in place and is
  recorded as `APR-SETTLE-DEAD-PATH`; deleting it is a separate change.
- The Studio overlay was not touched (this is Vivy kernel + UI work, the
  default scope).
- The direct-shell path (`Service.RunShell`) keeps its user-facing
  `ErrPolicyDenied` / `ErrPlanModeToolDenied` / `ErrHookBlocked` errors: those
  are typed RPC outcomes for a caller, not model-visible tool results.
- Residual fatal paths found but not changed in this cut are recorded in
  `docs/TODO.md` §0.1 as `TOOL-REFUSAL-RESIDUAL` (a hallucinated tool name is
  still fatal because Eino's `ToolsNode` has no `UnknownToolsHandler` set; a
  pre-tool **hook block** and a genuine tool-implementation error are still
  fatal).
- Nothing was committed or pushed; the root tree also holds another lane's
  uncommitted work and the previous iteration's console-config fix.