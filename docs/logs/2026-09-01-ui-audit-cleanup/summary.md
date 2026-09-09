# Summary: UI audit cleanup (maintainer instruction list)

Follow-up to the 2026-08-31 audit (`docs/logs/2026-09-01-remove-chat-preflight`
uncovered a wider pattern). The maintainer reviewed the audit findings and gave
a numbered ruling; this iteration implements it.

## Done

1. **Remove orphaned `ScanPrompt` (kernel).** The prompt-injection scanner in
   `internal/tools/security.go` lost its only consumer when the preflight was
   removed in `86c94fa`. Deleted `ScanPrompt`, `SafetyFinding`, the
   `promptInjectionPattern` regex, and its test. `ValidateArgsSafety` and
   `RedactSensitive` stay (live callers in tooladapter/broker/worker). A
   `vivy dry-run` command was explicitly rejected by the maintainer.
2. **Dashboard: remove the Session (overview) tab.** `DashboardDemoView` now has
   exactly two tabs — Token statistics and Trajectory (both kept per maintainer
   ruling).
   Deleted the fake session/run/review numbers and the recent-activity feed,
   plus `getDemoDashboard`, `DemoDashboardSnapshot`, the
   `vivy.demo.dashboard` storage key, and their tests/i18n.
3. **Skills: remove the Change requests tab.** The staged-revision
   review surface in `SkillsView` is gone along with `listSkillRevisions`,
   `SkillRevision`, and their i18n. The backend `skills/revisions/list` RPC and
   the SkillRevisions store stay untouched — they are written by the runtime's
   staged skill-write path, not by this UI.
4. **Chat input: relocate New session.** The bottom-row Plus button (previously
   a "More" stub that only raised "not wired up") is now the real New session
   button; the duplicate New session button in the upper toolbar row was
   deleted, as was the More stub (`chatInput.more`/`moreUnavailable` i18n
   removed).

## Explicitly not done (maintainer ruling)

- Memory / Notebook / Persona / Evolution demo pages **stay** as-is with their
  demo data;
  they are slated for real backend integration later.
- Remaining hookless buttons (Attachment, AutoDream, Thinking mode, Agent mode
  ask-branch, and message "Edit / Return here / Fork from here") stay for now.
- Backend `skills/revisions/list` method and store unchanged.
