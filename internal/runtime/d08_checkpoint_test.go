package runtime

import (
	"agent-vivy/internal/domain"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"
)

func TestCheckpointV2CarriesOnlyOwnedSnapshotIdentity(t *testing.T) {
	store, blobs := newCheckpointFixture(t, "v0.9.21")
	snapshot, err := buildPromptSnapshot(PromptInput{Policy: defaultPolicyForTest(t, domain.PolicyProfileDefault), RunID: "run-d08", GenerationID: "generation-d08", Capture: promptCapture(nil), Face: "web"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := withRunPrompt(context.Background(), snapshot)
	id := checkpointIDFor(snapshot.RunID)
	if err := store.Set(ctx, id, []byte("native opaque payload")); err != nil {
		t.Fatal(err)
	}
	raw, ok, err := blobs.Get(ctx, id)
	if err != nil || !ok {
		t.Fatalf("raw checkpoint: %v / %v", ok, err)
	}
	n := binary.BigEndian.Uint32(raw[:4])
	var header map[string]json.RawMessage
	if err := json.Unmarshal(raw[4:4+n], &header); err != nil {
		t.Fatal(err)
	}
	if string(header["format"]) != "2" || header["checksum_sha256"] != nil {
		t.Fatalf("checkpoint is not the checksum-free v2 envelope: %s", raw[4:4+n])
	}
	var prompt map[string]json.RawMessage
	if err := json.Unmarshal(header["prompt"], &prompt); err != nil {
		t.Fatal(err)
	}
	if string(prompt["run_id"]) != `"run-d08"` || prompt["payload_sha256"] != nil {
		t.Fatalf("snapshot binding: %s", header["prompt"])
	}
	delete(header, "format")
	legacy, _ := json.Marshal(header)
	legacyBlob := binary.BigEndian.AppendUint32(nil, uint32(len(legacy)))
	legacyBlob = append(append(legacyBlob, legacy...), raw[4+n:]...)
	if err := blobs.Put(ctx, id, legacyBlob); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(ctx, id); !errors.Is(err, ErrCheckpointCorrupted) {
		t.Fatalf("legacy envelope accepted: %v", err)
	}
}

func TestCheckpointRequiresAdmittedSnapshotBeforeWrite(t *testing.T) {
	store, blobs := newCheckpointFixture(t, "v0.9.21")
	const id = "ckpt-run-d08-unbound"
	if err := store.Set(context.Background(), id, []byte("payload")); !errors.Is(err, ErrCheckpointPromptMismatch) {
		t.Fatalf("unbound write: %v", err)
	}
	if _, ok, err := blobs.Get(context.Background(), id); ok || err != nil {
		t.Fatalf("unbound write left durable bytes: %v / %v", ok, err)
	}
}

func TestCheckpointRejectsForeignOrChangedSnapshotIdentity(t *testing.T) {
	store, _ := newCheckpointFixture(t, "v0.9.21")
	admitted, err := buildPromptSnapshot(PromptInput{
		Policy: defaultPolicyForTest(t, domain.PolicyProfileDefault),
		RunID:  "run-d08-matrix", GenerationID: "generation-d08", Capture: promptCapture(nil), Face: domain.FaceWeb,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := checkpointIDFor(admitted.RunID)
	if err := store.Set(withRunPrompt(context.Background(), admitted), id, []byte("opaque")); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"run", "schema", "composer", "generation", "payload"} {
		t.Run(field, func(t *testing.T) {
			changed := admitted
			switch field {
			case "run":
				changed.RunID = "foreign"
			case "schema":
				changed.SchemaVersion++
			case "composer":
				changed.ComposerVersion = "unsupported"
			case "generation":
				changed.GenerationID = "foreign"
			case "payload":
				changed.Payload = []byte(`{"Instruction":"invalid incomplete snapshot"}`)
			}
			if _, _, err := store.Get(withRunPrompt(context.Background(), changed), id); !errors.Is(err, ErrCheckpointPromptMismatch) {
				t.Fatalf("changed %s accepted: %v", field, err)
			}
		})
	}
}
