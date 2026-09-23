# PG-2 Task 2: exact review and model work identity

Task 2 makes `create_goal` require a human decision before durable Goal work, even when the configured approval policy is `never`, and binds model-originated work identity to the Eino tool-call ID. The existing Service, Eino middleware, approval, Journal, Policy, and WorkStore paths remain authoritative.

The approval regression covers pending approval before any Goal-created/admitted event; denial leaves no Goal, while approval creates and admits a bounded Goal run. Identity coverage proves same-call retries replay, distinct identical calls remain distinct, and changed arguments under the same call ID conflict in the SQLite WorkStore.

Explicitly not done: Eino upgrade, second agent loop, new dependency/schema/public Port/policy source, Plan submission changes, PG-4 report/progress work, unrelated transition/race work, PostgreSQL acceptance, or release. Excluded uncommitted UI generated files remain untouched by this delivery and are not staged.
