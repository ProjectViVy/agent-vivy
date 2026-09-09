# VC-1e — AGENTS.md injection (direct eino agentsmd integration) + `vivy init`

Date: 2026-08-31 | Branch: `feat/vc1a-bash-tool` (vc0 worktree) | Track: VIVY-CODE VC-1

## What changed

**D6 context-file injection (read-only AGENTS.md → run preamble)**

- Uses the native eino v0.9.13 `adk/middlewares/agentsmd` middleware (D6 direct pass-through; no custom injection logic).
  - `internal/runtime/agentsmd.go`: `AgentsMDFileName = "AGENTS.md"` (D6 decision: no multi-file priority involving CLAUDE.md/VIVY.md), a cumulative 64 KiB injection budget, and the `AgentsMDBackend` type alias (the app layer does not touch eino types, D-007).
  - `internal/runtime/engine.go`: `EngineConfig.AgentsMDBackend`; middleware registration occurs **after** the compaction handlers (the officially recommended order: injected content is transient and does not participate in summary compaction).
  - Backend = the existing `EinoFilesystemBackend` (its shape satisfies the single `Read` method of `agentsmd.Backend`), resolving the request context's run ID to `AGENTS.md` inside **this run's private workspace**—isolated per run, with no cross-run leakage.
- Behavior: missing file → middleware warning + skip (the run is unaffected); existing file → one user message is injected before the first real user message on every model call (with an idempotency Extra marker, injected only once per run); **transient**—never written to the Journal or message storage.
- Companion fix: `safeWorkspacePath` now wraps `os.ErrNotExist` for the "path does not exist" error— the agentsmd loader uses `errors.Is(err, os.ErrNotExist)` to distinguish "file missing (skip)" from "other read error (fatal)"; the old generic error caused runs without AGENTS.md to fail immediately. Message text is unchanged; only the sentinel chain was completed.
- App assembly: `buildEngineConfig` adds an `agentsMDBackend` parameter, and both the startup and settings-save reload paths connect it to `fileBackend` (no injection occurs naturally when no workspace root is configured).

**`vivy init` generates AGENTS.md**

- `cmd/vivy/init.go`: new subcommand `vivy init` (main.go dispatches it before worker/tui).
- Behavior follows the initialize points in research §8.4 (behavioral alignment, no code copying, FSL-1.1-MIT):
  - **Rejects empty directories** (directories containing only hidden entries such as `.git` count as empty)—init describes existing projects;
  - **Refuses to overwrite** an existing AGENTS.md (exit 1; the original file is preserved byte-for-byte);
  - **Detects existing rule files** (`.cursorrules`, `.cursor/rules`, `.github/copilot-instructions.md`) and, when found, lists a "Keep in sync or reference" reminder at the end of the generated AGENTS.md;
  - The template's core principle is "Record only non-obvious knowledge—an agent can read the code but cannot guess intent," with four sections: Project overview / Build, test, verify / Conventions the code does not show / Known pitfalls.

## Scope / not done

- **Stale-read protection (filetracker/file_versions):** the RB-1 conclusion merges stale-read protection and file-version history into one storage design, pending the user's O1..O6 approval; not implemented here (same as VC-1d).
- No global (user-level) AGENTS.md: D6 covers workspace files only; Crush's global `~/.config/crush/CRUSH.md` layer is not included (do not add unapproved scope).
- No injection configuration switch: when AGENTS.md is absent, the middleware is a no-op (one Debug log), so normal runs have zero cost; no unapproved knob was added.
- `vivy init` does not analyze code to generate content (Crush uses a model to generate summaries); V0 delivers a structured template + rule-file detection, and "generate content from a run" can use normal chat.

## FSL compliance

Crush is FSL-1.1-MIT: this only aligns behavior/protocol (initialize's empty-directory refusal, rule-file detection, and the "record only non-obvious knowledge" principle); the template text and implementation were written from scratch, with no source copied.

## Files

- `internal/runtime/agentsmd.go` (new): D6 constants, `AgentsMDBackend` alias, and middleware construction.
- `internal/runtime/engine.go`: EngineConfig field + handler assembly (after compaction).
- `internal/runtime/filesystem_backend.go`: ErrNotExist sentinel wrap (one line + comment).
- `internal/app/compaction.go` / `internal/app/app.go`: `buildEngineConfig` wiring on both paths.
- `cmd/vivy/init.go` (new) + `cmd/vivy/main.go`: `vivy init` subcommand.
- `config.example.yaml`: workspace_root comment documenting injection behavior.
- Tests: `internal/runtime/agentsmd_test.go` (5), `cmd/vivy/init_test.go` (4).
