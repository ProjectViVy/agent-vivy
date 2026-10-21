# D08: admitted values and checkpoint ownership

Implementation baseline: stable `c065a82b4618e34dddd32893abd0557bd79c5b44`
(accepted D03). The issue-91 index owns status; verification is recorded separately.

ContextHost deduplicates bounded exact content and allocates an independent
query ID. SkillHost returns actual content, IDs and explicit provider versions
when available; local unversioned files do not invent a revision. MCP names
encode instance/name boundaries reversibly, and schema reconciliation compares
canonical JSON while preserving number literals. ToolWorld and ToolHost no
longer carry digest projections.

The immutable private RunPromptSnapshot owns admitted prompt and policy values.
Checkpoint v2 binds its existing Run/schema/composer/Generation identity and
the Eino engine version, retaining opaque native payload bytes and Blob atomic
generations. Missing admission and old outer formats refuse recovery. Public
Journal/Inspect/UI projections carry no prompt/policy digest or private policy
substitute. Direct shell and workflow keep their existing private state owners;
they do not acquire a second prompt service. Policy drift refuses execution.

Paired migration 037 removes prompt and Blob checksum columns. Offline target
fragments revoke old unconsumed authority, disable native checkpoint pointers,
retain historical generations and settle old Runs/questions atomically. They
are resources for D14's finite converter, not a runtime compatibility reader.

Pinned Eino v0.9.21 `adk.CheckPointStore`, `CheckPointDeleter`,
`TypedRunner.ResumeWithParams` and `WithCheckPointID` remain the execution and
recovery mechanism. The existing Vivy adapter alone binds business ownership.

D10 retains mask Definition/Metadata/create-request and skill/cognitive mutation
CAS digests; D11 retains child/workflow/Work business digest closure; D09 retains
file preconditions. D12 owns migration metadata and D14 final upgrade assembly.
D04 waits for independently accepted D06. No push or Windows full CI is in scope.

Closed backlog item: **MASK-CONTINUITY-SNAPSHOT** (found 2026-10-06 during
headless mask arming). Continuity admissions previously stamped a public prompt
marker without saving its private row, so later recovery failed with
`snapshot_missing`. `ContinuityAdmission` now carries the same Prompt/ExpectedMask
as ordinary admission, validates the locked actual mask definition and persists
the snapshot in the existing SQLite/Postgres transaction. Shared conformance
covers failure after the prompt step, no leaked row after rollback, successful
commit and reopen. The original discovery record remains
`docs/logs/2026-10-06-headless-form-mask-arming/`.
