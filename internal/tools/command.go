package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	ExecuteName     = "execute"
	CommandlineName = "commandline"
)

type CommandRequest struct {
	Command   string
	Args      []string
	Cwd       string
	Env       map[string]string
	TimeoutMS int
}

type CommandResult struct {
	Command     string `json:"command"`
	Cwd         string `json:"cwd"`
	ExitCode    int    `json:"exit_code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	StdoutTrunc bool   `json:"stdout_truncated,omitempty"`
	StderrTrunc bool   `json:"stderr_truncated,omitempty"`
	TimedOut    bool   `json:"timed_out,omitempty"`
	DurationMS  int64  `json:"duration_ms"`
	Untrusted   bool   `json:"untrusted"`
}

type CommandOperations interface {
	Execute(context.Context, domain.RunID, CommandRequest) (CommandResult, error)
}

type commandProposalOperations interface {
	PrepareCommand(context.Context, domain.RunID, CommandRequest) (domain.ToolProposal, error)
}

type commandTool struct {
	name string
	ops  CommandOperations
}

func NewExecute(ops CommandOperations) Tool     { return &commandTool{name: ExecuteName, ops: ops} }
func NewCommandline(ops CommandOperations) Tool { return &commandTool{name: CommandlineName, ops: ops} }

func (t *commandTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: t.name, Description: "Runs one allowlisted executable inside the current run workspace; always approval-gated.", Readonly: false, Keywords: []string{"execute", "command", "shell", "process"}, Params: map[string]domain.ToolParam{
		"command":    {Desc: "Executable basename from the configured command allowlist.", Required: true},
		"args":       {Desc: "Argument vector; no shell expansion is performed.", Type: "array"},
		"cwd":        {Desc: "Optional workspace-relative working directory."},
		"env":        {Desc: "Optional non-sensitive allowlisted environment overrides.", Type: "object"},
		"timeout_ms": {Desc: "Optional timeout, bounded by the runtime.", Type: "integer"},
	}}
}
func (t *commandTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	request, err := decodeCommandRequest(args)
	if err != nil {
		return "", err
	}
	request, err = expandExecuteRequest(t.name, request)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: %s backend not wired", t.name)
	}
	result, err := t.ops.Execute(ctx, RunIDFromContext(ctx), request)
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *commandTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	request, err := decodeCommandRequest(args)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	request, err = expandExecuteRequest(t.name, request)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if planner, ok := t.ops.(commandProposalOperations); ok {
		return planner.PrepareCommand(ctx, RunIDFromContext(ctx), request)
	}
	payload, _ := json.Marshal(request)
	return domain.ToolProposal{Action: t.name, Target: request.Command, Preview: string(payload), RiskFindings: []string{"local process execution"}, Data: payload}, nil
}

func expandExecuteRequest(name string, request CommandRequest) (CommandRequest, error) {
	if name != ExecuteName || !strings.ContainsAny(request.Command, " \t\r\n") {
		return request, nil
	}
	if strings.ContainsAny(request.Command, ";&|><$()") {
		return CommandRequest{}, fmt.Errorf("%s: command line contains shell syntax", name)
	}
	parts := strings.Fields(request.Command)
	if len(parts) == 0 {
		return CommandRequest{}, &ArgError{Field: "command", Reason: "is required"}
	}
	request.Command = parts[0]
	request.Args = append(parts[1:], request.Args...)
	return request, nil
}

func decodeCommandRequest(args json.RawMessage) (CommandRequest, error) {
	var input struct {
		Command   string            `json:"command"`
		Args      []string          `json:"args"`
		Cwd       string            `json:"cwd"`
		Env       map[string]string `json:"env"`
		TimeoutMS int               `json:"timeout_ms"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return CommandRequest{}, fmt.Errorf("command: invalid arguments: %w", err)
	}
	if strings.TrimSpace(input.Command) == "" {
		return CommandRequest{}, &ArgError{Field: "command", Reason: "is required"}
	}
	return CommandRequest{Command: strings.TrimSpace(input.Command), Args: input.Args, Cwd: strings.TrimSpace(input.Cwd), Env: input.Env, TimeoutMS: input.TimeoutMS}, nil
}
