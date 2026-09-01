package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type backfillOps struct {
	recordingFileOps
	lines []string
	got   []string
}

func (o *backfillOps) WriteDiagnostics(_ context.Context, paths []string) []string {
	o.got = append(o.got, paths...)
	return o.lines
}

type unchangedBackfillOps struct {
	staticReadOps
	lines []string
}

func (o *unchangedBackfillOps) WriteDiagnostics(context.Context, []string) []string {
	return o.lines
}

func TestWriteToolAttachesDiagnostics(t *testing.T) {
	ops := &backfillOps{lines: []string{"main.go:1:1: error: boom", "main.go:2:5: warning: hmm"}}
	ctx := WithRunID(context.Background(), domain.RunID("run_diag"))
	out, err := NewWriteFile(ops).InvokableRun(ctx, json.RawMessage(`{"path":"main.go","content":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result FileMutationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Diagnostics != "main.go:1:1: error: boom\nmain.go:2:5: warning: hmm" {
		t.Fatalf("diagnostics = %q", result.Diagnostics)
	}
	if len(ops.got) != 1 || ops.got[0] != "main.go" {
		t.Fatalf("paths = %v", ops.got)
	}
}

func TestPatchToolAttachesDiagnostics(t *testing.T) {
	ops := &backfillOps{lines: []string{"a.go:3:1: error: x"}}
	ctx := WithRunID(context.Background(), domain.RunID("run_diag"))
	out, err := NewPatch(ops).InvokableRun(ctx, json.RawMessage(`{"path":"a.go","old_string":"p","new_string":"q"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result FileMutationResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Diagnostics != "a.go:3:1: error: x" {
		t.Fatalf("diagnostics = %q", result.Diagnostics)
	}
}

func TestMutationToolsSkipDiagnosticsWhenUnchangedOrUnwired(t *testing.T) {
	ctx := WithRunID(context.Background(), domain.RunID("run_diag"))

	// Changed=false: a no-op mutation must not report diagnostics.
	unchanged := &unchangedBackfillOps{lines: []string{"should not appear"}}
	out, err := NewWriteFile(unchanged).InvokableRun(ctx, json.RawMessage(`{"path":"a.go","content":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(unchanged.lines) != 1 {
		t.Fatalf("source must not be called on unchanged file")
	}

	// No WriteDiagnosticsSource on ops: result carries no field.
	plain := &recordingFileOps{}
	out, err = NewWriteFile(plain).InvokableRun(ctx, json.RawMessage(`{"path":"a.go","content":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, present := result["diagnostics"]; present {
		t.Fatalf("unwired ops must not add diagnostics: %s", out)
	}

	// Source wired but silent: field stays absent.
	quiet := &backfillOps{}
	out, err = NewWriteFile(quiet).InvokableRun(ctx, json.RawMessage(`{"path":"a.go","content":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	result = map[string]any{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, present := result["diagnostics"]; present {
		t.Fatalf("silent source must not add diagnostics: %s", out)
	}
}
