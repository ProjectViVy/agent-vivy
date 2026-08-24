package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// WorkerModelMessage is the runtime-side model message. It deliberately
// mirrors only the bounded worker contract and does not retain history.
type WorkerModelMessage struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []WorkerModelToolCall
}

type WorkerModelToolCall struct {
	ID        string
	Name      string
	Arguments any
}

type WorkerModelParam struct {
	Description string
	Required    bool
}

type WorkerModelTool struct {
	Name        string
	Description string
	Readonly    bool
	Params      map[string]WorkerModelParam
}

type WorkerModelRequest struct {
	RunID       string
	ParentRunID string
	Messages    []WorkerModelMessage
	Tools       []WorkerModelTool
}

type WorkerModelResponse struct {
	Status     string
	Message    WorkerModelMessage
	StopReason string
	Usage      *WorkerModelUsage
}

type WorkerModelUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	ReasoningTokens  int
}

// WorkerModelBroker is the parent-owned model boundary used by supervised
// child workers. The app layer adapts the JSON-RPC wire shape to this seam.
type WorkerChatModel interface {
	Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error)
	WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error)
}

type WorkerModelBroker struct {
	model  WorkerChatModel
	budget *BudgetLedger
}

// NewWorkerModelBroker returns a broker over the already configured parent
// model. The optional ledger makes every child model turn consume the same
// parent run-tree budget.
func NewWorkerModelBroker(m WorkerChatModel, budget *BudgetLedger) (*WorkerModelBroker, error) {
	if m == nil {
		return nil, fmt.Errorf("runtime: worker model broker needs a model")
	}
	return &WorkerModelBroker{model: m, budget: budget}, nil
}

// Complete performs one bounded model turn. It does not retain history;
// the worker sends the current child turn context on every call.
func (b *WorkerModelBroker) Complete(ctx context.Context, request WorkerModelRequest) (WorkerModelResponse, error) {
	if b == nil || b.model == nil {
		return WorkerModelResponse{}, fmt.Errorf("runtime: worker model broker is not configured")
	}
	if len(request.Messages) == 0 || request.RunID == "" || request.ParentRunID == "" {
		return WorkerModelResponse{}, fmt.Errorf("runtime: worker model request is incomplete")
	}
	if b.budget != nil {
		if err := b.budget.ReserveModelCall(); err != nil {
			return WorkerModelResponse{}, err
		}
	}
	tools, err := workerToolInfos(request.Tools)
	if err != nil {
		return WorkerModelResponse{}, err
	}
	bound, err := b.model.WithTools(tools)
	if err != nil {
		return WorkerModelResponse{}, fmt.Errorf("runtime: bind worker tools: %w", err)
	}
	message, err := bound.Generate(ctx, workerSchemaMessages(request.Messages))
	if err != nil {
		return WorkerModelResponse{}, fmt.Errorf("runtime: complete worker model turn: %w", err)
	}
	if message == nil {
		return WorkerModelResponse{}, fmt.Errorf("runtime: worker model returned nil message")
	}
	response := WorkerModelResponse{
		Status:  "completed",
		Message: workerMessage(message),
	}
	if message.ResponseMeta != nil {
		response.StopReason = message.ResponseMeta.FinishReason
		if usage := message.ResponseMeta.Usage; usage != nil {
			response.Usage = &WorkerModelUsage{
				PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
				TotalTokens:     usage.TotalTokens,
				ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
			}
		}
	}
	return response, nil
}

func workerToolInfos(tools []WorkerModelTool) ([]*schema.ToolInfo, error) {
	result := make([]*schema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			return nil, fmt.Errorf("runtime: worker tool name is empty")
		}
		params := make(map[string]*schema.ParameterInfo, len(tool.Params))
		for name, param := range tool.Params {
			params[name] = &schema.ParameterInfo{Type: schema.String, Desc: param.Description, Required: param.Required}
		}
		info := &schema.ToolInfo{Name: tool.Name, Desc: tool.Description}
		if len(params) > 0 {
			info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
		}
		result = append(result, info)
	}
	return result, nil
}

func workerSchemaMessages(messages []WorkerModelMessage) []*schema.Message {
	result := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		converted := &schema.Message{Role: workerSchemaRole(message.Role), Content: message.Content, ToolCallID: message.ToolCallID}
		for _, call := range message.ToolCalls {
			arguments, _ := json.Marshal(call.Arguments)
			converted.ToolCalls = append(converted.ToolCalls, schema.ToolCall{
				ID: call.ID, Type: "function",
				Function: schema.FunctionCall{Name: call.Name, Arguments: string(arguments)},
			})
		}
		result = append(result, converted)
	}
	return result
}

func workerSchemaRole(role string) schema.RoleType {
	switch role {
	case "system":
		return schema.System
	case "tool":
		return schema.Tool
	case "assistant":
		return schema.Assistant
	default:
		return schema.User
	}
}

func workerMessage(message *schema.Message) WorkerModelMessage {
	result := WorkerModelMessage{Role: string(message.Role), Content: message.Content, ToolCallID: message.ToolCallID}
	for _, call := range message.ToolCalls {
		var arguments any
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil {
				arguments = call.Function.Arguments
			}
		}
		result.ToolCalls = append(result.ToolCalls, WorkerModelToolCall{ID: call.ID, Name: call.Function.Name, Arguments: arguments})
	}
	return result
}
