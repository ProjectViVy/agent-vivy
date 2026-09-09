# Acceptance — Split PROC-COMMIT into separate deliveries

Date: 2026-08-26

## How to confirm closure

1. The top of `git log --oneline -5` contains the three focused feature commits
   plus this closing docs commit:

   ```
   <hash> docs: close PROC-COMMIT after split commits
   1a3f0c7 feat(ui): add chat message action bar with copy and regenerate
   9a11370 feat(ui): add first-run welcome wizard
   a857976 feat(ui): add evolution page with demo governance workflow
   ```

2. `git show --stat a857976` / `9a11370` / `1a3f0c7`: each commit contains only
   its topic’s files + its own `docs/logs/<topic>/`; no commit contains core files
   from two topics (for example, the Evolution commit does not contain
   `WelcomeWizard.tsx`, and the Chat commit does not contain `EvolutionView.tsx`).

3. The split is visible in shared files:
   - `git show a857976 -- ui/src/i18n/zh.ts` contains only nav/skills.status/
     evolution/demo.evolution entries; `git show 1a3f0c7 -- ui/src/i18n/zh.ts`
     contains only entries such as `chat.copy`; `git show 9a11370 --
     ui/src/i18n/zh.ts` contains only `welcome.*` entries.
   - `git show 9a11370 -- ui/e2e/runtime.spec.ts` and `git show 1a3f0c7 --
     ui/e2e/runtime.spec.ts` contain only the wizard adaptation and action-bar
     assertion hunks, respectively.

4. `git status` is clean (the root tree returns to a single-lane state with zero uncommitted changes).

5. `docs/TODO.md`: §0.1 no longer has a PROC-COMMIT row (UI-EVO and UI-CHAT-ACT
   remain OPEN items for later real backend wiring); §10 Completion log has a new
   2026-08-26 PROC-COMMIT completion entry at the top.

6. Nothing was pushed: `git status`’s ahead count is the number of commits the
   local branch has over the remote; push awaits user authorization.
