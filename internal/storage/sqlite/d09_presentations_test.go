package sqlite

import (
	"agent-vivy/internal/storage/conformance"
	"testing"
)

func TestD09DeliverablePresentations(t *testing.T) {
	conformance.AssertDeliverablePresentations(t, continuitySlot(t))
}
func TestD09PresentationRollbackAndValidation(t *testing.T) {
	conformance.AssertPresentationRollbackAndValidation(t, continuitySlot(t))
}
func TestD09PresentationVisibility(t *testing.T) {
	conformance.AssertPresentationVisibility(t, continuitySlot(t))
}
func TestD09PresentationHistoryTransactions(t *testing.T) {
	conformance.AssertPresentationHistoryTransactions(t, continuitySlot(t))
}
