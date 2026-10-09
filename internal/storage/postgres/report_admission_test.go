package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func reportPGDigest(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

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

func reportPGBackend(t *testing.T) *Backend {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	b, err := OpenSchema(context.Background(), dsn, fmt.Sprintf("test_r0_%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// TestReportRootAdmission mirrors the SQLite root-admission contract on the
// Postgres dialect: row-lock serialization, namespace dedup, conflict, busy
// target, and validation parity.
func TestReportRootAdmission(t *testing.T) {
	ctx := context.Background()
	b := reportPGBackend(t)
	now := time.Now().UnixMilli()
	control := domain.Session{ID: "reportctl_test", Title: "report control", CreatedAt: now, UpdatedAt: now,
		Purpose: domain.SessionPurposeReportControl}
	if err := b.CreateSession(ctx, control); err != nil {
		t.Fatal(err)
	}
	if sessions, err := b.ListSessions(ctx); err != nil || len(sessions) != 0 {
		t.Fatalf("hidden session must not list: %v %d", err, len(sessions))
	}
	digest := hex.EncodeToString(reportPGDigest("req"))
	result, err := b.CommitWorkflowAdmission(ctx, reportAdmissionFixture("op-1", digest))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Busy || result.Run.Purpose != domain.RunPurposeReport {
		t.Fatalf("unexpected result %+v", result)
	}
	again, err := b.CommitWorkflowAdmission(ctx, reportAdmissionFixture("op-1", digest))
	if err != nil {
		t.Fatal(err)
	}
	if again.Created || again.Run.ID != result.Run.ID {
		t.Fatalf("dedup replay must return stored run: %+v", again)
	}
	conflict := reportAdmissionFixture("op-1", digest)
	conflict.Revision.RequestDigest = hex.EncodeToString(reportPGDigest("changed"))
	if _, err := b.CommitWorkflowAdmission(ctx, conflict); err == nil {
		t.Fatal("same key with changed request must conflict")
	}
	busy, err := b.CommitWorkflowAdmission(ctx, reportAdmissionFixture("op-2", digest))
	if err != nil {
		t.Fatal(err)
	}
	if !busy.Busy || busy.Created || busy.Run.ID != result.Run.ID {
		t.Fatalf("distinct key on active target must report busy: %+v", busy)
	}
}

// TestReportRootAdmissionParallel: concurrent connections racing the same
// operation key converge to one admitted run under the row lock.
func TestReportRootAdmissionParallel(t *testing.T) {
	ctx := context.Background()
	b := reportPGBackend(t)
	now := time.Now().UnixMilli()
	if err := b.CreateSession(ctx, domain.Session{ID: "reportctl_test", Title: "report control",
		CreatedAt: now, UpdatedAt: now, Purpose: domain.SessionPurposeReportControl}); err != nil {
		t.Fatal(err)
	}
	digest := hex.EncodeToString(reportPGDigest("req"))
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
