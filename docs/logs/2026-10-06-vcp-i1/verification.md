# I1 — verification

Environment: Go 1.26.4 (`~/goroot`), node v24.19.0 + pnpm (`~/.nvm`), sibling
clone `../laputa` for the go.mod `replace` directives.

- `go run ./sdk/internal/cmd/generate-default --repo . --output /tmp/zz_default.go`
  → `diff /tmp/zz_default.go internal/generated/assembly/zz_default.go`
  **IDENTICAL** — local-llm is not in `recipes/default.vivy.yml`, so the
  default assembly is unchanged.
- `go run ./sdk pack --recipe recipes/vivy-code.vivy.yml --output /tmp/vivy-code-pack --source plugins/coding/local-llm`
  → green after two fix-ups (below); artifact at `/tmp/vivy-code-pack`,
  `generationId bcfbbfc7682b316a7ef69fb5b2e447a76198ed49e900395af458c49ce1ee03a1`.
  - First failure: `missing provider for core/action-host@v1` /
    `core/status-host@v1` → recipe gained `vivy/status-host` +
    `vivy/action-host`.
  - Second failure: `grant net.client is not allowed by selected Ports for
    vivy/local-llm` → `sdk/port/catalog.go` `std/control-action@v1`
    `AllowedGrants` gained `net.client` (kept after `rpc.client`).
  - Third/fourth failures were environment: `ui/node_modules` and `ui/dist`
    absent → `pnpm install --frozen-lockfile && pnpm build` in `ui/`.
- `go run ./sdk inspect-artifact /tmp/vivy-code-pack` → `vivy/local-llm`
  provides `std/control-action@v1` `local_llm.manage` and
  `std/status-source@v1` `vivy.local-llm.status`; port edges wire it to
  `vivy/action-host` and `vivy/status-host`; `effectiveGrants` records
  `net.client` with the loopback host/scheme/port constraints.
- `go run ./sdk verify plugins/coding/local-llm` → `ok vivy/local-llm`.
- `cd plugins/coding/local-llm && go test ./... -count=1` → ok.
- `go run ./sdk/internal/cmd/source-hash internal ""` →
  `a6e7d5b2adb379e34c1a6f97b152833492958f73e38c7b2553d08173482918e4`, equal to
  the value stored in `conformance_results.json` → **no re-pin required**
  (a251ada changed `sdk/internal/`, not the hashed `internal/` tree).
- `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1`
  → ok (55.961s). First run failed on `"node": executable file not found`
  (env issue, not evidence) — re-ran with node on PATH.
- `go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' -count=1`
  → ok (0.026s).
- `go test ./sdk/internal -run 'Test(GenerationFailureMatrixExecutesEveryCase|MinimalArtifactPhysicallyOmitsOptionalModules)' -count=1`
  → ok (45.556s).
- `go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' -count=1`
  → ok (0.021s).
- `go test ./internal/toolhost -run 'TestMiddleware(TimeoutFailsClosed|PanicAndInvalidDecisionFailClosed)' -count=1`
  → ok (0.006s).
- `go test ./sdk/port/... -count=1` → all packages ok (catalog diff sanity).

Not run (deferred to a later story): `just ci`. Live `discover`/`pull`
against real llama.cpp/Ollama/LM Studio/vLLM endpoints untested — no local
servers on this VM; service layer covers stub-server paths via injected
http hooks.
