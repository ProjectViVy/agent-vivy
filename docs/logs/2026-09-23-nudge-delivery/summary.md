# ND-4 Delivery Summary — Integrated acceptance and handoff

Date: 2026-09-23. Story: [ND-4](../../plans/nudge/ND-4.md), issue [#58](https://github.com/ProjectViVy/agent-vivy/issues/58). Branch `docs/issue58-nudge-design`.

## What was delivered

`internal/runtime/nudge_acceptance_test.go` — the ND-4 acceptance suite. It drives the **product path** end to end: real `NewEngine` (production middleware chain incl. `newNudgeMiddleware`), real `Service`/`Run`, real sqlite Journal (wrapped by the ND-0 `contractJournal` decorator for pause/fail injection), real `WorkspaceManager`/`SandboxManager`/`NewEinoFilesystemBackend`/`NewCommandBackend` behind the stock `write_file`/`read_file`/`bash` tools, and the scripted `contractCaptureModel` as the only fake — no network, isolated `t.TempDir()` database and workspaces.

All ten plan cases (A–J) are implemented and passing:

- **A** missing-file read → typed `not_found` error → corrected read → `run.completed`.
- **B** command exit 1 → `command_failed`/`effects=unknown` stays inspectable → corrected command → completion.
- **C** MCP `IsError` through the real `mcphost.Host` → `ToolWorld` → `toolhost.Host` → governedTool-shaped wrapper → enhanced adapter; arrives as `remote_tool_error`; correction in the same Run.
- **D** six identical failing calls → nudges at counts 3 and 5 only, six durable results, one `run.failed` (`tool loop detected`).
- **E** denied write under `never` policy → `refused`/`not_executed` per call, one nudge at 3, zero filesystem mutation (glob-verified).
- **F1** Journal append failure while the model waits → `run.failed`, no further model request. **F2** cancel while the finished-append is held → `run.cancelled`, no deadlock, no new model request.
- **G** two parallel batches, first tool-run gated until the second's results are durable → request-order detection (the single notice names `call-g3@3`), paired results, one notice per boundary.
- **H** `ask`-policy approval pause with checkpoints enabled → resume leg → exactly one nudge at 3, no stale/duplicate scheduling.
- **I** partial-effect command (`touch marker && exit 2`) → `command_failed`/`effects=unknown`, marker written exactly once, no automatic replay.
- **J** legacy untyped `tool.finished`/`tool.nudge` payloads decode with empty typed fields; every new payload decodes into the typed structs.

## Real-path smoke (split runtime)

Launched the documented dev pair — backend `go run ./cmd/vivy` on `127.0.0.1:8787` and `pnpm dev` Vite UI on `127.0.0.1:3015` — with `VIVY_USER_HOME=/home/ubuntu/smoke/home` so the Journal (`vivy.db`), workspaces and settings all live in a disposable directory. `data/` was never touched (it does not exist in this checkout). A scripted OpenAI-compatible stub on `127.0.0.1:8399` stood in for the model via the documented ENV-session (`DEEPSEEK_API_KEY` + `VIVY_API_BASE` + `VIVY_MODEL=smoke-1`); every other component is production code.

Observed at `http://127.0.0.1:3015`, run `run_3362b503793c9356`: three `read_file missing.txt` tool cards each rendered "Failed: filesystem: path does not exist: file does not exist", one `tool.nudge` journaled after the third (`{tool_call_id: call_r3, reason: not_found, repeat_count: 3, template_version: nudge-v1}`), `write_file hello.txt` waited on a real approval card (diff preview shown, approved as `local_user`), `read_file hello.txt` succeeded, and the run ended in a single `run.completed`. A second session (`run_4770db62e35535ca`) proved cancel-while-waiting: `write_file` sat in `tool.approval_required`, the UI Cancel produced `tool.approval_cancelled{reason:"run cancelled"}` then `run.cancelled{user_requested}` — no write landed, no deadlock. Journal inspection: every run has exactly one terminal event; no payload contains the provider credential.

## Gates

- `go test -timeout 20m ./internal/runtime ./internal/mcphost ./internal/toolhost ./internal/domain -count=1` — PASS (all packages).
- `just ci` (fmt-check + ui-ci + vet + `go test -timeout 20m ./...` + headless-compile + plugin-ci) — PASS after regenerating `sdk/internal/assembly/conformance_results.json` for the new internal source digest.
- Index race command `go test -race ... -run 'TestNudge|TestToolFailure'` — ran on a supported host (linux/amd64, Go 1.26 race toolchain) and **did not pass**: it surfaces the preexisting upstream defect `EINO-TOOLSNODE-ERR-RACE` (eino v0.9.13 `tool_node.go:1253`, shared captured `err` across parallel enhanced converters), already OPEN in `docs/TODO.md` §0.1 since the ND-0 suite found it. Reproduced with the ND-4 test excluded, confirming it is not introduced here. Not called passed.

## Files

- New: `internal/runtime/nudge_acceptance_test.go`.
- Updated: `sdk/internal/assembly/conformance_results.json` (internal source digest refresh, same procedure as every ND commit), `docs/plans/nudge/README.md` (index statuses), `docs/COMPLETE.MD` (completion record).
- Evidence: this directory's `verification.md` and `acceptance.md`; smoke artifacts under `/home/ubuntu/smoke/` (not committed).
