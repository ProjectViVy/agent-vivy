# Acceptance

With a Goal candidate held before durable admission, pause or edit the Goal. Releasing the candidate creates no run, user message, or charged round. With an admitted run held in the model, edit the Goal; its old-revision `report_goal` call receives a stale-reference result and cannot complete or block the revised Goal. Once the old run has cleaned up, the revised active Goal admits its next round, with a distinct RunID and message provenance.

An admitted Goal run waiting on an approval or question remains the one active session run despite another wake. Public `run/cancel` durably blocks a matching active Goal before signalling its run; a failed Work commit returns an error without signalling cancellation. The cancelled terminal does not start a replacement round. Cancelling an ordinary non-Goal run still leaves Work unchanged. A normal last allowed round completes its run and leaves a durable `goal round limit reached` block tied to that RunID.

If a process is lost after atomic GoalRun commit but before the Service drives the model, startup recovery fails that existing run and blocks the Goal. The consumed round, original message, and admission event remain; no new run replays the work.

For pause, clear, and direct cancel while a run is still cancelling, the mutation response or `session/work/get` shows `activation: disarmed` with the still-owned `current_run_id`; that ID clears only at terminal cleanup. `session/work/subscribe` publishes durable event metadata and does not project activation; clients obtain the current WorkView through `session/work/get`.
