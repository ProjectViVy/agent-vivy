package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/rpc"
)

type Spec struct {
	RunID         domain.RunID         `json:"run_id"`
	ParentRunID   domain.RunID         `json:"parent_run_id"`
	PolicyProfile domain.PolicyProfile `json:"policy_profile"`
	PolicyHash    string               `json:"policy_hash"`
	WorkspaceID   string               `json:"workspace_id"`
	Text          string               `json:"text"`
	ToolName      string               `json:"tool_name,omitempty"`
	ToolArgs      any                  `json:"tool_args,omitempty"`
}

type Authority struct {
	ParentRunID domain.RunID
	Snapshot    domain.PolicySnapshot
	WorkspaceID string
	Budget      EventBudget
}

type EventBudget interface {
	ReserveEvent() error
}

func (a Authority) Validate(spec Spec) error {
	if spec.RunID == "" || spec.ParentRunID == "" || spec.PolicyProfile == "" || spec.PolicyHash == "" || spec.WorkspaceID == "" {
		return errors.New("worker: spec is missing authority fields")
	}
	if a.ParentRunID != "" && spec.ParentRunID != a.ParentRunID {
		return errors.New("worker: parent run mismatch")
	}
	if spec.PolicyProfile != a.Snapshot.Profile || spec.PolicyHash != a.Snapshot.Hash {
		return errors.New("worker: policy snapshot cannot be widened")
	}
	if a.WorkspaceID != "" && spec.WorkspaceID != a.WorkspaceID {
		return errors.New("worker: workspace mismatch")
	}
	if !spec.PolicyProfile.Valid() {
		return errors.New("worker: invalid policy profile")
	}
	return nil
}

type ToolCall struct {
	RunID         domain.RunID
	ParentRunID   domain.RunID
	PolicyProfile domain.PolicyProfile
	PolicyHash    string
	WorkspaceID   string
	ToolName      string
	Args          any
}

type ToolResult struct {
	Result string `json:"result"`
}

type ToolBroker interface {
	Execute(context.Context, ToolCall) (ToolResult, error)
}

type WorkerEvent struct {
	RunID domain.RunID
	Type  string
	Error string
}

type Supervisor struct {
	cmd       *exec.Cmd
	peer      *rpc.Peer
	cancel    context.CancelFunc
	authority Authority

	mu     sync.Mutex
	closed bool
}

func Start(ctx context.Context, authority Authority, broker ToolBroker, onEvent func(WorkerEvent)) (*Supervisor, error) {
	if authority.Snapshot.Profile == "" || authority.Snapshot.Hash == "" {
		return nil, errors.New("worker: authority needs a policy snapshot")
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("worker: resolve executable: %w", err)
	}
	serveCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(serveCtx, executable, "worker")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("worker: open stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("worker: open stdout: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("worker: start process: %w", err)
	}

	handler := &supervisorHandler{authority: authority, broker: broker, onEvent: onEvent}
	transport := rpc.NewJSONLTransport(stdout, stdin, func() error {
		_ = stdin.Close()
		return stdout.Close()
	})
	peer := rpc.NewPeer(transport, handler, rpc.Options{OutgoingBuffer: 32})
	serveDone := make(chan error, 1)
	go func() { serveDone <- peer.Serve(serveCtx) }()

	initCtx, initCancel := context.WithCancel(serveCtx)
	defer initCancel()
	result, err := peer.Call(initCtx, "initialize", map[string]string{"protocol_version": rpc.ProtocolVersion})
	if err != nil {
		cancel()
		_ = cmd.Wait()
		return nil, fmt.Errorf("worker: initialize: %w", err)
	}
	var initResponse struct {
		ProtocolVersion string `json:"protocol_version"`
	}
	if err := json.Unmarshal(result, &initResponse); err != nil || initResponse.ProtocolVersion != rpc.ProtocolVersion {
		cancel()
		_ = cmd.Wait()
		return nil, errors.New("worker: protocol version mismatch")
	}

	supervisor := &Supervisor{cmd: cmd, peer: peer, cancel: cancel, authority: authority}
	go func() {
		if err := <-serveDone; err != nil && !errors.Is(err, rpc.ErrPeerClosed) {
			_ = supervisor.Close()
		}
	}()
	return supervisor, nil
}

func (s *Supervisor) Run(ctx context.Context, spec Spec) (string, error) {
	if err := s.authority.Validate(spec); err != nil {
		return "", err
	}
	if s.authority.Budget != nil {
		if err := s.authority.Budget.ReserveEvent(); err != nil {
			return "", err
		}
	}
	result, err := s.peer.Call(ctx, "worker/run", RunRequest{
		RunID: string(spec.RunID), ParentRunID: string(spec.ParentRunID),
		PolicyProfile: string(spec.PolicyProfile), PolicyHash: spec.PolicyHash,
		WorkspaceID: spec.WorkspaceID, Text: spec.Text, ToolName: spec.ToolName, ToolArgs: spec.ToolArgs,
	})
	if err != nil {
		return "", err
	}
	var response RunResult
	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("worker: decode result: %w", err)
	}
	if response.Status != "completed" {
		return "", fmt.Errorf("worker: returned status %q", response.Status)
	}
	return response.Result, nil
}

func (s *Supervisor) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	if err := s.cmd.Wait(); err != nil {
		return err
	}
	return nil
}

type supervisorHandler struct {
	authority Authority
	broker    ToolBroker
	onEvent   func(WorkerEvent)
}

func (h *supervisorHandler) Handle(ctx context.Context, _ *rpc.Peer, request rpc.Request) (any, *rpc.Error) {
	switch request.Method {
	case "worker/event":
		var event Event
		if err := json.Unmarshal(request.Params, &event); err != nil {
			return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "invalid worker event"}
		}
		if h.onEvent != nil {
			h.onEvent(WorkerEvent{RunID: domain.RunID(event.RunID), Type: event.Type, Error: event.Error})
		}
		return nil, nil
	case "tool/execute":
		if h.broker == nil {
			return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: "tool broker is not configured"}
		}
		var data struct {
			RunID         string `json:"run_id"`
			ParentRunID   string `json:"parent_run_id"`
			PolicyProfile string `json:"policy_profile"`
			PolicyHash    string `json:"policy_hash"`
			WorkspaceID   string `json:"workspace_id"`
			ToolName      string `json:"tool_name"`
			Args          any    `json:"args"`
		}
		if err := json.Unmarshal(request.Params, &data); err != nil {
			return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "invalid tool broker request"}
		}
		if data.ToolName == "" {
			return nil, &rpc.Error{Code: rpc.InvalidParams, Message: "tool_name is required"}
		}
		spec := Spec{RunID: domain.RunID(data.RunID), ParentRunID: domain.RunID(data.ParentRunID), PolicyProfile: domain.PolicyProfile(data.PolicyProfile), PolicyHash: data.PolicyHash, WorkspaceID: data.WorkspaceID}
		if err := h.authority.Validate(spec); err != nil {
			return nil, &rpc.Error{Code: rpc.InvalidParams, Message: err.Error()}
		}
		result, err := h.broker.Execute(ctx, ToolCall{
			RunID: spec.RunID, ParentRunID: spec.ParentRunID, PolicyProfile: spec.PolicyProfile,
			PolicyHash: spec.PolicyHash, WorkspaceID: spec.WorkspaceID, ToolName: data.ToolName, Args: data.Args,
		})
		if err != nil {
			return nil, &rpc.Error{Code: rpc.InternalError, Message: "tool broker rejected the request"}
		}
		return result, nil
	default:
		return nil, &rpc.Error{Code: rpc.MethodNotFound, Message: "unknown worker request"}
	}
}
