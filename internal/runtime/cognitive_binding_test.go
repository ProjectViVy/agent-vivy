package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ProjectViVy/inofy"
	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

// scriptedModel answers cognitive inference children with stage-keyed
// canned JSON. It is a test fixture, not a product provider.
type scriptedModel struct {
	replies  map[string]string
	calls    int
	onStream func()
}

func (m *scriptedModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.calls++
	if m.onStream != nil {
		m.onStream()
	}
	lastUser := ""
	for i := len(input) - 1; i >= 0; i-- {
		if input[i] != nil && input[i].Role == domain.RoleUser {
			lastUser = input[i].Content
			break
		}
	}
	body := "{}"
	for key, reply := range m.replies {
		if strings.Contains(lastUser, key) {
			body = reply
			break
		}
	}
	return &scriptedStream{message: &domain.Message{Role: domain.RoleAssistant, Content: body}}, nil
}

type scriptedStream struct {
	message *domain.Message
	done    bool
}

func (s *scriptedStream) Recv() (*domain.Message, error) {
	if s.done {
		return nil, io.EOF
	}
	s.done = true
	return s.message, nil
}

// fakeCognitiveDomain is the bound evolution Domain for trusted strategy
// tests: scripted evidence plus a receipt-indexed effect sink.
type fakeCognitiveDomain struct {
	batch      laputaevolution.EvidenceBatch
	collects   int
	applyCalls int
	failAt     int
	applied    []laputaevolution.Effect
	receipts   map[string]laputaevolution.EffectReceipt
	applyErr   error
	collectErr error
	// gate, when non-nil, blocks Collect until closed so a test can hold a
	// workflow run mid-flight.
	gate       chan struct{}
	entered    chan struct{}
	lastWindow laputaevolution.Window
}

func (d *fakeCognitiveDomain) Collect(ctx context.Context, w laputaevolution.Window) (laputaevolution.EvidenceBatch, error) {
	d.collects++
	d.lastWindow = w
	if d.collectErr != nil {
		return laputaevolution.EvidenceBatch{}, d.collectErr
	}
	if d.gate != nil {
		if d.entered != nil {
			select {
			case d.entered <- struct{}{}:
			default:
			}
		}
		select {
		case <-d.gate:
		case <-ctx.Done():
			return laputaevolution.EvidenceBatch{}, ctx.Err()
		}
	}
	b := d.batch
	b.Window = w
	return b, nil
}

func (d *fakeCognitiveDomain) Apply(_ context.Context, e laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	d.applyCalls++
	if d.applyErr != nil {
		return laputaevolution.EffectReceipt{}, d.applyErr
	}
	if d.failAt > 0 && d.applyCalls == d.failAt {
		return laputaevolution.EffectReceipt{}, errors.New("fixture: bound domain rejected the effect")
	}
	receipt := laputaevolution.EffectReceipt{
		OperationID:   e.OperationID,
		PayloadDigest: e.PayloadDigest,
		Status:        laputaevolution.StatusApplied,
		TargetRef:     "fixture-target",
		Revision:      1,
	}
	if d.receipts == nil {
		d.receipts = map[string]laputaevolution.EffectReceipt{}
	}
	d.receipts[e.OperationID] = receipt
	d.applied = append(d.applied, e)
	return receipt, nil
}

func (d *fakeCognitiveDomain) Lookup(_ context.Context, operationID string) (laputaevolution.EffectReceipt, error) {
	if r, ok := d.receipts[operationID]; ok {
		return r, nil
	}
	return laputaevolution.EffectReceipt{}, &laputaevolution.ContractError{Code: laputaevolution.ErrEffectNotFound, Message: "not recorded"}
}

type atomicMissionCognitiveDomain struct {
	*fakeCognitiveDomain
	revision uint64
	err      error
}

func (d *atomicMissionCognitiveDomain) ApplyAtMissionRevision(_ context.Context, revision uint64, _ laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	d.revision = revision
	return laputaevolution.EffectReceipt{}, d.err
}

func TestMissionPinnedDomainUsesAtomicOwnerGate(t *testing.T) {
	wantErr := errors.New("fixture: owner gate rejected stale Mission")
	domain := &atomicMissionCognitiveDomain{fakeCognitiveDomain: &fakeCognitiveDomain{}, err: wantErr}
	pinned := missionPinnedCognitiveDomain{
		delegate: domain,
		binding:  laputaevolution.RunBinding{MissionRevision: 7},
	}
	_, err := pinned.Apply(context.Background(), laputaevolution.Effect{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("atomic owner gate error = %v, want %v", err, wantErr)
	}
	if domain.revision != 7 {
		t.Fatalf("owner gate checked Mission revision %d, want per-run pin 7", domain.revision)
	}
	if domain.applyCalls != 0 {
		t.Fatalf("non-atomic Apply called %d times", domain.applyCalls)
	}
}

func TestMissionPinnedDomainFailsClosedWithoutAtomicOwnerGate(t *testing.T) {
	domain := &fakeCognitiveDomain{}
	pinned := missionPinnedCognitiveDomain{
		delegate: domain,
		mission:  &fakeMission{rev: 7},
		binding:  laputaevolution.RunBinding{MissionRevision: 7},
	}
	_, err := pinned.Apply(context.Background(), laputaevolution.Effect{})
	if !errors.Is(err, errCognitiveMissionGateUnavailable) {
		t.Fatalf("missing atomic owner gate error = %v", err)
	}
	if domain.applyCalls != 0 {
		t.Fatalf("unguarded Apply called %d times", domain.applyCalls)
	}
}

// cognitiveFixtureRunBinding is the scope the trusted fixtures persist; the
// bound composition must carry the same pins before effects may run.
func cognitiveFixtureRunBinding() laputaevolution.RunBinding {
	return laputaevolution.RunBinding{
		SubjectID: "profile-1", WorkspaceID: "ws-1", DestinationID: "mentle",
		PolicyRevision: "pol-1", StrategyDigest: "dig-1", MissionRevision: 1,
	}
}

func cognitiveInput(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(laputaevolution.Input{
		Binding: cognitiveFixtureRunBinding(),
		Window:  laputaevolution.Window{SourceID: "activity", After: 0, Through: 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCognitiveModelAuthoredEvolutionNodeRejected keeps the authored graph
// ceiling: a model-supplied definition may never reference the trusted
// laputa.evolution strategy nodes.
func TestCognitiveModelAuthoredEvolutionNodeRejected(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-cog-authored")
	parentRunID := domain.RunID("run-cog-authored-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})

	evil := `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
		{"id":"c","kind":"call","type":"laputa.evolution.collect@1"}
	],"edges":[],"exits":["c"],"outputs":{"o":{"source":"c"}}}}`
	if _, err := svc.ProposeINOFYWorkflow(ctx, parentRunID, json.RawMessage(evil)); err == nil {
		t.Fatal("authored laputa.evolution graph admitted")
	}
	if _, err := svc.StartINOFYWorkflow(ctx, parentRunID, "cog-evil", json.RawMessage(evil)); err == nil {
		t.Fatal("authored laputa.evolution graph started")
	}
}

// TestCognitiveStartRequiresBinding fails closed when no trusted Domain is
// wired: a trusted strategy can never run unbound.
func TestCognitiveStartRequiresBinding(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	parentRunID := domain.RunID("run-cog-nobinding-parent")
	prepareChildSessionAuthorizer(t, svc, backend, "sess-cog-nobinding", parentRunID, nil)

	if _, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-nobinding", TrustedStrategyDIVA, cognitiveInput(t)); !errors.Is(err, ErrCognitiveUnavailable) {
		t.Fatalf("unbound start = %v", err)
	}
}

// TestCognitiveForgedStrategyRejected rejects strategy ids that the host
// catalog does not define: callers cannot forge a host binding.
func TestCognitiveForgedStrategyRejected(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Cognitive = &CognitiveBinding{Domain: &fakeCognitiveDomain{}, Binding: cognitiveFixtureRunBinding()}
	parentRunID := domain.RunID("run-cog-forged-parent")
	prepareChildSessionAuthorizer(t, svc, backend, "sess-cog-forged", parentRunID, nil)

	if _, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-forged", "model/evil-strategy", cognitiveInput(t)); err == nil {
		t.Fatal("forged strategy admitted")
	}
}

// TestCognitiveWorkflowRunsToNoChangeOutcome drives a committed trusted
// workflow end to end: an empty evidence batch short-circuits to a
// no_change outcome with zero model calls.
func TestCognitiveWorkflowRunsToNoChangeOutcome(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{}
	svc, backend := inofyExecService(t, &scriptedModel{replies: map[string]string{}})
	svc.deps.Cognitive = &CognitiveBinding{Domain: d, Binding: cognitiveFixtureRunBinding()}
	sessionID := domain.SessionID("sess-cog-empty")
	parentRunID := domain.RunID("run-cog-empty-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, nil)

	started, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-empty", TrustedStrategyDIVA, cognitiveInput(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !started.Created || started.Run.Kind != domain.RunKindWorkflow {
		t.Fatalf("start result = %+v", started)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	details, err := svc.GetWorkflow(ctx, started.Run.ID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if details.EngineStatus != string(inofy.RunSucceeded) {
		t.Fatalf("engine status = %q", details.EngineStatus)
	}
	if d.collects != 1 {
		t.Fatalf("collects = %d", d.collects)
	}
	if len(d.applied) != 0 {
		t.Fatalf("empty batch applied effects: %+v", d.applied)
	}
	var outcome map[string]any
	if err := json.Unmarshal([]byte(details.Outputs["outcome"]), &outcome); err != nil {
		t.Fatalf("outcome not JSON: %v (%q)", err, details.Outputs["outcome"])
	}
	if outcome["status"] != "no_change" {
		t.Fatalf("outcome = %v", outcome)
	}
}

// TestCognitiveWorkflowAppliesBoundEffects proves strategy effects reach the
// bound Domain through the durable workflow path: the scripted model's Work
// patch is applied under the committed operation key.
func TestCognitiveWorkflowAppliesBoundEffects(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{
		ActivityRevision: 3,
		Entries: []laputaevolution.Entry{{
			ID: "e1", Section: "work", Field: "next", SessionID: "s1",
			EventID: "ev1", OccurredAt: "2026-10-02T00:00:00Z", Body: "follow up",
		}},
		Sources: []laputaevolution.SourceRef{{
			SourceID: "activity", RecordID: "rec-1", Revision: 1,
			Scope: laputaevolution.Scope{SubjectID: "profile-1", Kind: laputaevolution.ScopePersonal},
		}},
	}}
	model := &scriptedModel{replies: map[string]string{
		"stage=reconcile": `{"base_revision":3,"changes":[{"kind":"add","entry_id":"","field":"next","body":"ship the fix","sources":[]}]}`,
		"stage=reflect":   `{"no_change_reason":"reconcile covered it"}`,
	}}
	svc, backend := inofyExecService(t, model)
	svc.deps.Cognitive = &CognitiveBinding{Domain: d, Binding: cognitiveFixtureRunBinding()}
	sessionID := domain.SessionID("sess-cog-apply")
	parentRunID := domain.RunID("run-cog-apply-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, nil)

	started, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-apply", TrustedStrategyDIVA, cognitiveInput(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	details, err := svc.GetWorkflow(ctx, started.Run.ID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if details.EngineStatus != string(inofy.RunSucceeded) {
		t.Fatalf("engine status = %q", details.EngineStatus)
	}
	if len(d.applied) != 1 || d.applied[0].WorkPatch == nil {
		t.Fatalf("applied = %+v", d.applied)
	}
	if !strings.HasSuffix(d.applied[0].OperationID, ":work") {
		t.Fatalf("work effect op id = %q", d.applied[0].OperationID)
	}
	var outcome map[string]any
	if err := json.Unmarshal([]byte(details.Outputs["outcome"]), &outcome); err != nil {
		t.Fatalf("outcome not JSON: %v", err)
	}
	if outcome["status"] != "applied" {
		t.Fatalf("outcome = %v", outcome)
	}
}

func TestCognitiveMissionChangeDuringInferenceBlocksApply(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{
		ActivityRevision: 3,
		Entries: []laputaevolution.Entry{{
			ID: "e1", Section: "work", Field: "next", SessionID: "s1",
			EventID: "ev1", OccurredAt: "2026-10-02T00:00:00Z", Body: "follow up",
		}},
	}}
	mission := &fakeMission{rev: 2}
	model := &scriptedModel{
		replies: map[string]string{
			"stage=reconcile": `{"base_revision":3,"changes":[{"kind":"add","entry_id":"","field":"next","body":"ship the fix","sources":[]}]}`,
			"stage=reflect":   `{"no_change_reason":"reconcile covered it"}`,
		},
		onStream: func() { mission.rev = 3 },
	}
	service, backend := inofyExecService(t, model)
	binding := cognitiveFixtureRunBinding()
	binding.MissionRevision = 2
	service.deps.Cognitive = &CognitiveBinding{Domain: d, Binding: binding, Mission: mission}
	sessionID := domain.SessionID("sess-cog-mission-race")
	parentRunID := domain.RunID("run-cog-mission-race-parent")
	prepareChildSessionAuthorizer(t, service, backend, sessionID, parentRunID, nil)
	input, err := json.Marshal(laputaevolution.Input{
		Binding: binding,
		Window:  laputaevolution.Window{SourceID: "activity", After: 0, Through: 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartCognitiveWorkflow(ctx, parentRunID, "cog-mission-race", TrustedStrategyDIVA, input)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	var terminal domain.Run
	for time.Now().Before(deadline) {
		current, getErr := backend.GetRun(ctx, started.Run.ID)
		if getErr == nil && current.Status.Terminal() {
			terminal = current
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if terminal.ID == "" {
		t.Fatal("cognitive workflow did not settle")
	}
	if len(d.applied) != 0 {
		t.Fatalf("Mission changed during inference (run status %s) but effects applied: %+v", terminal.Status, d.applied)
	}
}

func TestCognitiveMissionAssignmentAfterAdmissionFailsBeforeEffect(t *testing.T) {
	ctx := context.Background()
	service, _ := inofyExecService(t, cognitiveTestModel())
	binding := cognitiveFixtureRunBinding()
	binding.MissionRevision = 0
	service.deps.Cognitive = &CognitiveBinding{
		Domain: &fakeCognitiveDomain{}, Binding: binding, Mission: &fakeMission{rev: 1},
	}
	input, err := json.Marshal(laputaevolution.Input{
		Binding: binding,
		Window:  laputaevolution.Window{SourceID: "activity", After: 0, Through: 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.workflowNodes(ctx, "parent", TrustedStrategyDIVA, input); laputaevolution.CodeOf(err) != laputaevolution.ErrMissionRevisionChanged {
		t.Fatalf("unassigned Mission pin accepted after assignment: %v", err)
	}
}

// TestCognitiveEffectFailureStopsRemaining proves a bound-Domain rejection
// (the grant-revocation analogue) preserves the committed receipt trail and
// stops all later effects: the run finishes with a partial outcome, never
// applying the queued candidates.
func TestCognitiveEffectFailureStopsRemaining(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{
		failAt: 2,
		batch: laputaevolution.EvidenceBatch{
			ActivityRevision: 3,
			Entries: []laputaevolution.Entry{{
				ID: "e1", Section: "work", Field: "next", SessionID: "s1",
				EventID: "ev1", OccurredAt: "2026-10-02T00:00:00Z", Body: "follow up",
			}},
		},
	}
	model := &scriptedModel{replies: map[string]string{
		"stage=reconcile": `{"base_revision":3,"changes":[{"kind":"add","entry_id":"","field":"next","body":"ship the fix","sources":[]}]}`,
		"stage=reflect": `{"candidates":[
			{"kind":"memory_mutation","memory_mutation":{"operation":"create","record_id":"r1","body":"observed a","sources":[],"inference":"observed"}},
			{"kind":"memory_mutation","memory_mutation":{"operation":"create","record_id":"r2","body":"observed b","sources":[],"inference":"observed"}}
		]}`,
	}}
	svc, backend := inofyExecService(t, model)
	svc.deps.Cognitive = &CognitiveBinding{Domain: d, Binding: cognitiveFixtureRunBinding()}
	parentRunID := domain.RunID("run-cog-stopfx-parent")
	prepareChildSessionAuthorizer(t, svc, backend, "sess-cog-stopfx", parentRunID, nil)

	started, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-stopfx", TrustedStrategyDIVA, cognitiveInput(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	details, err := svc.GetWorkflow(ctx, started.Run.ID)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if details.EngineStatus != string(inofy.RunSucceeded) {
		t.Fatalf("engine status = %q", details.EngineStatus)
	}
	if len(d.applied) != 1 || d.applied[0].WorkPatch == nil {
		t.Fatalf("later effects were not stopped: %+v", d.applied)
	}
	var outcome map[string]any
	if err := json.Unmarshal([]byte(details.Outputs["outcome"]), &outcome); err != nil {
		t.Fatalf("outcome not JSON: %v", err)
	}
	if outcome["status"] != "partial" || outcome["reason"] != "apply_error" {
		t.Fatalf("outcome = %v", outcome)
	}
}

// TestCognitiveStartDedupesOnOperationKey binds one run per operation key:
// identical input joins, a changed payload is a revision conflict.
func TestCognitiveStartDedupesOnOperationKey(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Cognitive = &CognitiveBinding{Domain: &fakeCognitiveDomain{}, Binding: cognitiveFixtureRunBinding()}
	parentRunID := domain.RunID("run-cog-dedupe-parent")
	prepareChildSessionAuthorizer(t, svc, backend, "sess-cog-dedupe", parentRunID, nil)

	first, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-dedupe", TrustedStrategyDIVA, cognitiveInput(t))
	if err != nil || !first.Created {
		t.Fatalf("first start = %+v %v", first, err)
	}
	waitForRunStatus(t, backend, first.Run.ID, domain.RunCompleted)

	second, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-dedupe", TrustedStrategyDIVA, cognitiveInput(t))
	if err != nil {
		t.Fatalf("rejoin = %v", err)
	}
	if second.Created || second.Run.ID != first.Run.ID {
		t.Fatalf("rejoin created a duplicate: %+v", second)
	}
	other := json.RawMessage(`{"binding":{"subject_id":"profile-2"},"window":{"source_id":"activity","after":0,"through":1}}`)
	if _, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-dedupe", TrustedStrategyDIVA, other); !errors.Is(err, storage.ErrWorkflowRevisionConflict) {
		t.Fatalf("changed input = %v", err)
	}
}

// TestCognitiveChildTaskWideningRejected proves the trusted catalog admits
// only the laputa.evolution node set: a trusted definition cannot smuggle
// child-task effects inside the strategy catalog.
func TestCognitiveChildTaskWideningRejected(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Cognitive = &CognitiveBinding{Domain: &fakeCognitiveDomain{}, Binding: cognitiveFixtureRunBinding()}
	parentRunID := domain.RunID("run-cog-wide-parent")
	prepareChildSessionAuthorizer(t, svc, backend, "sess-cog-wide", parentRunID, nil)

	if _, err := svc.StartCognitiveWorkflow(ctx, parentRunID, "cog-wide", "vivy.child-task@1", cognitiveInput(t)); err == nil {
		t.Fatal("non-strategy id admitted through the trusted path")
	}

	// The authored path is untouched: child-task graphs still admit there.
	admitted, err := validateINOFYDefinition(ctx, json.RawMessage(inofyTwoNodeDefinition), []string{tools.EchoInfoName})
	if err != nil || admitted.Program == nil {
		t.Fatalf("authored child-task admission broke: %v", err)
	}
}
