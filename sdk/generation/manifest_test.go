package generation

import (
	"bytes"
	"testing"
)

func TestExtractEmbeddedManifestDoesNotExecuteArtifact(t *testing.T) {
	raw := []byte(`{"generationId":"test"}`)
	binary := append([]byte("not-an-executable\x00"), []byte(FrameEmbeddedManifest(raw))...)
	got, err := ExtractEmbeddedManifest(binary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("manifest = %q, want %q", got, raw)
	}
}

func TestExtractEmbeddedManifestRejectsAmbiguousBinding(t *testing.T) {
	first := []byte(FrameEmbeddedManifest([]byte(`{"generationId":"first"}`)))
	second := []byte(FrameEmbeddedManifest([]byte(`{"generationId":"second"}`)))
	binary := append(append([]byte{}, first...), second...)
	if _, err := ExtractEmbeddedManifest(binary); err == nil {
		t.Fatal("ambiguous executable binding was accepted")
	}
}
