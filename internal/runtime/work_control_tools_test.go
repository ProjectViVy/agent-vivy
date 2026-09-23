package runtime

import "testing"

func TestModelWorkIdentityIsDeterministicAndRunScoped(t *testing.T) {
	id1, hash1, err := modelWorkIdentity("run-1", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() error = %v", err)
	}
	id2, hash2, err := modelWorkIdentity("run-1", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() repeat error = %v", err)
	}
	if id1 != id2 || hash1 != hash2 {
		t.Fatalf("identity changed across retries: (%q, %q) vs (%q, %q)", id1, hash1, id2, hash2)
	}
	id3, hash3, err := modelWorkIdentity("run-2", "submit-plan", "call-1", map[string]string{"markdown": "# plan"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() second run error = %v", err)
	}
	if id1 == id3 || hash1 == hash3 {
		t.Fatal("different run identities must not share an idempotency key")
	}
}

func TestModelWorkIdentityDistinguishesCallsAndRejectsChangedArgs(t *testing.T) {
	payload := map[string]string{"objective": "ship the change"}
	requestID, requestHash, err := modelWorkIdentity("run-1", "create-goal", "call-1", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() error = %v", err)
	}
	retryID, retryHash, err := modelWorkIdentity("run-1", "create-goal", "call-1", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() retry error = %v", err)
	}
	if requestID != retryID || requestHash != retryHash {
		t.Fatalf("exact same-call retry changed identity: (%q, %q) vs (%q, %q)", requestID, requestHash, retryID, retryHash)
	}

	distinctID, distinctHash, err := modelWorkIdentity("run-1", "create-goal", "call-2", payload)
	if err != nil {
		t.Fatalf("modelWorkIdentity() distinct call error = %v", err)
	}
	if distinctID == requestID || distinctHash == requestHash {
		t.Fatal("distinct Eino tool-call IDs must identify distinct model work requests")
	}

	changedID, changedHash, err := modelWorkIdentity("run-1", "create-goal", "call-1", map[string]string{"objective": "different change"})
	if err != nil {
		t.Fatalf("modelWorkIdentity() changed arguments error = %v", err)
	}
	if changedID != requestID || changedHash == requestHash {
		t.Fatalf("same call ID with changed args must retain request ID but change payload hash: (%q, %q) vs (%q, %q)", requestID, requestHash, changedID, changedHash)
	}
}
