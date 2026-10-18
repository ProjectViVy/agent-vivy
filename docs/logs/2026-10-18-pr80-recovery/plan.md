# PR #80 recovery completion plan

Scope: Eino v0.9.21, pending-run recovery, related delivery evidence. No AgenticModel migration or #79 work.

## Design and Eino capability check

Use the existing generation-based BlobStore for a small durable cleanup intent. Persist it before cancelling an interaction, retain it until its cancellation event, exactly one terminal, and Plan cancellation are durable. Startup drains intents before scanning active runs, including intents whose run already ended. Replay is the event deduplication authority; existing lifecycle CAS and deterministic Work request IDs remain authoritative. Failure returns an error and leaves the intent for another Recover/restart. An all-in-one transaction would couple runtime to dialect storage and Plan internals; an intent reuses current storage seams without DDL.

Inspected pinned Eino v0.9.21 `adk/runner.go`: `RunnerConfig.CheckPointStore`, `CheckPointStore` alias and typedRunnerResumeInternalImpl persist opaque engine continuation only. They do not own Vivy Approval/Question/Plan rows or Journal terminal ordering. Keep NewEinoCheckpointAdapter and VersionedCheckpointStore unchanged. The intent is product lifecycle consistency, not a substitute Eino checkpoint or agent loop. Remove this seam only if the product storage contract later provides atomic interaction + Journal + Work settlement.

## Steps and acceptance

- [ ] Integrate latest main without replacing development-environment files. Conflict is only the obsolete open ToolsNode race row; retain its completed record.
- [ ] Add failing real-SQLite tests for cancellation write failure, cancellation event failure, terminal failure, post-terminal Plan failure, intent creation/deletion failures, restart and repeated Recover. Require no duplicate/missing cancellation or terminal events.
- [ ] Implement durable retry and error propagation, preserving version guard and checkpoint bytes.
- [ ] Test actual old-main and candidate binaries with isolated paused Approval/Question/Plan databases. Establish unsupported direct historical rollback and supported pre-cleaned rollback. No version string substitution as binary evidence.
- [ ] Run current just ci, focused recovery failures, relevant race, native Windows binary smoke; reproduce any baseline failures on untouched main in the same host.
- [ ] Review final diff, record candidate SHA, local versus remote evidence and remaining B5/B6/B7; commit and push to existing PR only, update its body/comment. Never merge or deploy.
