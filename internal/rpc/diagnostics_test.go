package rpc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/logging"
)

func newDiagHandler(t *testing.T) (Handler, *logging.Diagnostics, string) {
	t.Helper()
	dir := t.TempDir()
	svc, err := logging.NewDiagnostics(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.Diagnostics = svc })
	return env.handler, svc, dir
}

func TestDiagnosticsRPCMissingCapability(t *testing.T) {
	handler := newControlTestEnv(t).handler
	_, rpcErr := callControl(t, handler, "diagnostics/logs", map[string]string{"source": "runtime"})
	if rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("logs unwired = %+v", rpcErr)
	}
	_, rpcErr = callControl(t, handler, "diagnostics/gui/append", map[string]any{"records": []any{}})
	if rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("append unwired = %+v", rpcErr)
	}
	// Capabilities must not advertise diagnostics when unwired.
	result, rpcErr := callControl(t, handler, "capabilities", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	caps := fmt.Sprintf("%v", result)
	if strings.Contains(caps, "diagnostics.logs") {
		t.Fatalf("capabilities = %v", caps)
	}
}

func TestDiagnosticsRPCRoundtrip(t *testing.T) {
	handler, _, dir := newDiagHandler(t)
	date := time.Now().Format("2006-01-02")
	writeDiagLog(t, dir, logging.FilePrefix, date, `{"level":"INFO","msg":"hello diag"}`)

	result, rpcErr := callControl(t, handler, "diagnostics/logs",
		map[string]string{"source": "runtime", "date": date})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	page := result.(logging.DiagnosticPage)
	if len(page.Records) != 1 || page.Records[0].Message != "hello diag" {
		t.Fatalf("page = %+v", page)
	}

	ack, rpcErr := callControl(t, handler, "diagnostics/gui/append",
		map[string]any{"records": []map[string]string{{"level": "INFO", "message": "ui ping"}}})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if ack.(logging.GuiLogAck).Accepted != 1 {
		t.Fatalf("ack = %+v", ack)
	}

	// Capability advertisement.
	caps, rpcErr := callControl(t, handler, "capabilities", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	cs := fmt.Sprintf("%v", caps)
	if !strings.Contains(cs, "diagnostics.logs") || !strings.Contains(cs, "diagnostics.gui_append") {
		t.Fatalf("capabilities = %v", cs)
	}
}

func TestDiagnosticsRPCInvalidParams(t *testing.T) {
	handler, _, _ := newDiagHandler(t)
	_, rpcErr := callControl(t, handler, "diagnostics/logs", map[string]string{"source": "/etc/passwd"})
	if rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("bad source = %+v", rpcErr)
	}
	_, rpcErr = callControl(t, handler, "diagnostics/logs",
		map[string]string{"source": "runtime", "date": "not-a-date"})
	if rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("bad date = %+v", rpcErr)
	}
	_, rpcErr = callControl(t, handler, "diagnostics/gui/append",
		map[string]any{"records": make([]map[string]string, logging.DiagMaxRecords+1)})
	if rpcErr == nil || rpcErr.Code != InvalidParams {
		t.Fatalf("oversize batch = %+v", rpcErr)
	}
}

func TestDiagnosticsBundleRPC(t *testing.T) {
	dir := t.TempDir()
	svc, err := logging.NewDiagnostics(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	bundleDir := filepath.Join(t.TempDir(), "exports")
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Diagnostics = svc
		deps.DiagnosticsBundleDir = bundleDir
	})

	date := time.Now().Format("2006-01-02")
	writeDiagLog(t, dir, logging.FilePrefix, date, `{"level":"INFO","msg":"bundle line"}`)

	result, rpcErr := callControl(t, env.handler, "diagnostics/bundle", map[string]string{"session_id": "sess-x"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	payload := result.(map[string]any)
	path := payload["path"].(string)
	if filepath.Dir(path) != bundleDir || !strings.HasPrefix(filepath.Base(path), "bug-sess-x-") {
		t.Fatalf("path = %q", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"# Vivy bug report", "session_id: sess-x", "bundle line"} {
		if !strings.Contains(text, want) {
			t.Fatalf("bundle missing %q:\n%s", want, text)
		}
	}

	// Capabilities advertise the verb only when the dir is wired.
	caps, rpcErr := callControl(t, env.handler, "capabilities", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !strings.Contains(fmt.Sprintf("%v", caps), "diagnostics.bundle") {
		t.Fatalf("capabilities = %v", caps)
	}

	// Unwired dir and missing diagnostics both report MethodNotFound.
	noDir := newControlTestEnv(t, func(deps *ControlDeps) { deps.Diagnostics = svc }).handler
	if _, rpcErr = callControl(t, noDir, "diagnostics/bundle", map[string]string{}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unwired dir = %+v", rpcErr)
	}
	bare := newControlTestEnv(t).handler
	if _, rpcErr = callControl(t, bare, "diagnostics/bundle", map[string]string{}); rpcErr == nil || rpcErr.Code != MethodNotFound {
		t.Fatalf("unwired diagnostics = %+v", rpcErr)
	}
}

func writeDiagLog(t *testing.T, dir, prefix, date, line string) {
	t.Helper()
	path := filepath.Join(dir, fmt.Sprintf("%s.%s", prefix, date))
	if err := writeFileString(path, line+"\n"); err != nil {
		t.Fatal(err)
	}
}

func writeFileString(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
