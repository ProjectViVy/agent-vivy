// Package worker is the deliberately small child-process side of the Vivy
// harness. It owns no product storage; the parent remains the authority for
// tool calls and durable events.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"agent-vivy/internal/rpc"
)

type RunRequest struct {
	RunID         string `json:"run_id"`
	ParentRunID   string `json:"parent_run_id"`
	PolicyProfile string `json:"policy_profile"`
	PolicyHash    string `json:"policy_hash"`
	WorkspaceID   string `json:"workspace_id"`
	Text          string `json:"text"`
	ToolName      string `json:"tool_name,omitempty"`
	ToolArgs      any    `json:"tool_args,omitempty"`
}

type RunResult struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}

type Event struct {
	RunID string `json:"run_id"`
	Type  string `json:"type"`
	Error string `json:"error,omitempty"`
}

// Run serves one worker process over stdio JSONL until the parent closes the
// pipe. No worker response contains a host filesystem path.
func Run(ctx context.Context, in io.Reader, out io.Writer) error {
	handler := &serverHandler{}
	peer := rpc.NewPeer(rpc.NewJSONLTransport(in, out, func() error { return nil }), handler, rpc.Options{OutgoingBuffer: 32})
	err := peer.Serve(ctx)
	if errors.Is(err, rpc.ErrPeerClosed) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

type serverHandler struct {
}

func (h *serverHandler) Handle(ctx context.Context, peer *rpc.Peer, request rpc.Request) (any, *rpc.Error) {
	switch request.Method {
	case "initialize":
		return map[string]any{
			"protocol_version": rpc.ProtocolVersion,
			"capabilities":     []string{"worker.run", "tool.broker"},
		}, nil
	case "worker/run":
		return h.run(ctx, peer, request)
	default:
		return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: "worker method not found"}
	}
}

func (h *serverHandler) run(ctx context.Context, peer *rpc.Peer, request rpc.Request) (any, *rpc.Error) {
	var params RunRequest
	if len(request.Params) == 0 {
		return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "worker run params are required"}
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "worker run params must be JSON"}
	}
	if params.RunID == "" || params.ParentRunID == "" || params.PolicyProfile == "" || params.PolicyHash == "" || params.WorkspaceID == "" {
		return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "run_id, parent_run_id, policy_profile, policy_hash, and workspace_id are required"}
	}
	if err := peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.started"}); err != nil {
		return nil, &rpc.Error{Code: rpc.ServerOverload, Message: "worker event queue is full"}
	}
	resultText := params.Text
	if params.ToolName != "" {
		brokerResult, err := peer.Call(ctx, "tool/execute", map[string]any{
			"run_id": params.RunID, "parent_run_id": params.ParentRunID,
			"policy_profile": params.PolicyProfile, "policy_hash": params.PolicyHash,
			"workspace_id": params.WorkspaceID, "tool_name": params.ToolName, "args": params.ToolArgs,
		})
		if err != nil {
			_ = peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.failed", Error: "tool broker rejected the request"})
			return nil, &rpc.Error{Code: rpc.InternalError, Message: err.Error()}
		}
		var toolResponse struct {
			Result string `json:"result"`
		}
		if err := json.Unmarshal(brokerResult, &toolResponse); err != nil {
			return nil, &rpc.Error{Code: rpc.InternalError, Message: "invalid tool broker response"}
		}
		resultText = toolResponse.Result
	}
	if err := peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.completed"}); err != nil {
		return nil, &rpc.Error{Code: rpc.ServerOverload, Message: "worker event queue is full"}
	}
	return RunResult{RunID: params.RunID, Status: "completed", Result: resultText}, nil
}
