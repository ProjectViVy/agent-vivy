package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/runtime"
	controlaction "agent-vivy/sdk/port/controlaction"

	laputaevolution "github.com/dashimaki/laputa/evolution"
)

type envelope struct {
	Status string          `json:"status"`
	Value  json.RawMessage `json:"value"`
	Error  *struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable"`
	} `json:"error"`
}

func invokeAction(t *testing.T, bundle cognitivecontract.Bundle, actionID, input string) envelope {
	t.Helper()
	dispatcher := bundle.(cognitivecontract.DispatcherProvider).Dispatcher()
	raw, err := dispatcher.Invoke(context.Background(), actionID, json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s transport error = %v", actionID, err)
	}
	var out envelope
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s payload = %s err=%v", actionID, raw, err)
	}
	if out.Status == "" {
		t.Fatalf("%s outcome missing status = %s", actionID, raw)
	}
	return out
}

func invokeActionErr(t *testing.T, bundle cognitivecontract.Bundle, actionID, input string) error {
	t.Helper()
	dispatcher := bundle.(cognitivecontract.DispatcherProvider).Dispatcher()
	_, err := dispatcher.Invoke(context.Background(), actionID, json.RawMessage(input))
	return err
}

func armedBundle(t *testing.T, control cognitivecontract.ControlPort) (context.Context, cognitivecontract.Bundle) {
	t.Helper()
	ctx, bundle := openBundle(t)
	if err := bundle.AttachRuntime(control); err != nil {
		t.Fatalf("attach runtime: %v", err)
	}
	return ctx, bundle
}

// --- inventory + strict input ---------------------------------------------

func TestEveryContractActionHasAHandler(t *testing.T) {
	if len(ActionIDs) != 20 {
		t.Fatalf("contract inventory = %d, want 20", len(ActionIDs))
	}
	for _, id := range ActionIDs {
		if cognitiveHandlers[id] == nil {
			t.Fatalf("no handler bound for %s", id)
		}
	}
	for _, provider := range ActionProviders() {
		def := provider.Definition()
		if !json.Valid(def.InputSchema) {
			t.Fatalf("%s input schema is not valid JSON", def.ID)
		}
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(def.InputSchema, &schema); err != nil {
			t.Fatalf("%s schema parse: %v", def.ID, err)
		}
		if len(schema.Required) == 0 || schema.Required[0] != "session_id" {
			t.Fatalf("%s does not require session_id: %v", def.ID, schema.Required)
		}
	}
}

func TestStrictInputRejectsUnknownFieldsTrailingDataAndSecrets(t *testing.T) {
	ctx, bundle := armedBundle(t, &fakeControl{state: cognitivecontract.ControlState{Enabled: true}})
	_ = ctx
	cases := []struct {
		name    string
		action  string
		input   string
		wantErr error
	}{
		{"unknown field", ActionStatus, `{"session_id":"s","bogus":1}`, controlaction.ErrInvalidInput},
		{"trailing JSON", ActionStatus, `{"session_id":"s"} {"extra":true}`, controlaction.ErrInvalidInput},
		{"secret sentinel", ActionPolicySet, `{"session_id":"s","enabled":true,"min_interval_ms":1,"base_revision":0,"api_key":"sk-live"}`, controlaction.ErrInvalidInput},
		{"secret sentinel nested", ActionPersonaSave, `{"session_id":"s","kind":"identity","content":"x","base_revision":0,"token":"abc"}`, controlaction.ErrInvalidInput},
		{"missing session_id", ActionStatus, `{}`, controlaction.ErrInvalidInput},
		{"malformed", ActionStatus, `{"session_id":`, controlaction.ErrInvalidInput},
	}
	for _, tc := range cases {
		if err := invokeActionErr(t, bundle, tc.action, tc.input); !errors.Is(err, tc.wantErr) {
			t.Fatalf("%s: err = %v, want %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestUnknownActionIsInvalidInput(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	err := invokeActionErr(t, bundle, "diva.cognitive.nonexistent", `{"session_id":"s"}`)
	if !errors.Is(err, controlaction.ErrInvalidInput) {
		t.Fatalf("unknown action err = %v", err)
	}
}

// --- typed handler fixtures ------------------------------------------------

func TestStatusAssemblesCapabilityStatus(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{state: cognitivecontract.ControlState{
		Enabled: true, MinIntervalMS: 500, PolicyRevision: 3, SourceID: "src-1", Phase: "idle",
	}})
	out := invokeAction(t, bundle, ActionStatus, `{"session_id":"s"}`)
	if out.Status != "ok" {
		t.Fatalf("status = %s", out.Status)
	}
	var status struct {
		ProfileID string `json:"profile_id"`
		Scope     struct {
			SubjectID string `json:"subject_id"`
			Kind      string `json:"kind"`
		} `json:"scope"`
		DestinationID string `json:"destination_id"`
		Persona       struct {
			State            string           `json:"state"`
			CurrentRevisions map[string]int64 `json:"current_revisions"`
		} `json:"persona"`
		Frozen struct {
			State string `json:"state"`
		} `json:"frozen"`
		Memory struct {
			BackendID string `json:"backend_id"`
			Health    string `json:"health"`
		} `json:"memory"`
		Cognition struct {
			Enabled        bool   `json:"enabled"`
			PolicyRevision uint64 `json:"policy_revision"`
			MinIntervalMS  int64  `json:"min_interval_ms"`
			SourceID       string `json:"source_id"`
			Phase          string `json:"phase"`
		} `json:"cognition"`
	}
	if err := json.Unmarshal(out.Value, &status); err != nil {
		t.Fatalf("status value: %v", err)
	}
	if status.ProfileID != profileID || status.Scope.Kind != "personal" || status.DestinationID != destinationID {
		t.Fatalf("status identity = %s", out.Value)
	}
	if status.Persona.State != "uninitialized" {
		t.Fatalf("persona state = %s", status.Persona.State)
	}
	if status.Frozen.State != "not_captured" {
		t.Fatalf("frozen state = %s", status.Frozen.State)
	}
	if status.Memory.Health != "unavailable" || status.Memory.BackendID == "" {
		t.Fatalf("memory block = %s", out.Value)
	}
	if !status.Cognition.Enabled || status.Cognition.PolicyRevision != 3 || status.Cognition.SourceID != "src-1" {
		t.Fatalf("cognition block = %s", out.Value)
	}
}

func TestFrozenReadFreshAuthorityReportsNotCaptured(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	out := invokeAction(t, bundle, ActionFrozenRead, `{"session_id":"s"}`)
	if out.Status != "ok" {
		t.Fatalf("frozen.read = %s", out.Status)
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out.Value, &v); err != nil || v.State != "not_captured" {
		t.Fatalf("frozen value = %s err=%v", out.Value, err)
	}
}

func TestPersonaInitializeThenReadThenSaveConflict(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	// persona.initialize seeds the authority directory.
	out := invokeAction(t, bundle, ActionPersonaInitialize, `{"session_id":"s","initialization":{"identity":"who","relationship":"rel","redline":"line","user":"usr","world":"wld"}}`)
	if out.Status != "ok" {
		t.Fatalf("persona.initialize = %s", out.Status)
	}
	// read a real kind: ok with content+revision.
	out = invokeAction(t, bundle, ActionPersonaRead, `{"session_id":"s","kind":"identity"}`)
	if out.Status != "ok" {
		t.Fatalf("persona.read = %s", out.Status)
	}
	var doc struct {
		Content  string `json:"content"`
		Revision uint64 `json:"revision"`
	}
	if err := json.Unmarshal(out.Value, &doc); err != nil {
		t.Fatalf("persona.read value: %v", err)
	}
	// stale base_revision fails, and a second failing save does not corrupt
	// the authority (no domain effects on denial).
	out = invokeAction(t, bundle, ActionPersonaSave,
		`{"session_id":"s","kind":"identity","content":"forged","base_revision":999,"reason":"probe"}`)
	if out.Status != "failed" {
		t.Fatalf("persona.save stale revision status = %s", out.Status)
	}
	out = invokeAction(t, bundle, ActionPersonaRead, `{"session_id":"s","kind":"identity"}`)
	var again struct {
		Content  string `json:"content"`
		Revision uint64 `json:"revision"`
	}
	if err := json.Unmarshal(out.Value, &again); err != nil || again.Content == "forged" {
		t.Fatalf("denied save leaked domain effect = %s", out.Value)
	}
}

func TestPersonaReadRejectsForeignKind(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	if err := invokeActionErr(t, bundle, ActionPersonaRead, `{"session_id":"s","kind":"world domination"}`); !errors.Is(err, controlaction.ErrInvalidInput) {
		t.Fatalf("foreign kind err = %v", err)
	}
}

func TestReviewDecideOnlyAcceptsAcceptOrReject(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	if err := invokeActionErr(t, bundle, ActionPersonaReviewDecide, `{"session_id":"s","review_id":"r1","decision":"maybe"}`); !errors.Is(err, controlaction.ErrInvalidInput) {
		t.Fatalf("bad decision err = %v", err)
	}
}

func TestPolicySetCASConflictsAreHonest(t *testing.T) {
	control := &casControl{revision: 2}
	_, bundle := armedBundle(t, control)
	out := invokeAction(t, bundle, ActionPolicySet, `{"session_id":"s","enabled":true,"min_interval_ms":60,"base_revision":2}`)
	if out.Status != "ok" {
		t.Fatalf("policy.set = %s", out.Status)
	}
	if control.calls != 1 || control.lastPolicy.MinIntervalMS != 60 {
		t.Fatalf("policy CAS not routed: %#v", control)
	}
	// stale base revision -> failed/revision_conflict, no second write
	out = invokeAction(t, bundle, ActionPolicySet, `{"session_id":"s","enabled":false,"min_interval_ms":0,"base_revision":2}`)
	if out.Status != "failed" || out.Error == nil || out.Error.Code != "revision_conflict" {
		t.Fatalf("stale policy.set outcome = %s", out.Value)
	}
	if control.calls != 1 {
		t.Fatal("conflicting CAS still wrote")
	}
}

func TestPolicyGetReflectsArmedState(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{state: cognitivecontract.ControlState{Enabled: true, MinIntervalMS: 250, PolicyRevision: 7}})
	out := invokeAction(t, bundle, ActionPolicyGet, `{"session_id":"s"}`)
	if out.Status != "ok" {
		t.Fatalf("policy.get = %s", out.Status)
	}
	var v struct {
		Enabled        bool   `json:"enabled"`
		MinIntervalMS  int64  `json:"min_interval_ms"`
		PolicyRevision uint64 `json:"policy_revision"`
	}
	if err := json.Unmarshal(out.Value, &v); err != nil || !v.Enabled || v.PolicyRevision != 7 || v.MinIntervalMS != 250 {
		t.Fatalf("policy value = %s", out.Value)
	}
}

func TestTriggerAndCancelHitArmedControl(t *testing.T) {
	control := &fakeControl{state: cognitivecontract.ControlState{Enabled: true}}
	_, bundle := armedBundle(t, control)
	out := invokeAction(t, bundle, ActionTrigger, `{"session_id":"s"}`)
	if out.Status != "ok" || !control.triggers {
		t.Fatalf("trigger outcome = %s triggers=%v", out.Status, control.triggers)
	}
	out = invokeAction(t, bundle, ActionCancel, `{"session_id":"s","run_id":"run-9"}`)
	if out.Status != "ok" || control.cancelled != "run-9" {
		t.Fatalf("cancel outcome = %s cancelled=%q", out.Status, control.cancelled)
	}
}

func TestResultsListPagesRealReceipts(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	out := invokeAction(t, bundle, ActionResultsList, `{"session_id":"s","limit":5}`)
	if out.Status != "ok" {
		t.Fatalf("results.list = %s", out.Status)
	}
	var page struct {
		Entries []json.RawMessage `json:"entries"`
		Next    string            `json:"next_cursor"`
	}
	if err := json.Unmarshal(out.Value, &page); err != nil {
		t.Fatalf("results value: %v", err)
	}
}

func TestMemorySurfaceDegradesWithoutBackend(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	for _, call := range []struct {
		action string
		input  string
	}{
		{ActionMemorySearch, `{"session_id":"s","query":"q","limit":5,"budget_chars":100}`},
		{ActionMemoryExpand, `{"session_id":"s","card_id":"c","expected_revision":0,"budget_chars":100}`},
		{ActionMemoryMutate, `{"session_id":"s","mutation":{"operation":"put","body":"{}"}}`},
		{ActionMemoryReceipt, `{"session_id":"s","operation_id":"op"}`},
	} {
		out := invokeAction(t, bundle, call.action, call.input)
		if out.Status == "ok" {
			t.Fatalf("%s succeeded without a memory backend", call.action)
		}
	}
}

func TestDispatcherNeverExposesInternals(t *testing.T) {
	_, bundle := armedBundle(t, &fakeControl{})
	out := invokeAction(t, bundle, ActionMemorySearch, `{"session_id":"s","query":"q","limit":5,"budget_chars":100}`)
	if out.Error == nil || out.Error.Code == "" || out.Error.Message == "" {
		t.Fatalf("error block missing: %s", out.Value)
	}
	if strings.Contains(out.Error.Message, "goroutine") || strings.Contains(out.Error.Message, ".go:") {
		t.Fatalf("internals leaked: %s", out.Value)
	}
}

// --- helpers ----------------------------------------------------------------

// casControl records CAS writes and fails on stale base revisions, matching
// the runtime contract the module routes into.
type casControl struct {
	fakeControl
	revision   uint64
	calls      int
	lastPolicy laputaevolution.TriggerPolicy
}

func (c *casControl) SetPolicyCAS(_ context.Context, policy laputaevolution.TriggerPolicy, base uint64) (cognitivecontract.ControlState, error) {
	if base != c.revision {
		return cognitivecontract.ControlState{}, runtime.ErrPolicyConflict
	}
	c.calls++
	c.lastPolicy = policy
	c.revision++
	return c.state, nil
}
