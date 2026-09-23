package rpc

import (
	"context"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
)

// deliverablesListParams pages a session's committed delivery sets. The
// session id is the only authority input; the connection identity proves it.
type deliverablesListParams struct {
	SessionID string `json:"session_id"`
	Cursor    string `json:"cursor,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// deliverablesGetParams addresses one committed set inside its session.
type deliverablesGetParams struct {
	SessionID string `json:"session_id"`
	SetID     string `json:"set_id"`
}

// deliverablesReadParams pages verified bytes of one presented item. The
// caller declares the digest it expects; a changed file answers "changed"
// instead of serving wrong bytes.
type deliverablesReadParams struct {
	SessionID      string `json:"session_id"`
	ItemID         string `json:"item_id"`
	ExpectedDigest string `json:"expected_digest"`
	TransferID     string `json:"transfer_id,omitempty"`
	Offset         int64  `json:"offset"`
	Length         int    `json:"length"`
}

// deliverablesCloseParams releases one live transfer handle early.
type deliverablesCloseParams struct {
	SessionID  string `json:"session_id"`
	TransferID string `json:"transfer_id"`
}

func (h *controlHandler) deliverablesAuthority(ctx context.Context, sessionID string) (context.Context, *Error) {
	if h.deps.Deliverables == nil {
		return nil, &Error{Code: MethodNotFound, Message: "deliverable operations are not configured"}
	}
	// The same session-existence check + trusted identity projection used by
	// history/*: connection binding supplies the session, the body cannot.
	return h.historyAuthorityContext(ctx, sessionID)
}

func mapDeliverableError(err error, operation string) *Error {
	var delErr runtime.DeliverableError
	if errors.As(err, &delErr) {
		return &Error{Code: InvalidParams, Message: fmt.Sprintf("%s failed: %s: %s", operation, delErr.Status, delErr.Reason)}
	}
	return internalError(err)
}

func (h *controlHandler) deliverablesList(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	var params deliverablesListParams
	if rpcErr := decodeStrictJSON(request.Params, &params); rpcErr != nil {
		return nil, rpcErr
	}
	authority, rpcErr := h.deliverablesAuthority(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	page, err := h.deps.Deliverables.List(authority, domain.SessionID(params.SessionID), params.Cursor, params.Limit)
	if err != nil {
		return nil, mapDeliverableError(err, "deliverables list")
	}
	return page, nil
}

func (h *controlHandler) deliverablesGet(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	var params deliverablesGetParams
	if rpcErr := decodeStrictJSON(request.Params, &params); rpcErr != nil {
		return nil, rpcErr
	}
	if params.SetID == "" {
		return nil, &Error{Code: InvalidParams, Message: "set_id is required"}
	}
	authority, rpcErr := h.deliverablesAuthority(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	set, err := h.deps.Deliverables.Get(authority, domain.SessionID(params.SessionID), params.SetID)
	if err != nil {
		return nil, mapDeliverableError(err, "deliverables get")
	}
	return set, nil
}

func (h *controlHandler) deliverablesRead(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	var params deliverablesReadParams
	if rpcErr := decodeStrictJSON(request.Params, &params); rpcErr != nil {
		return nil, rpcErr
	}
	authority, rpcErr := h.deliverablesAuthority(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	chunk, err := h.deps.Deliverables.Read(authority, domain.DeliveryReadRequest{
		ItemID:         params.ItemID,
		ExpectedDigest: params.ExpectedDigest,
		TransferID:     params.TransferID,
		Offset:         params.Offset,
		Length:         params.Length,
	})
	if err != nil {
		return nil, mapDeliverableError(err, "deliverables read")
	}
	return chunk, nil
}

func (h *controlHandler) deliverablesClose(ctx context.Context, request Request) (any, *Error) {
	if rpcErr := rejectHistorySpoofFields(request); rpcErr != nil {
		return nil, rpcErr
	}
	var params deliverablesCloseParams
	if rpcErr := decodeStrictJSON(request.Params, &params); rpcErr != nil {
		return nil, rpcErr
	}
	if params.TransferID == "" {
		return nil, &Error{Code: InvalidParams, Message: "transfer_id is required"}
	}
	authority, rpcErr := h.deliverablesAuthority(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if err := h.deps.Deliverables.CloseTransfer(authority, params.TransferID); err != nil {
		return nil, mapDeliverableError(err, "deliverables close")
	}
	return map[string]any{"closed": true}, nil
}
