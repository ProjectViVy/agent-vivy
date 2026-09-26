package memory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"agent-vivy/internal/modules/memory"
	controlaction "agent-vivy/sdk/port/controlaction"
	"github.com/ProjectViVy/agent-vivy/bml"
)

func openService(t *testing.T) *memory.Service {
	t.Helper()
	service, err := memory.Open(context.Background(), testConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	return service
}

func actionProvider(t *testing.T, id string) controlaction.Provider {
	t.Helper()
	for _, provider := range memory.ActionProviders() {
		if provider.Definition().ID == id {
			return provider
		}
	}
	t.Fatalf("provider %s not in ActionProviders()", id)
	return nil
}

func invokeRaw(t *testing.T, id, input string) json.RawMessage {
	t.Helper()
	raw, err := actionProvider(t, id).Invoke(context.Background(), nil, json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s Invoke() error = %v", id, err)
	}
	return raw
}

func invokeOutcome(t *testing.T, id, input string) bml.MemoryCrudOutcome {
	t.Helper()
	raw := invokeRaw(t, id, input)
	var outcome bml.MemoryCrudOutcome
	if err := json.Unmarshal(raw, &outcome); err != nil {
		t.Fatalf("%s output is not a four-state outcome: %v (%s)", id, err, raw)
	}
	return outcome
}

func wantStatus(t *testing.T, outcome bml.MemoryCrudOutcome, status bml.CrudOutcomeStatus) {
	t.Helper()
	if outcome.Status != status {
		t.Fatalf("status = %q (reason %q), want %q", outcome.Status, outcome.Reason, status)
	}
}

func wantFailure(t *testing.T, outcome bml.MemoryCrudOutcome, reason string) {
	t.Helper()
	wantStatus(t, outcome, bml.CrudOutcomeFailed)
	if outcome.Reason != reason {
		t.Fatalf("reason = %q, want %q", outcome.Reason, reason)
	}
}

func TestActionProvidersMatchDeclaredInventory(t *testing.T) {
	want := map[string]controlaction.Effect{
		memory.ActionList:       controlaction.EffectRead,
		memory.ActionSearch:     controlaction.EffectRead,
		memory.ActionGet:        controlaction.EffectRead,
		memory.ActionAdd:        controlaction.EffectWrite,
		memory.ActionUpdate:     controlaction.EffectWrite,
		memory.ActionRemove:     controlaction.EffectWrite,
		memory.ActionRulesRead:  controlaction.EffectRead,
		memory.ActionRulesWrite: controlaction.EffectWrite,
		memory.ActionStatus:     controlaction.EffectRead,
	}
	providers := memory.ActionProviders()
	if len(providers) != len(want) {
		t.Fatalf("provider count = %d, want %d", len(providers), len(want))
	}
	for _, provider := range providers {
		definition, err := provider.Definition().Normalize()
		if err != nil {
			t.Fatalf("invalid definition %q: %v", provider.Definition().ID, err)
		}
		effect, ok := want[definition.ID]
		if !ok {
			t.Fatalf("unexpected action %q", definition.ID)
		}
		delete(want, definition.ID)
		if definition.Owner != memory.ID || definition.ModuleID != memory.ID {
			t.Fatalf("%s owner/module = %q/%q, want %q", definition.ID, definition.Owner, definition.ModuleID, memory.ID)
		}
		if definition.Effect != effect {
			t.Fatalf("%s effect = %q, want %q", definition.ID, definition.Effect, effect)
		}
		if len(definition.InputSchema) == 0 || len(definition.ResultSchema) == 0 {
			t.Fatalf("%s missing input/result schema", definition.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing providers: %#v", want)
	}
}

func TestListAndGetRoundTrip(t *testing.T) {
	openService(t)

	outcome := invokeOutcome(t, memory.ActionList, `{}`)
	wantStatus(t, outcome, bml.CrudOutcomeListed)
	if len(outcome.Entries) != 0 {
		t.Fatalf("fresh store list entries = %d, want 0", len(outcome.Entries))
	}

	outcome = invokeOutcome(t, memory.ActionAdd, `{"kind":"long_term","content":"prefers dark mode"}`)
	wantStatus(t, outcome, bml.CrudOutcomeApplied)
	if outcome.Entry == nil || outcome.Entry.ID == "" {
		t.Fatalf("add outcome entry = %#v", outcome.Entry)
	}
	id := outcome.Entry.ID
	if outcome.Entry.Content != "prefers dark mode" {
		t.Fatalf("add entry content = %q", outcome.Entry.Content)
	}

	outcome = invokeOutcome(t, memory.ActionList, `{"limit":10}`)
	wantStatus(t, outcome, bml.CrudOutcomeListed)
	if len(outcome.Entries) != 1 || outcome.Entries[0].ID != id {
		t.Fatalf("list entries = %#v, want the added record", outcome.Entries)
	}

	outcome = invokeOutcome(t, memory.ActionGet, fmt.Sprintf(`{"id":%q}`, id))
	wantStatus(t, outcome, bml.CrudOutcomeListed)
	if len(outcome.Entries) != 1 || outcome.Entries[0].ID != id {
		t.Fatalf("get entries = %#v", outcome.Entries)
	}
}

func TestSearchFindsAddedRecord(t *testing.T) {
	openService(t)
	outcome := invokeOutcome(t, memory.ActionAdd, `{"kind":"long_term","content":"deploys happen on Tuesdays"}`)
	wantStatus(t, outcome, bml.CrudOutcomeApplied)

	outcome = invokeOutcome(t, memory.ActionSearch, `{"query":"Tuesdays"}`)
	wantStatus(t, outcome, bml.CrudOutcomeListed)
	if len(outcome.Entries) == 0 {
		t.Fatal("search returned no entries for indexed content")
	}
}

func TestUpdateAndRemoveEnforceRevisionCAS(t *testing.T) {
	openService(t)
	outcome := invokeOutcome(t, memory.ActionAdd, `{"kind":"long_term","content":"original"}`)
	wantStatus(t, outcome, bml.CrudOutcomeApplied)
	id := outcome.Entry.ID
	revision := outcome.Entry.Revision

	outcome = invokeOutcome(t, memory.ActionUpdate,
		fmt.Sprintf(`{"id":%q,"content":"stale write","base_revision":%d}`, id, revision+99))
	wantFailure(t, outcome, bml.HomeCodeRevisionConflict)

	outcome = invokeOutcome(t, memory.ActionUpdate,
		fmt.Sprintf(`{"id":%q,"content":"updated","base_revision":%d}`, id, revision))
	wantStatus(t, outcome, bml.CrudOutcomeApplied)
	newRevision := outcome.Entry.Revision

	outcome = invokeOutcome(t, memory.ActionGet, fmt.Sprintf(`{"id":%q}`, id))
	wantStatus(t, outcome, bml.CrudOutcomeListed)
	if outcome.Entries[0].Content != "updated" {
		t.Fatalf("record content = %q, want %q", outcome.Entries[0].Content, "updated")
	}

	outcome = invokeOutcome(t, memory.ActionRemove,
		fmt.Sprintf(`{"id":%q,"reason":"superseded","base_revision":%d}`, id, revision))
	wantFailure(t, outcome, bml.HomeCodeRevisionConflict)

	outcome = invokeOutcome(t, memory.ActionRemove,
		fmt.Sprintf(`{"id":%q,"reason":"superseded","base_revision":%d}`, id, newRevision))
	wantStatus(t, outcome, bml.CrudOutcomeApplied)

	outcome = invokeOutcome(t, memory.ActionGet, fmt.Sprintf(`{"id":%q}`, id))
	wantFailure(t, outcome, bml.HomeCodeNotFound)
}

func TestAddRejectsUnsupportedKind(t *testing.T) {
	openService(t)
	outcome := invokeOutcome(t, memory.ActionAdd, `{"kind":"history","content":"event"}`)
	wantFailure(t, outcome, bml.HomeCodeKindForbidden)
}

func TestRulesReadWriteRoundTripWithCAS(t *testing.T) {
	openService(t)

	raw := invokeRaw(t, memory.ActionRulesRead, `{}`)
	var read struct {
		Status   bml.CrudOutcomeStatus `json:"status"`
		Content  string                `json:"content"`
		Source   string                `json:"source"`
		Revision int64                 `json:"revision"`
	}
	if err := json.Unmarshal(raw, &read); err != nil {
		t.Fatalf("rules.read output = %s: %v", raw, err)
	}
	if read.Status != bml.CrudOutcomeListed || read.Content == "" || read.Source == "" {
		t.Fatalf("rules.read = %#v", read)
	}

	outcome := invokeOutcome(t, memory.ActionRulesWrite,
		fmt.Sprintf(`{"content":"# Rules\nAlways cite evidence.","base_revision":%d}`, read.Revision+99))
	wantFailure(t, outcome, bml.HomeCodeRevisionConflict)

	outcome = invokeOutcome(t, memory.ActionRulesWrite,
		fmt.Sprintf(`{"content":"# Rules\nAlways cite evidence.","base_revision":%d}`, read.Revision))
	wantStatus(t, outcome, bml.CrudOutcomeApplied)

	raw = invokeRaw(t, memory.ActionRulesRead, `{}`)
	read = struct {
		Status   bml.CrudOutcomeStatus `json:"status"`
		Content  string                `json:"content"`
		Source   string                `json:"source"`
		Revision int64                 `json:"revision"`
	}{}
	if err := json.Unmarshal(raw, &read); err != nil {
		t.Fatalf("rules.read after write = %s: %v", raw, err)
	}
	if read.Content != "# Rules\nAlways cite evidence." || read.Source != string(bml.MemRulesSourceFile) {
		t.Fatalf("rules.read after write = %#v", read)
	}
}

func TestStatusReportsAvailableService(t *testing.T) {
	openService(t)

	// Fresh home: the store file only exists after the first write.
	raw := invokeRaw(t, memory.ActionStatus, `{}`)
	var status struct {
		Status          bml.CrudOutcomeStatus `json:"status"`
		Available       bool                  `json:"available"`
		StartupRevision uint64                `json:"startup_revision"`
		DatabasePresent bool                  `json:"database_present"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("status output = %s: %v", raw, err)
	}
	if status.Status != bml.CrudOutcomeListed || !status.Available {
		t.Fatalf("status = %#v", status)
	}

	outcome := invokeOutcome(t, memory.ActionAdd, `{"kind":"long_term","content":"needs the store"}`)
	wantStatus(t, outcome, bml.CrudOutcomeApplied)
	raw = invokeRaw(t, memory.ActionStatus, `{}`)
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("status output = %s: %v", raw, err)
	}
	if status.Status != bml.CrudOutcomeListed || !status.Available || !status.DatabasePresent {
		t.Fatalf("status after write = %#v", status)
	}
}

func TestInvalidInputsReturnExplicitOutcomes(t *testing.T) {
	openService(t)
	cases := []struct {
		name  string
		id    string
		input string
	}{
		{"malformed json", memory.ActionList, `{`},
		{"unknown field", memory.ActionGet, `{"id":"memory-1","extra":true}`},
		{"missing id", memory.ActionGet, `{}`},
		{"empty query", memory.ActionSearch, `{"query":""}`},
		{"missing kind", memory.ActionAdd, `{"content":"x"}`},
		{"missing base revision", memory.ActionUpdate, `{"id":"memory-1","content":"x"}`},
		{"negative base revision", memory.ActionRemove, `{"id":"memory-1","reason":"r","base_revision":-1}`},
		{"missing reason", memory.ActionRemove, `{"id":"memory-1","base_revision":1}`},
		{"rules write missing revision", memory.ActionRulesWrite, `{"content":"x"}`},
		{"rules read with field", memory.ActionRulesRead, `{"verbose":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := invokeOutcome(t, tc.id, tc.input)
			wantFailure(t, outcome, bml.HomeCodeInvalidRequest)
		})
	}
}

func TestOversizedInputReportsInvalidRequest(t *testing.T) {
	openService(t)
	big := fmt.Sprintf(`{"kind":"long_term","content":%q}`, string(make([]byte, 40<<10)))
	outcome := invokeOutcome(t, memory.ActionAdd, big)
	wantFailure(t, outcome, bml.HomeCodeInvalidRequest)
}

func TestAllActionsReportUnavailableWithoutService(t *testing.T) {
	if err := memory.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	inputs := map[string]string{
		memory.ActionList:       `{}`,
		memory.ActionSearch:     `{"query":"x"}`,
		memory.ActionGet:        `{"id":"memory-1"}`,
		memory.ActionAdd:        `{"kind":"long_term","content":"x"}`,
		memory.ActionUpdate:     `{"id":"memory-1","content":"x","base_revision":1}`,
		memory.ActionRemove:     `{"id":"memory-1","reason":"r","base_revision":1}`,
		memory.ActionRulesRead:  `{}`,
		memory.ActionRulesWrite: `{"content":"x","base_revision":0}`,
		memory.ActionStatus:     `{}`,
	}
	for _, provider := range memory.ActionProviders() {
		input, ok := inputs[provider.Definition().ID]
		if !ok {
			t.Fatalf("no test input for %s", provider.Definition().ID)
		}
		raw, err := provider.Invoke(context.Background(), nil, json.RawMessage(input))
		if err != nil {
			t.Fatalf("%s Invoke() error = %v", provider.Definition().ID, err)
		}
		var outcome bml.MemoryCrudOutcome
		if err := json.Unmarshal(raw, &outcome); err != nil {
			t.Fatalf("%s output is not an outcome: %v", provider.Definition().ID, err)
		}
		if outcome.Status != bml.CrudOutcomeFailed || outcome.Reason != bml.HomeCodeBmlUnavailable {
			t.Fatalf("%s outcome = %#v, want failed/bml_unavailable", provider.Definition().ID, outcome)
		}
	}
}
