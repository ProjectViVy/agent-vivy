package mcphost

// ToolExecutionError carries a remote tool's own error result — the
// MCP CallTool response arrived with IsError — as a typed error on the
// internal channel (NUDGE-DESIGN §4/§5). It is a remote *tool result*
// error, not a JSON-RPC/transport failure: those keep their existing
// untyped error path. The bounded untrusted response text travels in
// Text so the runtime can present it to the model without inventing a
// retry contract.
type ToolExecutionError struct {
	Text string
}

func (e *ToolExecutionError) Error() string { return e.Text }
