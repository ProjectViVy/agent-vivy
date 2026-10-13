# Windows Overall Smoke Blocker Repair

The 2026-10-13 Windows overall smoke exposed four blockers that prevented a clean repository gate and a Toolkit preview after viewing a completed Run. This delivery fixes those regressions and verifies the full Vivy CI path.

The RPC Peer now keeps the validated session identity but clears its RunID when `run/get` resolves a terminal Run from the durable RunStore. Accepted, queued, and active Runs remain bound to both session and RunID, so direct Toolkit management is still denied while a Run is active. Windows absolute local Git paths are recognized before URL-scheme parsing; origin validation still rejects credentials, unsafe protocols, query strings, fragments, and control characters.

The Windows test fixtures now use the shell syntax and built-ins available on the default Windows environment. The scoped Hook fixture still verifies explicit working-directory and environment isolation, including that inherited secrets are not passed through. Toolkit composition tests pin the repository's Eino version when running from test binaries, and the Toolkit browser test scopes status assertions to their corresponding notices.

No model loop, Eino integration, storage schema, or Studio surface changed. The run identity continues to derive from the authoritative Vivy RunStore and existing `domain.RunStatus`; no second runtime or source of truth was added. Browser smoke data used a disposable VIVY home and local mock provider in the repair worktree. Production Journal paths were not accessed.

Explicitly not done: push, Studio work, or unrelated open TODO items. The pre-existing 2026-10-13 smoke record remains the original evidence; this directory records the fixes and final verification.
