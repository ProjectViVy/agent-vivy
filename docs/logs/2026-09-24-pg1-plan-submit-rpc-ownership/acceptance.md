# Acceptance

The GUI cannot create a Plan submission through JSON-RPC. A request to `plan/submit`, even with a plausible active Run ID and a caller-forged ToolCall ID, returns method-not-found and writes no work event. The current work state and complete replay are unchanged by the rejected request.

Model-created submissions continue to use the runtime `SubmitPlan` path, where origin Run and ToolCall data come from the model tool execution context. The GUI continues to inspect Plan state and immutable submission details through read-only work/Plan queries.
