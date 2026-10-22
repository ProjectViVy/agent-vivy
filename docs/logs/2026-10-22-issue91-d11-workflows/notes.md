# Finite converter handoff

D14 owns the standalone tools/storage-upgrade artifact outside this final runtime tree. No old digest parser or converter is added to the runtime branch. The baseline support matrix remains design section 11.7: supported original prefixes normalize to baseline035 on a disposable copy; unknown/post-baseline original source formats are refused before writes. D14, not the new runtime Open path, owns that finite route.

## Capture before applying 044/045

Read old result references in deterministic owner order:

```sql
SELECT workflow_run_id,path,attempt,blob_id,bytes,created_at
FROM workflow_results ORDER BY workflow_run_id,path,attempt;
```

Resolve every blob_id through the old source's BlobStore implementation (Core storage blobs table), not a filesystem path guessed from the ID. Capture exact bytes keyed by existing (workflow_run_id,path,attempt). Require every reference to exist and byte count to match; missing/inaccessible bytes are incomplete_conversion. Verify source/target owner counts, path/attempt identity and actual bytes. Capture before 044 removes blob_id. After 044 adds output, write those captured bytes into target workflow_results.output at the same existing owner key. Do not regenerate node output from Journal summaries or rerun children. Existing output blobs may remain historical data; do not delete unrelated blob owners.

Preserve workflow revision lineage, operation keys, exact descriptor_json/authority_json/input_json, limits, compiler/Eino metadata and definition lineage. The old schema2 program/catalog/input digests do not establish a new ProgramID. Keep schema_version 1/2 historical and program_id NULL; never label those rows schema3 or invent a binding. New schema3 is admitted only through the new owner path.

Read old execution status/waits/interrupts/gates/checkpoint/checkpoint_blob_id/resume_key before 044. Old native paused payloads and digest-only resume claims cannot prove the new ProgramID/catalog/serializer input contract. Set old nonterminal Run state using the shared upgrade_recovery_required policy: active -> failed, accepted/queued -> cancelled; settle pending interactions and append one terminal event atomically. Existing terminal Runs and historical node outcomes remain unchanged. Clear unbindable checkpoint/wait/resume projections and mark workflow execution recovery_required where nonterminal; retain unresolved operation fences and never automatically retry a claimed unknown effect. Do not reconstruct resume answers from public events. New resume_answers_json remains NULL on old rows.

Old workflow_commits retain CommitID, sequences, projection revision and timestamps. New input_json stays NULL: a historical digest cannot reconstruct the original commit input and cannot authorize replay. A reused old commit owner must return an explicit binding/recovery conflict, not an empty-body success. Existing opaque IDs/ETags, even if originally derived by old code, remain historical owner identity; new ETags derive only from monotone owner update position.

Draft/revision artifacts and used_implementations already contain actual JSON. Preserve those documents, author ownership, ETags, monotone publication revisions and historical times. Publication equality in the new runtime uses canonical actual documents and actual used implementations; no old digest fields are needed to recover those values.

## Transaction and cutover

Quiesce the organism. Retain matching old executable plus a consistent source backup (including SQLite WAL or verified restored PostgreSQL copy). Capture/stage owner-keyed output bytes in a private stable conversion allocation before dropping old refs on the separate target. Convert and verify target SQL atomically; source is untouched. Require all target DB/artifact components and target reopen verification before switching configured roots. Partial copies are never selected. Inject missing-blob, SQL failure, verification and switch failures; refuse incomplete_conversion and keep source complete. After cutover, rollback requires quiescence, old executable/backup, and separately preserving post-cutover data. Never run the old executable against the converted target.

044/045 are target format source, not a promise that Open performs conversion. Full supported-source/rollback/real-Postgres fixture evidence remains with D14.
