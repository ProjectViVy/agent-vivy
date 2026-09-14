# Plugin Platform v1 release verification

Date: 2026-09-14

Go 1.26.4 was used with `GOFLAGS=-buildvcs=false` for linked-worktree builds.
The local Linux worker does not provide the repository's PowerShell-backed
`just` runner or a Playwright Chromium binary. Equivalent local commands were
executed directly; `.github/workflows/ci.yml` remains authoritative for the
Windows `backend-ci`, `ui-ci`, and browser jobs.

## Conformance and repository gates

| Command | Result |
| --- | --- |
| `go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1` | PASS, 150.301s; independently executes all 23 source-bound cases and the cited UI Vitest suite |
| `go test ./sdk/internal -run '^TestGenerationFailureMatrixExecutesEveryCase$' -count=1` | PASS, all 25 cases, 84.296s |
| `go test -timeout 25m ./...` | PASS; `sdk/internal` 695.524s, assembly 35.531s, producer 150.952s |
| `go vet ./...` and `go build ./...` | PASS |
| `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | PASS |
| tracked `gofmt -l` check and `git diff --check` | PASS |
| focused catalog canonicalization/semantic-identity selectors in `sdk/internal` and `sdk/internal/assembly` | PASS |
| `go test ./internal/runtime ./internal/codeface -count=1` after the TTY-discovered skill correction | PASS; runtime 26.226s, codeface 0.449s |

## Published Windows CI closure

The final code head `2e715cf` passed [GitHub Actions run #147](https://github.com/ProjectViVy/agent-vivy/actions/runs/34825393484):

| Required check | Result |
| --- | --- |
| `backend ci` | PASS |
| `ui ci` | PASS |
| `full UI browser smoke` | PASS; Windows Playwright at `http://127.0.0.1:3015` |
| aggregate `just ci` | PASS |

The aggregate is the workflow's required-lane guard; its backend and UI lanes
run the split equivalents of the repository `just ci` recipe. This closes the
previously skipped browser slice. The local Linux worker still lacks
PowerShell-backed `just` and Chromium, so the browser result is intentionally
the dedicated Windows result above rather than a local claim.

The complete local UI gate passed before final publication: typecheck; 35
Vitest files and 316 tests; a 2,280-module production build; 1,396-key English
and Chinese catalog completeness with 138 placeholder checks; eight cross-face
tests; and the SDK UI TypeScript build plus 27 tests. The final conformance
producer separately reran `ui/src/plugins/conformance.test.tsx` (six tests).

Every standalone Module under `plugins/` and `faces/` passed its own
`go vet ./...` and `go test ./...` gate. `vivy-sdk verify` accepted these selected
public Modules:

- `faces/headless`, `faces/tui`;
- `plugins/dingtalk`, `plugins/discord`, `plugins/feishu`;
- `plugins/governance`, `plugins/hello-fs`, `plugins/lsp`;
- `plugins/qq`, `plugins/scx-reference`, `plugins/telegram`;
- `sdk/internal/testdata/full-ui-module`.

## Final sealed artifacts

Each command used a new output directory under `.workspace/p9-release-final`,
and `go run ./sdk inspect-artifact <output>` accepted the result.

| Recipe | Generation ID | Modules | Conformance records | Executable SHA-256 |
| --- | --- | ---: | ---: | --- |
| `recipes/default.vivy.yml` | `14d576e125a2f163a4aed10c18145ee284a90a7c09bc981c493211fd3fd0a40e` | 25 | 75 | `1bf5ba5a50ce38661628928695db85d7d6889aea5e07bb35c4e5511a4fa4b186` |
| `recipes/minimal.vivy.yml` | `6ddd63d4aeddebdb1699a3b4c0bed11f10464bb8a4627aa7b713a66b0e469b7e` | 7 | 0 | `0cfe366cd6bb47d6cfe13e0bef5cb2eb403b777a0810765c7364adc13c08e7da` |
| `sdk/internal/testdata/full-ui.vivy.yml` | `41c73c255cb7fc932a4628b5ed5bf6da75f813b58a81fc3465271865b32d184e` | 10 | 45 | `9477423b11dd3d9dca34dc19984fb8ee1f2fb240e93c839f250474234339bf07` |
| `recipes/scx.vivy.yml` | `1dd5174c38c276b96eb1a50202cc72789c690827a501d483dfa7229faa855fb6` | 18 | 30 | `87dee33eb85099ab2acf1d940bce71ddc6d2923da8b87aba5d391c1fafea70a5` |

## Real paths

- VIVY CODE: built the headless-tag binary, verified `--help`, then launched
  the real terminal UI with credentials removed and an isolated
  `VIVY_USER_HOME`. It rendered the local-project surface, loaded all six
  repository skills, and exited cleanly on Ctrl+C. The first run exposed two
  invalid YAML descriptions; `TestRepositoryProjectSkillsLoad` reproduced the
  failure before the metadata correction.
- MCP: the real stdio subprocess, handshake, Tool projection/call,
  resource/prompt, environment/cwd, dead-session, and production
  MCPHost-to-ToolHost governance selectors passed.
- Channels: Telegram full-loop, DingTalk stream loopback, Discord reconnect,
  Feishu WebSocket loopback, and QQ reconnect/publish selectors passed in
  their standalone Modules.
- Full UI HTTP: the packed full-UI executable served `/healthz` with 200,
  served `/assets/index-DCRrQem7.js`, and that asset contained the selected
  full-UI copy and route markers.
- Browser behavior: not executed locally because Chromium is absent and its
  installer is unavailable in this worker's network venue. The required
  `pnpm exec playwright test e2e/plugin-full-ui.spec.ts` passed in the
  dedicated GitHub Actions Windows browser job recorded above.

No real-path smoke used tenant Journal data or live provider/channel
credentials.
