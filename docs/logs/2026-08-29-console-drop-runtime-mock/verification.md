# verification

## Commands

```text
# 1. Source + installed profile no longer emit runtime.mock: true
Select-String studio/dsh-vivy-console/index.js `
  data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/index.js `
  -Pattern 'mock:\s*true'
# → no matches (only comments mentioning removal)

# 2. Fixed scratch config parses and process starts
$env:VIVY_USER_HOME = (Resolve-Path data/studio-home/vivy-console/data).Path
$env:VIVY_CONFIG    = (Resolve-Path data/studio-home/vivy-console/config.yaml).Path
go run ./cmd/vivy
# → INFO settings overlay applied …
# → INFO vivy starting addr=127.0.0.1:8787
# → no "field mock not found" abort
```

## Gate notes

- Change is Studio overlay JS + scratch config only (no `internal/` / `cmd/` / `ui/` Go/TS).
- Product path smoke is config-load + process start under the console overlay;
  `just ci` was not re-run for this single-field generator fix.
- Host half of the console is loaded once at Studio boot: after syncing the
  installed profile copy, **restart Studio** so `prepare()` uses the new
  generator. Client-only refresh is not enough for this fix.
