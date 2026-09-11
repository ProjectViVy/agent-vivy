package app

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/toolhost"
)

func (tool governedTool) ApplyPreTool(
	ctx context.Context,
	arguments json.RawMessage,
	revalidate toolhost.RevalidateFunc,
) (toolhost.MiddlewareResult, error) {
	return tool.host.ApplyMiddleware(ctx, toolhost.Request{
		ID:   tool.id,
		Args: append(json.RawMessage(nil), arguments...),
	}, revalidate)
}
