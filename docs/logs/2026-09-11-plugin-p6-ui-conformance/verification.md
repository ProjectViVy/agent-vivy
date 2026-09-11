# PLG-P6 UI conformance verification

## Focused checks

```text
cd ui && pnpm typecheck
cd ui && pnpm exec vitest run src/plugins/conformance.test.tsx
cd ui && pnpm exec vitest run src/plugins/presentation-host.test.tsx src/plugins/conformance.test.tsx
PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOFLAGS=-buildvcs=false go test ./sdk/internal/assembly -run 'Task7|GenerateNonEmpty' -count=1
PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOFLAGS=-buildvcs=false go test ./sdk/internal/assembly -count=1
git diff --check
```

All listed focused checks passed. The Go fixture matrix uses temporary
in-process sources and a checked-in local TypeScript compiler; it performs no
network or runtime source discovery. The deliberate malformed fixture failed
typechecking and left no `generation.json` publish marker.

## Consolidated verification

The final P6 pass also succeeded for:

```text
GOFLAGS=-buildvcs=false go test ./...
GOFLAGS=-buildvcs=false go vet ./...
GOFLAGS=-buildvcs=false go build ./...
GOFLAGS=-buildvcs=false go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui
cd ui && pnpm test -- --no-file-parallelism
cd ui && pnpm typecheck && pnpm build
cd sdk/ui && pnpm test && pnpm typecheck
go test -race ./sdk/port/controlaction ./internal/actionhost ./internal/rpc ./sdk/internal/assembly
node scripts/check-i18n-completeness.js
node --test scripts/check-i18n-cross-face.test.js
```

Every independent `plugins/*` and `faces/*` module also passed `go vet ./...`
and `go test ./...`. A loopback split smoke served the Vite index, backend
`/healthz`, and proxied `/rpc/bootstrap` successfully.

## Review correction pass

The final read-only review identified four boundary issues; each is now
covered by code and regression tests:

- Pack overlays the selected Vite `dist` into the Go embed tree, and the
  packed executable test checks selected UI markers in the binary.
- UI source and dependency-lock hashes are derived from the sealed source
  catalog; package manifests are required to use exact versions that match the
  installed compiler toolchain, and emitted asset hashes are authoritative.
- WebSocket action bridges bind SessionID/RunID only from server-owned Session
  and durable Run records; browser claims cannot acquire or replace the pair.
- Audit invocation IDs use a monotonic host sequence and are asserted unique
  and stable across concurrent invocations.

The focused race gate for `controlaction`, ActionHost, RPC, and UI Assembly
passes. A broader `go test -race ./internal/app` remains outside the P6 gate
because an existing Claude/Eino streaming test reports a dependency race; the
ordinary repository-wide test, vet, and build gates pass.

## Environment-limited checks

- `just ci`: cannot execute because this runner has neither `just` nor
  PowerShell; the equivalent recipes above were run directly with the pinned
  Go toolchain.
- Playwright smoke: cannot launch because no Chromium executable is installed;
  `pnpm exec playwright install chromium` timed out against
  `cdn.playwright.dev`. The split-pair spec remains opt-in through
  `VIVY_FULL_UI_URL`.
- The final read-only review corrections are recorded above; P9 release
  conformance remains separate.
