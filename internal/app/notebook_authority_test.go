package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
)

// notebookAuthorityConfig keeps the default governance profile with no
// default decision: notebook EffectWrite actions fall to policy prompt,
// which is exactly the decision the human-origin exemption relaxes.
func notebookAuthorityConfig(t *testing.T, rules ...config.GovernanceRule) config.Config {
	t.Helper()
	cfg := uninitializedDeepSeekTestConfig(t)
	initializeTestPersona(t, cfg)
	cfg.Governance = config.Governance{
		Profile: string(domain.PolicyProfileDefault),
		Profiles: map[string]config.GovernanceProfile{
			string(domain.PolicyProfileDefault): {Rules: rules},
		},
	}
	return cfg
}

func callSmokeErr(t *testing.T, client *rpcSmokeClient, method string, params any) (json.RawMessage, string) {
	t.Helper()
	id := fmt.Sprintf("%d", client.next+1)
	client.next++
	sendSmokeID(t, client, id, method, params)
	for {
		var message rpcSmokeMessage
		if err := client.conn.ReadJSON(&message); err != nil {
			t.Fatalf("read RPC %s: %v", method, err)
		}
		if message.ID != id {
			continue
		}
		if message.Error != nil {
			return nil, message.Error.Message
		}
		return message.Result, ""
	}
}

func newNotebookAuthorityApp(t *testing.T, cfg config.Config) (*App, *rpcSmokeClient, func()) {
	t.Helper()
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "notebook-authority-test-key")
	assembly := genassembly.BuildDefault()
	assembly.GenerationID = "generation-notebook-authority-test"
	a, err := NewWithAssembly(context.Background(), cfg, assembly)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(a.httpServer.Handler)
	client := connectSmokeRPC(t, server.URL, a.rpcToken)
	callSmoke(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
	return a, client, func() {
		_ = client.conn.Close()
		server.Close()
		_ = a.Close()
	}
}

// TestNotebookOriginCannotBeForged sends JSON claiming human origin, actor,
// scope_id, and generated provenance. The strict action schema rejects extra
// fields before any decode, and a foreign workspace can never be claimed.
func TestNotebookOriginCannotBeForged(t *testing.T) {
	_, client, cleanup := newNotebookAuthorityApp(t, notebookAuthorityConfig(t))
	defer cleanup()

	created := callSmoke(t, client, "session/create", map[string]string{"title": "authority"})
	var session struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, created, &session)

	forgery := map[string]any{
		"operation_key": "forge-1",
		"origin":        "human",
		"actor":         "peer:admin",
		"scope_id":      "ws.v1:other-session",
		"request":       map[string]string{"section_id": "section-notes", "title": "t", "markdown": "m"},
	}
	_, errText := callSmokeErr(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.create",
		"input":     forgery,
	})
	if errText == "" {
		t.Fatal("forged authority claims accepted")
	}

	// A nested claim inside request is equally rejected (strict inner schema).
	_, errText = callSmokeErr(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.create",
		"input": map[string]any{
			"operation_key": "forge-2",
			"request":       map[string]string{"section_id": "section-notes", "title": "t", "markdown": "m", "origin": "human"},
		},
	})
	if errText == "" {
		t.Fatal("nested forged claim accepted")
	}

	// Nothing was written through the forged attempts.
	listed := callSmoke(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.list",
		"input":     map[string]any{},
	})
	if !strings.Contains(string(listed), `"status":"ok"`) || strings.Contains(string(listed), `"title"`) {
		t.Fatalf("forged invocations persisted entries: %s", listed)
	}
}

// A direct authenticated human edit is admitted under the normal policy
// profile without full-auto — the narrow trusted-origin rule relaxes only the
// profile default prompt.
func TestNotebookHumanEditAdmittedUnderDefaultProfile(t *testing.T) {
	_, client, cleanup := newNotebookAuthorityApp(t, notebookAuthorityConfig(t))
	defer cleanup()

	created := callSmoke(t, client, "session/create", map[string]string{"title": "human edit"})
	var session struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, created, &session)

	result := callSmoke(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.create",
		"input": map[string]any{
			"operation_key": "human-create-1",
			"request":       map[string]string{"section_id": "section-notes", "title": "seed", "markdown": "human body"},
		},
	})
	if !strings.Contains(string(result), `"status":"ok"`) {
		t.Fatalf("human edit denied: %s", result)
	}

	// Replays reconcile: same operation key returns the original receipt.
	replay := callSmoke(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.create",
		"input": map[string]any{
			"operation_key": "human-create-1",
			"request":       map[string]string{"section_id": "section-notes", "title": "seed", "markdown": "human body"},
		},
	})
	if !strings.Contains(string(replay), `"replayed":true`) {
		t.Fatalf("replayed receipt missing: %s", replay)
	}
}

// An explicit deny rule remains decisive over the human-origin exemption.
func TestNotebookExplicitDenyStillDenies(t *testing.T) {
	cfg := notebookAuthorityConfig(t, config.GovernanceRule{Tool: "vivy.notebook.entries.create", Decision: "deny"})
	_, client, cleanup := newNotebookAuthorityApp(t, cfg)
	defer cleanup()

	created := callSmoke(t, client, "session/create", map[string]string{"title": "deny"})
	var session struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, created, &session)

	_, errText := callSmokeErr(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.create",
		"input": map[string]any{
			"operation_key": "denied-1",
			"request":       map[string]string{"section_id": "section-notes", "title": "t", "markdown": "m"},
		},
	})
	if errText == "" {
		t.Fatal("explicit deny admitted a human write")
	}
	listed := callSmoke(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.list",
		"input":     map[string]any{},
	})
	if strings.Contains(string(listed), `"title"`) {
		t.Fatalf("denied invocation persisted an entry: %s", listed)
	}
}

// An explicit prompt rule also stays decisive — the exemption only relaxes
// the profile default, never an operator-written rule.
func TestNotebookExplicitPromptRuleStaysDecisive(t *testing.T) {
	cfg := notebookAuthorityConfig(t, config.GovernanceRule{Tool: "vivy.notebook.entries.create", Decision: "prompt"})
	_, client, cleanup := newNotebookAuthorityApp(t, cfg)
	defer cleanup()

	created := callSmoke(t, client, "session/create", map[string]string{"title": "prompt"})
	var session struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, created, &session)

	_, errText := callSmokeErr(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.create",
		"input": map[string]any{
			"operation_key": "prompt-1",
			"request":       map[string]string{"section_id": "section-notes", "title": "t", "markdown": "m"},
		},
	})
	if errText == "" {
		t.Fatal("explicit prompt rule bypassed by human origin")
	}
}

// An in-run (agent) invocation is bound to the session workspace scope and
// keeps frozen-run policy: under the default profile the write requires
// approval instead of inheriting the human exemption.
func TestNotebookAgentInvocationKeepsFrozenPolicy(t *testing.T) {
	_, client, cleanup := newNotebookAuthorityApp(t, notebookAuthorityConfig(t))
	defer cleanup()

	created := callSmoke(t, client, "session/create", map[string]string{"title": "agent"})
	var session struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, created, &session)

	started := callSmoke(t, client, "turn/start", map[string]any{"session_id": session.ID, "text": "noop turn"})
	var accepted struct {
		RunID string `json:"run_id"`
		ID    string `json:"id"`
	}
	decodeSmoke(t, started, &accepted)
	if accepted.RunID == "" && accepted.ID == "" {
		t.Fatalf("turn/start returned no run: %s", started)
	}

	_, errText := callSmokeErr(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/notebook-core",
		"action_id": "vivy.notebook.entries.create",
		"input": map[string]any{
			"operation_key": "agent-1",
			"request":       map[string]string{"section_id": "section-notes", "title": "t", "markdown": "m"},
		},
	})
	if errText == "" {
		t.Fatal("agent-bound write inherited the human exemption")
	}
}

// TestNotebookHeadlessActionExercise drives every §7 notebook surface through
// the real ActionHost + sqlite backend (no UI): sections, entry CRUD with CAS
// versioning, revisions, comments, and export.
func TestNotebookHeadlessActionExercise(t *testing.T) {
	_, client, cleanup := newNotebookAuthorityApp(t, notebookAuthorityConfig(t))
	defer cleanup()
	created := callSmoke(t, client, "session/create", map[string]string{"title": "headless"})
	var session struct {
		ID string `json:"id"`
	}
	decodeSmoke(t, created, &session)

	invoke := func(actionID string, input map[string]any) json.RawMessage {
		t.Helper()
		return callSmoke(t, client, "module.action.invoke", map[string]any{
			"module_id": "vivy/notebook-core", "action_id": actionID, "input": input,
		})
	}
	keyed := func(opKey string, request map[string]any) map[string]any {
		return map[string]any{"operation_key": opKey, "request": request}
	}

	// Sections: create + list.
	secRes := invoke("vivy.notebook.sections.create", keyed("sec-1", map[string]any{"title": "Work"}))
	if !strings.Contains(string(secRes), `"status":"ok"`) {
		t.Fatalf("section create: %s", secRes)
	}
	secs := invoke("vivy.notebook.sections.list", map[string]any{})
	if !strings.Contains(string(secs), "Work") || !strings.Contains(string(secs), "section-notes") {
		t.Fatalf("section list: %s", secs)
	}

	// Entries: create then CAS save with returned version + head revision.
	entry := invoke("vivy.notebook.entries.create", keyed("e-1", map[string]any{"section_id": "section-notes", "title": "spec", "markdown": "v1 body"}))
	var createdEntry struct {
		Status string `json:"status"`
		Data   struct {
			ResourceID string `json:"resource_id"`
			Version    int64  `json:"version"`
			RevisionID string `json:"revision_id"`
		} `json:"data"`
	}
	decodeSmoke(t, entry, &createdEntry)
	if createdEntry.Data.ResourceID == "" || createdEntry.Data.RevisionID == "" {
		t.Fatalf("entry create: %s", entry)
	}
	saved := invoke("vivy.notebook.entries.save", keyed("e-2", map[string]any{
		"entry_id": createdEntry.Data.ResourceID, "expected_version": createdEntry.Data.Version,
		"base_revision_id": createdEntry.Data.RevisionID, "title": "spec", "markdown": "v2 body",
	}))
	if !strings.Contains(string(saved), `"status":"ok"`) {
		t.Fatalf("CAS save: %s", saved)
	}
	var savedEntry struct {
		Data struct {
			RevisionID string `json:"revision_id"`
		} `json:"data"`
	}
	decodeSmoke(t, saved, &savedEntry)

	// A stale base_version/base_revision must surface as a version conflict.
	stale := invoke("vivy.notebook.entries.save", keyed("e-3", map[string]any{
		"entry_id": createdEntry.Data.ResourceID, "expected_version": createdEntry.Data.Version,
		"base_revision_id": createdEntry.Data.RevisionID, "title": "spec", "markdown": "v3 clobber",
	}))
	if !strings.Contains(string(stale), `"status":"error"`) {
		t.Fatalf("stale CAS accepted: %s", stale)
	}

	// Read back + revisions prove the mutation history.
	got := invoke("vivy.notebook.entries.get", map[string]any{"id": createdEntry.Data.ResourceID})
	if !strings.Contains(string(got), "v2 body") || strings.Contains(string(got), "clobber") {
		t.Fatalf("entry get: %s", got)
	}
	revs := invoke("vivy.notebook.revisions.list", map[string]any{"entry_id": createdEntry.Data.ResourceID})
	if !strings.Contains(string(revs), savedEntry.Data.RevisionID) || !strings.Contains(string(revs), createdEntry.Data.RevisionID) {
		t.Fatalf("revisions: %s", revs)
	}

	// Comments.
	comment := invoke("vivy.notebook.comments.create", keyed("c-1", map[string]any{"entry_id": createdEntry.Data.ResourceID, "body": "review note"}))
	if !strings.Contains(string(comment), `"status":"ok"`) {
		t.Fatalf("comment create: %s", comment)
	}
	comments := invoke("vivy.notebook.comments.list", map[string]any{"entry_id": createdEntry.Data.ResourceID})
	if !strings.Contains(string(comments), "review note") {
		t.Fatalf("comments list: %s", comments)
	}

	// Export bundles one revision with its sidecar data, read-only.
	exported := invoke("vivy.notebook.export", map[string]any{"entry_id": createdEntry.Data.ResourceID, "revision_id": savedEntry.Data.RevisionID})
	if !strings.Contains(string(exported), "v2 body") || !strings.Contains(string(exported), "review note") {
		t.Fatalf("export: %s", exported[:400])
	}
}
