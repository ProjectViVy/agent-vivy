package toolhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/sdk/port/pretool"
)

var (
	ErrMiddlewareDenied = errors.New("pre-tool middleware denied request")
	ErrMiddlewareFailed = errors.New("pre-tool middleware failed closed")
)

type RevalidateFunc func(context.Context, Request) error

type MiddlewareResult struct {
	ToolID          string
	Arguments       json.RawMessage
	ApprovalClasses []string
}

type middlewareOutcome struct {
	decision pretool.Decision
	err      error
}

func (h *Host) ApplyMiddleware(ctx context.Context, request Request, revalidate RevalidateFunc) (MiddlewareResult, error) {
	toolID := strings.TrimSpace(request.ID)
	arguments := append(json.RawMessage(nil), request.Args...)
	result := MiddlewareResult{ToolID: toolID, Arguments: append(json.RawMessage(nil), arguments...)}

	for _, provider := range h.middleware {
		decision, err := h.callMiddleware(ctx, provider, pretool.NewRequest(toolID, arguments))
		if err != nil {
			return MiddlewareResult{}, err
		}
		switch decision.Kind {
		case pretool.Pass:
		case pretool.Deny:
			return MiddlewareResult{}, fmt.Errorf("%w: %s: %s", ErrMiddlewareDenied, decision.ReasonCode, decision.SafeMessage)
		case pretool.RequireApproval:
			result.ApprovalClasses = append(result.ApprovalClasses, strings.TrimSpace(decision.ApprovalClass))
		case pretool.RewriteArgs:
			arguments = append(json.RawMessage(nil), decision.Arguments...)
			next := Request{ID: toolID, Args: append(json.RawMessage(nil), arguments...)}
			if revalidate != nil {
				if err := revalidate(ctx, next); err != nil {
					return MiddlewareResult{}, err
				}
			}
			result.Arguments = append(json.RawMessage(nil), arguments...)
		default:
			return MiddlewareResult{}, fmt.Errorf("%w: %s returned unsupported decision %q", ErrMiddlewareFailed, provider.ID(), decision.Kind)
		}
	}
	result.Arguments = append(json.RawMessage(nil), arguments...)
	return result, nil
}

func (h *Host) callMiddleware(ctx context.Context, provider pretool.Provider, request pretool.Request) (pretool.Decision, error) {
	callCtx, cancel := context.WithTimeout(ctx, h.middlewareTimeout)
	defer cancel()
	outcomes := make(chan middlewareOutcome, 1)
	go func() {
		outcome := middlewareOutcome{}
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.err = fmt.Errorf("middleware panic: %v", recovered)
			}
			outcomes <- outcome
		}()
		outcome.decision, outcome.err = provider.Evaluate(callCtx, request)
	}()

	select {
	case <-callCtx.Done():
		return pretool.Decision{}, fmt.Errorf("%w: %s: %w", ErrMiddlewareFailed, provider.ID(), callCtx.Err())
	case outcome := <-outcomes:
		if outcome.err != nil {
			return pretool.Decision{}, fmt.Errorf("%w: %s: %w", ErrMiddlewareFailed, provider.ID(), outcome.err)
		}
		if !outcome.decision.Valid() {
			return pretool.Decision{}, fmt.Errorf("%w: %s returned invalid decision", ErrMiddlewareFailed, provider.ID())
		}
		return outcome.decision, nil
	}
}
