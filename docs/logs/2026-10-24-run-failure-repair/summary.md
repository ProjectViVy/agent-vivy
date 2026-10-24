# Repair failed runs: unapprovable proposals, tool-pair ordering, provider status messages

Date: 2026-10-24
Branch: `fix/run-failure-repair` (worktree from `0f1f7720`, root tree was dirty)

## Why

Session `sess_76225446f4024875` (dev home `.workspace/dev/home`) failed three
times in a row:

1. `read_file` returned `The run could not be paused for approval. Please try
   again.`
2. After the steer `我的意思是你自己的子agent`, the next request failed with HTTP
   400 `tool call result does not follow tool call (2013)`, surfaced as
   `无法连接！请检查供应商配置！`.
3. The session had no workspace binding, so the model tried to reach the repo
   with `commandline {"command":"git","args":["rev-parse","--show-toplevel"],
   "cwd":"../../../../.."}`. `resolveCommandContext` rejected the cwd inside
   `PrepareProposal` *after* the run had suspended; `handleInterrupt` turned
   that recoverable argument error into a fatal terminal.

Root causes:

- `internal/runtime/service.go` prepared the approval proposal only after the
  run was already interrupted, so any preparation error was run-fatal.
- `pairToolTurns` (`internal/runtime/context.go`) only checked that calls and
  results existed in the same window; it never restored adjacency. A steer
  persisted mid-group was projected between the calls and their results, which
  providers reject.
- `terminalEvent` collapsed every text containing `status code` into
  `provider_error` + `无法连接！请检查供应商配置！`, and the UI discarded
  `payload.message` for that category. A self-inflicted 400 and an upstream 500
  gave the same, wrong advice.
- The daily install (`~/.vivy`) refused to start because its database was
  written by the `main` lineage (`036_notebook_content`) while this checkout is
  `stable` (`036_tool_operation_private_binding`). Rejection is correct; the
  message was not actionable.

## What changed

### 1. An unapprovable proposal is a tool refusal, not a dead run

- New optional `tools.ProposalPreflight` capability
  (`internal/tools/tools.go`). The runtime checks a would-be approval on the
  final arguments *before* suspending; a provider that stages durable state
  implements the capability, everyone else is checked by `PrepareProposal`.
- `internal/runtime/tooladapter.go`: the preflight runs only for calls that
  need approval and only on first execution (a resumed call was already
  validated before human review). `*tools.ArgError` becomes an
  `invalid_arguments` refusal; `ErrSandboxDenied`/`ErrPolicyDenied` become a
  `policy_denied` refusal; anything else stays run-fatal (wiring failures must
  not blame the model).
- `internal/app/assembly_tools.go`: both governed proposal wrappers forward the
  capability.
- `internal/runtime/command_backend.go`: cwd rejections (absolute, escaping,
  unresolvable, not a directory) are now `*tools.ArgError`; a symlink escape is
  `ErrSandboxDenied`.
- `internal/runtime/skills_backend.go`: the pure plan (`buildSkillProposal` /
  `buildRollbackProposal`) is separated from staging; `PreflightSkillProposal`
  validates without writing a revision. A read-only skill root is classified as
  `ErrPolicyDenied` at the proposal seam so the same refusal path applies.
- `internal/tools/skills.go`: `skill_manage` exposes `PreflightProposal`.

Repro outcome: `commandline did not run: tool argument "cwd": must stay inside
the run workspace. Choose a different tool or arguments.` The run continues.

### 2. Tool call/result adjacency

- `stabilizeToolTurns` (`internal/runtime/context.go`) defers user rows that
  were persisted inside an open tool-call group until that group's results are
  emitted; no row is dropped. `pairToolTurns` now drops unpaired rows first,
  then stabilizes.
- `internal/runtime/overflow.go`: both the kept pre-run tail and the pending
  turn are stabilized (never paired: pending calls legitimately have no results
  yet).
- Queue write order and steer semantics are unchanged; only the projection is
  normalized.

### 3. Provider failures split by HTTP status

- `internal/runtime/service.go`: `providerStatus` reads the status embedded by
  the pinned Eino OpenAI adapter, and `providerHTTPFailure` maps it for both the
  parent and child terminal branches before any string-marker fallback:
  - 401/403 → `provider_error`, API-key text (keeps the UI settings action);
  - 429 → `provider_error`, rate-limit text;
  - ≥500 → `provider_error`, service-unavailable text;
  - other 4xx → `internal_error`, request-rejected text.
  Transport failures keep `providerUnavailableMessage`; that stays the only
  "check your provider configuration" message.
- `ui/src/lib/failure.ts`: `runFailedMessage` now prefers a non-empty
  `payload.message`; a status-aware provider branch renders the backend text as
  the detail; `'api key'` routes to model settings.

### 4. Actionable daily-install failure

- `cmd/vivy/main.go`: when composition fails with
  `migrations.ErrUnsupportedSourceFormat` or `ErrUpgradeRequired`, a second log
  line names the data directory and suggests `VIVY_USER_HOME` or the matching
  build. No auto-fallback home, no migration, no reset.
- `internal/storage/migrations/preflight.go` is unchanged: the rejection is
  correct.

## Explicitly not done

- `~/.vivy` data was not modified, migrated, reset, or backed up over.
- No workspace-binding change, no subagent change, no model-tool-choice change.
- The pinned Eino adapter's SSE error-frame decoding gap is recorded as a
  follow-up, not worked around with a second client.
- Compaction fold-index safety around tool groups is recorded as a follow-up
  (summary-quality issue, not a provider-400 path).
