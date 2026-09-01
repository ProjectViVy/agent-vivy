package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeAgentOps struct {
	gotTask string
	gotMask string
	result  string
	err     error
}

func (f *fakeAgentOps) StartAgentTask(_ context.Context, task, mask string) (string, error) {
	f.gotTask = task
	f.gotMask = mask
	return f.result, f.err
}

func TestAgentToolDelegatesTaskAndMask(t *testing.T) {
	ops := &fakeAgentOps{result: "sub-agent answer"}
	tool := NewAgent(ops)
	args, err := json.Marshal(map[string]string{"task": "  find the bug  ", "mask": "  terse reviewer "})
	if err != nil {
		t.Fatal(err)
	}
	out, err := tool.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if out != "sub-agent answer" {
		t.Fatalf("result = %q", out)
	}
	if ops.gotTask != "find the bug" || ops.gotMask != "terse reviewer" {
		t.Fatalf("ops got task=%q mask=%q", ops.gotTask, ops.gotMask)
	}
}

func TestAgentToolValidatesArguments(t *testing.T) {
	tool := NewAgent(&fakeAgentOps{})
	cases := []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "missing task", args: `{}`, wantErr: `"task"`},
		{name: "empty task", args: `{"task":"   "}`, wantErr: "must not be empty"},
		{name: "bad json", args: `not-json`, wantErr: "JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tool.InvokableRun(context.Background(), json.RawMessage(tc.args))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want contains %q", err, tc.wantErr)
			}
		})
	}
}

func TestAgentToolBoundsTaskAndMask(t *testing.T) {
	tool := NewAgent(&fakeAgentOps{})
	bigTask := strings.Repeat("a", maxAgentTaskBytes+1)
	if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"task":"`+bigTask+`"}`)); err == nil || !strings.Contains(err.Error(), "64") {
		t.Fatalf("oversize task err = %v, want 64 KiB bound", err)
	}
	args, err := json.Marshal(map[string]string{"task": "ok", "mask": strings.Repeat("m", maxAgentMaskBytes+1)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(context.Background(), args); err == nil || !strings.Contains(err.Error(), "2") {
		t.Fatalf("oversize mask err = %v, want 2 KiB bound", err)
	}
}

func TestAgentToolWithoutOperationsFails(t *testing.T) {
	tool := NewAgent(nil)
	_, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"task":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("err = %v, want not-wired failure", err)
	}
}

func TestAgentToolSpec(t *testing.T) {
	spec := NewAgent(nil).Spec()
	if spec.Name != AgentName {
		t.Fatalf("name = %q", spec.Name)
	}
	if !spec.Readonly {
		t.Fatal("agent tool must be readonly so sub-agents stay within the read-only surface")
	}
	if !spec.Params["task"].Required {
		t.Fatal("task param must be required")
	}
	if _, ok := spec.Params["mask"]; !ok {
		t.Fatal("mask param must exist")
	}
}

func TestRegistryRegistersAgentWithOperations(t *testing.T) {
	registry := BuiltinWithAgent(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &fakeAgentOps{})
	found := false
	for _, spec := range registry.Specs() {
		if spec.Name == AgentName {
			found = true
			if !spec.Readonly {
				t.Fatal("registered agent tool must be readonly")
			}
		}
	}
	if !found {
		t.Fatal("agent tool missing from registry built with AgentOperations")
	}
	without := BuiltinWithWeb(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, spec := range without.Specs() {
		if spec.Name == AgentName {
			t.Fatal("agent tool must not register without AgentOperations")
		}
	}
}
