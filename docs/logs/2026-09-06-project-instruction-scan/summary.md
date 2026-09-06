# Project instruction scan (cwd AGENTS.md + SKILL)

## Changed

- Added a Vivy discovery adapter for launch-directory instructions. Eino v0.9.13 injects `AGENTS.md` and exposes the `skill` tool, but it does not scan cwd, git roots, or `.agents/skills`.
- `AGENTS.md` is collected by walking up from the launch directory to the git worktree root (D6: only `AGENTS.md`). Nested crate files such as `ui/AGENTS.md` are not dumped when launched at the repo root.
- Project skills are first-level packages under `.agents/skills` and `.vivy/skills` at cwd, then the git root. They overlay `runtime.skills_root` with closer-wins name collision. Project packages are read-only.
- The same instruction root is wired into `vivy.exe`, `vivy-code.exe`, `vivy tui`, and `vivy run`. Web/sandbox compositions inject host instructions without setting `WithCodeProjectRoot` (no host `@file` / `project-context/list`).
- Skills catalog, sidebar, `/skills`, and the Skills page expose `origin` (`project` | `user`). Project skills cannot be toggled or mutated.

## Explicitly not done

- CLAUDE.md / CRUSH.md / `.cursorrules` (D6).
- Recursive whole-tree `SKILL.md` or every nested `AGENTS.md`.
- Cross-client dirs (`.claude/skills`, `.cursor/skills`, `.crush/skills`).
- `~/.agents/skills` (user-global skills stay in `skills_root`).
- Turning the web sandbox into `world: local`.
- Watcher / hot-reload of instruction files (middleware already re-reads per model call).

This is a focused source delivery, not a release; no release record is included.
