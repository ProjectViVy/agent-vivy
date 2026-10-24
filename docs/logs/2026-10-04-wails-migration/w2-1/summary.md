# W2 iteration 1 — Sealed Go-host packaging

Story: `agent-diva/docs/plans/diva-next/wails/W2.md` (DN-W3-P1).
Spec: `docs/plans/diva-next/backend-separation-contracts.md` §W3-4.

## Delivered

New pack target `go-host` (`sdk pack --target go-host`) that seals an
external Go desktop host executable with the VIVY Generation compiler:

- `sdk/internal/go_host.go` — new implementation (~1000 lines):
  - Strict `diva.go-host-inputs/v1` lock decoder (`DisallowUnknownFields`).
    `host.commit`/`host.treeSHA256` are optional: the tracked lock must not
    pin its own DIVA commit; pack resolves them from the live checkout.
  - Git-worktree staging: `git ls-files` snapshot of vivy/host/deps (no
    `.git`, no ignored outputs, symlinks preserved, gitlinks recorded).
  - `hashSourceTree`: sha256 over `relpath\x00perm\x00payload\x00` per
    tracked file — byte-identical to the DIVA wrapper's Python port
    (verified on a fixture including a symlink).
  - Consumer modfile generator: host `go.mod` + `require agent-vivy v0.0.0`
    + every VIVY local replace rewritten to staged paths; host replaces
    must stay inside the staged host; external replaces must land in the
    declared laputa closure, else "escapes the declared source closure".
    Replacements are staged-relative (`../vivy`, `../deps/laputa/...`) so
    the sealed modfile is identical from any staging location.
  - `go mod tidy` + `go mod download` under `-modfile` resolves the full
    closure; build runs `go build -modfile consumer.mod -mod=readonly
    -overlay overlay.json -tags vivy_headless -o <binary> ./<pkg>` — VIVY's
    web face is never compiled.
  - Generated `zz_default.go` + framed manifest overlays land at the exact
    staged VIVY paths the dependency compiler reads.
  - Output: binary, `generation.json`, `zz_assembly.go`, `ui-assembly.ts`,
    `frontend/` (bound host-asset bytes, never rebuilt), `consumer.mod`,
    `consumer.sum`, `build-report.json` (`vivy.go-host-report/v1`),
    `checksums.sha256` (every file except report/itself).
- `sdk/internal/assembly/manifest.go` — optional typed `hostBuild`
  (`vivy.go-host/v1`) on `GenerationManifest`/`SealInputs` with full
  validation wired into `SealManifest` and `InspectManifest`; participates
  in the Generation ID. Legacy artifacts without `hostBuild` still decode
  and inspect.
- `sdk/internal/frontend_v1.go` — `packOptions` gained `HostDir`,
  `HostPackage`, `HostAssets`, `HostLock`, `GoHost`; parser accepts
  `--target go-host` + the four host flags and rejects them on
  executable/shared; `Pack()` normalizes an empty target to executable;
  empty-dist UI path now serves shared and go-host; go-host branch routes
  to `packGoHostArtifact`; `InspectArtifact` routes host-bearing manifests
  to `inspectGoHostArtifact`.
- `sdk/internal/testdata/go-host/` — checked-in external host fixture
  (`module example.com/vivy-go-host`, `cmd/gohost` imports `sdk/host/v1`,
  `agent-diva-gui/dist` asset tree). Its go.mod only pins
  `replace agent-vivy`, so a plain consumer build demonstrably fails on
  `dashimaki/{laputa,garden}`, `agent-vivy/bml`, `agent-vivy/plugins/*`
  (replace non-inheritance, Task 1 evidence).
- `agent-diva` (branch `feat/wails-go-host`): `scripts/build-desktop.py`
  with `--mode build|test|repin` + `--development`; `just desktop-build`,
  `desktop-build-dev`, `go-test`, `desktop-repin` recipes. Build mode runs
  the frozen `pnpm install --frozen-lockfile` + `pnpm build` once and
  consumes those bytes; test mode stages the same closure and runs
  `go build`/`go test -race` under the resolved `consumer.mod` with
  `vivy_headless`.

## Key findings

- The embedded manifest var is linker dead code unless `host.Open`'s call
  chain is reachable from the host's live code — a host that never calls
  `Open` produces a binary without the sealed manifest. Fixture keeps the
  chain live; real DIVA hosts call `Open` by definition.
- Reproducibility: two unrelated staging paths produce byte-identical
  `generation.json` and `zz_assembly.go` (same Generation ID). Raw binary
  bytes are not promised bit-identical (build path/toolchain metadata);
  identity lives in `build-report.json` + `checksums.sha256` instead.
- `go mod tidy` on the consumer modfile is required: pruned module graph
  rejects readonly builds when transitive requires are unlisted.
