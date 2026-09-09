# UI-INIT-RACE Delivery Summary

## Delivery topic
Fixed lost actions when a user explicitly created or switched sessions during the application's initial loading window, caused by the asynchronous `initialize()` snapshot being overwritten by the trailing automatic selection (UI-INIT-RACE closure).

## Change scope

1. **Session merging and intent protection (`ui/src/lib/store.ts`)**:
   - After `initialize()` returns the asynchronously fetched backend snapshot (`api.listSessions()`), deduplicates and merges sessions the user created or updated in memory during the wait (`freshlyCreated` + `inFlightMap`), preventing a newly created session from being overwritten and disappearing from the drawer.
   - If a session already exists after merging (the user created one during the wait), does not call `api.createSession('')` again to redundantly create a blank fallback session.
   - Checks `get().activeSessionId`: if the store already has a valid active session (present in the merged list), never writes over it or triggers a redundant `selectSession`; only when no valid session is currently selected does initialization follow the localStorage value or the first list item.

2. **Send-discard protection and alerting (`ui/src/lib/store.ts`, `ui/src/i18n/{zh,en}.ts`)**:
   - When `startRun` and `editSession` detect `get().activeSessionId !== sessionId`, they no longer silently `return`; instead, they set `runError` and throw `Error(t('errors.sessionMismatch'))`.
   - The caller's `ChatInput` `try...catch` catches the exception, preserves the user's draft text and pending attachments (no longer executes `setValue('')`), and `ChatView` renders a `RecoverableError` alert bar, preventing messages and drafts from being silently swallowed or lost.
   - Added bilingual localization copy for `errors.sessionMismatch`.

3. **Unit-test coverage (`ui/src/lib/store.test.ts`)**:
   - Added four targeted unit tests covering protection against overwriting a session created while initialization is suspended, merged-list deduplication, avoiding redundant fallback-session creation, and throwing plus recording `runError` when sessions do not match.

4. **Audit and ledger (`docs/TODO.md`)**:
   - Updated the `UI-INIT-RACE` entry in §0.1 to `DONE 2026-09-04` and recorded the closure details.

## Explicitly not done (Out of Scope)
- Does not modify the server-side session creation and listing RPC protocol.
- Does not affect queue management while a session is running (messages during a run are still enqueued normally under the established Crush conventions).
