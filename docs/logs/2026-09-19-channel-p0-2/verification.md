# CH-P0-2 verification

## Environment

- Linux worktree on `feat/channel-p0-1-contracts`.
- Go `1.26.4`, Node `v24.19.0`, pnpm `11.19.0`.
- `just` and PowerShell are unavailable. The repository `justfile` fixes its
  shell to `powershell.exe`, so literal `just ci` was not claimed. Its Linux
  equivalents were run individually.
- Go commands used `GOFLAGS=-buildvcs=false` because this isolated worktree
  cannot obtain VCS stamping status.
- The repository's existing `internal/workflow` carve-out was retained for the
  full Go vet/test gate; no additional package was excluded.

## TDD and migration evidence

| Boundary | RED evidence | GREEN evidence |
|---|---|---|
| canonical owner | missing Factory/Owned implementation | Module construction, inert `WithoutEars`, lifecycle, binding and Grant tests pass |
| management ownership | handlers existed only in `internal/rpc/control.go` | Module-owned inspect/get/update compatibility and failure tests pass |
| generic RPC attachment | core dispatcher had no contribution dispatch | validated dispatch, capability, collision, Peer/context, and AST vocabulary tests pass |
| generated selection | Assembly had no Channel factory field | default and fixture generation emit one canonical factory and omit the placeholder lifecycle owner |
| app composition | fake factory was neither constructed nor started | construction, normal start/ready, contribution dispatch, close, and failed-start rollback tests pass |
| SDK conformance owner | suite still referenced deleted `app.BindChannels` | suite uses canonical Module binding and passes |

## Commands and results

| Command | Result |
|---|---|
| focused Channel/app/RPC/Assembly suite | PASS |
| `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | PASS |
| `pnpm --dir ui typecheck` | PASS |
| `pnpm --dir ui test` | PASS, 38 files / 334 tests |
| `pnpm --dir ui build` | PASS; existing large-chunk warning only |
| `node scripts/check-i18n-completeness.js` | PASS, 1409 keys and 138 placeholders per locale |
| `node --test scripts/check-i18n-cross-face.test.js` | PASS, 8 tests |
| `node scripts/check-i18n-cross-face.js` | PASS, 13 shared semantic units |
| full repository `go vet` excluding only `internal/workflow` | PASS |
| full repository `go test -timeout 20m` excluding only `internal/workflow` | PASS; `sdk/internal` 297.958s, `sdk/internal/conformance` 75.734s |
| all independent `plugins/*` and `faces/*`: `go vet ./...` and `go test ./...` | PASS, 10 modules |
| forbidden app/RPC implementation import scan | PASS; empty output |
| Recipe/UI scope-fence diff from CH-P0-1 | PASS; empty output |

The focused command was:

```text
go test ./internal/channelcontract ./internal/rpccontract \
  ./internal/modules/channel ./internal/channelhost ./internal/rpc \
  ./internal/app/settings ./internal/app ./internal/moduleport \
  ./internal/modules/... ./sdk/internal/assembly -count=1
```

## Source-bound conformance refresh

The producer suite executed and rejected only the stale `internal/` source
digest, as expected after this slice changed files below that root. The
repository command produced:

```text
go run ./sdk/internal/cmd/source-hash internal ""
3e38b4e959697c19c50cd2cf2b8e9cd9d8056604c85cf3ff6609c3c83dba45ee
```

Only the five internal-rooted entries in
`sdk/internal/assembly/conformance_results.json` were mechanically changed
from `5b6912e3...`; external Provider evidence was untouched. The producer then
passed:

```text
ok agent-vivy/sdk/internal/conformance 67.607s
```

The final full repository test repeated that proof successfully.

## Scope fence

The three planned reduced Recipes do not exist. There is no diff from CH-P0-1
under `recipes`, `ui/src/components/settings`, or
`ui/src/generated/assembly.ts`. Neither `internal/app` nor `internal/rpc`
imports `internal/channelhost` or `internal/modules/channel`.

The generated default backend does import the canonical implementation through
the transitional defaults bridge. That is intentional CH-P0-2 behavior and is
not physical omission evidence.
