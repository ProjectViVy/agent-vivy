# Backend CI split

## What changed

- Added a standalone `just backend-ci` recipe for Go formatting, vet, tests,
  headless compilation, and independent plugin-module checks.
- Kept the existing `just ci` recipe as the complete local aggregate.
- Split GitHub Actions into independent `backend ci` and `ui ci` jobs so a UI
  failure cannot prevent backend evidence from being produced. The UI gate
  includes the merged cross-face I18N conformance checks.
- Added an aggregate `just ci` status that requires both component jobs. This
  preserves one stable required-check name for branch protection.
- Kept the embedded-asset requirement explicit: the backend job builds
  `ui/dist` but does not run UI typechecks or tests.
- Stabilized four cron scheduler timing fixtures exposed by the new Windows
  backend gate. Recurring write-backs stay observable, and one-shot jobs keep
  enough startup margin not to be misclassified as offline missed jobs.

## Scope boundaries

- No UI source, UI tests, translations, or other active I18N files changed.
- No Go product code or plugin compiler fixtures changed; four Go test timing
  fixtures changed without altering runtime behavior.
- Branch protection was not changed by this delivery.
- This lane started from `chore/plugin-v1-preflight` without modifying that
  worktree. After PR #15 and the I18N PR #5 merged, the branch was updated to
  current `main` before its pull request was opened.
