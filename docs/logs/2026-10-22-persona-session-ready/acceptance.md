# Acceptance

1. Cold-open or fully reload /persona with an existing active session. The page waits for the host session load, then displays the real selected Persona document without an authorization-denial alert or a manual retry.
2. Edit the document and save. The existing revision CAS remains authoritative. A stale revision still fails; refresh/retry reads the current document rather than overwriting it.
3. Delay session/messages. No cognitive status/read/review action runs before that selected-session response succeeds. The ready/empty transition loads the document automatically.
4. Fail session loading. The page shows the real host error and Retry calls the host session loader. Pending retry shows loading, and recovery loads the document; it does not spin forever after failure.
5. Existing dirty-draft confirmations, stale selected-kind responses and conflicting-operation controls remain effective.

Focused automated evidence is recorded in verification.md. Actual integrated browser rerun belongs to the parent lane. Rollback is a focused Git revert; no data migration or production-state rollback is involved.
