package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

func TestApprovalRecoveryKeepsPrivateAdmittedPolicy(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	const canary = "D08-PRIVATE-POLICY-PATH-CANARY"
	policy, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {Rules: []PolicyRule{{Tool: "unused", Field: "path", Prefix: canary, Decision: domain.PolicyDeny}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := svc.engine.cfg
	cfg.Policy = policy
	svc.engine, err = NewEngine(ctx, NewApprovalFlowModel(), svc.engine.activeTools, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mustCreateSession(t, backend, "d08-policy-restart")
	runID, err := svc.Run(ctx, "d08-policy-restart", "write a note after approval")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)
	admitted, err := backend.LoadRunPrompt(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := storage.ValidateRunPromptSnapshot(admitted)
	if err != nil || !bytes.Contains([]byte(payload.Policy.Definition), []byte(canary)) {
		t.Fatalf("actual private policy absent: %v", err)
	}
	// A fresh process has different current configuration. Its pending request
	// must retain the admitted private value rather than manufacture current truth.
	fresh, _ := restartService(t, backend)
	t.Cleanup(func() { fresh.CancelAll(); fresh.WaitIdle(context.Background()) })
	if err := fresh.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	fresh.mu.Lock()
	recovered := fresh.pending[runID].snapshot
	fresh.mu.Unlock()
	if recovered != payload.Policy {
		t.Fatal("restart replaced admitted policy with current configuration")
	}
	if err := fresh.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	notes, err := backend.ListNotes(ctx)
	if err != nil || len(notes) != 0 {
		t.Fatalf("changed policy authorized an effect: %d / %v", len(notes), err)
	}
	for _, event := range replayAll(t, backend, runID) {
		if bytes.Contains(event.Payload, []byte(canary)) || bytes.Contains(event.Payload, []byte(`"policy_hash"`)) || bytes.Contains(event.Payload, []byte(`"prompt_digest"`)) {
			t.Fatalf("private policy or digest leaked in %s", event.Type)
		}
	}
	var roundtrip storage.RunPromptPayload
	if err := json.Unmarshal(admitted.Payload, &roundtrip); err != nil || roundtrip.Policy != payload.Policy {
		t.Fatal("private snapshot value changed")
	}
}

type d08MaskResolver func(context.Context, domain.SessionID) (maskcontract.Capture, error)

func (resolve d08MaskResolver) Capture(ctx context.Context, session domain.SessionID) (maskcontract.Capture, error) {
	return resolve(ctx, session)
}

func TestApprovalRecoveryReusesAdmittedPromptAfterMaskEdit(t *testing.T) {
	ctx := context.Background()
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	const session domain.SessionID = "d08-mask-restart"
	mustCreateSession(t, backend, session)
	definition, err := backend.CreateCustomMask(ctx, maskcontract.CreateRequest{OperationID: "00000000-0000-4000-8000-000000000008", Name: "Admitted style", Body: "D08-OLD-MASK-BODY"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.SetMaskSelection(ctx, maskcontract.SetSelectionRequest{SessionID: session, MaskID: definition.ID}); err != nil {
		t.Fatal(err)
	}
	svc.deps.MaskResolver = d08MaskResolver(backend.ReadMaskCapture)
	svc.deps.MaskFrame = "D08-ADMITTED-FRAME"
	runID, err := svc.Run(ctx, session, "write a note after approval")
	if err != nil {
		t.Fatal(err)
	}
	approval := waitForPendingApproval(t, backend, runID)
	waitForApprovalEvent(t, backend, runID)
	if _, err := backend.UpdateCustomMask(ctx, maskcontract.UpdateRequest{ID: definition.ID, ExpectedRevision: definition.Revision, Name: definition.Name, Body: "D08-NEW-MASK-BODY"}); err != nil {
		t.Fatal(err)
	}
	fresh, _ := restartService(t, backend)
	t.Cleanup(func() { fresh.CancelAll(); fresh.WaitIdle(context.Background()) })
	fresh.deps.MaskResolver = d08MaskResolver(func(context.Context, domain.SessionID) (maskcontract.Capture, error) {
		return maskcontract.Capture{}, errors.New("current catalog must not be resolved on resume")
	})
	fresh.deps.MaskFrame = "D08-CURRENT-FRAME"
	recorder := &recordingChatModel{inner: NewScriptedModel(testAssistant("finished"))}
	fresh.engine, err = NewEngine(ctx, recorder, fresh.engine.activeTools, fresh.engine.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := fresh.DecideApproval(ctx, approval.ID, domain.ApprovalApproved); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
	if len(recorder.inputs) != 1 {
		t.Fatalf("resumed model calls=%d", len(recorder.inputs))
	}
	input := ""
	for _, message := range recorder.inputs[0] {
		input += agenticMessageText(message)
	}
	if !strings.Contains(input, "D08-OLD-MASK-BODY") || !strings.Contains(input, "D08-ADMITTED-FRAME") || strings.Contains(input, "D08-NEW-MASK-BODY") || strings.Contains(input, "D08-CURRENT-FRAME") {
		t.Fatal("resume substituted current configuration for admitted prompt")
	}
	for _, event := range replayAll(t, backend, runID) {
		if bytes.Contains(event.Payload, []byte("D08-OLD-MASK-BODY")) {
			t.Fatalf("private prompt leaked in %s", event.Type)
		}
	}
}
