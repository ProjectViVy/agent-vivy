# Human acceptance

1. Fresh disposable SQLite/PostgreSQL startup reaches the integrated schema
   with version/name/applied_at metadata only, no retired checkpoint/prompt
   checksum columns and no mutation of unknown/partial/old live formats.
2. An existing exact metadata3 target through040 appends041 and reopens without
   changing admitted values, opaque Generation IDs, references, presentation
   bytes, receipts, history or completed operation outcomes.
3. Admit a task with exact private prompt/policy values; change current config
   and resume its v2 checkpoint. It retains the original Run-owned admission.
   Foreign/missing/wrong schema/engine/Generation bindings refuse recovery.
4. Repeat a D09 request and download across restart. Original receipts and
   immutable bytes remain. Edit/rewind/delete still prune hidden private bytes
   atomically; lost-response retries and rollback remain safe.
5. Copied finite transitions settle only supported old live authority and
   checkpoint pointers, preserve historical outcomes and roll back completely
   on failure. D14 must assemble and accept the final artifact separately.

These are reproducible criteria, not human execution claims. Standard3015 smoke
and D09/D12 human acceptance remain open. D10/D11 upstream gates are unchanged.
