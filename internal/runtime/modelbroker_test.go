package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestWorkerModelBrokerTranslatesToolCalls(t *testing.T) {
	model := NewScriptedModel(schema.AssistantMessage("", []schema.ToolCall{{
		ID: "call-1", Function: schema.FunctionCall{Name: "echo_info", Arguments: `{"text":"hello"}`},
	}}))
	broker, err := NewWorkerModelBroker(model, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := broker.Complete(context.Background(), WorkerModelRequest{
		RunID: "child-1", ParentRunID: "run-1",
		Messages: []WorkerModelMessage{{Role: "user", Content: "echo hello"}},
		Tools: []WorkerModelTool{{Name: "echo_info", Params: map[string]WorkerModelParam{
			"text": {Description: "text", Required: true},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "completed" || len(response.Message.ToolCalls) != 1 {
		t.Fatalf("response = %+v", response)
	}
	call := response.Message.ToolCalls[0]
	if call.ID != "call-1" || call.Name != "echo_info" {
		t.Fatalf("tool call = %+v", call)
	}
	var args map[string]string
	if err := jsonUnmarshal(call.Arguments, &args); err != nil || args["text"] != "hello" {
		t.Fatalf("arguments = %#v, err = %v", call.Arguments, err)
	}
}

func TestWorkerModelBrokerSharesBudget(t *testing.T) {
	ledger, err := NewBudgetLedger(BudgetPolicy{MaxModelCalls: 1})
	if err != nil {
		t.Fatal(err)
	}
	model := NewScriptedModel(schema.AssistantMessage("done", nil), schema.AssistantMessage("again", nil))
	broker, err := NewWorkerModelBroker(model, ledger)
	if err != nil {
		t.Fatal(err)
	}
	request := WorkerModelRequest{RunID: "child-1", ParentRunID: "run-1", Messages: []WorkerModelMessage{{Role: "user", Content: "hello"}}}
	if _, err := broker.Complete(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Complete(context.Background(), request); err == nil {
		t.Fatal("second model call must trip the shared budget")
	}
}

func jsonUnmarshal(value any, target any) error {
	// Keep this test package independent of the worker wire JSON details while
	// still checking that the adapter preserves structured arguments.
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}
