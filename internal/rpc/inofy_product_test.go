package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/actionhost"
)

const inofyRPCDefinition = `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
	{"id":"a","kind":"call","type":"vivy.child-task@1","config":{"task":"rpc node","tool_names":["echo_info"]}}
],"edges":[],"exits":["a"],"outputs":{"answer":{"source":"a","pointer":"/result"}}}}`

func inofyRPCArtifact() map[string]any {
	return map[string]any{"definition": json.RawMessage(inofyRPCDefinition)}
}

// callINOFY invokes one inofy.* method on a websocket-attributed peer so the
// session binding path mirrors the browser surface.
func callINOFY(t *testing.T, handler Handler, peer *Peer, method string, params any) (any, *Error) {
	t.Helper()
	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return handler.Handle(context.Background(), peer, Request{JSONRPC: "2.0", Method: method, Params: encoded})
}

func newINOFYPeer(t *testing.T, handler Handler) *Peer {
	t.Helper()
	return NewPeer(nil, handler, Options{Identity: actionhost.Identity{ID: "face/connection", Face: "web"}})
}

func createINOFYSession(t *testing.T, handler Handler) string {
	t.Helper()
	created, rpcErr := callControl(t, handler, "session/create", map[string]string{"title": "inofy"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	createdJSON, _ := json.Marshal(created)
	var session sessionResult
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	return string(session.ID)
}

func TestINOFYProductRPCSurface(t *testing.T) {
	env := newControlTestEnv(t)
	sessionID := createINOFYSession(t, env.handler)
	peer := newINOFYPeer(t, env.handler)

	caps, rpcErr := callINOFY(t, env.handler, peer, "inofy.capabilities", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	capsJSON, _ := json.Marshal(caps)
	var capabilities struct {
		SchemaVersion  string   `json:"schema_version"`
		Features       []string `json:"features"`
		SupportsWait   bool     `json:"supports_wait"`
		SupportsResume bool     `json:"supports_resume"`
	}
	if err := json.Unmarshal(capsJSON, &capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.SchemaVersion != "inofy.workflow/v1" || capabilities.SupportsWait || capabilities.SupportsResume {
		t.Fatalf("capabilities = %s", capsJSON)
	}

	types, rpcErr := callINOFY(t, env.handler, peer, "inofy.nodeTypes", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	typesJSON, _ := json.Marshal(types)
	var descriptors []map[string]any
	if err := json.Unmarshal(typesJSON, &descriptors); err != nil || len(descriptors) != 1 {
		t.Fatalf("nodeTypes = %s", typesJSON)
	}

	// Session-scoped methods reject callers with no bound session.
	if _, rpcErr := callINOFY(t, env.handler, newINOFYPeer(t, env.handler), "inofy.loadDraft", map[string]any{"workflow": "wf-rpc"}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("unbound loadDraft = %v", rpcErr)
	}

	saved, rpcErr := callINOFY(t, env.handler, peer, "inofy.saveDraft", map[string]any{
		"session_id": sessionID, "workflow": "wf-rpc", "artifact": inofyRPCArtifact(), "etag": "",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	savedJSON, _ := json.Marshal(saved)
	var draft struct {
		ETag     string          `json:"etag"`
		Artifact json.RawMessage `json:"artifact"`
	}
	if err := json.Unmarshal(savedJSON, &draft); err != nil || draft.ETag == "" {
		t.Fatalf("saveDraft = %s", savedJSON)
	}

	// Stale etag and foreign-session writes conflict.
	if _, rpcErr := callINOFY(t, env.handler, peer, "inofy.saveDraft", map[string]any{
		"workflow": "wf-rpc", "artifact": inofyRPCArtifact(), "etag": "stale",
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("stale etag saveDraft = %v", rpcErr)
	}
	otherSession := createINOFYSession(t, env.handler)
	otherPeer := newINOFYPeer(t, env.handler)
	if _, rpcErr := callINOFY(t, env.handler, otherPeer, "inofy.saveDraft", map[string]any{
		"session_id": otherSession, "workflow": "wf-rpc", "artifact": inofyRPCArtifact(), "etag": draft.ETag,
	}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("foreign saveDraft = %v", rpcErr)
	}

	validated, rpcErr := callINOFY(t, env.handler, peer, "inofy.validate", map[string]any{"workflow": "wf-rpc", "etag": draft.ETag})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	validatedJSON, _ := json.Marshal(validated)
	var outcome struct {
		Valid       bool             `json:"valid"`
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal(validatedJSON, &outcome); err != nil || !outcome.Valid {
		t.Fatalf("validate = %s", validatedJSON)
	}

	published, rpcErr := callINOFY(t, env.handler, peer, "inofy.publish", map[string]any{"workflow": "wf-rpc", "etag": draft.ETag})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	publishedJSON, _ := json.Marshal(published)
	var revision struct {
		Workflow         string `json:"workflow"`
		Revision         uint64 `json:"revision"`
		DefinitionDigest string `json:"definition_digest"`
	}
	if err := json.Unmarshal(publishedJSON, &revision); err != nil || revision.Revision != 1 || revision.DefinitionDigest == "" {
		t.Fatalf("publish = %s", publishedJSON)
	}

	fetched, rpcErr := callINOFY(t, env.handler, peer, "inofy.getRevision", map[string]any{"workflow": "wf-rpc", "revision": 1})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	fetchedJSON, _ := json.Marshal(fetched)
	var revisionView struct {
		Artifact          json.RawMessage `json:"artifact"`
		UsedCatalogDigest string          `json:"used_catalog_digest"`
	}
	if err := json.Unmarshal(fetchedJSON, &revisionView); err != nil || len(revisionView.Artifact) == 0 || revisionView.UsedCatalogDigest == "" {
		t.Fatalf("getRevision = %s", fetchedJSON)
	}

	listed, rpcErr := callINOFY(t, env.handler, peer, "inofy.listWorkflows", map[string]any{})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	listedJSON, _ := json.Marshal(listed)
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(listedJSON, &page); err != nil || len(page.Items) != 1 || page.Items[0]["workflow_id"] != "wf-rpc" {
		t.Fatalf("listWorkflows = %s", listedJSON)
	}

	// Run surface: missing runs and foreign runs answer not-found.
	for _, method := range []string{"inofy.getRun", "inofy.nodeOutput", "inofy.cancelRun", "inofy.events"} {
		params := map[string]any{"run_id": "run-missing"}
		if method == "inofy.nodeOutput" {
			params["node"] = "a"
		}
		if _, rpcErr := callINOFY(t, env.handler, peer, method, params); rpcErr == nil || rpcErr.Code != CodeNotFound {
			t.Fatalf("%s missing run = %v", method, rpcErr)
		}
	}
	if _, rpcErr := callINOFY(t, env.handler, peer, "inofy.startRun", map[string]any{"workflow": "wf-rpc", "revision": 1}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("startRun without parent = %v", rpcErr)
	}
	if _, rpcErr := callINOFY(t, env.handler, peer, "inofy.startRun", map[string]any{
		"workflow": "wf-rpc", "revision": 1, "parent_run_id": "run-missing", "operation_id": "op-1",
	}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("startRun missing parent = %v", rpcErr)
	}

	// Honest unsupported surfaces.
	if _, rpcErr := callINOFY(t, env.handler, peer, "inofy.resumeRun", map[string]any{"run_id": "run-missing", "answers": map[string]any{}}); rpcErr == nil || rpcErr.Code != CodeConflict {
		t.Fatalf("resumeRun = %v", rpcErr)
	}
	for _, method := range []string{"inofy.putConnection", "inofy.deleteConnection"} {
		if _, rpcErr := callINOFY(t, env.handler, peer, method, map[string]any{"id": "openai", "kind": "llm", "base_url": "x", "model": "y"}); rpcErr == nil || rpcErr.Code != CodeConflict {
			t.Fatalf("%s = %v", method, rpcErr)
		}
	}
	connections, rpcErr := callINOFY(t, env.handler, peer, "inofy.listConnections", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	connectionsJSON, _ := json.Marshal(connections)
	if string(connectionsJSON) != "[]" {
		t.Fatalf("listConnections = %s", connectionsJSON)
	}
}
