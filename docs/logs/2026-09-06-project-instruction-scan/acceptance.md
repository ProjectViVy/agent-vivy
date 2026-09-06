# Acceptance

A human can tell this worked without reading the implementation:

1. Start Vivy from a project that has a root `AGENTS.md` and `.agents/skills/<name>/SKILL.md`.
2. Open the Skills page (`http://127.0.0.1:3015/skills`) or run `/skills` in VIVY CODE. The project package appears and is labeled as project / read-only.
3. Send a turn. The model sees the launch-directory `AGENTS.md` (for example a project-specific rule from that file) even when the web composition is still `world: sandbox`.
4. Enabling or editing that project skill fails closed. Marketplace install still writes to `skills_root`.
5. Launching from a git subdirectory also picks up the repo-root `AGENTS.md` and repo-root `.agents/skills`.
6. Launching from a directory with neither file does not fail the process; injection is simply empty and the catalog is the user `skills_root` only.
