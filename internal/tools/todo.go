package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	TaskCreateName = "task_create"
	TaskGetName    = "task_get"
	TaskUpdateName = "task_update"
	TaskListName   = "task_list"
)

type TodoOperations interface {
	CreateTodo(context.Context, domain.SessionID, domain.Todo) (domain.Todo, error)
	GetTodo(context.Context, domain.SessionID, string) (domain.Todo, error)
	ListTodos(context.Context, domain.SessionID) ([]domain.Todo, error)
	UpdateTodo(context.Context, domain.Todo) (domain.Todo, error)
}

type taskCreateTool struct{ ops TodoOperations }
type taskGetTool struct{ ops TodoOperations }
type taskUpdateTool struct{ ops TodoOperations }
type taskListTool struct{ ops TodoOperations }

func NewTaskCreate(ops TodoOperations) Tool { return &taskCreateTool{ops: ops} }
func NewTaskGet(ops TodoOperations) Tool    { return &taskGetTool{ops: ops} }
func NewTaskUpdate(ops TodoOperations) Tool { return &taskUpdateTool{ops: ops} }
func NewTaskList(ops TodoOperations) Tool   { return &taskListTool{ops: ops} }

func (t *taskCreateTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: TaskCreateName, Description: "Creates a durable session task with task compatible fields.", Readonly: false,
		Keywords: []string{"task", "todo", "plan", "create"}, Params: map[string]domain.ToolParam{
			"subject":     {Desc: "Short actionable task title.", Required: true},
			"description": {Desc: "Bounded task details.", Required: true},
			"active_form": {Desc: "Present-progress label.", Required: false},
			"metadata":    {Desc: "Optional JSON metadata.", Type: "object", Required: false},
		}}
}

func (t *taskCreateTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Subject     string         `json:"subject"`
		Description string         `json:"description"`
		ActiveForm  string         `json:"active_form"`
		Metadata    map[string]any `json:"metadata"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	if strings.TrimSpace(input.Subject) == "" || strings.TrimSpace(input.Description) == "" {
		return "", &ArgError{Field: "subject/description", Reason: "must not be empty"}
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: todo backend not wired")
	}
	metadata, _ := json.Marshal(input.Metadata)
	todo, err := t.ops.CreateTodo(ctx, SessionIDFromContext(ctx), domain.Todo{Subject: input.Subject, Description: input.Description, ActiveForm: input.ActiveForm, Metadata: metadata, Status: domain.TodoPending})
	if err != nil {
		return "", err
	}
	return marshalToolResult(todo)
}

func (t *taskGetTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: TaskGetName, Description: "Gets one durable session task.", Readonly: true,
		Keywords: []string{"task", "todo", "get", "status"}, Params: map[string]domain.ToolParam{"task_id": {Desc: "Task ID.", Required: true}}}
}

func (t *taskGetTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		TaskID string `json:"task_id"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: todo backend not wired")
	}
	todo, err := t.ops.GetTodo(ctx, SessionIDFromContext(ctx), input.TaskID)
	if err != nil {
		return "", err
	}
	return marshalToolResult(todo)
}

func (t *taskUpdateTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: TaskUpdateName, Description: "Updates a durable task and validates dependencies/status invariants.", Readonly: false,
		Keywords: []string{"task", "todo", "update", "complete", "progress"}, Params: map[string]domain.ToolParam{
			"task_id":        {Desc: "Task ID.", Required: true},
			"subject":        {Desc: "Optional new title.", Required: false},
			"description":    {Desc: "Optional new description.", Required: false},
			"active_form":    {Desc: "Optional progress label.", Required: false},
			"status":         {Desc: "pending, in_progress, completed, or cancelled.", Enum: []string{"pending", "in_progress", "completed", "cancelled"}, Required: false},
			"add_blocks":     {Desc: "Task IDs this task blocks.", Type: "array", Required: false},
			"add_blocked_by": {Desc: "Task IDs that block this task.", Type: "array", Required: false},
			"owner":          {Desc: "Optional owner.", Required: false},
			"metadata":       {Desc: "Optional metadata merge object.", Type: "object", Required: false},
		}}
}

func (t *taskUpdateTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		TaskID       string         `json:"task_id"`
		Subject      string         `json:"subject"`
		Description  string         `json:"description"`
		ActiveForm   string         `json:"active_form"`
		Status       string         `json:"status"`
		AddBlocks    []string       `json:"add_blocks"`
		AddBlockedBy []string       `json:"add_blocked_by"`
		Owner        string         `json:"owner"`
		Metadata     map[string]any `json:"metadata"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: todo backend not wired")
	}
	current, err := t.ops.GetTodo(ctx, SessionIDFromContext(ctx), input.TaskID)
	if err != nil {
		return "", err
	}
	if input.Subject != "" {
		current.Subject = input.Subject
	}
	if input.Description != "" {
		current.Description = input.Description
	}
	if input.ActiveForm != "" {
		current.ActiveForm = input.ActiveForm
	}
	if input.Status != "" {
		current.Status = domain.TodoStatus(input.Status)
	}
	current.Blocks = appendUniqueStrings(current.Blocks, input.AddBlocks...)
	current.BlockedBy = appendUniqueStrings(current.BlockedBy, input.AddBlockedBy...)
	if input.Owner != "" {
		current.Owner = input.Owner
	}
	if input.Metadata != nil {
		current.Metadata, _ = json.Marshal(input.Metadata)
	}
	updated, err := t.ops.UpdateTodo(ctx, current)
	if err != nil {
		return "", err
	}
	return marshalToolResult(updated)
}

func (t *taskListTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: TaskListName, Description: "Lists durable tasks for the current session.", Readonly: true, Keywords: []string{"task", "todo", "list", "plan"}}
}

func (t *taskListTool) InvokableRun(ctx context.Context, _ json.RawMessage) (string, error) {
	if t.ops == nil {
		return "", fmt.Errorf("tools: todo backend not wired")
	}
	todos, err := t.ops.ListTodos(ctx, SessionIDFromContext(ctx))
	if err != nil {
		return "", err
	}
	return marshalToolResult(todos)
}

func appendUniqueStrings(base []string, items ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(items))
	out := append([]string(nil), base...)
	for _, item := range out {
		seen[item] = struct{}{}
	}
	for _, item := range items {
		if item != "" {
			if _, ok := seen[item]; !ok {
				out = append(out, item)
				seen[item] = struct{}{}
			}
		}
	}
	return out
}
