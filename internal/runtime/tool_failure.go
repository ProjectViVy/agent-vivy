package runtime

// toolFailure records one classified, model-correctable tool invocation
// outcome (NUDGE-DESIGN §4/§5). Absence of a record means ordinary
// success. The classifier arrives in ND-1; the record and its side
// channel (nudgeState.MarkFailure/Failure) are defined here so journal
// metadata and detection can be built first.
//
// Status is "recoverable" or "refused". Reason is a stable vocabulary
// word (e.g. "invalid_arguments", "not_found", "policy_denied",
// "remote_tool_error", "command_failed"). Diagnostic is redacted,
// bounded text — never raw credentials. Effects is "not_executed",
// "none" or "unknown".
type toolFailure struct {
	Status     string
	Reason     string
	Diagnostic string
	Effects    string
}
