# PROC-COMMIT closure: split three completed root-tree topics into separate deliveries

Date: 2026-08-26
Status: complete

## What changed

Closed PROC-COMMIT in `docs/TODO.md` §0.1: the three completed but uncommitted
deliveries accumulated in the root worktree were split into three focused commits
under `commit-one-concern-per-deliverable`; no code was changed.

| Topic | Commit | Contents |
|---|---|---|
| Evolution page | `a857976` `feat(ui): add evolution page with demo governance workflow` | `EvolutionView` / `useEvolution` / `demo-api` + `types` domain types / route and `routeTree.gen.ts` / sidebar entry / `skills.status` badges + its `docs/logs/2026-08-25-evolution-page/` + §0.1 `UI-EVO` entry |
| Welcome Wizard | `9a11370` `feat(ui): add first-run welcome wizard` | `WelcomeWizard` / `use-welcome` (+ tests) / `_layout` mount / Settings-page rerun entry / `welcome-float` styles / e2e spec + `playwright.config.ts` zh-CN / `docs/logs/2026-08-25-welcome-wizard/` |
| Chat message actions | `1a3f0c7` `feat(ui): add chat message action bar with copy and regenerate` | `chat-actions` (+ tests) / `MessageBubble` action bar / `ChatView` wiring / e2e assertions + `docs/logs/2026-08-26-chat-message-actions/` + §0.1 `UI-CHAT-ACT` entry |

## Shared-file split method

Shared files with cross-topic edits were staged in topic-specific hunks (after
checking `git diff` hunk ownership, using a subset patch with `git apply --cached`):

- `ui/src/i18n/zh.ts` / `en.ts`: 6 hunks—1/3/4/5 (nav-entry removal,
  `skills.status`, `evolution` domain, `demo.evolution`) → Evolution; 2
  (`chat.copy`, and so on) → Chat; 6 (`welcome.*`) → Welcome. zh/en were staged
  together, keeping dictionary structure consistent in intermediate commits.
- `ui/e2e/runtime.spec.ts`: 3 hunks—1 (wizard skip), 3
  (`vivy.ui.welcome.completed` allowlist + “Back to List” → “Back”) → Welcome;
  2 (action-bar assertions + clipboard) → Chat.
- `docs/TODO.md`: 3 new-entry lines were surgically committed with their
  respective deliveries; the PROC-COMMIT line was closed with this final delivery
  (§0.1 removal, §10 entry).

Before committing, single-topic files were checked for cross-topic contamination
(`demo-api`/`types` had no Welcome/Chat changes, `ChatView`/`MessageBubble` had no
Evolution/Welcome changes, and so on; all grep checks returned 0).

## Explicitly not done

- Nothing was pushed (push requires explicit user authorization).
- `just ci` was not run separately for the three intermediate commit states (the
  split changed only the index, the worktree stayed in the merged state; the final
  tree rerun of `just ci` was all green, and each intermediate state kept zh/en
  dictionary structure synchronized with no dangling references).
- No product code was changed; each delivery’s verification record is in its own
  `docs/logs/*/verification.md`.
