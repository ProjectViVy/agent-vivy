# Plugin v1 preflight acceptance

- [x] Work is isolated in a new worktree and branch.
- [x] GitHub Actions runs fixture checks and the complete repository `just ci`
  gate on Windows.
- [ ] `main` branch protection requires pull requests and the `just ci` check.
- [x] Plugin-plan dependencies and `ui/src/styles.css` are corrected.
- [x] PLG-P1 through PLG-P9 have open, unscheduled tracking issues.
- [x] Acceptance fixtures cover success, v0 rejection, duplicate Modules,
  missing/duplicate Providers, graph cycles, denied Grants, and bad hashes.
- [x] No changed path overlaps the active I18N draft PR.
- [ ] Remote `just ci` is green.
