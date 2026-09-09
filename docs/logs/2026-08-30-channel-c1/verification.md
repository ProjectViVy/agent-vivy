# CH-C1 — verification

Date: 2026-08-30. Worktree: `agent-vivy-channel-c1`, branch `feat/channel-c1`,
baseline 82ecf14.

## GOAL execution (subagent roles)

- explore (read-only): scanned repository-wide `domain.Message{` literals (all keyed,
  so adding fields is harmless), sqlite/pg schemas, the conformance entry point, and
  the `EventTypes`/schema mirror relationship. **Corrected one fact in PLAN §9**:
  the sqlite test database is not full DDL, but an ordered migration chain
  (migration001–015, with the full chain run through `Open`); postgres is the
  one-shot full bootstrap. Therefore sqlite uses `migration016`, while postgres uses
  a version bump plus in-place ALTER.
- executor (writes, cwd locked to the C1 worktree): implemented the 12 files in the
  CH-C1 file list; an additional task added the postgres v14→15 in-place upgrade test.
- reviewer (independent read-only review): **PASS**, no blocker; two should-fix items
  (no test coverage for the postgres upgrade branch → `upgrade_test.go` added; missing
  log/TODO artifacts → this directory and TODO update close the loop).

## Commands and results (all run at the C1 worktree root)

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test ./...` (after executor implementation) | All packages ok; runtime 60.9s, storage/sqlite 40.3s, storage/postgres 6.9s (DSN-gated cases SKIP); `TestBackendConformance/CN-17_message_provenance_round-trip` PASS (sqlite harness) |
| `gofmt -l ./internal ./sdk ./cmd` | No output (clean) |
| reviewer recheck `go test ./internal/... ./sdk/... ./cmd/...` | All packages ok (runtime 37.6s, etc.) |
| `go test ./internal/storage/postgres/... -v` (after adding tests, `env -u VIVY_POSTGRES_TEST_DSN`) | exit 0: `TestMigrateUpgradesV14InPlace` SKIP (DSN unset), `TestBackendConformance` SKIP, existing cases PASS |
| `just ci` (final run after `upgrade_test.go` was written) | **exit 0**: `fmt-check`/`vet`/`test`/`headless-compile`/`ui-ci` all green; all Go packages ok (runtime 43.9s), UI `21 passed (21)` files / `175 passed (175)` tests, `vite build` green |

## Acceptance checklist (CH-C1.md §7)

- `just ci` green ✅
- `git diff 82ecf14 -- go.mod go.sum` empty; no telego/discordgo/lark/botgo ✅
- `channel.inbound` schema exists; no token/key/raw-body fields; fixtures (including
  CN-17 and unit tests) contain no secrets ✅
- Old Message rows can be listed: migration columns use `DEFAULT ''`; the CN-17 empty
  Source row reads back as `EffectiveSource()=="ui"`; `TestMessageEffectiveSource`
  covers zero-value compatibility ✅
- UI path regression: the full runtime test suite is green, user rows explicitly use
  `Source:"ui"`, and assistant/tool projections are unchanged ✅
- `internal/channelhost` does not exist; `sdk/plugin`, `pluginhost`, and
  `zz_register.go` (still `return nil`) are unchanged; no new eino import (Eino
  remains quarantined to `internal/runtime`/`internal/provider`) ✅

## Honest statement (environment limitation)

- This machine has no Docker and no Postgres on 5432: the
  `VIVY_POSTGRES_TEST_DSN`-gated `TestBackendConformance` (postgres harness,
  including the CN-17 pg side) and new `TestMigrateUpgradesV14InPlace` **test bodies
  were not run against real Postgres**; only compilation, vet, and clean SKIP were
  verified. Two assertions that depend on standard behavior (`information_schema`
  `column_default` rendering as `''`, and multi-statement fixture exec) need one
  run in DSN-enabled CI or an environment with Postgres. The repository's existing
  convention makes the DSN optional and keeps it outside the `just ci` gate, unchanged
  from before this work.
- The `ui/src/routeTree.gen.ts` dirtied by `pnpm build` (generated file / line-ending
  noise) was restored with `git checkout --` before commit and did not enter the
  commit.

## Scope-guard evidence

- `git status`: 12 modified + 2 new (`channel.inbound.json`, `upgrade_test.go`), all
  within the CH-C1 §5 list (`postgres/postgres.go`'s migrate branch is required for
  the version upgrade and is explained above).
- The dirty root-tree `main` and `feat/channel-super-contract` documentation branch
  received no writes.
- Not pushed (not authorized).
