package app

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage/conformance"
)

// Actual authenticated control Peer -> ActionHost -> the one bound Garden C1;
// the only injected authority is the trusted #66 producer (absent in product).
func TestIssue58RecapAuthenticatedActions(t *testing.T) {
	source := conformance.Issue58Source("unused", true)
	f := newCognitiveOriginFixture(t, func(in *cognitivecontract.FactoryInput) {
		in.ResolveRecapBinding = func(context.Context, string) (cognitivecontract.RecapBinding, error) {
			return cognitivecontract.RecapBinding{SubjectID: source.SubjectID, DomainID: source.DomainID}, nil
		}
	})
	callControl(t, f.client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	sid := createCognitiveSession(t, f.client)
	source.SessionID = domain.SessionID(sid)
	conformance.Issue58Identity(t, f.backend)
	service := runtime.NewService(nil, "", "", runtime.ServiceDeps{Sessions: f.backend, MainConversation: issue58AppMain{source}})
	if err := f.bundle.AttachRuntime(&cognitiveControlPort{svc: service, bundle: f.bundle}); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"session_id": sid, "base_revision": 0, "operation_id": "http-edit", "text": "完整真实 Recap\n\n第二段\r\n"}
	raw, err := invokeCognitive(t, f.client, "vivy.recap.edit", input)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Status string `json:"status"`
		Head   struct {
			Revision uint64 `json:"revision"`
			Text     string `json:"text"`
		} `json:"head"`
	}
	json.Unmarshal(raw, &out)
	if out.Status != "applied" || out.Head.Revision != 1 || out.Head.Text != input["text"] {
		t.Fatalf("action=%s", raw)
	}
	owner := f.bundle.(cognitivecontract.RecapProvider).Recap()
	head, err := owner.ReadRecap(context.Background())
	if err != nil || head.Text != out.Head.Text || head.Revision != 1 {
		t.Fatalf("owner=%+v %v", head, err)
	}
	input["operation_id"] = "stale"
	raw, err = invokeCognitive(t, f.client, "vivy.recap.edit", input)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(raw, &out)
	if out.Status != "conflict" || out.Head.Revision != 1 {
		t.Fatalf("stale=%s", raw)
	}
	ordinary := createCognitiveSession(t, f.client)
	raw, err = invokeCognitive(t, f.client, "vivy.recap.read", map[string]any{"session_id": ordinary})
	if err != nil {
		t.Fatal(err)
	}
	var denied map[string]any
	json.Unmarshal(raw, &denied)
	if denied["reason"] != "recap_scope_denied" || denied["head"] != nil {
		t.Fatalf("ordinary=%s", raw)
	}
	if _, err = invokeCognitive(t, f.client, "vivy.recap.read", map[string]any{"session_id": sid}); err == nil {
		t.Fatal("foreign transport session reached recap")
	}
}
