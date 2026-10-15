package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
)

func TestIssue58CodingWorkerWorkspaceIsolation(t *testing.T) {
	for _, kind := range []string{"ordinary-coding", "resident-coding-frozen", "resident-first-arrival", "worker", "private", "restricted", "foreign"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t)
			f.completed(t, "source-A", "shared-private-original")
			if _, err := f.owner.CommitRecap(context.Background(), cognitivecontract.RecapCommit{OperationID: "explicit", Cause: "edit", Text: "resident-only recap secret"}); err != nil {
				t.Fatal(err)
			}
			preparer := &recordingPreparer{prepared: cognitivecontract.PreparedPrimaryContext{Text: frozenSlotOrderText, Digest: "fixture-frozen"}}
			f.svc.deps.Cognitive = &CognitiveBinding{Primary: preparer}
			f.svc.deps.Admission = f.b
			f.svc.deps.GenerationID = "issue58-isolation"
			if kind == "resident-first-arrival" {
				preparer.err = cognitivecontract.ErrPersonaUninitialized
			}
			if kind == "worker" {
				workspaces, err := NewSessionWorkspaceManager(t.TempDir(), f.b, f.b)
				if err != nil {
					t.Fatal(err)
				}
				f.svc.deps.Workspaces = workspaces
				prepareChildSessionAuthorizer(t, f.svc, f.b, "main-reader", "worker-parent", nil)
				child, err := f.svc.StartOneShotChild(context.Background(), OneShotChildRequest{ParentRunID: "worker-parent", Task: "explicit worker task"})
				if err != nil {
					t.Fatal(err)
				}
				waitForRunStatus(t, f.b, child.Run.ID, domain.RunCompleted)
				f.svc.WaitIdle(context.Background())
			} else {
				sid := domain.SessionID("main-reader")
				face := domain.FaceCode
				if kind == "ordinary-coding" {
					sid = "ordinary-code"
					mustCreateSession(t, f.b, sid)
				}
				if kind == "resident-first-arrival" {
					face = domain.FaceWeb
				}
				if kind == "private" || kind == "restricted" || kind == "foreign" {
					sid = domain.SessionID("ineligible-" + kind)
					mustCreateSession(t, f.b, sid)
					source := conformance.Issue58Source(sid, false)
					if kind == "foreign" {
						source.SubjectID = "foreign-subject"
					} else {
						source.Scope = kind
					}
					f.sources[sid] = source
					if _, err := f.svc.Run(context.Background(), sid, "must refuse"); err == nil || len(f.capture()) != 0 {
						t.Fatal("ineligible source acquired resident path")
					}
					return
				}
				rid, err := f.svc.RunWithOptions(context.Background(), sid, "governed coding still works", RunOptions{Face: face})
				if err != nil {
					t.Fatal(err)
				}
				waitForRunStatus(t, f.b, rid, domain.RunCompleted)
				f.svc.WaitIdle(context.Background())
			}
			requests := f.capture()
			if len(requests) != 1 {
				t.Fatalf("HTTP count=%d", len(requests))
			}
			wire := requests[0]
			t.Logf("HTTP_CAPTURE %s", wire)
			if kind == "worker" || kind == "ordinary-coding" {
				for _, forbidden := range []string{"Recap data", "resident-only recap secret", "shared-private-original", "MISSION BODY"} {
					if strings.Contains(wire, forbidden) {
						t.Fatalf("implicit context leaked %q", forbidden)
					}
				}
				if preparer.calls != 0 {
					t.Fatal("worker/Coding acquired persona authority")
				}
				var request map[string]any
				json.Unmarshal([]byte(wire), &request)
				if request["max_tokens"] != nil {
					t.Fatal("worker/ordinary Coding inherited LIFE output policy")
				}
			} else if kind == "resident-coding-frozen" {
				if preparer.calls != 1 || strings.Count(wire, "MISSION BODY") != 1 || !strings.Contains(wire, "resident-only recap secret") {
					t.Fatalf("resident coding lost FrozenCore or Recap calls=%d wire=%s", preparer.calls, wire)
				}
				if strings.Contains(wire, "You are Vivy,") {
					t.Fatal("initialized FrozenCore admitted fallback persona")
				}
			} else {
				if preparer.calls != 1 || strings.Contains(wire, "MISSION BODY") || strings.Contains(wire, "You are Vivy,") {
					t.Fatal("first arrival invented an initialized persona")
				}
				if !strings.Contains(wire, "Your personal identity has not been initialized.") {
					t.Fatal("missing unnamed first-arrival guidance")
				}
			}
		})
	}
	// Contribution membership alone never grants the complete resident domain.
	f := newIssue58Wire(t)
	row := f.completed(t, "narrow-source", "narrow")
	if _, err := f.svc.AuthorizeSubjectHistory(context.Background(), row.SessionID); err != storage.ErrSubjectBinding {
		t.Fatal("narrow source acquired resident domain")
	}
}
