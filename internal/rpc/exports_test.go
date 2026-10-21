package rpc

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newExportsHandler(t *testing.T) (Handler, string) {
	t.Helper()
	dir := t.TempDir()
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.ExportsDir = dir })
	return env.handler, dir
}

func TestExportsReadRoundtrip(t *testing.T) {
	handler, dir := newExportsHandler(t)
	body := []byte("<html>export body</html>")
	if err := os.WriteFile(filepath.Join(dir, "sess-1.html"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	result, rpcErr := callControl(t, handler, "exports/read", map[string]any{
		"name": "sess-1.html",
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	page, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T", result)
	}
	if _, exists := page["digest"]; exists {
		t.Fatal("retired digest response")
	}
	if page["name"] != "sess-1.html" {
		t.Fatalf("page = %+v", page)
	}
	raw, err := base64.StdEncoding.DecodeString(fmt.Sprintf("%v", page["data_base64"]))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(body) {
		t.Fatalf("payload = %q", raw)
	}
	caps, rpcErr := callControl(t, handler, "capabilities", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !strings.Contains(fmt.Sprintf("%v", caps), "exports.read") {
		t.Fatalf("capabilities = %v", caps)
	}
}

func TestExportsReadGuards(t *testing.T) {
	handler, dir := newExportsHandler(t)
	body := []byte("x")
	if err := os.WriteFile(filepath.Join(dir, "a.html"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"", "../escape", "..", ".", "a/b.html", "a\\b.html", "a b.html"} {
		if _, rpcErr := callControl(t, handler, "exports/read", map[string]any{"name": name}); rpcErr == nil || rpcErr.Code != InvalidParams {
			t.Fatalf("name %q = %+v, want InvalidParams", name, rpcErr)
		}
	}
	if _, rpcErr := callControl(t, handler, "exports/read", map[string]any{"name": "missing.html"}); rpcErr == nil || rpcErr.Code != CodeNotFound {
		t.Fatalf("missing = %+v, want not found", rpcErr)
	}
	if _, rpcErr := callControl(t, handler, "exports/read", map[string]any{
		"name":            "a.html",
		"expected_digest": "retired",
	}); rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("retired digest = %+v, want InvalidParams", rpcErr)
	}
	// Unwired dir hides the method and the capability.
	unwired := newControlTestEnv(t).handler
	if _, rpcErr := callControl(t, unwired, "exports/read", map[string]any{"name": "a.html"}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unwired = %+v, want MethodNotFound", rpcErr)
	}
}
