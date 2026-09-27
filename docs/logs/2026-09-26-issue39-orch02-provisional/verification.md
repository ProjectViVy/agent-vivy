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
| `pnpm build` / `pnpm typecheck` / `pnpm test` (from `ui/`) | PASS — 56 test files, 416 tests |
| `node scripts/check-i18n-completeness.js` + `node --test scripts/check-i18n-cross-face.test.js` + `node scripts/check-i18n-cross-face.js` | PASS — `common.loading` added to en/zh; 25 `runInspector.*` keys registered in the cross-face contract |
| `GOFLAGS=-buildvcs=false go vet ./...` | PASS — repository-wide |
| `GOFLAGS=-buildvcs=false go build ./cmd/vivy ./cmd/vivy-code` | PASS — includes headless-compile lane |
| `GOFLAGS=-buildvcs=false go test -timeout 20m -count=1 ./...` | PASS — repository-wide, 72 packages ok, zero failures (includes `internal/provider`; no unapproved egress was attempted) |
| `VIVY_POSTGRES_TEST_DSN=postgres://vivy:vivy@127.0.0.1:5432/vivy?sslmode=disable go test -count=1 ./internal/storage/postgres` | PASS — full CN-01..CN-34 conformance + migrations incl. V14 in-place upgrade on real Postgres 16 |
| `go test ./plugins/...` (plugin-ci equivalent, per prior pass) | PASS |
| Browser E2E at `http://127.0.0.1:3015` (backend `:8787` + Vite dev) | PASS — recorded; details below |
| Real model-path browser E2E (SenseNova `sensenova-6.8-flash-lite`, OpenAI-compatible custom provider) | PASS — recorded; details below |
| `pnpm exec playwright test --config playwright.masks.config.ts` | PASS — 5/5 mask e2e against the packed `masks-selected` artifact |
| `go run ./sdk pack` + `inspect-artifact` on `masks-selected` / `masks-omitted` / `masks-backend-only` recipes | PASS — mask UI refs 2/0/0; artifacts smoke-run |

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

### New findings surfaced by the real-provider pass

1. **FIXED — Node children could call `ask_user`, which always fails headless.** A vague node task led the node child to call `ask_user`; nobody answered → `run.failed` (`cause_category: human_timeout`) → workflow "The child task did not complete successfully." Owner decision: children must not hold human-interaction or other flow-affecting tools until a designed child-agent enhancement round (a child's question should route to the parent, which decides whether to escalate to the human). `ask_user` is now excluded from `readOnlyChildTools` (so continuable ceilings, reauthorization, and node descriptor validation all drop it) and explicitly rejected in the one-shot explicit-selection path; covered by `TestChildrenCannotSelectHumanInteractionTool`.
2. **FIXED — Composer queue did not auto-drain.** A message queued while a run waited on approval stayed queued after the gate cleared; queued messages now dispatch automatically when the pending approval settles.
3. **FIXED — Inspector is currentRun-scoped with no run picker.** The Inspector now offers a run switcher covering previous runs, and re-expanding a child row refetches its history instead of showing stale data.
4. **FIXED — `model.usage` mislabeled the custom provider.** Usage rows now carry the resolved provider profile id instead of hardcoding `"deepseek"`. Remaining cosmetic note: a sandbox-denied `sleep` escalates to run failure via the pause path; message timestamps render in a different timezone than UTC.

### Mask subsystem status — MASK-4 now wired (this pass)

- `core/mask-service@v1` promoted to SUPPORTED; `vivy/masks` + `vivy-masks-ui` are selected in `recipes/default.vivy.yml` and present in the embedded `zz_default` assembly (baseline inventory golden updated).
- Placeholder UI torn down: `ui/src/components/masks/mask-catalog.ts`, `MaskSelector`'s localStorage `vivy.ui.activeMask` authority, the core `masks.*` locale block, and the `ui/AGENTS.md` "chat/toolbox/面具 always rendered" clause are removed; mask UI now lives in the `vivy-masks-ui` module assembled by the recipe (`mask-omission.test.ts` asserts no core mask code survives in the shell).
- Acceptance recipes `masks-selected`/`masks-omitted`/`masks-backend-only` packed and inspected: assembled UI carries exactly 2/0/0 mask refs; artifact smoke runs the packed binaries.
- `ui/e2e/masks.spec.ts` (5 tests, Playwright, packed binary): catalog/navigation, selection persistence across reload, custom mask create + stale-revision conflict + in-use delete refusal, code-mode independence, next-run selection during an active run — all green.
- Real defects fixed while writing those e2e tests: peer-bind race on `selection.get` after reload (bounded retry in `MaskClient`), `definition/loaded` dispatch leaving Save permanently disabled, error alert wiped by a trailing `catalog/load-success`, and `RpcClientError` dropping `error.data` (contract `data.code` now surfaces).
- Sealed-generation semantics: `maskManagerForAssembly` keeps masks dormant for unsealed dev/test embedders (`go run`, unit tests) and fails closed only when a sealed composition selected `vivy/masks` without proving identity — same treatment as `primaryAdmissionForComposition`; `TestMaskManagerForAssemblyIsDormantWhenUnsealed` pins this.
- Masks are prompt-only by spec (M2: selection cannot change grants/tools/model), and child runs are deliberately unmasked. The ask_user exclusion is orthogonal to masks; mask→tool coupling is future work with a clean seam (mask field on child admission, ceiling = readOnly ∩ maskDeclared ∩ requested).

### R13 — descendant resource accounting (this pass)

- `internal/storage/conformance` CN-34 "durable active-child slot limit" proves on BOTH backends (SQLite + Postgres 16): exactly `MaxActiveChildrenPerRun` (4) children are admitted, a 5th gets `ErrChildConcurrencyLimit` in-transaction, and completing a child reopens the slot.
- `TestRecoveredLedgerReplaysJournalOnceAndCapsResumedRun` proves restart/reauthorization reconciliation: a recovered ledger replays the child's durable journal exactly once (2 model + 1 tool calls from `tool.requested`+`model.completed`), a second recovery reuses the same ledger without recharging, and resumed reservations consume only the remaining shared headroom (`ErrBudgetExceeded` at the cap).
- Non-duplicating usage/cost projection holds by construction: usage derives from the append-only journal (each event journaled once; replay is read-only), sibling runs share one `budgetAccount` (`TestRecoveredSiblingRunsShareOneBudgetAccount`), and `sqliteCheckChildConcurrency`/`postgresCheckChildConcurrency` count child slots in-transaction against durable run rows — a crash between enqueue and child start cannot over-admit.

## Windows CI fixes (this pass)

Three preexisting Windows defects that kept `backend ci` red are fixed; CI is green end-to-end (backend ci on windows-2025 included):

- Embedded mask prompt assets inherit the checkout's line endings, so a CRLF checkout failed "definition body is not normalized" and the packed default generation refused to boot (`TestMaskCatalogNormalizesCRLFAssetBytes` pins normalization before validation).
- `settleApproval` could observe the durable child-approval row before in-memory suspension registration and fail closed with "child approval is not active"; `waitChildApprovalRegistration` now waits boundedly, mirroring `waitShellApprovalReady`.
- The embedded bash interpreter (`mvdan.cc/sh`, every Windows `bash` call) returned `interp.ExitStatus`, which the job registry collapsed to `exit_code -1`; `tools.ExitStatusError` now preserves it (`TestCommandBackendEmbeddedBashReportsExitStatus` covers the `!shell` embedded path cross-platform; `TestNudgeAcceptanceCommandRetry` on Windows was the original witness).

## Still not verified (requires owner)

- Literal `just ci` outside CI: the GitHub `just ci` aggregator job is green, but a local run still needs the PowerShell toolchain.
- Live-model mask semantics eval (whether a mask persona measurably steers a real model) — wiring and transport are proven, persona quality was not in scope.
- Owner E2E and final release review (G4).

## Gate disposition

G0–G3: the previously missing machine-checkable, both-backend and browser evidence — including a real model path, full MASK-4 wiring, and R13 resource accounting — is now produced and recorded. All four GitHub checks are green (backend ci on Windows included). Remaining gap is a local `just ci` run on the PowerShell toolchain plus G4 owner E2E and release review, which remain **BLOCKED** — not delegated.
