package sdk

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyHelloFS(t *testing.T) {
	dir := filepath.Join("..", "..", "plugins", "hello-fs")
	rep, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK {
		t.Fatalf("hello-fs issues: %v", rep.Issues)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".dll") || strings.HasSuffix(name, ".wasm") {
			t.Fatalf("verify produced a binary: %s", entry.Name())
		}
	}
}

func TestVerifyRejectsForbiddenPlugins(t *testing.T) {
	cases := []struct {
		dir  string
		want string
	}{
		{"bad-internal-import", "agent-vivy/internal/"},
		{"bad-eino-import", "github.com/cloudwego/eino"},
		{"bad-seam-journal", "seam"},
		{"name-mismatch", "does not match directory"},
		{"bad-main", "package main"},
		{"bad-os-open", "os.Open"},
		{"bad-channel-tools", "seam channel forbids tools"},
		{"bad-channel-listen", "opens a listen socket"},
		{"bad-channel-grant", "is not allowed in this batch"},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			rep, err := Verify(filepath.Join("testdata", tc.dir))
			if err != nil {
				t.Fatal(err)
			}
			if rep.OK {
				t.Fatal("expected verify to fail")
			}
			joined := strings.Join(rep.Issues, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("issues = %s, want substring %q", joined, tc.want)
			}
		})
	}
}

// TestVerifyFakeChannel asserts the seam-channel path accepts a
// well-formed channel plugin that lives in its own go.mod.
func TestVerifyFakeChannel(t *testing.T) {
	dir := filepath.Join("testdata", "fake-channel")
	rep, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK {
		t.Fatalf("fake-channel issues: %v", rep.Issues)
	}
	if len(rep.Issues) != 0 {
		t.Fatalf("fake-channel issues = %v, want zero", rep.Issues)
	}
}

func TestSDKCLIVerifyAndUnimplemented(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"verify", filepath.Join("..", "..", "plugins", "hello-fs")}, &out, &errBuf); code != 0 {
		t.Fatalf("verify hello-fs exit %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "ok ") {
		t.Fatalf("stdout = %q", out.String())
	}
	errBuf.Reset()
	if code := run([]string{"pack"}, &out, &errBuf); code == 0 || !strings.Contains(errBuf.String(), "--with") {
		t.Fatalf("pack without --with = %d %q", code, errBuf.String())
	}
}
