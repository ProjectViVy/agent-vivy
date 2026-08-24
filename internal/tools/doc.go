// Package tools owns the Vivy ToolSpec registry and approval policy.
//
// V0 registers read-only tools that execute automatically (still emitting
// tool.started/tool.finished), an effectful tool that emits
// tool.approval_required, and the ask_user control tool that emits a distinct
// user.question_required lifecycle. Registration is Vivy-owned, not Eino's.
//
// All tools cross the runtime policy gate before their implementations can
// run; approval and question state are persisted separately.
package tools
