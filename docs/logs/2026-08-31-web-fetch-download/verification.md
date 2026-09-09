# Verification — web_fetch + download(2026-08-31)

Environment: worktree `../agent-vivy-web-fetch`, branch `feat/web-fetch-download`
(the root tree had uncommitted changes from parallel lanes, isolated according to parallel-worktree-isolation).

## Commands and results

| Command | Result |
|---|---|
| `go get html-to-markdown@v1.6.0 goquery@v1.12.0` + plugin-module `go mod tidy` | OK (go.mod/go.sum written; discord/qq tidied with the root dependency upgrade) |
| `go build ./internal/...` | OK |
| `go vet ./...` | OK(0 findings) |
| `go test ./internal/runtime/ ./internal/tools/ ./internal/config/` | OK (all new cases passed) |
| `go test ./...` | OK (all packages; the first failure was caused by an untidied plugin go.mod, then passed after tidy) |
| `just ci` | **EXIT=0** (fmt-check / vet / test / headless-compile / ui-ci all green; the first UI smoke 503 was caused by the new worktree lacking ui/dist, then passed after `pnpm build`) |

## New tests (deterministic, local httptest, no external network)

- `internal/runtime/web_fetch_test.go` — three-format conversion and noise stripping, JSON pretty-printing,
  bounded non-2xx results, truncation marker, private-target rejection by default (no test seam), credential-query rejection,
  binary rejection, credentialed-redirect rejection, invalid-URL rejection, and table-driven timeout clamp.
- `internal/runtime/download_test.go` — write to disk + SHA256 + parent-directory creation, overwrite +
  precondition stale protection, over-limit rejection without writing the target, credential-query rejection, private-target rejection without a test seam,
  non-2xx errors, proposal fields/overwrite warnings; **restricted sandbox (workspace-write) + new nested-directory
  regression test** (pins the order "validation must occur after MkdirAll").
- `internal/tools/tools_test.go` — web_fetch/download Spec read-only contract,
  argument validation (*ArgError), and the generic proposal fallback for download `ProposalProvider`.
- `internal/config/config_test.go` — the default enabled list includes `web_fetch`/`download`.

## Real-path smoke

1. **Real public fetch** (production constructor, no test seam, real DNS/dial):
   `web_fetch https://example.com` → status=200, format=markdown,
   content `# Example Domain` + `[Learn more](https://iana.org/domains/example)`;
   the same backend rejects `http://127.0.0.1:8787/`
   (`public fetch: target address is private or local`).
2. **Real public download** (production default workspace-write sandbox mode):
   `download https://example.com → smoke/example.html` → 559 bytes written,
   SHA256, content_type passed through; a second proposal for the same target correctly warns
   `overwrites existing file`.
3. **Real startup path**: worktree backend `VIVY_ADDR=127.0.0.1:8799 go run ./cmd/vivy`
   started successfully, `/rpc/bootstrap` returned 200—proving the new `Tools.Enabled` names
   (`web_fetch`/`download`) work through `Registry.Resolve` and the full config → registration → assembly path.
   (The persistent backend on 8787 belongs to a parallel lane and was not touched.)
4. Browser UI: `http://127.0.0.1:3015` was served normally by the existing dev pair (this branch has
   **zero UI changes**; the tools are model-surface only, with no new UI surface).

## Known limitations (recorded plainly)

- **Model-driven end-to-end session smoke was not run**: the local shell has no provider API key
  (config stores only the env_key name; the secret is not persisted), so the model could not call both tools in a real session;
  it was replaced with a backend-level smoke test of "real public network + production sandbox + real startup," covering the execution path
  (dialer → HTTP → conversion/write-to-disk). Only model-side tool selection remains uncovered.
- The temporary harness and config used for smoke were deleted and were not checked in.
