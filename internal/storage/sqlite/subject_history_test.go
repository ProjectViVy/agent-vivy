package sqlite

import (
	"agent-vivy/internal/storage/conformance"
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
