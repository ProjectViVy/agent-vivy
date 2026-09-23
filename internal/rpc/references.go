package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
)

// referencePreviewParams carries exactly the operator-declared inputs of
// reference/preview: the destination session plus one bounded selection.
type referencePreviewParams struct {
	SessionID string                  `json:"session_id"`
	Selection domain.HistorySelection `json:"selection"`
}

func (h *controlHandler) referencePreview(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	if h.deps.References == nil {
		return nil, &Error{Code: MethodNotFound, Message: "reference operations are not configured"}
	}
	var params referencePreviewParams
	if rpcErr := decodeStrictJSON(request.Params, &params); rpcErr != nil {
		return nil, rpcErr
	}
	authority, rpcErr := h.historyAuthorityContext(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	preview, err := h.deps.References.Preview(authority, params.Selection)
	if err != nil {
		var refErr runtime.ReferenceError
		if errors.As(err, &refErr) {
			return nil, &Error{Code: InvalidParams, Message: fmt.Sprintf("reference preview failed: %s: %s", refErr.Status, refErr.Reason)}
		}
		return nil, internalError(err)
	}
	return preview, nil
}

// referenceGetParams addresses one committed snapshot inside its destination
// session. The reply is the destination-owned copy plus live source and feed
// status — never a fresh read of the source session.
type referenceGetParams struct {
	SessionID   string `json:"session_id"`
	ReferenceID string `json:"reference_id"`
}

func (h *controlHandler) referenceGet(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	if h.deps.References == nil {
		return nil, &Error{Code: MethodNotFound, Message: "reference operations are not configured"}
	}
	var params referenceGetParams
	if rpcErr := decodeStrictJSON(request.Params, &params); rpcErr != nil {
		return nil, rpcErr
	}
	if params.SessionID == "" || params.ReferenceID == "" {
		return nil, &Error{Code: InvalidParams, Message: "session_id and reference_id are required"}
	}
	view, err := h.deps.References.Get(ctx, domain.SessionID(params.SessionID), params.ReferenceID)
	if err != nil {
		var refErr runtime.ReferenceError
		if errors.As(err, &refErr) {
			return nil, &Error{Code: InvalidParams, Message: fmt.Sprintf("reference get failed: %s: %s", refErr.Status, refErr.Reason)}
		}
		return nil, internalError(err)
	}
	return view, nil
}

// decodeStrictJSON decodes one body exactly: unknown keys are rejected so a
// forged caller body (raw items, fabricated digest fields, spoofed authority)
// fails closed instead of being silently ignored.
func decodeStrictJSON(raw json.RawMessage, target any) *Error {
	if len(raw) == 0 {
		return &Error{Code: InvalidParams, Message: "params are required"}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return &Error{Code: InvalidParams, Message: fmt.Sprintf("params do not satisfy the declared shape: %v", err)}
	}
	return nil
}

// decodeContinuityInput folds the continuity-bearing turn/start fields into
// the validated admission input. References are strictly decoded per entry so
// a forged reference body (items/content/actor/accepted_scope inside the
// object) is rejected rather than ignored.
func decodeContinuityInput(params *turnParams) *Error {
	if len(params.References) == 0 && len(params.HistoryScope) == 0 && params.RequestID == "" {
		return nil
	}
	if params.RequestID == "" {
		return &Error{Code: InvalidParams, Message: "request_id is required with references or history_scope"}
	}
	input := &domain.ContinuityInput{RequestID: params.RequestID}
	for _, raw := range params.References {
		var selection domain.ReferenceSelection
		if rpcErr := decodeStrictJSON(raw, &selection); rpcErr != nil {
			return rpcErr
		}
		input.References = append(input.References, selection)
	}
	if len(params.HistoryScope) > 0 && string(params.HistoryScope) != "null" {
		var scope domain.HistoryScope
		if rpcErr := decodeStrictJSON(params.HistoryScope, &scope); rpcErr != nil {
			return rpcErr
		}
		input.HistoryScope = scope
	}
	if err := input.Validate(domain.DefaultContinuityLimits()); err != nil {
		return &Error{Code: InvalidParams, Message: fmt.Sprintf("continuity submission is invalid: %v", err)}
	}
	params.continuity = input
	return nil
}
