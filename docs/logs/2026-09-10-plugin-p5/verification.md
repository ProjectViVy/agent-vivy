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

To be completed as the phase tasks land.

