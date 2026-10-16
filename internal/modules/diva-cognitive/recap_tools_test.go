package divacognitive

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/toolhost"
	"agent-vivy/internal/tools"
)

type recapBoundToolHost struct{ implementation tools.Tool }

func (h recapBoundToolHost) ModuleID() string { return "vivy/recap-tools" }
func (h recapBoundToolHost) InvokeTool(ctx context.Context, id string, raw json.RawMessage) (string, error) {
	if id != h.implementation.Spec().Name {
		panic("foreign implementation")
	}
	return h.implementation.InvokableRun(ctx, raw)
}
func TestIssue58RecapToolsSameOwner(t *testing.T) {
	b, c := managedRecap(t)
	implementations := NewRecapRuntimeTools(func() cognitivecontract.Bundle { return b })
	bindings := []toolhost.StaticBinding{}
	for i, provider := range RecapToolProviders() {
		bindings = append(bindings, toolhost.StaticBinding{OwnerID: "vivy/recap-tools", Provider: provider, Host: recapBoundToolHost{implementations[i]}})
	}
	host, err := toolhost.New(toolhost.Config{Static: bindings})
	if err != nil {
		t.Fatal(err)
	}
	ctx := tools.WithRunID(tools.WithSessionID(context.Background(), domain.SessionID("main")), domain.RunID("run-main"))
	result, err := host.Invoke(ctx, toolhost.Request{ID: "actmem_update", Args: json.RawMessage(`{"base_revision":0,"operation_id":"tool-edit","text":"同一个 owner\n完整多行\n"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var out recapWireTest
	json.Unmarshal([]byte(result.Text), &out)
	if out.Status != "applied" || out.Head == nil || out.Head.Revision != 1 {
		t.Fatalf("tool=%s", result.Text)
	}
	got := recapCall(t, b, "read", map[string]any{"session_id": "main"})
	if got.Head == nil || got.Head.Text != "同一个 owner\n完整多行\n" || got.Head.Revision != 1 {
		t.Fatalf("action=%+v", got)
	}
	for _, badCtx := range []context.Context{context.Background(), tools.WithSessionID(ctx, "worker"), tools.WithRunID(ctx, "unknown")} {
		if _, err = host.Invoke(badCtx, toolhost.Request{ID: "actmem_read", Args: json.RawMessage(`{}`)}); err == nil {
			t.Fatal("guessed call authorized")
		}
	}
	for _, raw := range []string{`{"session_id":"main"}`, `{"subject_id":"subject"}`, `{"receipt":"runtime"}`} {
		if _, err = host.Invoke(ctx, toolhost.Request{ID: "actmem_read", Args: json.RawMessage(raw)}); err == nil {
			t.Fatal("model identity/receipt accepted")
		}
	}
	c.deny = true
	if _, err = host.Invoke(ctx, toolhost.Request{ID: "actmem_clear", Args: json.RawMessage(`{"base_revision":1,"operation_id":"denied"}`)}); err == nil {
		t.Fatal("revoked call authorized")
	}
	c.deny = false
	head, _ := recapPort(t, b).ReadRecap(context.Background())
	if head.Revision != 1 {
		t.Fatal("denied mutation changed owner")
	}
}

func TestIssue58RecapToolsRejectMalformedExactText(t *testing.T) {
	b, _ := managedRecap(t)
	ctx := tools.WithRunID(tools.WithSessionID(context.Background(), "main"), "run-main")
	var update tools.Tool
	for _, tool := range NewRecapRuntimeTools(func() cognitivecontract.Bundle { return b }) {
		if tool.Spec().Name == "actmem_update" {
			update = tool
		}
	}
	for _, raw := range []json.RawMessage{
		append(append([]byte(`{"base_revision":0,"operation_id":"invalid","text":"`), 0xff), []byte(`"}`)...),
		json.RawMessage(`{"base_revision":0,"operation_id":"invalid","text":"\ud800"}`),
		json.RawMessage(`{"base_revision":0,"operation_id":"invalid","text":"x","text":"y"}`),
	} {
		if _, err := update.InvokableRun(ctx, raw); err == nil {
			t.Fatal("tool silently normalized or accepted duplicate exact text")
		}
	}
	head, _ := recapPort(t, b).ReadRecap(context.Background())
	if head.Revision != 0 {
		t.Fatal("invalid input committed")
	}
}
