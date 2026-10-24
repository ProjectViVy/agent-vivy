# A5 — `vivy-code mcp` and `vivy-code config` subcommands

**Goal:** `vivy-code mcp list|add|remove|status` and `vivy-code config get|set` without entering the TUI.
**Epic:** A. **Requirements:** RQ-SUB. **Predecessor:** A1 (flag parsing shape — subcommand argv handling).
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.1.

## Scope

**Files:** `cmd/vivy-code` subcommand dispatch; `internal/mcphost` config write path (servers live in `config.yaml` `runtime.mcp_servers[]` — mutate via settings/config layer, NOT raw file edits); `internal/config` get/set helpers with dotted-path addressing (`model.default`, `theme`, …).

## Tasks

- [ ] `mcp list` → table of configured servers + live reachability (reuse status surface); `mcp add <name> --command|--endpoint ...` → validated config write; `mcp remove <name>`; `mcp login` deferred (OAuth is O2 scope — print "deferred" cleanly if invoked).
- [ ] `config get <path>` / `config set <path> <value>` with schema validation; secret paths rejected.
- [ ] Tests: add/remove round-trip writes valid yaml; invalid endpoint rejected; `config set` refuses unknown paths and secrets.
- [ ] `go test ./cmd/vivy-code ./internal/config ./internal/mcphost`; `just ci`.
- [ ] Commit `feat(vivy-code): mcp and config subcommands`.

## Boundary

No `auth`/`update` subcommands (O6). Config writes go through the same schema/merge path the UI uses — no second writer.

## Acceptance

`vivy-code mcp add x --command npx -y foo` results in a running MCP integration on next TUI launch.
