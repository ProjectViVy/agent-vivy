package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"agent-vivy/internal/domain"
)

// TestModelCallPayloadSchemas pins the OBS-02 wire contract: the closed
// schemas validate real journaled lifecycle events (v3 request, v2 usage,
// v1 call.finished) plus legacy/v2 fixtures, and reject payloads that fall
// outside every version branch.
func TestModelCallPayloadSchemas(t *testing.T) {
	svc, backend, _ := newObservedService(t, &scriptedUsageModel{chunks: usageScript()}, DefaultBudgetPolicy())
	mustCreateSession(t, backend, "sess-schema")
	ctx := context.Background()
	runID, err := svc.Run(ctx, "sess-schema", "hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	k := classifyObservedCall(replayAll(t, backend, runID))
	if len(k.requests) != 1 || len(k.samples) == 0 || len(k.finished) != 1 {
		t.Fatalf("lifecycle = %d requests, %d samples, %d finishes", len(k.requests), len(k.samples), len(k.finished))
	}

	t.Run("actual v5 request validates", func(t *testing.T) {
		if k.requests[0].PayloadVersion != 5 {
			t.Fatalf("request version = %d, want 5", k.requests[0].PayloadVersion)
		}
		validatePayloadSchema(t, "model.request", k.requests[0].Payload)
	})
	t.Run("actual v2 usage samples validate", func(t *testing.T) {
		for i, ev := range k.samples {
			if ev.PayloadVersion != 2 {
				t.Fatalf("sample %d version = %d, want 2", i, ev.PayloadVersion)
			}
			validatePayloadSchema(t, "model.usage", ev.Payload)
		}
	})
	t.Run("actual finish validates", func(t *testing.T) {
		if k.finished[0].PayloadVersion != 1 {
			t.Fatalf("finish version = %d, want 1", k.finished[0].PayloadVersion)
		}
		validatePayloadSchema(t, "model.call.finished", k.finished[0].Payload)
	})

	// Serialized fixtures: legacy v1/v2 requests, a Context-View-less v3
	// request (summary route), v1 usage, and finish variants.

	fixtures := []struct {
		name    string
		event   string
		payload string
	}{
		{"request v3 summary without view", "model.request", `{"selected_tools":[],"preamble_bytes":0,"messages":[],"call_id":"c1","mode":"generate","provider":"p","model":"m","source":"summary"}`},
		{"request v5 full shape", "model.request", `{"selected_tools":["echo_info"],"preamble_bytes":3,"messages":[{"role":"system","byte_len":3,"blocks":[{"kind":"user_input_text","byte_len":3}]},{"role":"assistant","byte_len":0,"blocks":[{"kind":"function_tool_call","byte_len":0,"call_id":"c1","name":"echo_info"}]},{"role":"tool","byte_len":2,"tool_call_id":"c1","blocks":[{"kind":"function_tool_result","byte_len":2,"call_id":"c1","name":"echo_info","content_kinds":["text"]}]}],"call_id":"call-1","mode":"stream","provider":"p","model":"m","source":"main","call_ids":["c1"],"result_call_ids":["c1"],"tool_declarations":[{"name":"echo_info","parameters":[{"name":"text","type":"string","required":true}]}],"counts":{"messages":3,"blocks":3,"tool_calls":1,"tool_results":1,"system":1,"user":0,"assistant":1,"tool":1},"shape_truncated":false}`},
		{"usage v1 legacy", "model.usage", `{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}`},
		{"usage v2 full", "model.usage", `{"call_id":"c1","provider":"p","model":"m","source":"main","usage_kind":"cumulative","prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"reasoning_tokens":2,"cached_tokens":4,"normalization_partial":true,"settlement":true}`},
		{"finish failed with error", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"child","status":"failed","error":{"name":"ProviderError","message":"boom"},"response_complete":false}`},
		{"finish failed with wire", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"failed","error":{"name":"ProviderError","message":"invalid params, 400 (2013)"},"response_complete":false,"wire":{"status":400,"model":"m","stream":true,"messages":7,"roles":["system","system","user","user","assistant","user"],"content_forms":["string","string","array","array","array","string"],"call_ids":["c1"],"result_call_ids":["c1"],"tool_count":14,"tool_names":["echo_info"],"tool_choice":"auto","extra_keys":["reasoning_effort"],"body_bytes":27495,"truncated":false}}`},
		{"finish completed with usage", "model.call.finished", `{"call_id":"c1","mode":"generate","provider":"p","model":"m","source":"summary","status":"completed","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3,"reasoning_tokens":1},"response_complete":true}`},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			validatePayloadSchema(t, f.event, json.RawMessage(f.payload))
		})
	}

	invalid := []struct {
		name    string
		event   string
		payload string
	}{
		{"request v3 missing source", "model.request", `{"selected_tools":[],"preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream","provider":"p","model":"m"}`},
		{"request unknown field", "model.request", `{"selected_tools":[],"preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","bogus":1}`},
		{"request bad mode enum", "model.request", `{"selected_tools":[],"preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream2","provider":"p","model":"m","source":"main"}`},
		{"request v5 unknown block kind", "model.request", `{"selected_tools":[],"preamble_bytes":0,"messages":[{"role":"user","byte_len":0,"blocks":[{"kind":"bogus_block","byte_len":0}]}],"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","counts":{"messages":1,"blocks":1,"tool_calls":0,"tool_results":0,"system":0,"user":1,"assistant":0,"tool":0}}`},
		{"request v5 call ids not strings", "model.request", `{"selected_tools":[],"preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","call_ids":[1],"counts":{"messages":0,"blocks":0,"tool_calls":0,"tool_results":0,"system":0,"user":0,"assistant":0,"tool":0}}`},
		{"request v5 counts incomplete", "model.request", `{"selected_tools":[],"preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","counts":{"messages":0,"blocks":0,"tool_calls":0,"tool_results":0,"system":0,"user":0,"assistant":0}}`},
		{"usage v2 missing usage_kind", "model.usage", `{"call_id":"c1","provider":"p","model":"m","source":"main","prompt_tokens":1,"completion_tokens":2,"total_tokens":3}`},
		{"usage bad source enum", "model.usage", `{"call_id":"c1","provider":"p","model":"m","source":"x","usage_kind":"cumulative","prompt_tokens":1,"completion_tokens":2,"total_tokens":3}`},
		{"finish bad status enum", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"done","response_complete":true}`},
		{"finish unknown field", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"completed","response_complete":true,"latency_ms":5}`},
		{"finish missing response_complete", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"completed"}`},
		{"finish wire unknown field", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"failed","response_complete":false,"wire":{"status":400,"stream":true,"messages":1,"tool_count":0,"body_bytes":10,"raw_body":"secret"}}`},
		{"finish wire missing status", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"failed","response_complete":false,"wire":{"stream":true,"messages":1,"tool_count":0,"body_bytes":10}}`},
		{"finish wire bad content form", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"failed","response_complete":false,"wire":{"status":400,"stream":true,"messages":1,"content_forms":["blob"],"tool_count":0,"body_bytes":10}}`},
	}
	for _, f := range invalid {
		t.Run("reject "+f.name, func(t *testing.T) {
			if err := payloadSchemaError(t, f.event, json.RawMessage(f.payload)); err == nil {
				t.Fatalf("%s: invalid payload validated", f.name)
			}
		})
	}
}

// payloadSchemaError compiles payloads/<eventType>.json and returns the
// violation for payload, or nil when it validates.
func payloadSchemaError(t *testing.T, eventType string, payload json.RawMessage) error {
	t.Helper()
	rawSchema, err := os.ReadFile(filepath.Join("..", "..", "schemas", "events", "payloads", eventType+".json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(rawSchema))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(eventType, document); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(eventType)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	return compiled.Validate(instance)
}
