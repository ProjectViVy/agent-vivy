# I18N Task 9 closure

Task 9 documents the reviewed Tasks 1–7 baseline (`1b1fdd3`) and closes the
runtime Han audit without claiming whole-repository acceptance.

- README and design now explain English default, exact en/zh, process-over-
  `.env` development/pack input, sealed Generation immutability, and the shared
  global workspace override through `settings/get` and `settings/locale`.
- `just ci` includes `i18n-check` after `ui-ci`, preserving every original gate.
- One reveal-engine console diagnostic now uses both catalogs. Its scanner
  exception was removed; two locale failure-path tests and a scanner
  regression test were added test-first.
- Task 8 remains a non-normative proposal, deliberately deferred until the
  unscheduled PLG-P1 v1 Module-ID/artifact foundation exists and the extension
  is explicitly accepted/scheduled. No rejected v0 path was extended.
- The plan records the superseding authority ruling and corrected audit globs;
  the TODO board retains Go/TUI verification and cross-face conformance limits.

No plugin code, new features, dependencies, Go changes, or unrelated refactors
were added. No subagents, workflow runs, merge, or push were used. This is a
source/documentation handoff, not a packaged release; release/rollback notes
are omitted because no Generation was packed or deployed.
