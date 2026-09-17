# Verification — Workflow WF-1 slice

All commands run from the `feat/workflow-wf1` worktree
(`.worktrees/workflow-wf1`) on Windows. Postgres parity stays
environment-gated (`VIVY_POSTGRES_TEST_DSN` was not set, so the gated
CN-28 parity case skipped, by design).

## Focused Go tests

| Command | Result |
|---|---|
| `go build ./...` | pass |
| `go test ./internal/workflow/... ./internal/domain/... ./internal/config/... ./internal/modules/workflow/... ./internal/workflowhost/... ./internal/rpc/... ./internal/tools/... ./internal/storage/...` | pass (first full run surfaced only the shared conformance gate, fixed below) |
| `go test ./internal/app/ -run "TestNewWorkflowOperations"` | pass — new composition tests: host over a single backend, capability snapshot flowing into validation, run refusing `-32011` without orphan rows |
| `go test ./internal/workflowhost/` | pass — new: constructor guards, unavailable-before-mutation, invalid-input preflight, previews persist nothing, capability hash stability |
| `go test ./internal/rpc/ -run "TestControlWorkflow"` | pass — new: nil-seam `-32601` for the whole family, define/list/get round-trip, structured diagnostics, `-32004` not-found, `-32011` unavailable with no run row, capability advertisement |
| `go test ./internal/tools/` | pass — new: strict schemas and readonly flags, bounded proposals without side effects, invokable paths |
| `go test ./internal/runtime/ -run "TestPolicyProfilesGateWorkflowToolsByEffect"` | pass — `plan`/`read_only` deny `workflow_define`/`workflow_run`, readonly members allowed |
| `go test ./internal/app/ ./sdk/internal/assembly/ ./sdk/internal/conformance/` (final pre-gate) | pass |
| `go test ./sdk/internal/ -run TestSupportState -count=1` | pass (2.75 s) — after the planned-Port gate fix below |

## Assembly regeneration

- `go generate ./internal/generated/assembly` — regenerated
  `zz_default.go`; the diff adds the `vivy/workflow` module, the six
  generated tool identities in the manifest, and the module's
  construction in `Start`. No generated file was hand-edited.
- `sdk/internal/testdata/default-generation.expected.json` updated with
  `vivy/workflow` and the six tool IDs (the baseline inventory fixture
  that `TestDefaultGenerationBaselineInventory` guards).

## Conformance evidence re-pin

Adding source under `internal/` changes the shared first-party source
digest, so `sdk/internal/conformance/reproduction_test.go` and
`sdk/internal/assembly/conformance_results.json` were re-pinned:

- `838dda65…` (pre-WF-1) → `8776ade0a3a4596b51b2e681251e8230304aadca2d8c82478f5dc543a2ff8eec`
- `go test ./sdk/internal/conformance/ -run "TestCheckedInProviderConformanceMatchesExecutedSuites"` — pass (~143 s; producer gate recomputes every digest and executes the owning provider/host tests).

The last change in this lane (`sdk/internal/support_state_test.go`, below)
is outside `internal/`, so the digest stayed at `8776ade0…` and no further
re-pin was required; the green conformance producer gate in the final
`just ci` run confirms it.

## UI

- `cd ui && pnpm exec vitest run src/lib/api.test.ts` — pass (workflow
  method tracking + snake_case wire mapping).
- `sdk/ui/src/module.ts` extended with the `FaceWorkflow*` DTOs and six
  `FaceClientAPI` operations (the ui-sdk face-compat type gate requires
  the two surfaces to move together); `sdk/ui/src/module.test.ts` fixture
  stubs them as unavailable.
- `cd ui && pnpm exec tsc --noEmit` — pass.
- `cd ui && pnpm exec vitest run` — pass (37 files, 334 tests).

## Formatting

- `gofmt -l internal/ sdk/` — clean after formatting
  `internal/modules/defaults/catalog_test.go` and
  `internal/workflowhost/host_test.go`.
- `git diff --check` — clean (CRLF normalization warnings only, no
  whitespace errors).

## Product gate

- `just ci` — **green** on the final tree (exit 0):
  `fmt-check → ui-ci → vet → test → headless-compile → plugin-ci`.

  Final run detail:

  - `fmt-check` pass; `ui-ci` pass — `pnpm install --frozen-lockfile`,
    `tsc --noEmit`, vitest 37 files / 334 tests, `vite build`, i18n
    completeness (1410 en / 1410 zh keys), cross-face 8/8 tests and 13
    shared semantic units.
  - `vet ./...` pass; `go test -timeout 20m ./...` pass. Long packages:
    `internal/app` 92 s, `internal/rpc` 188 s, `internal/runtime` 314 s,
    `sdk/internal` 665 s, `sdk/internal/conformance` 317 s. No known
    TFLAKE-* flake was hit.
  - `headless-compile` pass; `plugin-ci` pass (10 independent modules
    under `plugins/` and `faces/`).

  Three earlier runs in this lane each surfaced a real finding, fixed
  before the final green run:

  1. run 1 failed `fmt-check` (`internal/modules/defaults/catalog_test.go`,
     `internal/workflowhost/host_test.go` not gofmt-formatted) — fixed with
     `gofmt -w`, digest re-pinned, focused tests re-run green.
  2. run 2 failed `ui-core` typecheck: `ui-sdk-face-compat.test.ts` requires
     the `api.ts` operation surface to match `FaceClientAPI` — fixed by
     adding the `FaceWorkflow*` DTOs and six operations to
     `sdk/ui/src/module.ts` and the fixture stubs in `sdk/ui/src/module.test.ts`;
     `tsc --noEmit` and the full UI vitest suite then passed.
  3. run 3 failed the `test` recipe (`go test -timeout 20m ./...`) in
     `sdk/internal`:
     `TestSupportStateRequiresEveryArtifactForEveryPublicPort` failed with
     `release state for std/workflow-node@v1 = SPECIFIED, want SUPPORTED`.
     That gate asserted every cataloged public Port evaluates `SUPPORTED`,
     while WF-1 intentionally catalogs `std/workflow-node@v1` as
     `SPECIFIED`/`PLANNED` — its seven-artifact evidence (real node
     Provider, Failure Model, Conformance Suite, Inspect Projection)
     arrives with WF-2 (`docs/architecture/VIVY-WORKFLOW.md` §9).

     Fix: `sdk/internal/support_state_test.go` now carries a commented
     `plannedPublicPorts` exemption that cites §9. A planned Port must
     evaluate something other than `SUPPORTED` (the test still fails if it
     ever claims support) and is skipped by the evidence-removal mutation
     loop. No product code, catalog entry, or evidence was touched: the
     Port stays `SPECIFIED`/`PLANNED`, and the test-only change is
     confined to `sdk/`.

- Postgres parity: skipped without `VIVY_POSTGRES_TEST_DSN` (existing
  environment-gated case, unchanged policy).

## Real-path smoke

Executed on the split pair from this worktree. Ports were confirmed free
before starting and both processes were stopped afterwards (confirmed
free again).

1. `just run` → control plane `127.0.0.1:8787`; `cd ui && pnpm dev` →
   Vite `127.0.0.1:3015`.
2. `node %TEMP%/wf1-smoke.mjs` (Node 24 built-in `fetch` + `WebSocket`):
   `GET http://127.0.0.1:3015/rpc/bootstrap`, then the real
   `ws://127.0.0.1:3015/rpc?token=…` upgrade through the Vite proxy, then
   `initialize`, `workflow/define`, `workflow/get`, `workflow/validate`,
   `workflow/run`, `workflow/runs`.

Result (exit 0):

```text
PASS  bootstrap handshake — protocol=vivy.rpc.v1 path=/rpc
PASS  websocket upgrade through Vite proxy — ws://127.0.0.1:3015/rpc
PASS  initialize advertises workflow capability tokens — workflow.definitions=true workflow.run=true
PASS  workflow/define returns id + rev 1 + hash — id=wf1-smoke rev=1 hash=3c63ed56cadc…
PASS  workflow/get round-trips the canonical definition — rev=1 canonical-round-trip=true
PASS  workflow/validate returns structured diagnostics for an invalid definition — valid=false diagnostics=topology:outputs[0].template
PASS  workflow/run refuses with -32011 in a Generation without a real executor — code=-32011 message=workflow execution is not configured in this generation
PASS  workflow/runs shows no orphan run row after the refusal — runs=[]

SMOKE-PASS (8 checks)
```

The definition is io-only (`${{ inputs.name }}` → output binding), so no
model profile or credential is involved; the refusal path (`-32011`) is
the expected WF-1 behavior until the WF-2 real executor lands.
