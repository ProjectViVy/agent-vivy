package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestImageMIMEByExtension(t *testing.T) {
	cases := map[string]string{
		"a.png": "image/png", "b.JPG": "image/jpeg", "c.jpeg": "image/jpeg",
		"d.gif": "image/gif", "e.webp": "image/webp",
	}
	for path, want := range cases {
		if got := imageMIME(path); got != want {
			t.Fatalf("imageMIME(%q) = %q, want %q", path, got, want)
		}
	}
	for _, path := range []string{"a.txt", "b.go", "png", "c.png.bak"} {
		if got := imageMIME(path); got != "" {
			t.Fatalf("imageMIME(%q) = %q, want empty", path, got)
		}
	}
}

func TestReadFileAttachesImageBytes(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	raw := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 64)...)
	if err := os.WriteFile(filepath.Join(workspace.Path, "shot.png"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := backend.ReadFile(context.Background(), runID, tools.FileReadRequest{Path: "shot.png"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Binary {
		t.Fatal("image must not be reported as binary")
	}
	if result.ImageMIME != "image/png" || string(result.ImageData) != string(raw) {
		t.Fatalf("image = %q, %d bytes", result.ImageMIME, len(result.ImageData))
	}

	// Text reads are unchanged.
	if err := os.WriteFile(filepath.Join(workspace.Path, "a.txt"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := backend.ReadFile(context.Background(), runID, tools.FileReadRequest{Path: "a.txt"})
	if err != nil || text.Binary || text.ImageMIME != "" || text.Content != "hi\n" {
		t.Fatalf("text read = %+v, %v", text, err)
	}
}

func TestReadFileRejectsOversizedImageInsteadOfTruncating(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	big := append([]byte{0x89, 'P', 'N', 'G'}, make([]byte, defaultFilesystemMaxBytes)...)
	if err := os.WriteFile(filepath.Join(workspace.Path, "big.png"), big, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := backend.ReadFile(context.Background(), runID, tools.FileReadRequest{Path: "big.png"})
	if err == nil || !strings.Contains(err.Error(), "read cap") {
		t.Fatalf("oversized image err = %v", err)
	}
}

func TestNormalizeEnhancedResultKeepsImageOverBudget(t *testing.T) {
	envelope := `{"parts":[` +
		`{"type":"text","text":"` + strings.Repeat("x", 40000) + `"},` +
		`{"type":"image","mime_type":"image/png","base64data":"` + strings.Repeat("QUJD", 100) + `"}]}`
	result, err := normalizeEnhancedResult(untrustedToolResultHeader+envelope, 32<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Parts) != 2 {
		t.Fatalf("parts = %d", len(result.Parts))
	}
	if result.Parts[1].Image == nil || result.Parts[1].Image.MIMEType != "image/png" {
		t.Fatalf("image part = %+v", result.Parts[1])
	}
	if !strings.Contains(result.Parts[0].Text, "collapsed") {
		t.Fatalf("over-budget text not compacted: %d bytes", len(result.Parts[0].Text))
	}
}

func TestNormalizeEnhancedResultRejectsUnknownPart(t *testing.T) {
	if _, err := normalizeEnhancedResult(`{"parts":[{"type":"hologram"}]}`, 1024); err == nil {
		t.Fatal("unknown part type must fail")
	}
}

type envelopeStubTool struct {
	result string
}

func (t envelopeStubTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: "envelope_stub", Description: "stub", Readonly: true}
}

func (t envelopeStubTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	return t.result, nil
}

func TestToolAdapterRunPassesEnvelopeUncompacted(t *testing.T) {
	envelope := `{"parts":[{"type":"text","text":"hello"},{"type":"image","mime_type":"image/png","base64data":"` + strings.Repeat("QUJD", 200) + `"}]}`
	adapter := &toolAdapter{t: envelopeStubTool{result: envelope}, maxResultBytes: 64}
	out, err := adapter.run(context.Background(), "{}")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, untrustedToolResultHeader) {
		t.Fatalf("missing untrusted header: %q", out[:40])
	}
	if strings.Contains(out, "collapsed") {
		t.Fatal("envelope must not be byte-compacted")
	}
	trimmed := strings.TrimPrefix(out, untrustedToolResultHeader)
	result, err := normalizeEnhancedResult(trimmed, 64)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(result.Parts) != 2 || result.Parts[1].Image == nil {
		t.Fatalf("parts = %+v", result.Parts)
	}

	// Plain oversized text still compacts.
	adapter = &toolAdapter{t: envelopeStubTool{result: strings.Repeat("y", 500)}, maxResultBytes: 64}
	out, err = adapter.run(context.Background(), "{}")
	if err != nil || !strings.Contains(out, "collapsed") {
		t.Fatalf("plain text = %v, %q", err, out)
	}
}
