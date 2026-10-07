# H1 verification

- `go run ./sdk verify plugins/coding/lsp` → `ok vivy/lsp`
- `go run ./sdk verify plugins/infra/llm` → `ok vivy/local-llm`
- Plugin tests: `agent-vivy/plugins/infra/llm` ok, `agent-vivy/plugins/coding/lsp` ok
- `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites` → ok (63.9s); expected-vs-actual diff was exactly the lsp pin set, resolved
- `go build ./...` clean; zero stale `plugins/lsp` / `plugins/coding/local-llm` references
- `go run ./sdk pack --recipe recipes/vivy-code.vivy.yml --output /tmp/vc-pack` + `inspect-artifact`:
  - `vivy/local-llm` source `repo:plugins/infra/llm` digest `9a8e049a…` — edges: consumes action-host+status-host, provides `local_llm.manage` + `vivy.local-llm.status`, net.client loopback-only (ports 8000/8080/11434/1234)
  - `vivy/lsp` source `repo:plugins/coding/lsp` digest `6220b77f…` — consumes `core/tool-host@v1`, provides `std/tool-world@v1` `vivy.lsp`, grants fs.read/fs.write + proc.spawn allowlist
- `generate-default -output internal/generated/assembly/zz_default.go` → byte-identical (vivy.exe species unaffected)
- `node scripts/check-i18n-cross-face.js` → PASS (13 shared units)
- **`just ci` → full green**: bootstrap-test 6/6, fmt-check, ui-ci (typecheck + 589 vitest + vite build), i18n completeness + cross-face, vet, `go test -timeout 35m ./...` all packages incl. sdk/internal 416s + conformance 68s, headless-compile, plugin-ci — all 18 module dirs incl. `plugins/coding/lsp`, `plugins/coding/session-tree`, `plugins/infra/llm`
