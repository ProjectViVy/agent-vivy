package sqlite

import (
	"agent-vivy/internal/storage/conformance"
	"context"
	"testing"
)

func TestIssue58InteractionOrderAndDedup(t *testing.T) {
	conformance.RunIssue58InteractionOrderAndDedup(t, openBackend(t))
}
func TestIssue58SubjectIsolation(t *testing.T) {
	conformance.RunIssue58SubjectIsolation(t, openBackend(t))
}

func TestIssue58AdmissionBoundaries(t *testing.T) {
	conformance.RunIssue58AdmissionBoundaries(t, openBackend(t))
}

func TestIssue58ContinuityReplayBinding(t *testing.T) {
	conformance.RunIssue58ContinuityReplayBinding(t, openBackend(t))
}

func TestIssue58FormattedGrantReplay(t *testing.T) {
	conformance.RunIssue58FormattedGrantReplay(t, openBackend(t))
}

func TestIssue58SubjectRetention(t *testing.T) {
	conformance.RunIssue58SubjectRetention(t, openBackend(t))
}

func TestIssue58MissingEvidenceBlocksNewSeal(t *testing.T) {
	conformance.RunIssue58MissingEvidenceBlocksNewSeal(t, openBackend(t))
}

func TestIssue58CopySources(t *testing.T) { conformance.RunIssue58CopySources(t, openBackend(t)) }

func TestIssue58SubjectContextSnapshot(t *testing.T) {
	conformance.RunIssue58SubjectContextSnapshot(t, openBackend(t))
}
func TestIssue58SubjectContextScope(t *testing.T) {
	conformance.RunIssue58SubjectContextScope(t, openBackend(t))
}

func TestIssue58SubjectContextCopyScope(t *testing.T) {
	conformance.RunIssue58SubjectContextCopyScope(t, openBackend(t))
}
func TestIssue58SubjectContextSnapshotRace(t *testing.T) {
	conformance.RunIssue58SubjectContextSnapshotRace(t, openBackend(t))
}

func TestIssue58SubjectContextInvalidMapping(t *testing.T) {
	b := openBackend(t)
	conformance.RunIssue58SubjectContextInvalidMapping(t, b, func(id string) error {
		_, err := b.db.ExecContext(context.Background(), `DELETE FROM message_copy_sources WHERE message_id=?`, id)
		return err
	})
}

func TestIssue58OperationProvenanceImmutable(t *testing.T) {
	conformance.RunIssue58OperationProvenanceImmutable(t, openBackend(t))
}
