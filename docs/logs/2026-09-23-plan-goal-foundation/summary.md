# Plan and Goal foundation

Completed the PG-0 backend foundation on `feat/issue-47-goal-plan-foundation`.

The branch persists an exact Plan review suspension and resumes the originating Eino call once after a human decision, fences unreviewed sibling tool calls, and recovers a pending review after Service restart. History message/work ordering is durable across timestamp ties; fork and rewind preserve spent Goal usage and do not transfer approval authority. Human turn admission registers intent before waiting on the session gate and commits a real RunID only with the stored run. Shared limits have one domain owner.

Added runtime/domain/storage/RPC evidence and paired history-anchor migrations. This slice does not deliver downstream Plan/Goal UI or claim PG-6 product acceptance. Live PostgreSQL, browser integration and a real coding walkthrough remain outside the completed evidence; see [verification](verification.md).
