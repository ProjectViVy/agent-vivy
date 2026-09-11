# Verification

All commands were run from the repository root on Windows. The `just` gate was given explicit paths to the installed Go 1.26.4 binaries because `go` was not present on this shell's `PATH`.

## Regression checks

- `node scripts/check-i18n-cross-face.js`
  - Passed after the eight intentional Web-only MCP keys were classified.
- `node --test scripts/check-i18n-cross-face.test.js`
  - Passed: 8 tests.
- `go test ./sdk/internal/assembly -run 'TestHashSourceTree|TestSourceCatalog'`
  - Passed. The new text line-ending regression was observed failing before the shared canonical hasher was implemented, and the binary-content regression confirms invalid UTF-8/NUL-containing assets remain byte-exact.
- `$expected = '249983bd553e0805a22e6f72594ff7e1498b057bb176c4c29f50c4c07196a921'; $actual = go run ./sdk/internal/cmd/source-hash plugins/dingtalk $expected; if ($actual -ne $expected) { exit 1 }`
  - Passed with exact equality to the repository-pinned DingTalk digest.
- `go test ./internal/app -run TestPluginWindowCannotImportInternal`
  - Passed.
- `go test ./internal/runtime -run TestServiceApprovalResumePersistsChunkBeforeProviderEOF -count=20 -timeout 3m`
  - Passed 20 consecutive runs.
- `go test ./sdk/internal -run 'TestV1PackAndInspectProveRecipeRemoval|TestPackAndInspectEveryShippedRecipe' -count=1 -timeout 5m`
  - Passed on Windows.

## Product gate

- `just --set go 'C:\Program Files\Go\bin\go.exe' --set gofmt 'C:\Program Files\Go\bin\gofmt.exe' ci`
  - Passed with exit code 0.
  - UI typecheck passed.
  - UI tests passed: 31 files and 275 tests.
  - Vite production build passed.
  - i18n completeness and cross-face contracts passed.
  - Go vet passed.
  - All Go packages passed, including `internal/runtime` in 207.403 seconds and `sdk/internal` in 183.080 seconds.
  - Headless compilation passed.
  - Independent plugin and face vet/test gates passed.
  - Vite reported its existing large-chunk advisory; it was not a gate failure and is outside this CI-closure scope.

## Real plugin path

- `go run ./sdk verify plugins/dingtalk`
  - Passed: `ok vivy/dingtalk`.
- `go run ./sdk pack --recipe recipes/default.vivy.yml --output C:\tmp\agent-vivy-pr18-default-generation-20260911-review`
  - Passed.
  - Generation ID: `fb05bd1980e74d5a6987347eaca68304de73d619c7ccbe8cde272ad23d87d007`.
  - The artifact retained the canonical DingTalk source digest `249983bd553e0805a22e6f72594ff7e1498b057bb176c4c29f50c4c07196a921`.
- `go run ./sdk inspect-artifact C:\tmp\agent-vivy-pr18-default-generation-20260911-review`
  - Passed and projected the expected Modules, Ports, Grants, and unconfigured channel/MCP capabilities.

## Smoke rationale

The plugin verify, pack, and inspect sequence exercises the changed executable source-hash path. A browser smoke was not applicable because this closure changes translation-key classification metadata, tests, and platform-independent build behavior without changing rendered UI or interaction behavior.
