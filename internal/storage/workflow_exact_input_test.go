package storage

import (
	"agent-vivy/internal/domain"
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestWorkflowStepInputBindsPrivateSemanticValues(t *testing.T) {
	original := WorkflowStepCommit{RunID: "run", CommitID: "claim", Epoch: 1, ProgramID: "program", HostBindingID: "host", Target: WorkflowStepAdmitted, AdmittedInput: json.RawMessage(`{"value":9007199254740993}`), ResumeKey: "resume", ResumeAnswersJSON: json.RawMessage(`{"question":"yes"}`), Events: []WorkflowStepEvent{{Type: domain.EventWorkflowAdmitted, CreatedAt: 10, PayloadVersion: 1, Payload: []byte(`{"limits":{}}`)}}, Results: []WorkflowStepResult{{Path: "node", Attempt: 1, Output: []byte(`{"result":"done"}`), Bytes: 17}}, Checkpoint: &WorkflowStepCheckpoint{Envelope: []byte(`{"program_id":"program","payload":"Ynl0ZXM="}`)}}
	baseline, err := WorkflowStepInput(original)
	if err != nil {
		t.Fatal(err)
	}
	copy := original
	copy.Epoch = 4
	copy.Events = append([]WorkflowStepEvent(nil), original.Events...)
	copy.Events[0].CreatedAt = 99
	replay, err := WorkflowStepInput(copy)
	if err != nil || !bytes.Equal(baseline, replay) {
		t.Fatalf("receipt metadata changed semantic input: %v", err)
	}
	changes := map[string]func(*WorkflowStepCommit){
		"private input":   func(c *WorkflowStepCommit) { c.AdmittedInput = json.RawMessage(`{"value":9007199254740992}`) },
		"resume key":      func(c *WorkflowStepCommit) { c.ResumeKey = "foreign" },
		"private answers": func(c *WorkflowStepCommit) { c.ResumeAnswersJSON = json.RawMessage(`{"question":"no"}`) },
		"program":         func(c *WorkflowStepCommit) { c.ProgramID = "foreign" },
		"host":            func(c *WorkflowStepCommit) { c.HostBindingID = "foreign" },
		"checkpoint": func(c *WorkflowStepCommit) {
			c.Checkpoint = &WorkflowStepCheckpoint{Envelope: []byte(`{"program_id":"foreign"}`)}
		},
		"result": func(c *WorkflowStepCommit) {
			c.Results = []WorkflowStepResult{{Path: "node", Attempt: 1, Output: []byte(`{"result":"changed"}`), Bytes: 20}}
		},
		"event": func(c *WorkflowStepCommit) {
			c.Events = []WorkflowStepEvent{{Type: domain.EventWorkflowStarted, Payload: []byte(`{}`)}}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			candidate := original
			change(&candidate)
			actual, err := WorkflowStepInput(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(actual, baseline) {
				t.Fatal("changed actual input remained replayable")
			}
		})
	}
}

func TestWorkflowResumeUsesPrivateAnswers(t *testing.T) {
	projection := &WorkflowStepProjection{Status: WorkflowStepWaiting, Epoch: 1, Revision: 3, WaitsJSON: []byte(`["q"]`)}
	commit := WorkflowStepCommit{RunID: "run", CommitID: "resume", Epoch: 2, ProgramID: "program", HostBindingID: "host", Expected: WorkflowStepWaiting, Target: WorkflowStepRunning, ResumeKey: "key", ResumeAnswersJSON: json.RawMessage(`{"q":"approved"}`), Events: []WorkflowStepEvent{{Type: domain.EventWorkflowResumed, Payload: []byte(`{"idempotency_key":"forged-public","answers":{"q":"wrong"}}`)}}}
	next, err := WorkflowStepDerive(projection, commit, domain.WorkflowRevision{})
	if err != nil {
		t.Fatal(err)
	}
	if next.ResumeKey != "key" || string(next.ResumeAnswersJSON) != `{"q":"approved"}` || len(next.WaitsJSON) != 0 {
		t.Fatalf("private claim not atomic: %+v", next)
	}
	commit.ResumeAnswersJSON = json.RawMessage(`{"q":"changed"}`)
	if _, err := WorkflowStepDerive(next, commit, domain.WorkflowRevision{}); !errors.Is(err, ErrWorkflowStepIdempotency) {
		t.Fatalf("changed answers under same key must conflict: %v", err)
	}
	commit.ResumeAnswersJSON = nil
	if _, err := WorkflowStepDerive(projection, commit, domain.WorkflowRevision{}); !errors.Is(err, ErrWorkflowStepIdentity) {
		t.Fatalf("missing private claim must not reconstruct public answers: %v", err)
	}
}

func TestWorkflowAdmissionInputPreservesJSONNumbers(t *testing.T) {
	original := domain.WorkflowRevision{SchemaVersion: 3, DescriptorJSON: []byte(`{"graph":{}}`), AuthorityJSON: []byte(`{"tools":[]}`), InputJSON: []byte(`{"value":9007199254740993}`), EffectiveLimits: []byte(`{}`)}
	changed := original
	changed.InputJSON = []byte(`{"value":9007199254740992}`)
	if EqualWorkflowAdmissionInput(original, changed) {
		t.Fatal("distinct exact integer inputs collapsed")
	}
	changed = original
	changed.InputJSON = []byte("{\n\"value\":9007199254740993}")
	if !EqualWorkflowAdmissionInput(original, changed) {
		t.Fatal("equal normalized input conflicted")
	}
}
