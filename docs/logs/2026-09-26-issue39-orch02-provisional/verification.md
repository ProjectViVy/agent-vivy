# Issue 39 provisional implementation verification — 2026-09-27

This log records implementation checks for ORCH-02–07. It does not represent G0/G1 acceptance, merge, or release; G4 remains an owner decision.

## Environment (2026-09-27 verification pass, Linux workspace)

- Branch: `feat/issue-39-agent-authored-dag-orchestration` (PR #64), fix commit `b8335280`.
- Go: 1.26.4 at `/usr/local/go/bin`; Node v24.9.0 + pnpm 11.19.0 at `~/.local/node/bin`.
- PostgreSQL 16 via Docker (`postgres:16`, `vivy/vivy` on `127.0.0.1:5432`), so `VIVY_POSTGRES_TEST_DSN` was configured this run.
- `just` and PowerShell: unavailable; `just ci` recipes were run as their raw command equivalents on Linux (the justfile is PowerShell-bound).
- No real provider API key. Browser E2E used a local hanging OpenAI-compatible mock registered via Settings → Model so parent runs stay server-side `active` (required by `child/*` and `workflow/*` admission).

## Passed checks

| Command | Result |
| --- | --- |
| `gofmt -l` on all changed files (fmt-check equivalent) | PASS — clean |
| `pnpm build` / `pnpm typecheck` / `pnpm test` (from `ui/`) | PASS — 51 test files, 402 tests |
| `node scripts/check-i18n-completeness.js` + `node --test scripts/check-i18n-cross-face.test.js` + `node scripts/check-i18n-cross-face.js` | PASS — `common.loading` added to en/zh; 25 `runInspector.*` keys registered in the cross-face contract |
| `GOFLAGS=-buildvcs=false go vet ./...` | PASS — repository-wide |
| `GOFLAGS=-buildvcs=false go build ./cmd/vivy ./cmd/vivy-code` | PASS — includes headless-compile lane |
| `GOFLAGS=-buildvcs=false go test -timeout 20m -count=1 ./...` | PASS — repository-wide, 72 packages ok, zero failures (includes `internal/provider`; no unapproved egress was attempted) |
| `VIVY_POSTGRES_TEST_DSN=postgres://vivy:vivy@127.0.0.1:5432/vivy?sslmode=disable go test -count=1 ./internal/storage/postgres` | PASS — full CN-01..CN-33 conformance + migrations incl. V14 in-place upgrade on real Postgres 16 |
| `go test ./plugins/...` (plugin-ci equivalent, per prior pass) | PASS |
| Browser E2E at `http://127.0.0.1:3015` (backend `:8787` + Vite dev) | PASS — recorded; details below |
| Real model-path browser E2E (SenseNova `sensenova-6.8-flash-lite`, OpenAI-compatible custom provider) | PASS — recorded; details below |

## Fixes applied on this pass (commit `b8335280`)

- Removed orphaned `internal/worker/server_test.go` (package implementation was deleted earlier on the branch; the test referenced `undefined: Run` and broke vet/build).
- Added `common.loading` to `ui/src/i18n/en.ts` and `zh.ts`; registered the 25 `runInspector.*` web keys in `scripts/i18n-cross-face-contract.json`.
- Re-pinned `sdk/internal/assembly/conformance_results.json` internal source digests (`source-hash` after the suite edits).
- `internal/storage/conformance/suite.go`: CN-19/20/21 now create the sessions they write against — Postgres enforces `session_id` foreign keys that SQLite only declares. Identical failures were reproduced on `origin/main` first: these were pre-existing suite defects, not PR regressions.
- `internal/storage/postgres/upgrade_test.go`: V14 signature column set ordered by column name (position differs on in-place upgrades); empty-text defaults accept Postgres 16's `''::text` rendering.

## Browser E2E results (real `127.0.0.1:3015`, recorded)

- No provider key: run fails gracefully — visible "Unable to connect! Check your provider configuration!" banner, composer recovers, no hang/crash.
- Run Inspector renders all child-run surfaces (delegated task form, policy profile, tool names, continuable-session toggle, history, pending panels) and the DAG workflows section with prefilled descriptor; zero raw i18n keys; controls correctly disabled on failed runs.
- Workflow: invalid JSON → "The descriptor JSON is invalid"; valid descriptor → `workflow/propose` digest, `workflow/start` admitted an active workflow, node spawned a real running child with "Open child run" link; Cancel rendered.
- Continuable child lifecycle: `child/start` → child session; mailbox send admitted; Interrupt cancelled the in-flight activation; follow-up reactivated the SAME child session with accumulated history; correct conflict error while an activation was in flight.
- Persistence: page reload and full backend kill+restart rehydrate sessions, children, and workflow state from the SQLite journal; orphaned node child shows the fenced "child worker was lost during server restart" message.
- Locale: zh renders all new keys (子 Run / DAG 工作流 / 委派任务 / 保留可续接的子会话 …); en/zh toggle clean.
- Keyboard: Tab reaches descriptor textarea, workflow items, and child links; Enter expands rows; disabled controls skipped correctly. Console clean.

## Real model-path E2E (SenseNova `sensenova-6.8-flash-lite`, recorded)

A real OpenAI-compatible provider was registered via Settings → Model and the full orchestration surface ran with actual model output:

- Normal runs produce real assistant replies and real tool calls (`write_file` executed end-to-end through the approval gate).
- Continuable child: `child/start` produced real assistant content in child history; a parent→child mailbox message was admitted and consumed by the follow-up activation (child acknowledged it verbatim); the follow-up ran on the same `csess_` session.
- `reply_parent` roundtrip: child replies surfaced in the "Messages waiting for the parent" panel and were consumed when a later parent run called `child_inbox`; all rows flipped to `consumed`.
- Workflow/DAG: `workflow_495867222813d7de` ran the `draft` node child to completion; the Declared outputs panel populated with real model text; the workflow reached `completed` and the node output was journaled into the parent transcript.
- Persistence: reload and backend kill/restart rehydrate all state; the journal shows the 42-event run ending `run.completed`; no ghost `active` runs.
- Provider compatibility: no console errors, panics, rate-limit or tool-call format failures; SenseNova streams `reasoning` + `content` and supports `tool_calls`.

### New findings surfaced by the real-provider pass (recorded, not fixed here)

1. **Node children can call `ask_user`, which always fails headless.** A vague node task led the node child to call `ask_user`; nobody answered → `run.failed` (`cause_category: human_timeout`) → workflow "The child task did not complete successfully." Machinery behaved correctly; the design gap is that human-interaction tools sit inside a node child's tool ceiling. Product decision needed.
2. **Composer queue did not auto-drain.** A message queued while a run waited on approval stayed queued after the gate cleared; had to be cleared and re-sent manually.
3. **Inspector is currentRun-scoped with no run picker.** Previous runs' children/history/mailbox become unreachable in the UI once a newer run exists; a re-expanded child row does not refetch (stale history until reload).
4. Cosmetic: `model.usage` payloads label the custom provider `"deepseek"`; a sandbox-denied `sleep` escalates to run failure via the pause path; message timestamps render in a different timezone than UTC.

## Still not verified (requires owner)

- Literal `just ci`: the justfile is PowerShell-bound; the raw equivalents above all pass on Linux, but `just ci` itself needs the supported environment.
- Descendant budget/usage reconciliation and slot/backpressure accounting across reauthorization and restart (R13).
- Owner E2E and final release review (G4).

## Gate disposition

G0–G3: the previously missing machine-checkable, both-backend and browser evidence — including a real model path — is now produced and recorded. Remaining gaps concentrate in descendant resource accounting (R13) and the product decisions listed above. G4 remains **BLOCKED** — owner E2E and release review are not delegated.
