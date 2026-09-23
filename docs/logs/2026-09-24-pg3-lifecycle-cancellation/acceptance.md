# Acceptance

With a Goal candidate held before durable admission, pause or edit the Goal. Releasing the candidate creates no run, user message, or charged round. With an admitted run held in the model, edit the Goal; its old-revision `report_goal` call receives a stale-reference result and cannot complete or block the revised Goal.

An admitted Goal run waiting on an approval or question remains the one active session run despite another wake. Cancelling that run yields one cancelled terminal and a durable blocked Goal reason; no replacement round starts. A normal last allowed round completes its run and leaves a durable `goal round limit reached` block tied to that RunID.

If a process is lost after atomic GoalRun commit but before the Service drives the model, startup recovery fails that existing run and blocks the Goal. The consumed round, original message, and admission event remain; no new run replays the work.

For both pause and clear while a run is still cancelling, the mutation response and a later `session/work/get` show `activation: disarmed` with no current RunID. `session/work/subscribe` publishes durable event metadata and does not project activation; clients obtain the current WorkView through `session/work/get`.
