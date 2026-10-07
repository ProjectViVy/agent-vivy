# A2A-01 — TaskHost contract and Assembly identity wiring

Executed both tasks of [A2A-01](../../superpowers/plans/2026-10-07-a2a-server/A2A-01.md)
on branch `A2A` after G0 adoption (A2A-00, commit `117adf1a`).

- **A2A-01.1** — created `sdk/port/channel/task.go` + `task_test.go`:
  all design §5.1 declarations copied verbatim (`TaskHost` five methods,
  `TaskServiceInfoHost`, `TaskRequest`/`TaskRef`/`TaskQuery`/`TaskSnapshot`/
  `TaskPage`/`TaskSubscription`/`TaskUpdate`/`TaskStream`, seven TaskState
  constants, eleven TaskErrorCode values, safe-message `TaskError`). Removed
  the empty `TaskLifecycle` placeholder from `channel.go`. Commit `54e47203`.
- **A2A-01.2** — preserved task capability and Module identity through
  wiring: generated `RuntimeAssembly.ChannelModuleIDs` (provider ID ->
  sealed Module ID, ambiguity rejected by the generator),
  `BindChannelsWithModuleIDs`, `taskGrantedChannelHost` +
  `taskInfoGrantedChannelHost` (grant re-checked per call, wrapped only
  around a real matching capability, typed-nil guarded), app validation
  for missing/empty/extra module identity. `zz_default.go` regenerated
  through `go generate`; internal source digest re-pinned in
  `conformance_results.json`.

No A2A SDK imports, no new method on `channel.Host`, no hand-edited
generated code, no manually maintained parallel Module map. The Story is
certified with Host test doubles; it does not claim the production
TaskHost or HTTP listener exists (A2A-02..05 territory).
