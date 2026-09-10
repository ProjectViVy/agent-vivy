# Verification — Plugin platform P1/P2

Environment: Linux, go1.26.4, node/pnpm available. `just` and PowerShell are
not installed in this workspace, so every `just ci` constituent was executed
directly with the same commands.

- Generated default Assembly reproducibility: consecutive
  `go run ./sdk/internal/cmd/generate-default` runs were byte-identical.
- v1 source verification: all five Channels, two Faces, hello-fs, and LSP
  accepted by `go run ./sdk verify`; modified source and forbidden direct
  capability calls were rejected.
- Recipe packing: default, minimal, headless, and vivy-code artifacts all built
  and passed `inspect-artifact`. Automated tests assert executable/sidecar
  identity, swapped-manifest rejection, failure atomicity, capability states,
  and a smaller minimal binary whose symbol table contains no Dingtalk package.
- External source packing: a temporary standalone Go Module passed source
  verification, was linked through `--source`, and appeared in the generated
  binder and inspected executable.
- Runtime Assembly integration: generated lifecycle success/rollback tests,
  authoritative protected-Tool removal, real ToolWorld invocation, governed
  file write/version recording, Channel partial-start cleanup, Face selection,
  and LSP process reaper shutdown passed.
- Authority regression tests: runtime Provider identity drift, cross-protected
  Tool invocation, ToolWorld root/command constraint escapes, Channel
  secret/network constraint escapes, and nonexistent evidence paths are
  rejected. LSP auxiliary consumers use the generated instance, and artifact
  inspection succeeds even when the target executable bit is removed.
- Default inactive-network test: `TestDefaultGenerationLeavesUnconfiguredNetworkInactive`
  passed with zero started Channels.
- Source removal audit: `TestLegacyPluginSurfaceIsPhysicallyRemoved` passed.
- UI gate: frozen install, typecheck, 274 tests, and production build passed.
- I18N gate: 1,388 keys / 138 placeholders in both locales; 13 cross-face
  semantic units and all fixture-corpus tests passed.
- Go gates: `go vet ./...`, uncached `go test ./...`, headless compile, and
  standalone vet/test for dingtalk, discord, feishu, lsp, qq, telegram,
  headless, and tui passed.

## Final checkpoint verification

The final authority-hardening checkpoint was verified again from the feature
worktree before commit. `just` and PowerShell remained unavailable, so the
Linux-equivalent commands below executed every `just ci` dependency directly:

- `pnpm install --frozen-lockfile && pnpm typecheck && pnpm test && pnpm build`
  in `ui/`: 31 files / 274 tests passed and the production build completed.
- `node scripts/check-i18n-completeness.js`, the cross-face Node test, and the
  cross-face projection check: 1,388 keys / 138 placeholders per locale, eight
  tests, and 13 shared semantic units passed.
- `go vet ./...`, uncached `go test ./...`, and the `vivy_headless` compile:
  passed.
- Standalone `go vet ./...` plus uncached `go test ./...` for both Faces and
  all six independent plugin modules: passed.
- Focused Assembly/SDK/App/ChannelHost tests and all five Channel module tests:
  passed.
- `go run ./sdk verify` for hello-fs, LSP, all five Channels, headless, and TUI:
  all nine sources passed.
- `go run ./sdk pack --recipe recipes/default.vivy.yml` followed by
  `inspect-artifact`: passed with Generation ID
  `59d3bf42dda4a3e7301cab1f39c2ebb729ebc050e48fb9e4860e980b07a1254f`.
- Regenerating the default Assembly produced byte-identical source to the
  tracked `internal/generated/assembly/zz_default.go`; `git diff --check`
  passed.
