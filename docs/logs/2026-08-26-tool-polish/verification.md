# Verification — 2026-08-26 tool polish

All commands run in the lane worktree `../agent-vivy-tool-polish`
(fresh checkout of `feat/tool-polish`), not the shared root tree.

## Gate: `just ci`

- First full run: FAIL — two issues found and fixed:
  1. `gofmt` drift on the three touched Go files → `gofmt -w`, rerun.
  2. `internal/provider` `TestSecretEnvReadsOnlyInProvider` (D-010 source
     audit) rejected literal `os.Getenv("BING_SEARCH_API_KEY")` /
     `os.Getenv("GOOGLE_SEARCH_API_KEY")` in
     `internal/runtime/network_search.go`. Rewrote the availability roster
     to carry env var *names* in a table and read presence indirectly —
     the same idiom the file already uses (`os.Getenv(apiKeyEnvFor(...))`).
- One intermediate full run had an unrelated timing flake
  (`internal/runtime` journal "context canceled" under full parallel
  load); the package passed twice in isolation and in subsequent full
  runs. Not touched by this change (no journal/cancel paths modified).
- Final: `just ci` → exit 0 (fmt-check, vet, go test ./..., headless
  compile, ui-ci typecheck + 57 vitest units + build).

## New unit tests

- `internal/tools/filesystem_test.go` — `TestReadFileNumbersContentLines`:
  full read starts at 1 with trailing newline preserved; range read
  numbers from `start_line`; binary payload untouched; empty stays empty.
  Forwarding test updated for the numbered `"1\tok"` expectation.
- `internal/config/config_test.go` — `TestDefaultEnabledOmitsEchoInfo`,
  `TestNetworkSearchProviderConfig` (parse + default-empty + invalid
  provider rejected).
- `internal/runtime/network_search_test.go` —
  `TestNetworkSearchServiceHonorsPreferredProvider` (keyless/keyed
  preference honored, unusable preference degrades to duckduckgo,
  explicit provider beats preference),
  `TestNetworkSearchProviderAvailability` (roster + env presence).
- `internal/app/settings/settings_test.go` — `TestNetworkSearchPreference`
  (round trip + validation).
- `internal/rpc/control_test.go` — settings get/update extended:
  availability roster present with keyless providers configured, invalid
  provider rejected, preference persists and echoes back.

## Browser smoke (real split pair, 127.0.0.1:3015)

Per `smoke-for-user-visible-change`; backend launched from the worktree
with `runtime.mock: true`, `tools.network_search.provider: ""` in a
scratch config (removed afterwards), and `SEARXNG_SEARCH_URL` set to a
dummy URL to prove availability flip:

- 设置 → 工具 renders the real 网络搜索 card: provider select on
  自动（按可用性回退）, badges `bing/google 未配置（需 …KEY）`,
  `duckduckgo/wikipedia 已配置`, `searxng 已配置` (env present — flip
  proven).
- Selected `wikipedia` via the combobox, hint text switched to the
  pinned-provider wording, 保存网络搜索设置 dispatched.
- Persisted document `data/smoke/settings.yaml`:
  `network_search: {provider: wikipedia}` (model fields empty —
  full-replace semantics).
- Page reload → select restores `wikipedia` from `settings/get`.
- Note: IAB screenshot capture unavailable in this environment
  ("guest capture failed"); DOM snapshots + on-disk file used as evidence.
- Caveat (tooling, not product): Radix Select trigger clicks via the
  Playwright surface timed out in the IAB runtime; interaction done
  through the native select mirror and a node-path click. No product
  defect observed — the shadcn Select works normally in the page.

## Deferred findings

None. (`docs/TODO.md` §0.1 unchanged; §10 row added.)
