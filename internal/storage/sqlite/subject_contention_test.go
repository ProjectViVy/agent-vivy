package sqlite

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestIssue58SnapshotContentionCanceled(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "contended.db")
	a, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO snapshots(key,value,version) VALUES('held',X'00',1)`); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = b.Snapshot().Put(canceled, "blocked", []byte("never"), 0)
	elapsed := time.Since(start)
	t.Logf("contended cancellation: %v after %s", err, elapsed)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause lost: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("driver ignored prompt cancellation: %s", elapsed)
	}
	tx.Rollback()
	raw, version, err := b.Snapshot().Get(ctx, "blocked")
	if err != nil || version != 0 || len(raw) != 0 {
		t.Fatalf("canceled CAS wrote data: %q %d %v", raw, version, err)
	}
	// Driver initialization applies the timeout to replacement connections too.
	b.db.SetConnMaxLifetime(time.Nanosecond)
	var timeout int
	if err := b.db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil || timeout != 25 {
		t.Fatalf("replacement connection timeout %d %v", timeout, err)
	}
}

func TestIssue58ContentionDoesNotRetryDomainOrUnknownErrors(t *testing.T) {
	for _, cause := range []error{storage.ErrSubjectBinding, storage.ErrVersionConflict, storage.ErrCommitUncertain, errors.New("unrecognized failure"), context.Canceled} {
		attempts := 0
		err := retrySQLiteBusy(context.Background(), func() error { attempts++; return cause })
		if !errors.Is(err, cause) || attempts != 1 {
			t.Fatalf("retried permanent/unknown error %v: %d %v", cause, attempts, err)
		}
	}
}
func TestIssue58SubjectSealAcrossHandles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "seals.db")
	a, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	conformance.Issue58Identity(t, a)
	source := conformance.Issue58Source("contended-seals", false)
	if err := a.CreateSession(ctx, domain.Session{ID: source.SessionID}); err != nil {
		t.Fatal(err)
	}
	inputs := make([]storage.SubjectInteraction, 8)
	for i := range inputs {
		inputs[i] = conformance.Issue58Completed(t, a, source, fmt.Sprintf("concurrent-seal-%d", i))
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, in := range inputs {
		backend := a
		if i%2 == 1 {
			backend = b
		}
		wg.Go(func() {
			<-start
			if _, err := backend.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, in); err != nil {
				t.Error(err)
			}
		})
	}
	close(start)
	wg.Wait()
	high, err := a.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding)
	if err != nil || high != 8 {
		t.Fatalf("contended sequence lost: %d %v", high, err)
	}
	rows, err := b.ListSubjectInteractions(ctx, source.SubjectHistoryBinding, 0, high, 8)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if row.Sequence != uint64(i+1) {
			t.Fatalf("contended sequence hole: %v", rows)
		}
	}
}
