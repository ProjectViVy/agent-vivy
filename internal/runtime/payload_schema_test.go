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

	t.Run("actual v3 request validates", func(t *testing.T) {
		if k.requests[0].PayloadVersion != 3 {
			t.Fatalf("request version = %d, want 3", k.requests[0].PayloadVersion)
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
	digest := sha256Hex([]byte("x"))
	fixtures := []struct {
		name    string
		event   string
		payload string
	}{
		{"request v1 legacy", "model.request", `{"selected_tools":["a"],"preamble_sha256":"` + digest + `","preamble_bytes":1,"messages":[]}`},
		{"request v2 context view", "model.request", `{"selected_tools":[],"preamble_sha256":"` + digest + `","preamble_bytes":0,"messages":[],"context_view":"view"}`},
		{"request v3 summary without view", "model.request", `{"selected_tools":[],"preamble_sha256":"` + digest + `","preamble_bytes":0,"messages":[],"call_id":"c1","mode":"generate","provider":"p","model":"m","source":"summary"}`},
		{"usage v1 legacy", "model.usage", `{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}`},
		{"usage v2 full", "model.usage", `{"call_id":"c1","provider":"p","model":"m","source":"main","usage_kind":"cumulative","prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"reasoning_tokens":2,"cached_tokens":4,"normalization_partial":true,"settlement":true}`},
		{"finish failed with error", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"child","status":"failed","error":{"name":"ProviderError","message":"boom"},"response_complete":false}`},
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
		{"request v3 missing source", "model.request", `{"selected_tools":[],"preamble_sha256":"` + digest + `","preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream","provider":"p","model":"m"}`},
		{"request unknown field", "model.request", `{"selected_tools":[],"preamble_sha256":"` + digest + `","preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","bogus":1}`},
		{"request bad mode enum", "model.request", `{"selected_tools":[],"preamble_sha256":"` + digest + `","preamble_bytes":0,"messages":[],"call_id":"c1","mode":"stream2","provider":"p","model":"m","source":"main"}`},
		{"usage v2 missing usage_kind", "model.usage", `{"call_id":"c1","provider":"p","model":"m","source":"main","prompt_tokens":1,"completion_tokens":2,"total_tokens":3}`},
		{"usage bad source enum", "model.usage", `{"call_id":"c1","provider":"p","model":"m","source":"x","usage_kind":"cumulative","prompt_tokens":1,"completion_tokens":2,"total_tokens":3}`},
		{"finish bad status enum", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"done","response_complete":true}`},
		{"finish unknown field", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"completed","response_complete":true,"latency_ms":5}`},
		{"finish missing response_complete", "model.call.finished", `{"call_id":"c1","mode":"stream","provider":"p","model":"m","source":"main","status":"completed"}`},
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
