package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func reportAdmissionFixture(opKey, requestDigest string) storage.WorkflowAdmission {
	descJSON := []byte(`{"schema":"report-test"}`)
	authJSON := []byte(`{"policy":"report-test"}`)
	desc := sha256.Sum256(descJSON)
	auth := sha256.Sum256(authJSON)
	id := domain.RunID("wfr-" + opKey)
	control := domain.SessionID("reportctl_test")
	return storage.WorkflowAdmission{
		Revision: domain.WorkflowRevision{
			RunID: id, ParentSessionID: control, RootRunID: id, OperationKey: opKey,
			RootPurpose: "report/v1", AdmissionNamespace: string(control),
			RequestDigest: requestDigest, TargetKey: "daily/completed/",
			DescriptorDigest: hex.EncodeToString(desc[:]), AuthorityDigest: hex.EncodeToString(auth[:]),
			DescriptorJSON: descJSON, AuthorityJSON: authJSON,
			SchemaVersion: 2, CreatedAt: 1,
			ProgramDigest:   "inofy-normal-v1:sha256:" + hex.EncodeToString(desc[:]),
			CatalogDigest:   "inofy-normal-v1:sha256:" + hex.EncodeToString(auth[:]),
			CompilerVersion: "test", InputDigest: hex.EncodeToString(desc[:]),
			InputJSON: []byte(`{}`), EffectiveLimits: []byte(`{"max_steps":4}`),
			HostBindingID: "hostbind",
		},
		Run: domain.Run{ID: id, SessionID: control, Status: domain.RunAccepted, CreatedAt: 1,
			Kind: domain.RunKindWorkflow, RootID: id, Purpose: domain.RunPurposeReport},
		Started: domain.RunEvent{RunID: id, Type: domain.EventRunStarted, CreatedAt: 1, PayloadVersion: 1,
			Payload: []byte(`{"mode":"report"}`)},
	}
}

func reportControlSession(t *testing.T, b *Backend, ctx context.Context) domain.Session {
	t.Helper()
	now := time.Now().UnixMilli()
	session := domain.Session{ID: "reportctl_test", Title: "report control", CreatedAt: now, UpdatedAt: now,
		Purpose: domain.SessionPurposeReportControl}
	if err := b.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	return session
}

// TestReportRootAdmission covers the trusted-root branch: commit, namespace
// dedup replay, same-key/different-request conflict, busy target decline,
// and validation rejections.
func TestReportRootAdmission(t *testing.T) {
	ctx := context.Background()
	b, err := Open(ctx, t.TempDir()+"/r0.db")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	session := reportControlSession(t, b, ctx)
	if !session.Hidden() {
		t.Fatal("control session must be hidden")
	}
	if sessions, err := b.ListSessions(ctx); err != nil || len(sessions) != 0 {
		t.Fatalf("hidden session must not list: %v %d", err, len(sessions))
	}
	if sessions, err := b.ListSessionsForRecovery(ctx); err != nil || len(sessions) != 1 {
		t.Fatalf("recovery list must include hidden session: %v %d", err, len(sessions))
	}
	digest := hex.EncodeToString(reportDigest("req"))
	admission := reportAdmissionFixture("op-1", digest)
	result, err := b.CommitWorkflowAdmission(ctx, admission)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Busy || result.Run.Purpose != domain.RunPurposeReport {
		t.Fatalf("unexpected result %+v", result)
	}
	// Same key + same request replays the committed run.
	again, err := b.CommitWorkflowAdmission(ctx, reportAdmissionFixture("op-1", digest))
	if err != nil {
		t.Fatal(err)
	}
	if again.Created || again.Run.ID != result.Run.ID {
		t.Fatalf("dedup replay must return stored run: %+v", again)
	}
	// Same key + different request digest is an idempotency conflict.
	conflict := reportAdmissionFixture("op-1", digest)
	conflict.Revision.RequestDigest = hex.EncodeToString(reportDigest("changed"))
	if _, err := b.CommitWorkflowAdmission(ctx, conflict); err == nil {
		t.Fatal("same key with changed request must conflict")
	}
	// Distinct key against the same active target declines as busy.
	busy, err := b.CommitWorkflowAdmission(ctx, reportAdmissionFixture("op-2", digest))
	if err != nil {
		t.Fatal(err)
	}
	if !busy.Busy || busy.Created || busy.Run.ID != result.Run.ID {
		t.Fatalf("distinct key on active target must report busy: %+v", busy)
	}
	// Different target key commits a second root.
	other := reportAdmissionFixture("op-3", digest)
	other.Revision.TargetKey = "weekly/current/sec/ent"
	res3, err := b.CommitWorkflowAdmission(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if !res3.Created || res3.Busy {
		t.Fatalf("different target must admit: %+v", res3)
	}
	// Validation rejections.
	for _, mutate := range []func(*storage.WorkflowAdmission){
		func(a *storage.WorkflowAdmission) { a.Revision.ParentRunID = "x" },
		func(a *storage.WorkflowAdmission) { a.Revision.AdmissionNamespace = "" },
		func(a *storage.WorkflowAdmission) { a.Revision.RequestDigest = "ZZ" },
		func(a *storage.WorkflowAdmission) { a.Revision.TargetKey = "" },
		func(a *storage.WorkflowAdmission) { a.Run.Purpose = "" },
		func(a *storage.WorkflowAdmission) { a.Run.RootID = "other" },
	} {
		bad := reportAdmissionFixture("op-bad", digest)
		mutate(&bad)
		if _, err := b.CommitWorkflowAdmission(ctx, bad); err == nil {
			t.Fatalf("invalid trusted-root admission must fail validation")
		}
	}
	// Child revision still admits under the parent-run namespace fallback.
	// The child authorizer is a primary run in the same (control) session.
	primary := domain.Run{ID: "prim-1", SessionID: session.ID, Status: domain.RunActive,
		Kind: domain.RunKindPrimary, RootID: "prim-1", CreatedAt: 1}
	if err := b.CreateRun(ctx, primary); err != nil {
		t.Fatal(err)
	}
	child := reportAdmissionFixture("child-1", digest)
	child.Revision.RootPurpose = ""
	child.Revision.AdmissionNamespace = ""
	child.Revision.RequestDigest = ""
	child.Revision.TargetKey = ""
	child.Revision.ParentRunID = primary.ID
	child.Revision.RootRunID = primary.ID
	child.Revision.RunID = "wfr-child-1"
	child.Run.ID = "wfr-child-1"
	child.Run.ParentID = primary.ID
	child.Run.RootID = primary.ID
	child.Run.Depth = 1
	child.Run.Purpose = ""
	child.Started.RunID = "wfr-child-1"
	if _, err := b.CommitWorkflowAdmission(ctx, child); err != nil {
		t.Fatalf("child admission under new schema: %v", err)
	}
}

// TestReportRootAdmissionParallel: N connections racing the same operation
// key converge to one admitted run.
func TestReportRootAdmissionParallel(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/r0p.db"
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	reportControlSession(t, b, ctx)
	digest := hex.EncodeToString(reportDigest("req"))
	const n = 8
	var wg sync.WaitGroup
	created := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := b.CommitWorkflowAdmission(ctx, reportAdmissionFixture("op-par", digest))
			if err != nil {
				t.Error(err)
				return
			}
			created <- res.Created
		}()
	}
	wg.Wait()
	close(created)
	winners := 0
	for c := range created {
		if c {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one admission must win, got %d", winners)
	}
}

func reportDigest(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}
