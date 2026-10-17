# Verification

Host: Windows, Go 1.26.4, Node 24.19.0, pnpm 10.11.0, just 1.46.0,
Playwright Chromium 1243 (the tool pin expects 1234; the smoke selected the
installed build explicitly via `executablePath`).

## Unit and static gates

| Check | Command | Result |
| --- | --- | --- |
| Types | `pnpm typecheck` (ui/) | clean |
| Copy audit | `node scripts/check-i18n-completeness.js` | PASS: en=1579 keys / 163 placeholders, zh=1579 keys / 163 placeholders, runtime copy audit clean |
| Cross-face i18n | `node scripts/check-i18n-cross-face.js` | PASS: 14 shared semantic units, Web and TUI en/zh projections and arguments conform |
| New card unit tests | `pnpm exec vitest run src/components/approvals/ApprovalQuickCard.test.tsx` | 9/9 |
| Cross-face render tests | `pnpm exec vitest run src/i18n/surfaces.test.tsx` | 8/8 |
| Full UI suite (inside `just ci`) | `pnpm test` | 93 files / 701 tests passed |
| Production UI build | `pnpm build` | `✓ built` with a workspace-local `TEMP` |

`ApprovalQuickCard.test.tsx` covers `pickPendingReview` precedence and fallback,
`clampOffset` clamping, command + preview rendering, Approve/Deny dispatch,
busy disabling with `Processing…`, a rejected response keeping the card usable
and surfacing the store error, question routing to the center with no blind
decision, the remaining count, close, and the empty state.

## `just ci`

The lane was verified in its own worktree (`agent-vivy-approval-quick-card`,
branch `feat/approval-quick-card`); the root checkout is held by the issue-58
lane. Every recipe ran there, with two environment tunings that change the
command, not the result (both explained below).

| Leg | Command | Result |
| --- | --- | --- |
| bootstrap | `just ensure-laputa bootstrap-test` | green |
| format | `just fmt-check` | green |
| UI | `just ui-ci` (install, typecheck, `pnpm test`, `pnpm build`, i18n completeness + cross-face) | green: 93 files / 701 tests, `✓ built` |
| vet | `just vet` | green |
| kernel + SDK tests | `go test -p 1 -timeout 120m -skip '^TestMemoryLoop' ./...` | exit 0; 84 `ok`, 0 `FAIL`, including `agent-vivy/ui`, `sdk/internal` (2342 s) and `sdk/internal/conformance` |
| memory-loop second pass | `./scripts/test-memory-loop.ps1` with `GOFLAGS=-p=1` | exit 0 |
| recall | `just diva-recall-test` | exit 0 |
| headless | `just headless-compile` | exit 0 |
| plugins | `just plugin-ci` | exit 0 |

Two host conditions shaped how the gate was run:

1. **Temp roots.** The packed-UI suite and the production Vite build need
   opposite temp placements on this host: Vite's esbuild teardown fails with
   `[vite:esbuild-transpile] remove …: Access is denied.` unless `TEMP` is
   inside the session workspace, while `TestPackAndInspectSealUIAssemblyIdentity`
   (`sdk/internal/frontend_v1_test.go:273`) fails at
   `symlink …\ui\node_modules …\vivy-ui-build-<n>\node_modules: A required
   privilege is not held by the client.` unless `TMP` is outside it. Splitting
   the roots satisfies both: `TEMP=<root-checkout>\.workspace\tmp\ci-temp`,
   `TMP=%LOCALAPPDATA%\VivyCi\tmp`. With that split `sdk/internal` and
   `sdk/internal/assembly` pass, so no test of this change is blocked.
2. **Concurrency.** Two other lanes ran their own full gates on this 15 GB host
   during the same window. `just ci`'s own `test` recipe then hit two resource
   walls, not code failures: `sdk/internal` was killed by
   `panic: test timed out after 35m0s` (the recipe's bound), and the
   memory-loop pass died with `fatal error: runtime: cannot allocate memory`
   (`FAIL … [build failed]`). Re-running the *identical* package set with
   `-timeout 120m`, and the *identical* memory-loop script with `GOFLAGS=-p=1`
   (serialized builds), both pass; no test was skipped or deselected.

An earlier root-checkout `just ci` failed even earlier for a third reason:
`sdk/internal` reported `sdk: source changed during build: source hash mismatch
for …\morediva\internal`, with the digest drifting on every attempt, because the
issue-58 lane was writing `internal/runtime` and `internal/tools` while the
packed-UI suite hashed the tree. A shared root tree cannot host two write lanes;
that is why this lane is verified in its own worktree.

The temp-root behaviour also means the `ERROR_PRIVILEGE_NOT_HELD` /
"every symlink variant fails" reading recorded in
`docs/logs/2026-10-15-issue-58/verification.md` does not reproduce here once
`TMP` is outside the session workspace: Go's `os.Symlink` into the isolated
build boundary succeeds. That row's host conclusion is worth re-measuring on
the current harness rather than being treated as closed; nothing in this change
depends on it.

## Real-path browser smoke (split Vite dev pair, no provider key)

Harness (temporary, under `.workspace/tmp`, removed afterwards): an isolated
backend with a disposable `VIVY_USER_HOME` and `VIVY_DEFAULT_LOCALE=zh`, the
frozen-env provider (`VIVY_PROVIDER=deepseek`,
`VIVY_API_BASE=http://127.0.0.1:9911/v1`, `VIVY_MODEL=mock-fast`), a local mock
model that answers a fresh user turn with a real `write_file` tool call
(`{"path":"quick-approval-smoke.txt",…}`) so the smart preset gates it and the
run really suspends on a real review, and Vite served on 127.0.0.1:3016 against
that backend. The pre-existing split pair on :3015/:8787 was left untouched.
The session was created over `session/create` and bound to a sandbox workspace
inside the smoke root.

14/14 checks passed:

```text
PASS composer defaults to the smart preset
PASS card shows the pending command                (write_file / 执行的指令)
PASS card preview carries the gated target         (quick-approval-smoke.txt)
PASS card offers quick approve and deny
PASS card floats above the composer, not over the input
     (card bottom 625 / composer top 636 / textarea top 679)
PASS opening the card shifts no layout
PASS card is draggable and stays inside the viewport   (815,387 → 555,237)
PASS approving from the card clears the pending badge
PASS card falls back to its empty state after the decision
PASS run resumed and completed after the quick approval     (transcript: DONE)
PASS approval center page shows the decision made in the card   (/approvals: 已批准)
PASS deciding in the approval center clears the pending badge
PASS card reflects the decision made in the approval center     (当前没有待处理事项)
PASS no uncaught page error
```

Turn 1 approved from the card and confirmed on `/approvals`; turn 2 denied in
the in-chat approval-center Sheet and confirmed back in the still-open card,
which is the two-way sync the request asked for. Drag, viewport clamping, the
zero-layout-shift placement and the empty state were exercised in the same run.

## Not covered

- No dedicated e2e spec was added to `ui/e2e`: the disposable-key harness above
  is the real-path evidence. `ui/playwright.config.ts` runs the embedded UI with
  a fixed tool preset (`echo_info`/`write_note`/`ask_user`) and no gated write
  tool, so a permanent spec would first need a mock-provider fixture in the
  repository's Playwright setup. That fixture is not part of this change; no
  board row was added because `docs/TODO.md` belongs to the issue-58 lane that
  holds the root checkout, and this lane did not edit that file.
