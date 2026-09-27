# Verification

Read current worktree code and scoped instructions: ChatInput/ChatView/store/api, run-row projection, SQL message/Journal writes, runtime startup, recipes and Playwright setup. Worktree baseline remains a2d2b59 plus the earlier design commit.

Checks performed: relative Markdown links resolve; 13 unique ordered task headings; balanced code fences; placeholder scan; spec-to-task coverage; authority and additive-delivery consistency; git diff --check.

Self-review clarified history_read selection_digest for model attachment, bounded operator metadata listing, oversized raw payload handling, queue retry idempotency, and Playwright download event listener ordering.

Product implementation tests, UI interaction tests and Postgres tests are planned, not executed. The required just ci command is attempted separately; no substitute test suite is presented as its success.

`just ci`: exit 127 (`just: command not found`). Full product gate remains unverified. Documentation checks passed.

Naming-only follow-up: reviewed spec/plan occurrences and ran git diff --check. Remaining minimal references are existing recipe paths or ordinary implementation wording. No runtime behavior changed; product CI was not repeated for this editorial correction.
