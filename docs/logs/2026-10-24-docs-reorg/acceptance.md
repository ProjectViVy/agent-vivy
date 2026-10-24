# Acceptance

A reviewer can confirm the reorganization from the repository root:

1. `ls docs` shows four top-level records (`README.md`, `TODO.md`,
   `COMPLETE.MD`, `DEFER.MD`) and seven category directories
   (`architecture`, `specs`, `plans`, `research`, `dev`, `logs`, `archive`).
2. `docs/README.md` names the reading order, each directory, and where a new
   document belongs.
3. `docs/superpowers` does not exist; `docs/plans/issue-91/index.md` and
   `docs/specs/2026-09-23-issue39-eino-orchestration-design.md` open from
   their new locations, and their relative links to logs and architecture
   resolve.
4. `docs/archive/README.md` lists every archived document with its former
   location and the closed track it belongs to.
5. `docs/plans/README.md` lists the active packages; package state still
   lives in each package's `index.md`.
6. Opening a recent delivery record such as
   `docs/logs/2026-10-24-dev-vivy-macos-linux/summary.md` still works, and
   `docs/logs/2026-10-04-wails-migration/` is the renamed Wails log.
