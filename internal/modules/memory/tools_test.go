package memory_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/modules/memory"
	toolport "agent-vivy/sdk/port/tool"
	"github.com/ProjectViVy/agent-vivy/bml"
)

func toolProvider(t *testing.T, id string) toolport.ToolProvider {
	t.Helper()
	for _, provider := range memory.ToolProviders() {
		if provider.Definition().ID == id {
			return provider
		}
	}
	t.Fatalf("provider %s not in ToolProviders()", id)
	return nil
}

func invokeTool(t *testing.T, id, input string) string {
	t.Helper()
	result, err := toolProvider(t, id).Invoke(context.Background(), nil, json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s Invoke() error = %v", id, err)
	}
	return result.Text
}

func invokeToolOutcome(t *testing.T, id, input string) bml.MemoryCrudOutcome {
	t.Helper()
	var outcome bml.MemoryCrudOutcome
	if err := json.Unmarshal([]byte(invokeTool(t, id, input)), &outcome); err != nil {
		t.Fatalf("%s output is not a four-state outcome: %v", id, err)
	}
	return outcome
}

func TestToolProvidersMatchDeclaredInventory(t *testing.T) {
	want := map[string]toolport.Effect{
		memory.ToolAdd:    toolport.EffectWrite,
		memory.ToolGet:    toolport.EffectRead,
		memory.ToolList:   toolport.EffectRead,
		memory.ToolSearch: toolport.EffectRead,
		memory.ToolUpdate: toolport.EffectWrite,
		memory.ToolRemove: toolport.EffectWrite,
	}
	got := make(map[string]toolport.Effect, len(want))
	for _, provider := range memory.ToolProviders() {
		def := provider.Definition()
		if _, dup := got[def.ID]; dup {
			t.Fatalf("duplicate tool id %q", def.ID)
		}
		got[def.ID] = def.Effect
		if def.Description == "" {
			t.Fatalf("%s: empty description", def.ID)
		}
		var schema map[string]any
		if err := json.Unmarshal(def.Schema, &schema); err != nil {
			t.Fatalf("%s: schema is not valid JSON: %v", def.ID, err)
		}
		if schema["type"] != "object" {
			t.Fatalf("%s: schema type = %v, want object", def.ID, schema["type"])
		}
	}
	if len(got) != len(want) {
		t.Fatalf("tool count = %d, want %d", len(got), len(want))
	}
	for id, effect := range want {
		if got[id] != effect {
			t.Fatalf("%s effect = %q, want %q", id, got[id], effect)
		}
	}
}

func TestToolIDsDoNotCollideWithProtectedTools(t *testing.T) {
	protected := map[string]bool{
		"ask_user": true, "list_dir": true, "read_file": true,
		"search_files": true, "write_file": true, "patch": true,
		"multiedit": true, "execute": true, "bash": true,
		"skills_list": true, "skill_view": true,
	}
	for _, provider := range memory.ToolProviders() {
		if protected[provider.Definition().ID] {
			t.Fatalf("tool id %q collides with a protected tool", provider.Definition().ID)
		}
	}
}

func TestToolsReportUnavailableWhenStoreClosed(t *testing.T) {
	if memory.Active() != nil {
		t.Fatal("memory service unexpectedly open")
	}
	for _, provider := range memory.ToolProviders() {
		id := provider.Definition().ID
		var outcome bml.MemoryCrudOutcome
		if err := json.Unmarshal([]byte(invokeTool(t, id, `{}`)), &outcome); err != nil {
			t.Fatalf("%s output is not a four-state outcome: %v", id, err)
		}
		if outcome.Status != bml.CrudOutcomeFailed || outcome.Reason != bml.HomeCodeBmlUnavailable {
			t.Fatalf("%s outcome = %+v, want failed/%s", id, outcome, bml.HomeCodeBmlUnavailable)
		}
	}
}

func TestToolInputCap(t *testing.T) {
	openService(t)
	oversized := `{"query":"` + strings.Repeat("q", 33<<10) + `"}`
	wantFailure(t, invokeToolOutcome(t, memory.ToolSearch, oversized), bml.HomeCodeInvalidRequest)
}

func TestToolOutputCap(t *testing.T) {
	service := openService(t)
	big := strings.Repeat("x", 128<<10)
	for i := 0; i < 3; i++ {
		outcome := service.Add(context.Background(), bml.MemoryAddRequest{Content: big})
		if outcome.Status != bml.CrudOutcomeApplied {
			t.Fatalf("seed add status = %q", outcome.Status)
		}
	}
	// Three 128KiB records exceed the 256KiB output cap once framed.
	wantFailure(t, invokeToolOutcome(t, memory.ToolList, `{}`), "output_too_large")
}

func TestMemoryAddToolAppliesLongTermRecord(t *testing.T) {
	openService(t)
	outcome := invokeToolOutcome(t, memory.ToolAdd,
		`{"kind":"long_term","content":"user prefers terse replies","trust":"user_asserted","provenance":"user_input","evidence_refs":[{"id":"e1","source":"user_input","uri":"chat:1"}]}`)
	wantStatus(t, outcome, bml.CrudOutcomeApplied)
	if outcome.Entry == nil {
		t.Fatal("applied outcome lacks the stored entry")
	}
	if outcome.Entry.Content != "user prefers terse replies" {
		t.Fatalf("entry content = %q", outcome.Entry.Content)
	}
	if outcome.Entry.Trust != "user_asserted" {
		t.Fatalf("entry trust = %q, want user_asserted", outcome.Entry.Trust)
	}
}

func TestMemoryAddToolRejectsNonLongTermKind(t *testing.T) {
	openService(t)
	wantFailure(t, invokeToolOutcome(t, memory.ToolAdd,
		`{"kind":"episodic","content":"x"}`), bml.HomeCodeKindForbidden)
}

func TestMemoryAddToolRejectsForgedTrust(t *testing.T) {
	openService(t)
	wantFailure(t, invokeToolOutcome(t, memory.ToolAdd,
		`{"kind":"long_term","content":"x","trust":"verified"}`), bml.HomeCodeInvalidRequest)
}

func TestMemoryAddToolRequiresKindAndContent(t *testing.T) {
	openService(t)
	wantFailure(t, invokeToolOutcome(t, memory.ToolAdd, `{"content":"x"}`), bml.HomeCodeInvalidRequest)
	wantFailure(t, invokeToolOutcome(t, memory.ToolAdd, `{"kind":"long_term"}`), bml.HomeCodeInvalidRequest)
}

func TestMemoryGetToolRoundTrip(t *testing.T) {
	openService(t)
	added := invokeToolOutcome(t, memory.ToolAdd,
		`{"kind":"long_term","content":"get me"}`)
	wantStatus(t, added, bml.CrudOutcomeApplied)
	got := invokeToolOutcome(t, memory.ToolGet, `{"id":"`+added.Entry.ID+`"}`)
	wantStatus(t, got, bml.CrudOutcomeListed)
	if len(got.Entries) != 1 || got.Entries[0].ID != added.Entry.ID {
		t.Fatalf("get entries = %+v", got.Entries)
	}
	wantFailure(t, invokeToolOutcome(t, memory.ToolGet, `{"id":"missing"}`), bml.HomeCodeNotFound)
	wantFailure(t, invokeToolOutcome(t, memory.ToolGet, `{}`), bml.HomeCodeInvalidRequest)
}

func TestMemoryListToolPagesEntries(t *testing.T) {
	service := openService(t)
	for i := 0; i < 3; i++ {
		outcome := service.Add(context.Background(), bml.MemoryAddRequest{Content: "list seed"})
		if outcome.Status != bml.CrudOutcomeApplied {
			t.Fatalf("seed add status = %q", outcome.Status)
		}
	}
	var page struct {
		Status     bml.CrudOutcomeStatus `json:"status"`
		Entries    []bml.MemoryEntry     `json:"entries"`
		NextCursor string                `json:"next_cursor"`
	}
	if err := json.Unmarshal([]byte(invokeTool(t, memory.ToolList, `{"kind":"long_term","scope":"machine-memory-home","limit":2}`)), &page); err != nil {
		t.Fatalf("list output does not decode: %v", err)
	}
	if page.Status != bml.CrudOutcomeListed || len(page.Entries) != 2 || page.NextCursor == "" {
		t.Fatalf("first page = %+v", page)
	}
	var second struct {
		Entries    []bml.MemoryEntry `json:"entries"`
		NextCursor string            `json:"next_cursor"`
	}
	if err := json.Unmarshal([]byte(invokeTool(t, memory.ToolList, `{"limit":2,"cursor":"`+page.NextCursor+`"}`)), &second); err != nil {
		t.Fatalf("second page does not decode: %v", err)
	}
	if len(second.Entries) != 1 || second.NextCursor != "" {
		t.Fatalf("second page = %+v", second)
	}
	if page.Entries[0].ID == second.Entries[0].ID {
		t.Fatal("pages overlap")
	}
}

func TestMemoryListToolRejectsInvisibleKindAndScope(t *testing.T) {
	openService(t)
	wantFailure(t, invokeToolOutcome(t, memory.ToolList, `{"kind":"history"}`), bml.HomeCodeKindForbidden)
	wantFailure(t, invokeToolOutcome(t, memory.ToolList, `{"scope":"other-tenant"}`), bml.HomeCodeInvalidRequest)
	wantFailure(t, invokeToolOutcome(t, memory.ToolList, `{"cursor":"zzz"}`), bml.HomeCodeInvalidRequest)
}

// A zero limit cannot advance the cursor: it would emit empty pages whose
// next_cursor replays the incoming cursor forever.
func TestMemoryToolsRejectZeroLimit(t *testing.T) {
	openService(t)
	wantFailure(t, invokeToolOutcome(t, memory.ToolList, `{"limit":0}`), bml.HomeCodeInvalidRequest)
	wantFailure(t, invokeToolOutcome(t, memory.ToolSearch, `{"query":"x","limit":0}`), bml.HomeCodeInvalidRequest)
}

func TestMemorySearchToolFindsContent(t *testing.T) {
	openService(t)
	added := invokeToolOutcome(t, memory.ToolAdd,
		`{"kind":"long_term","content":"the deploy key lives in vault"}`)
	wantStatus(t, added, bml.CrudOutcomeApplied)
	var page struct {
		Status  bml.CrudOutcomeStatus `json:"status"`
		Entries []bml.MemoryEntry     `json:"entries"`
	}
	if err := json.Unmarshal([]byte(invokeTool(t, memory.ToolSearch, `{"query":"deploy key"}`)), &page); err != nil {
		t.Fatalf("search output does not decode: %v", err)
	}
	if page.Status != bml.CrudOutcomeListed || len(page.Entries) == 0 {
		t.Fatalf("search page = %+v", page)
	}
	wantFailure(t, invokeToolOutcome(t, memory.ToolSearch, `{"limit":1}`), bml.HomeCodeInvalidRequest)
}

func TestMemoryUpdateToolHonorsCASAndPreservesContent(t *testing.T) {
	openService(t)
	added := invokeToolOutcome(t, memory.ToolAdd,
		`{"kind":"long_term","content":"original body","evidence_refs":[{"id":"e1","source":"user_input","uri":"chat:1"}]}`)
	wantStatus(t, added, bml.CrudOutcomeApplied)
	id, revision := added.Entry.ID, added.Entry.Revision

	// Omitted content preserves the stored body under the same CAS.
	updated := invokeToolOutcome(t, memory.ToolUpdate,
		`{"id":"`+id+`","base_revision":`+jsonNumber(revision)+`}`)
	wantStatus(t, updated, bml.CrudOutcomeApplied)
	if updated.Entry.Content != "original body" {
		t.Fatalf("preserved content = %q", updated.Entry.Content)
	}
	if len(updated.Entry.EvidenceRefs) != 1 {
		t.Fatalf("evidence not preserved: %+v", updated.Entry.EvidenceRefs)
	}

	// A stale revision is rejected.
	wantFailure(t, invokeToolOutcome(t, memory.ToolUpdate,
		`{"id":"`+id+`","content":"late writer","base_revision":0}`), bml.HomeCodeRevisionConflict)

	// Supplied content replaces the body under the new revision.
	replaced := invokeToolOutcome(t, memory.ToolUpdate,
		`{"id":"`+id+`","content":"new body","base_revision":`+jsonNumber(revision+1)+`,"reason":"correction"}`)
	wantStatus(t, replaced, bml.CrudOutcomeApplied)
	if replaced.Entry.Content != "new body" {
		t.Fatalf("updated content = %q", replaced.Entry.Content)
	}

	wantFailure(t, invokeToolOutcome(t, memory.ToolUpdate,
		`{"id":"`+id+`","content":"x"}`), bml.HomeCodeInvalidRequest)
}

func TestMemoryRemoveToolTombstonesUnderCAS(t *testing.T) {
	openService(t)
	added := invokeToolOutcome(t, memory.ToolAdd,
		`{"kind":"long_term","content":"remove me"}`)
	wantStatus(t, added, bml.CrudOutcomeApplied)
	id, revision := added.Entry.ID, added.Entry.Revision

	wantFailure(t, invokeToolOutcome(t, memory.ToolRemove,
		`{"id":"`+id+`","base_revision":`+jsonNumber(revision)+`}`), bml.HomeCodeInvalidRequest)
	removed := invokeToolOutcome(t, memory.ToolRemove,
		`{"id":"`+id+`","base_revision":`+jsonNumber(revision)+`,"reason":"no longer true"}`)
	wantStatus(t, removed, bml.CrudOutcomeApplied)
	wantFailure(t, invokeToolOutcome(t, memory.ToolGet, `{"id":"`+id+`"}`), bml.HomeCodeNotFound)
}

func jsonNumber(v int64) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
