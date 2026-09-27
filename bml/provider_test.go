package bml

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Compile-time contract assertion: NopProvider must satisfy the whole
// Provider surface.
var _ Provider = NopProvider{}

func TestNopProviderDefaults(t *testing.T) {
	ctx := context.Background()
	p := NopProvider{}
	crud := MemoryCrudContext{WorkspaceRoot: "/ws"}

	prompt, err := p.SystemPromptBlock(ctx, SystemPromptRequest{WorkspaceRoot: "/ws"})
	if err != nil {
		t.Fatalf("SystemPromptBlock: %v", err)
	}
	if prompt.Status.Kind != StartupStatusReady {
		t.Fatalf("SystemPromptBlock status = %q, want ready", prompt.Status.Kind)
	}
	if prompt.PromptBlock == nil ||
		prompt.PromptBlock.Shape != StartupInjectionShapeCompactRenderedMarkdown ||
		prompt.PromptBlock.Markdown != "" {
		t.Fatalf("SystemPromptBlock prompt_block = %+v, want empty compact markdown block", prompt.PromptBlock)
	}

	if rev := p.SystemPromptRevision(ctx, SystemPromptRequest{}); rev != 0 {
		t.Fatalf("SystemPromptRevision = %d, want 0", rev)
	}

	refresh, err := p.RefreshSystemPromptProjection(ctx, SystemPromptRefreshRequest{
		WorkspaceRoot:     "/ws",
		AuthorityRevision: 41,
	})
	if err != nil {
		t.Fatalf("RefreshSystemPromptProjection: %v", err)
	}
	if refresh.AuthorityRevision != 41 || refresh.ProjectionChanged {
		t.Fatalf("RefreshSystemPromptProjection = %+v, want echo revision 41 unchanged", refresh)
	}

	prefetch, err := p.Prefetch(ctx, PrefetchRequest{Intent: "x"})
	if err != nil || prefetch.Status.Kind != PrefetchStatusSkippedNoIntent || prefetch.PromptBlock != nil {
		t.Fatalf("Prefetch = %+v, %v, want skipped_no_intent", prefetch, err)
	}

	sync, err := p.SyncTurn(ctx, SyncTurnRequest{})
	if err != nil || sync.Status.Kind != SyncTurnStatusNoop {
		t.Fatalf("SyncTurn = %+v, %v, want noop", sync, err)
	}

	end, err := p.OnSessionEnd(ctx, SessionEndRequest{})
	if err != nil || end.Status.Kind != SessionEndStatusNoop {
		t.Fatalf("OnSessionEnd = %+v, %v, want noop", end, err)
	}

	for name, call := range map[string]func() error{
		"RecordRecallOutcome":  func() error { return p.RecordRecallOutcome(ctx, RecallOutcomeRequest{}) },
		"RecordUserPulse":      func() error { return p.RecordUserPulse(ctx, "s", "c") },
		"RecordAssistantRecap": func() error { return p.RecordAssistantRecap(ctx, "s", "c") },
		"FoldActmemSession":    func() error { return p.FoldActmemSession(ctx, "s") },
	} {
		if err := call(); err != nil {
			t.Fatalf("%s = %v, want nil", name, err)
		}
	}

	checkpoint, err := p.SessionCheckpointBlock(ctx, SessionCheckpointBlockRequest{})
	if err != nil || checkpoint.PromptBlock != nil {
		t.Fatalf("SessionCheckpointBlock = %+v, %v, want empty block", checkpoint, err)
	}

	unsupported := map[string]MemoryCrudOutcome{}
	if o, err := p.MemoryAdd(ctx, crud, MemoryAddRequest{}); err != nil {
		t.Fatalf("MemoryAdd: %v", err)
	} else {
		unsupported["memory_add"] = o
	}
	if o, err := p.MemoryList(ctx, crud, MemoryListRequest{}); err != nil {
		t.Fatalf("MemoryList: %v", err)
	} else {
		unsupported["memory_list"] = o
	}
	if o, err := p.MemoryGet(ctx, crud, MemoryGetRequest{}); err != nil {
		t.Fatalf("MemoryGet: %v", err)
	} else {
		unsupported["memory_get"] = o
	}
	if o, err := p.MemorySearch(ctx, crud, MemorySearchRequest{}); err != nil {
		t.Fatalf("MemorySearch: %v", err)
	} else {
		unsupported["memory_search"] = o
	}
	if o, err := p.MemoryUpdate(ctx, crud, MemoryUpdateRequest{}); err != nil {
		t.Fatalf("MemoryUpdate: %v", err)
	} else {
		unsupported["memory_update"] = o
	}
	if o, err := p.MemoryRemove(ctx, crud, MemoryRemoveRequest{}); err != nil {
		t.Fatalf("MemoryRemove: %v", err)
	} else {
		unsupported["memory_remove"] = o
	}
	if o, err := p.MemoryDistill(ctx, MemoryDistillRequest{}); err != nil {
		t.Fatalf("MemoryDistill: %v", err)
	} else {
		unsupported["memory_distill"] = o
	}
	if o, err := p.SessionCheckpointWrite(ctx, SessionCheckpointWriteRequest{}); err != nil {
		t.Fatalf("SessionCheckpointWrite: %v", err)
	} else {
		unsupported["session_checkpoint_write"] = o
	}
	for op, outcome := range unsupported {
		want := op + " not supported by this memory provider"
		if outcome.Status != CrudOutcomeFailed || outcome.Reason != want {
			t.Fatalf("%s outcome = %+v, want failed reason %q", op, outcome, want)
		}
	}

	for name, err := range map[string]error{
		"ActmemRead":     func() error { _, e := p.ActmemRead(ctx, ActmemReadRequest{}); return e }(),
		"ActmemEditWork": func() error { _, e := p.ActmemEditWork(ctx, ActmemEditWorkRequest{}); return e }(),
		"ActmemComplete": func() error { _, e := p.ActmemComplete(ctx, ActmemItemRequest{}); return e }(),
		"ActmemDrop":     func() error { _, e := p.ActmemDrop(ctx, ActmemItemRequest{}); return e }(),
		"MemoryRules":    func() error { _, e := p.MemoryRules(ctx); return e }(),
	} {
		want := ErrActmemUnavailable
		if name == "MemoryRules" {
			want = ErrMemoryRulesUnavailable
		}
		if !errors.Is(err, want) {
			t.Fatalf("%s error = %v, want %v", name, err, want)
		}
	}
}

// TestDTOJSONTags pins every ported DTO field to its serde snake_case wire
// name, verified field-by-field against agent-diva-core/src/memory/.
func TestDTOJSONTags(t *testing.T) {
	cases := map[string]map[string]string{
		"SystemPromptRequest":        {"WorkspaceRoot": "workspace_root"},
		"SystemPromptBlock":          {"Shape": "shape", "Markdown": "markdown"},
		"SystemPromptResponse":       {"Status": "status", "PromptBlock": "prompt_block"},
		"SystemPromptRefreshRequest": {"WorkspaceRoot": "workspace_root", "AuthorityRevision": "authority_revision"},
		"SystemPromptRefreshResponse": {
			"AuthorityRevision": "authority_revision", "ProjectionChanged": "projection_changed",
		},
		"WakeupPackSummary": {
			"Identity": "identity", "RecentState": "recent_state", "LatestCapsule": "latest_capsule",
			"KeyRelations": "key_relations", "UnresolvedThreads": "unresolved_threads",
			"GeneratedAt": "generated_at",
		},
		"StartupContextSnapshot": {
			"LaputaStateRoot": "laputa_state_root", "SoulMarkdown": "soul_markdown",
			"WakeupMarkdown": "wakeup_markdown", "WakeupPack": "wakeup_pack",
			"MemoryMarkdown": "memory_markdown",
		},
		"PrefetchRequest": {
			"WorkspaceRoot": "workspace_root", "Intent": "intent",
			"CurrentRoom": "current_room", "UserMessage": "user_message",
		},
		"PrefetchResponse": {"Status": "status", "PromptBlock": "prompt_block"},
		"SyncTurnRequest": {
			"WorkspaceRoot": "workspace_root", "MemoryUpdateMarkdown": "memory_update_markdown",
			"HistoryEntry": "history_entry",
		},
		"SyncTurnResponse": {"Status": "status"},
		"RecallOutcomeRequest": {
			"WorkspaceRoot": "workspace_root", "RequestID": "request_id",
			"Outcome": "outcome", "Corrected": "corrected",
		},
		"SessionEndRequest":  {"WorkspaceRoot": "workspace_root", "SessionID": "session_id"},
		"SessionEndResponse": {"Status": "status"},
		"MemoryCrudContext":  {"WorkspaceRoot": "workspace_root"},
		"MemoryAddRequest":   {"Content": "content", "EvidenceRefs": "evidence_refs"},
		"MemoryListRequest":  {"Limit": "limit"},
		"MemoryGetRequest":   {"RecordID": "record_id"},
		"MemorySearchRequest": {
			"Query": "query", "Limit": "limit",
		},
		"MemoryUpdateRequest": {
			"RecordID": "record_id", "Content": "content",
			"BaseRevision": "base_revision", "EvidenceRefs": "evidence_refs",
		},
		"MemoryRemoveRequest": {
			"RecordID": "record_id", "Reason": "reason", "BaseRevision": "base_revision",
		},
		"MemoryDistillRequest": {
			"SkillName": "skill_name", "Content": "content", "Evidence": "evidence",
		},
		"MemoryEntry": {
			"ID": "id", "Content": "content", "Trust": "trust", "Provenance": "provenance",
			"EvidenceRefs": "evidence_refs", "Revision": "revision",
			"CreatedAt": "created_at", "UpdatedAt": "updated_at",
		},
		"ActmemReadRequest":  {"Target": "target", "CapsuleName": "capsule_name"},
		"ActmemReadResponse": {"Revision": "revision", "Content": "content"},
		"ActmemEditWorkRequest": {
			"Section": "section", "Replacement": "replacement", "BaseRevision": "base_revision",
		},
		"ActmemItemRequest": {
			"Section": "section", "ItemIndex": "item_index", "BaseRevision": "base_revision",
		},
		"ActmemMutationResponse": {"Revision": "revision", "UpdatedAt": "updated_at"},
		"MemoryRulesResponse":    {"Content": "content", "Source": "source"},
		"SessionCheckpointBlockRequest": {
			"WorkspaceRoot": "workspace_root", "SessionID": "session_id",
		},
		"SessionCheckpointBlockResponse": {"PromptBlock": "prompt_block"},
		"SessionCheckpointWriteRequest": {
			"WorkspaceRoot": "workspace_root", "SessionID": "session_id", "KeyInfo": "key_info",
			"RelatedSops": "related_sops", "Content": "content",
		},
	}

	typeOf := map[string]reflect.Type{}
	for _, v := range []any{
		SystemPromptRequest{}, SystemPromptBlock{}, SystemPromptResponse{},
		SystemPromptRefreshRequest{}, SystemPromptRefreshResponse{},
		WakeupPackSummary{}, StartupContextSnapshot{}, PrefetchRequest{},
		PrefetchResponse{}, SyncTurnRequest{}, SyncTurnResponse{},
		RecallOutcomeRequest{}, SessionEndRequest{}, SessionEndResponse{},
		MemoryCrudContext{}, MemoryAddRequest{}, MemoryListRequest{},
		MemoryGetRequest{}, MemorySearchRequest{}, MemoryUpdateRequest{},
		MemoryRemoveRequest{}, MemoryDistillRequest{}, MemoryEntry{},
		ActmemReadRequest{}, ActmemReadResponse{}, ActmemEditWorkRequest{},
		ActmemItemRequest{}, ActmemMutationResponse{}, MemoryRulesResponse{},
		SessionCheckpointBlockRequest{}, SessionCheckpointBlockResponse{},
		SessionCheckpointWriteRequest{},
	} {
		typeOf[reflect.TypeOf(v).Name()] = reflect.TypeOf(v)
	}

	for name, fields := range cases {
		typ, ok := typeOf[name]
		if !ok {
			t.Fatalf("missing type %s", name)
		}
		seen := map[string]bool{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag := f.Tag.Get("json")
			tag, _, _ = strings.Cut(tag, ",")
			want, ok := fields[f.Name]
			if !ok {
				t.Errorf("%s.%s: unexpected field (tag %q)", name, f.Name, tag)
				continue
			}
			if tag != want {
				t.Errorf("%s.%s: json tag = %q, want %q", name, f.Name, tag, want)
			}
			seen[f.Name] = true
		}
		for field := range fields {
			if !seen[field] {
				t.Errorf("%s: missing field %s", name, field)
			}
		}
	}
}

func TestMemoryCrudOutcomeSerdeShape(t *testing.T) {
	// serde internally tagged: {"status":"applied","entry":null,...};
	// evidence_advisory is skipped when absent.
	applied := MemoryCrudOutcome{Status: CrudOutcomeApplied}
	raw, err := json.Marshal(applied)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"status":"applied"`) || !strings.Contains(s, `"entry":null`) {
		t.Fatalf("applied wire = %s", s)
	}
	if strings.Contains(s, "evidence_advisory") {
		t.Fatalf("applied wire must omit evidence_advisory: %s", s)
	}
	var back MemoryCrudOutcome
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Status != CrudOutcomeApplied || back.Entry != nil {
		t.Fatalf("round trip = %+v", back)
	}

	advisory := "no evidence_refs"
	withAdvisory := MemoryCrudOutcome{Status: CrudOutcomeApplied, EvidenceAdvisory: &advisory}
	raw, _ = json.Marshal(withAdvisory)
	if !strings.Contains(string(raw), `"evidence_advisory":"no evidence_refs"`) {
		t.Fatalf("applied+advisory wire = %s", raw)
	}

	proposed := MemoryCrudOutcome{Status: CrudOutcomeProposalCreated, ProposalID: "prop-42"}
	raw, _ = json.Marshal(proposed)
	if !strings.Contains(string(raw), `"status":"proposal_created"`) ||
		!strings.Contains(string(raw), `"proposal_id":"prop-42"`) {
		t.Fatalf("proposal_created wire = %s", raw)
	}

	listed := MemoryCrudOutcome{Status: CrudOutcomeListed}
	raw, _ = json.Marshal(listed)
	if !strings.Contains(string(raw), `"entries":[]`) {
		t.Fatalf("listed wire = %s, want empty entries array", raw)
	}

	failed := UnsupportedCrudOutcome("memory_add")
	raw, _ = json.Marshal(failed)
	if !strings.Contains(string(raw), `"status":"failed"`) ||
		!strings.Contains(string(raw), `"reason":"memory_add not supported by this memory provider"`) {
		t.Fatalf("failed wire = %s", raw)
	}
}

func TestStartupContextSnapshotRendersCompactMarkdown(t *testing.T) {
	// Mirrors upstream test_startup_context_snapshot_renders_compact_markdown.
	generatedAt := "2026-05-08 10:00 UTC"
	capsule := "Weekly review complete."
	soul := "# Identity\n\nGenerated soul"
	memory := "## Long-term Memory\nExisting durable memory"
	laputaRoot := "/tmp/diva/.laputa"
	snapshot := StartupContextSnapshot{
		LaputaStateRoot: &laputaRoot,
		SoulMarkdown:    &soul,
		WakeupPack: &WakeupPackSummary{
			Identity:          "You are Diva.",
			RecentState:       "- roadmap: Hot (heat: 5)",
			LatestCapsule:     &capsule,
			KeyRelations:      []string{"maintainer <-> roadmap"},
			UnresolvedThreads: []string{"ship provider boundary"},
			GeneratedAt:       &generatedAt,
		},
		MemoryMarkdown: &memory,
	}
	block := snapshot.IntoSystemPromptBlock()
	if block == nil {
		t.Fatal("expected a prompt block")
	}
	if block.Shape != StartupInjectionShapeCompactRenderedMarkdown {
		t.Fatalf("shape = %q", block.Shape)
	}
	for _, want := range []string{"## Long-term Memory", "## Soul Projection", "## Wakeup Summary"} {
		if !strings.Contains(block.Markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, block.Markdown)
		}
	}
	if strings.Contains(block.Markdown, "## Rhythm Signals") {
		t.Fatal("rhythm signals must never be injected")
	}

	if (StartupContextSnapshot{}).IntoSystemPromptBlock() != nil {
		t.Fatal("empty snapshot must render no block")
	}
}

func TestDegradedStartupOmitsCachedWakeup(t *testing.T) {
	// Mirrors upstream test_degraded_startup_explicitly_omits_cached_wakeup.
	resp := DegradedSystemPromptResponse(" wakeup generation failed ")
	if resp.Status.Kind != StartupStatusDegraded {
		t.Fatalf("status = %q, want degraded", resp.Status.Kind)
	}
	// The status carries the reason verbatim; the rendered markdown trims it.
	if resp.Status.Reason != " wakeup generation failed " {
		t.Fatalf("reason = %q", resp.Status.Reason)
	}
	if resp.Status.LastUsableWakeup != nil {
		t.Fatal("last_usable_wakeup must be nil")
	}
	if resp.PromptBlock == nil {
		t.Fatal("degraded startup still emits an explicit prompt block")
	}
	if !strings.Contains(resp.PromptBlock.Markdown, "status: degraded") ||
		!strings.Contains(resp.PromptBlock.Markdown, "reason: wakeup generation failed") ||
		!strings.Contains(resp.PromptBlock.Markdown, "last_usable_wakeup: omitted") {
		t.Fatalf("degraded markdown:\n%s", resp.PromptBlock.Markdown)
	}
}

func TestRenderSessionCheckpointBlock(t *testing.T) {
	block := RenderSessionCheckpointBlock(
		"migrating service B", []string{"rust-deploy"}, "port 8080 confirmed")
	for _, want := range []string{
		"## Session Checkpoint", "Key info:", "migrating service B",
		"Related SOPs:", "- rust-deploy", "port 8080 confirmed",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("checkpoint block missing %q:\n%s", want, block)
		}
	}
	if got := RenderSessionCheckpointBlock("", nil, ""); got != "## Session Checkpoint\n" {
		t.Fatalf("empty checkpoint = %q", got)
	}
}
