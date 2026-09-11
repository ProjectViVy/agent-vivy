// Package pretool defines the public pre-execution Middleware Port. Providers
// can constrain or normalize one Tool request, but receive no execution,
// Journal, credential, Policy, or registration authority.
package pretool

import (
	"context"
	"encoding/json"
	"strings"
)

type Kind string

const (
	Pass            Kind = "pass"
	Deny            Kind = "deny"
	RequireApproval Kind = "require_approval"
	RewriteArgs     Kind = "rewrite_args"
)

type Request struct {
	ToolID    string
	Arguments json.RawMessage
}

func NewRequest(toolID string, arguments json.RawMessage) Request {
	return Request{ToolID: strings.TrimSpace(toolID), Arguments: append(json.RawMessage(nil), arguments...)}
}

type Decision struct {
	Kind          Kind
	ReasonCode    string
	SafeMessage   string
	ApprovalClass string
	Arguments     json.RawMessage
	Rationale     string
}

func (decision Decision) Valid() bool {
	switch decision.Kind {
	case Pass:
		return true
	case Deny:
		return strings.TrimSpace(decision.ReasonCode) != "" && strings.TrimSpace(decision.SafeMessage) != ""
	case RequireApproval:
		return strings.TrimSpace(decision.ApprovalClass) != ""
	case RewriteArgs:
		return len(decision.Arguments) != 0 && json.Valid(decision.Arguments)
	default:
		return false
	}
}

type Provider interface {
	ID() string
	Evaluate(context.Context, Request) (Decision, error)
}
