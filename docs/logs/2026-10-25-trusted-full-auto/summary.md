# Trusted preset full-auto approval — summary

## What changed

The Trusted preset (`danger_full_access` + `auto`) now runs every effectful tool the governance
policy allows without opening an approval card. Previously `auto` only skipped approval for readonly
tools and a hard-coded name whitelist, so `write_file`, `patch`, `multiedit`, non-safe `bash`,
`execute`, `http_request`, and MCP calls kept prompting even in Trusted.

- `internal/runtime/policy.go` — `EvaluateApprovalPolicy` no longer takes an auto-approve list;
  `auto` returns `AutoApprove` for every effectful tool. `ask`, `never`, and unknown policies keep
  their previous behavior.
- `internal/runtime/tooladapter.go`, `internal/runtime/shell.go`, `internal/runtime/engine.go`,
  `internal/runtime/action_tool.go`, `internal/app/compaction.go` — removed the `autoApprove` /
  `AutoApproveTools` plumbing from the adapter and engine config.
- `internal/config/config.go` — deleted the `auto_approve_tools` default. The YAML field remains as
  a documented decode-only no-op because config loading uses `KnownFields(true)`; existing
  `config.yaml` files keep starting.
- `config.example.yaml`, `docs/dev/sandbox.md`, `internal/domain/sandbox.go` — describe the new
  `auto` and Trusted semantics.
- `ui/src/i18n/zh.ts`, `ui/src/i18n/en.ts` — chat selector, Trusted confirmation, and Settings
  preset copy no longer claim whitelist-only auto-approval.
- Tests — adapter/policy expectations now cover "mutating + auto executes", the plan-approval gate
  test pins `ask` (it exists to exercise the approval resume path), and every test harness literal
  for the removed config field was deleted.

Hard stops are unchanged and still settle before `EvaluateApprovalPolicy`: governance `PolicyDeny`,
the bash deny table (`InvocationDenied`), plan-mode refusal, `create_goal` human authorization, and
middleware `RequireApproval`.

## Scope

Kernel approval semantics plus user-facing preset copy. No storage/schema change, no RPC shape
change, no provider routing change, and no change to approval timeout behavior.

## Explicitly not done

- Approval timeout semantics are untouched: the smart preset still auto-approves on timeout, other
  presets still expire (`APR-TIMEOUT-UX` remains deferred).
- No browser smoke of the split dev pair was run in this environment; `pnpm typecheck` and the i18n
  key contracts were run instead.
- `just ci` / kernel CI were not run locally (Linux-filesystem WSL checkout unavailable here).
