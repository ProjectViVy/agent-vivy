// Package worker is the deliberately small child-process side of the Vivy
// harness. It owns no product storage; the parent remains the authority for
// tool calls and durable events.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"agent-vivy/internal/rpc"
)

const (
	maxChildTextBytes    = 64 << 10
	maxChildContextBytes = 128 << 10
	maxChildTurns        = 32
	maxChildTools        = 64
)

type RunRequest struct {
	RunID         string      `json:"run_id"`
	ParentRunID   string      `json:"parent_run_id"`
	PolicyProfile string      `json:"policy_profile"`
	PolicyHash    string      `json:"policy_hash"`
	WorkspaceID   string      `json:"workspace_id"`
	Text          string      `json:"text"`
	System        string      `json:"system,omitempty"`
	ToolName      string      `json:"tool_name,omitempty"`
	ToolArgs      any         `json:"tool_args,omitempty"`
	MaxTurns      int         `json:"max_turns,omitempty"`
	Tools         []ModelTool `json:"tools,omitempty"`
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
	return runServer(ctx, in, out, handler)
}

func runServer(ctx context.Context, in io.Reader, out io.Writer, handler *serverHandler) error {
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
	if len(params.Text) > maxChildTextBytes || len(params.System) > maxChildTextBytes || params.MaxTurns > maxChildTurns || len(params.Tools) > maxChildTools {
		return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "child request exceeds bounded harness limits"}
	}
	if err := peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.started"}); err != nil {
		return nil, &rpc.Error{Code: rpc.ServerOverload, Message: "worker event queue is full"}
	}
	if params.ToolName == "" && params.MaxTurns > 0 {
		return h.turnLoop(ctx, peer, params)
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

func (h *serverHandler) turnLoop(ctx context.Context, peer *rpc.Peer, params RunRequest) (any, *rpc.Error) {
	maxTurns := params.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 8
	}
	messages := make([]ChatMessage, 0, 2)
	if params.System != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: params.System})
	}
	messages = append(messages, ChatMessage{Role: "user", Content: params.Text})
	for turn := 0; turn < maxTurns; turn++ {
		if err := validateTurnContext(messages); err != nil {
			_ = peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.failed", Error: "child context budget exceeded"})
			return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "child context budget exceeded"}
		}
		result, err := peer.Call(ctx, "model/complete", ModelRequest{
			RequestID: newRequestID(), RunID: params.RunID, ParentRunID: params.ParentRunID,
			PolicyProfile: params.PolicyProfile, PolicyHash: params.PolicyHash,
			WorkspaceID: params.WorkspaceID, Messages: messages, Tools: params.Tools,
		})
		if err != nil {
			_ = peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.failed", Error: "model broker failed"})
			return nil, &rpc.Error{Code: rpc.InternalError, Message: "model broker failed"}
		}
		var modelResult ModelResponse
		if err := json.Unmarshal(result, &modelResult); err != nil || modelResult.Status != "completed" {
			return nil, &rpc.Error{Code: rpc.InternalError, Message: "model broker returned non-completed status"}
		}
		if len(modelResult.Message.Content) > maxChildTextBytes || len(modelResult.Message.ToolCalls) > maxChildTools {
			return nil, &rpc.Error{Code: rpc.InternalError, Message: "model response exceeds bounded harness limits"}
		}
		messages = append(messages, modelResult.Message)
		if len(modelResult.Message.ToolCalls) == 0 {
			if err := peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.completed"}); err != nil {
				return nil, &rpc.Error{Code: rpc.ServerOverload, Message: "worker event queue is full"}
			}
			return RunResult{RunID: params.RunID, Status: "completed", Result: modelResult.Message.Content}, nil
		}
		for _, call := range modelResult.Message.ToolCalls {
			toolResult, err := h.callTool(ctx, peer, params, call, "")
			if err != nil {
				_ = peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.failed", Error: "tool broker failed"})
				return nil, &rpc.Error{Code: rpc.InternalError, Message: "tool broker failed"}
			}
			if toolResult.Status == "approval_required" {
				approvalResult, err := peer.Call(ctx, "approval/wait", ApprovalWaitRequest{ApprovalID: toolResult.ApprovalID, RunID: params.RunID, ToolCallID: call.ID})
				if err != nil {
					return nil, &rpc.Error{Code: rpc.InternalError, Message: "approval wait failed"}
				}
				var decision ApprovalWaitResult
				if err := json.Unmarshal(approvalResult, &decision); err != nil {
					return nil, &rpc.Error{Code: rpc.InternalError, Message: "invalid approval wait result"}
				}
				if decision.Decision == "approved" {
					toolResult, err = h.callTool(ctx, peer, params, call, toolResult.ApprovalID)
					if err != nil {
						return nil, &rpc.Error{Code: rpc.InternalError, Message: "approved tool execution failed"}
					}
				} else {
					toolResult.Status = decision.Decision
					toolResult.Result = "tool call denied by the user"
				}
			}
			content := toolResult.Result
			if toolResult.Error != "" {
				content = toolResult.Error
			}
			messages = append(messages, ChatMessage{Role: "tool", ToolCallID: call.ID, Content: content})
		}
	}
	_ = peer.Notify("worker/event", Event{RunID: params.RunID, Type: "worker.failed", Error: "child turn budget exceeded"})
	return nil, &rpc.Error{Code: rpc.InternalError, Message: "child turn budget exceeded"}
}

func validateTurnContext(messages []ChatMessage) error {
	if len(messages) > maxChildTurns*maxChildTools+1 {
		return errors.New("too many child messages")
	}
	bytes := 0
	for _, message := range messages {
		if len(message.Content) > maxChildTextBytes {
			return errors.New("child message is too large")
		}
		bytes += len(message.Role) + len(message.Content) + len(message.ToolCallID)
		for _, call := range message.ToolCalls {
			if len(call.ID) > 256 || len(call.Name) > 256 {
				return errors.New("child tool call metadata is too large")
			}
			encoded, err := json.Marshal(call.Arguments)
			if err != nil || len(encoded) > maxChildTextBytes {
				return errors.New("child tool call arguments are too large")
			}
			bytes += len(encoded)
		}
	}
	if bytes > maxChildContextBytes {
		return errors.New("child context is too large")
	}
	return nil
}

type toolResult struct {
	Status     string `json:"status"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	ApprovalID string `json:"approval_id,omitempty"`
}

func (h *serverHandler) callTool(ctx context.Context, peer *rpc.Peer, params RunRequest, call ModelToolCall, approvalID string) (toolResult, error) {
	result, err := peer.Call(ctx, "tool/execute", map[string]any{
		"run_id": params.RunID, "parent_run_id": params.ParentRunID,
		"policy_profile": params.PolicyProfile, "policy_hash": params.PolicyHash,
		"workspace_id": params.WorkspaceID, "tool_name": call.Name, "args": call.Arguments,
		"tool_call_id": call.ID, "approval_id": approvalID,
	})
	if err != nil {
		return toolResult{}, err
	}
	var response toolResult
	if err := json.Unmarshal(result, &response); err != nil {
		return toolResult{}, err
	}
	return response, nil
}

func newRequestID() string {
	return "model-" + time.Now().UTC().Format("20060102150405.000000000")
}
