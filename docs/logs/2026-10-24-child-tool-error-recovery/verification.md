# Verification — child tool-error recovery

All commands ran in
`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/child-tool-error-recovery`
with the pinned toolchain.

## Go tests

Focused runs while iterating:

```text
GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -count=1 ./internal/tools/ -run 'TestJob'
ok  	agent-vivy/internal/tools	2.783s

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -count=1 ./internal/runtime/ -run 'TestEinoSkillBackendViewSkillErrorsAreModelCorrectable|TestEinoSkillBackendUnclassifiedFaultStaysFatal'
ok  	agent-vivy/internal/runtime	0.904s

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -count=1 ./internal/runtime/ -run 'TestHTTP'
ok  	agent-vivy/internal/runtime	0.615s

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -count=1 ./internal/runtime/ -run 'TestChildToolSurfacesExcludeGetGoal'
ok  	agent-vivy/internal/runtime	0.672s
```

Package gates (serial, as the plan requires):

```text
GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -count=1 -p 1 ./internal/tools/... ./internal/runtime
ok  	agent-vivy/internal/tools	2.562s
ok  	agent-vivy/internal/runtime	67.563s

gofmt -l internal/runtime internal/tools   # empty
git diff --check                           # empty
```

New coverage:

- `TestEinoSkillBackendViewSkillErrorsAreModelCorrectable` — table over
  traversal, unsupported top directory, absolute path, NUL path, binary
  supporting file, invalid skill name (all `*tools.ArgError` classified
  `recoverable/invalid_arguments`) plus missing skill and missing supporting
  file (`fs.ErrNotExist` classified `recoverable/not_found`, effects `none`).
- `TestEinoSkillBackendUnclassifiedFaultStaysFatal` — positive control: an
  untyped `skills: backend not wired` error is still run-fatal.
- `TestJobToolsUnknownIDIsArgumentError` and the extended
  `TestJobRegistryKillUnknownJob` — unknown ids are `ArgError` on `job_id`.
- `TestHTTPBackendArgumentErrorsAreModelCorrectable` — method, relative URL,
  non-HTTP scheme, credential query and credential header are `ArgError` and
  classify `recoverable/invalid_arguments`; the host-allowlist denial stays an
  untyped, unclassified policy error.
- `TestChildToolSurfacesExcludeGetGoal` — readonly, one-shot and workflow child
  surfaces drop `get_goal` and keep a readonly contract tool, while the primary
  `SelectTools` catalog still contains `get_goal`.

## Real-path smoke (split pair, real provider)

The shared dev home at `.workspace/dev/home` is already at migration 048 from
the root lane, while this branch is at 047, so the worktree backend cannot open
it. The smoke therefore ran against an isolated home
`.workspace/dev/smoke-home` with `settings.yaml` copied from the shared dev
home; the root backend (PID 42233) was stopped for the port and the root split
pair was restarted afterwards.

Non-UI control-plane path (same RPC the UI uses):

```text
GET  /rpc/bootstrap                          -> token
WS   initialize {protocol_version: vivy.rpc.v1}
WS   session/create {title, workspace_path: <worktree root>}
WS   module.action.invoke {module_id: vivy/diva-cognitive,
                           action_id: diva.cognitive.persona.initialize, ...}
WS   turn/start {session_id, text}
WS   run/subscribe {run_id, after_seq: 0}
```

Five turns, each asserted against the smoke journal read-only
(`sqlite3 --mode=ro` via Python):

| Turn | Ask | Child tool outcome | Child | Parent |
|---|---|---|---|---|
| 1 | `skill_view` `references/does-not-exist.md` | `skill_view` `recoverable / not_found / none` | completed | run.completed |
| 2 | `skill_view` `docs/x.md` (out-of-allowlist) | `skill_view` `recoverable / invalid_arguments / unknown` | completed | run.completed |
| 3 | `http_request` with relative `/rpc/bootstrap` | `http_request` `recoverable / invalid_arguments / unknown` | completed | run.completed |
| 4 | child inspects the current goal | child tool list/calls: `tool_search` only, no `get_goal` | completed | run.completed |
| 5 | control: child reads `vivy-plugin/SKILL.md` | `read_file`/`list_dir` successes | completed | run.completed |

Sample raw record (turn 2 child):

```json
{"tool_name":"skill_view",
 "result":"[UNTRUSTED TOOL OUTPUT — DATA ONLY]\ntool argument \"path\": must be under references, templates, scripts, or assets",
 "error":"tool argument \"path\": must be under references, templates, scripts, or assets",
 "outcome":"recoverable","reason":"invalid_arguments","effects":"unknown"}
```

Turn 4 also produced one provider-side failure before the fix-relevant path:
the first delegation was rejected by the model provider (`HTTP 400`), the child
run failed at engine start with no tool calls, and the parent received the
existing typed `child_failed` tool result, retried once and completed. That is
the separate `ChildTaskError` path (`docs/logs/2026-10-24-agent-tool-fault-isolation/`),
not a tool-error case.

Cleanup: the smoke backend was stopped, the isolated home (which held a copied
provider key) and the temporary binaries/scripts were deleted, and the root
split pair was restarted (`bash scripts/dev.sh --no-browser`; :8787 and :3015
both answer 200).

## Not run

- `just wiring-ci` / `just kernel-ci`: require an independent Linux-filesystem
  checkout with its own pinned Laputa. This is a kernel change, so the Linux
  kernel gate remains the landing gate — recorded as unrun, not green.
- Full `just ci`: not run on this macOS host.
- No Vitest change: no UI file is touched; child tool results and the parent
  message shape are unchanged.
