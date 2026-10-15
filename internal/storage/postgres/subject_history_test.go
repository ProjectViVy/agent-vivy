package postgres

import (
	"agent-vivy/internal/storage/conformance"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func issue58Backend(t *testing.T) *Backend {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set: real PostgreSQL subject history gate pending")
	}
	b, err := OpenSchema(context.Background(), dsn, fmt.Sprintf("subject_%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}
func TestIssue58InteractionOrderAndDedup(t *testing.T) {
	conformance.RunIssue58InteractionOrderAndDedup(t, issue58Backend(t))
}
func TestIssue58SubjectIsolation(t *testing.T) {
	conformance.RunIssue58SubjectIsolation(t, issue58Backend(t))
}

func TestIssue58AdmissionBoundaries(t *testing.T) {
	conformance.RunIssue58AdmissionBoundaries(t, issue58Backend(t))
}

func TestIssue58ContinuityReplayBinding(t *testing.T) {
	conformance.RunIssue58ContinuityReplayBinding(t, issue58Backend(t))
}

func TestIssue58FormattedGrantReplay(t *testing.T) {
	conformance.RunIssue58FormattedGrantReplay(t, issue58Backend(t))
}

func TestIssue58SubjectRetention(t *testing.T) {
	conformance.RunIssue58SubjectRetention(t, issue58Backend(t))
}

func TestIssue58MissingEvidenceBlocksNewSeal(t *testing.T) {
	conformance.RunIssue58MissingEvidenceBlocksNewSeal(t, issue58Backend(t))
}

func TestIssue58CopySources(t *testing.T) { conformance.RunIssue58CopySources(t, issue58Backend(t)) }

func TestIssue58SubjectContextSnapshot(t *testing.T) {
	conformance.RunIssue58SubjectContextSnapshot(t, issue58Backend(t))
}
func TestIssue58SubjectContextScope(t *testing.T) {
	conformance.RunIssue58SubjectContextScope(t, issue58Backend(t))
}

func TestIssue58SubjectContextCopyScope(t *testing.T) {
	conformance.RunIssue58SubjectContextCopyScope(t, issue58Backend(t))
}
func TestIssue58SubjectContextSnapshotRace(t *testing.T) {
	conformance.RunIssue58SubjectContextSnapshotRace(t, issue58Backend(t))
}

func TestIssue58SubjectContextInvalidMapping(t *testing.T) {
	b := issue58Backend(t)
	conformance.RunIssue58SubjectContextInvalidMapping(t, b, func(id string) error {
		_, err := b.db.SQL.ExecContext(context.Background(), `DELETE FROM message_copy_sources WHERE message_id=$1`, id)
		return err
	})
}

func TestIssue58OperationProvenanceImmutable(t *testing.T) {
	conformance.RunIssue58OperationProvenanceImmutable(t, issue58Backend(t))
}
