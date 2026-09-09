# Plugin v1 preflight summary

## Scope

Prepare repository governance, plan corrections, tracking, and compiler
acceptance inputs while the Web/TUI I18N change continues independently.
This delivery does not schedule or implement PLG-P1 through PLG-P9.

## Delivered

- Added a SHA-pinned Windows GitHub Actions job named `just ci`.
- Added a pure-data plugin v1 compiler fixture corpus and a Node integrity
  validator that can run before the Go v1 compiler exists.
- Corrected Gate A, P5/P7/Gate B, evidence-derived support-state, I18N-gate,
  and UI stylesheet dependencies in the plugin program plan.
- Created the nine GitHub trackers:
  [PLG-P1 #6](https://github.com/ProjectViVy/agent-vivy/issues/6),
  [PLG-P2 #7](https://github.com/ProjectViVy/agent-vivy/issues/7),
  [PLG-P3 #8](https://github.com/ProjectViVy/agent-vivy/issues/8),
  [PLG-P4 #9](https://github.com/ProjectViVy/agent-vivy/issues/9),
  [PLG-P5 #10](https://github.com/ProjectViVy/agent-vivy/issues/10),
  [PLG-P6 #11](https://github.com/ProjectViVy/agent-vivy/issues/11),
  [PLG-P7 #12](https://github.com/ProjectViVy/agent-vivy/issues/12),
  [PLG-P8 #13](https://github.com/ProjectViVy/agent-vivy/issues/13), and
  [PLG-P9 #14](https://github.com/ProjectViVy/agent-vivy/issues/14).

## Isolation

Work was performed in the dedicated `chore/plugin-v1-preflight` worktree. A
path intersection check against draft PR #5 found no overlap with its active
I18N changes.
