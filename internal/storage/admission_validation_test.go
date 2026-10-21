package storage

import (
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

func validAdmissionFixture(t *testing.T) RunAdmission {
	t.Helper()
	runID := domain.RunID("run-admission-test")
	sessionID := domain.SessionID("session-admission-test")
	payload, err := json.Marshal(RunPromptPayload{Instruction: "authoritative instruction", Policy: domain.PolicySnapshot{Profile: domain.PolicyProfileDefault, Definition: `{"Default":"","Rules":null}`}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &RunPromptSnapshot{
		RunID: runID, SchemaVersion: 1, ComposerVersion: "mask-prompt/1", GenerationID: "generation-test",
		Payload: payload,
	}
	marker, err := json.Marshal(map[string]any{"prompt_schema": snapshot.SchemaVersion})
	if err != nil {
		t.Fatal(err)
	}
	return RunAdmission{
		Message: domain.Message{ID: "message-admission-test", SessionID: sessionID, RunID: runID, Role: domain.RoleUser},
		Run:     domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, Kind: domain.RunKindPrimary},
		Started: domain.RunEvent{RunID: runID, Type: domain.EventRunStarted, PayloadVersion: 1, Payload: marker},
		Prompt:  snapshot,
	}
}

func TestValidateRunAdmissionRequiresPromptMarkerPair(t *testing.T) {
	in := validAdmissionFixture(t)
	if err := ValidateRunAdmissionInput(in); err != nil {
		t.Fatalf("valid admission rejected: %v", err)
	}

	in.Started.Payload = []byte(`{"prompt_schema":2}`)
	if err := ValidateRunAdmissionInput(in); err == nil {
		t.Fatal("prompt marker mismatch was accepted")
	}

	in = validAdmissionFixture(t)
	in.Prompt = nil
	if err := ValidateRunAdmissionInput(in); err == nil {
		t.Fatal("prompt marker without snapshot was accepted")
	}
}

func TestValidateRunAdmissionLegacyHasNoPromptMarker(t *testing.T) {
	in := validAdmissionFixture(t)
	in.Prompt = nil
	in.Started.Payload = []byte(`{"provider":"fixture"}`)
	if err := ValidateRunAdmissionInput(in); err != nil {
		t.Fatalf("legacy admission rejected: %v", err)
	}
}

func TestValidateRunPromptSnapshotRejectsNonCanonicalPayload(t *testing.T) {
	in := validAdmissionFixture(t)
	if in.Prompt == nil {
		t.Fatal("fixture did not include a prompt snapshot")
	}

	// Keep the payload semantically equivalent. Run-owned
	// snapshots are persisted from json.Marshal's deterministic bytes, so a
	// later reader must not silently normalize a different representation.
	in.Prompt.Payload = append(in.Prompt.Payload, '\n')
	if _, err := ValidateRunPromptSnapshot(*in.Prompt); err == nil {
		t.Fatal("non-canonical prompt payload was accepted")
	}
}
