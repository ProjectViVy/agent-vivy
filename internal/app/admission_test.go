package app

import "testing"

func TestPrimaryAdmissionGateFailsClosedForSealedComposition(t *testing.T) {
	if _, err := primaryAdmissionForComposition(struct{}{}, "", true); err == nil {
		t.Fatal("sealed composition without a generation identity was allowed to use legacy admission")
	}

	if _, err := primaryAdmissionForComposition(struct{}{}, "generation-1", true); err == nil {
		t.Fatal("sealed composition without RunAdmissionStore was allowed to use legacy admission")
	}
}

func TestPrimaryAdmissionGateKeepsUnsealedEmbedderCompatibility(t *testing.T) {
	if got, err := primaryAdmissionForComposition(struct{}{}, "", false); err != nil {
		t.Fatalf("unsealed legacy embedder was rejected: %v", err)
	} else if got != nil {
		t.Fatal("legacy embedder unexpectedly received an admission store")
	}
}
