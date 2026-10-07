# A5 — `vivy-code mcp` and `vivy-code config` subcommands

**Commit:** `feat(vivy-code): mcp and config subcommands`
**Depends on:** A1. Spec §5.1; boundary: settings overlay is the only writer.

## What landed

`cmd/vivy-code/subcommands.go` — argv[0] dispatch happens BEFORE flag
parsing, because `mcp add x --command npx -y foo` must let `-y foo` pass
through to the child argv untouched.

- `vivy-code mcp list` — effective server table (settings overlay when the
  operator owns the list, else the config.yaml default) with transport +
  enabled/deferred columns.
- `vivy-code mcp status` — reachability probe: HTTP GET for endpoints,
  `exec.LookPath` for stdio commands. Configured-probe only; live circuit
  state belongs to the running host.
- `vivy-code mcp add <name> --endpoint <url>` | `--command <exe> [args...]` —
  validated write via `settings.Update`/`UpsertMCPServer` (endpoint
  credential-free URL rules, command allowlist, env references only).
- `vivy-code mcp remove <name>` — case-insensitive delete via
  `DeleteMCPServer`; missing name exits 1.
- `vivy-code mcp login` — prints the O2 deferral note (OAuth via provider
  plugin), exit 0.

- `vivy-code config get <path>` / `set <path> <value>` — dotted paths over
  the settings.yaml schema (e.g. `provider`, `default_model`,
  `network_search.provider`, `compaction.enabled`). Writes round-trip the
  document through `settings.Update` so schema validation applies (e.g.
  `default_model` requires `provider` — verified). Unknown paths and
  leaf-vs-section misuse are rejected; secret leaves (`api_key`, `*_key`,
  `*_secret`, `*_token`) are refused outright — `*_env`/`env_from`
  references stay writable since they hold names, not values (D-010).

## Files

- `cmd/vivy-code/subcommands.go`, `subcommands_test.go` (new)
- `cmd/vivy-code/main.go` — subcommand dispatch before parseArgs
