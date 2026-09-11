# PLG-P5 Verification

## Baseline

| Command | Result |
|---|---|
| `pnpm install --frozen-lockfile && pnpm typecheck && pnpm test && pnpm build` (`ui/`) | Passed: 31 files, 274 tests, TypeScript, and production build. Existing chunk-size warning only. |
| `just ci` | Could not start locally: this Linux runner has no `just`, and the repository `justfile` requires PowerShell. |
| Direct Go equivalent on clean `main` | Exposed a stale import-firewall path (`sdk/plugin`, removed by P1/P2) and linked-worktree VCS stamping failures in nested build tests. The latter are avoided locally with `GOFLAGS=-buildvcs=false`; normal GitHub Actions checkouts are unaffected. |

## Pinned capability audit

Inspected the local module-cache sources for:

- EinoExt OpenAI `v0.1.13`: `NewChatModel`, `ChatModelConfig`, immutable
  `WithTools`, `Generate`, `Stream`, and provider-specific options.
- EinoExt Claude `v0.1.25`: `NewChatModel`, `Config`, immutable `WithTools`,
  `Generate`, `Stream`, `WithThinking`, and `WithThinkingConfig`.
- OAuth/auth surfaces in both pinned component trees.

Decisions and exact boundaries are recorded in
`docs/research/plugin-v1-provider-eino-matrix.md`.

## Implementation gates

| Task | Command | Result |
|---|---|---|
| Declarative Profiles | `go test ./sdk/port/providerprofile ./internal/modelhost` | Passed. Pure-data validation, duplicate/unsupported/deferred cases, and defensive copies covered. |
| ModelHost routing | `go test ./internal/modelhost ./internal/provider ./internal/runtime -run Model` | Passed. Host-required routing and raw gateway model IDs covered. |
| Default Generation | focused defaults/config/app/assembly tests | Passed. Generated manifest and runtime profile inventory agree. |
| Capability projection | focused ModelHost/RPC/provider/app tests plus `provider-catalog` and `custom-providers` UI tests | Passed. Five states project without Secrets; deferred/unavailable selection is rejected in UI helpers and RPC. |

The local Go commands use `GOFLAGS=-buildvcs=false` because this linked
worktree cannot be VCS-stamped by nested build tests. Task 6 records the full
gate evidence and the repository-level limitation separately.

## Phase exit gates

| Command | Result |
|---|---|
| `go test ./sdk/internal -run '^TestV1PackAndInspectProveRecipeRemoval$' -count=1` | Passed. Default and minimal Generations packed and their binary-bound manifests inspected; physical removal assertion passed. |
| `pnpm install --frozen-lockfile`, `pnpm typecheck`, `pnpm test`, `pnpm build` | Passed: 31 files, 280 tests, TypeScript, production build. Existing chunk-size warning only. |
| i18n completeness and cross-face scripts | Passed: 1,388 keys / 138 placeholders per locale and 13 shared semantic units. |
| `go vet ./...` | Passed. |
| `go test ./... -count=1` | Passed. |
| headless compile gate | Passed for `cmd/vivy`, `cmd/vivy-code`, and `ui`. |
| independent `faces/*` and `plugins/*` module vet/tests | Passed for headless, tui, dingtalk, discord, feishu, lsp, qq, and telegram. |
| fake/local Provider conformance smoke | Passed. Local HTTP adapters cover failure, timeout, cancellation, stream startup error, raw model ID, and Secret redaction without live credentials. |
| `just ci` | Runner limitation: `just` is not installed (`exit 127`). The commands represented by the recipe were executed directly above; its PowerShell recipes are not runnable on this Linux host. |
