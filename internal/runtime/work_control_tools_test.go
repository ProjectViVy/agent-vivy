package runtime

import "testing"

func TestModelWorkIdentityIsDeterministicAndRunScoped(t *testing.T) {
	id1, hash1, err := modelWorkIdentity("run-1", "submit-plan", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() error = %v", err)
	}
	id2, hash2, err := modelWorkIdentity("run-1", "submit-plan", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() repeat error = %v", err)
	}
	if id1 != id2 || hash1 != hash2 {
		t.Fatalf("identity changed across retries: (%q, %q) vs (%q, %q)", id1, hash1, id2, hash2)
	}
	id3, hash3, err := modelWorkIdentity("run-2", "submit-plan", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() second run error = %v", err)
	}
	if id1 == id3 || hash1 == hash3 {
		t.Fatal("different run identities must not share an idempotency key")
	}
}
