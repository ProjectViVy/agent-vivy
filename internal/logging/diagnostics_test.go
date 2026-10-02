package logging

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newDiagnostics(t *testing.T) (*Diagnostics, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := NewDiagnostics(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d, dir
}

func writeLogFile(t *testing.T, dir, prefix, date string, lines []string) {
	t.Helper()
	path := filepath.Join(dir, fmt.Sprintf("%s.%s", prefix, date))
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

const diagTestDate = "2026-10-02"

func TestDiagnosticsReadJSONLines(t *testing.T) {
	d, dir := newDiagnostics(t)
	writeLogFile(t, dir, FilePrefix, diagTestDate, []string{
		`{"time":"2026-10-02T10:00:00Z","level":"INFO","msg":"run started","run_id":"r1"}`,
		`{"time":"2026-10-02T10:00:01Z","level":"ERROR","msg":"boom","component":"engine","api_key":"sk-abcdef1234567890"}`,
	})
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 2 || page.Gap || page.HasMore {
		t.Fatalf("page = %+v", page)
	}
	first, second := page.Records[0], page.Records[1]
	if first.Level != "info" || first.Message != "run started" || first.Fields["run_id"] != "r1" {
		t.Fatalf("first = %+v", first)
	}
	if first.At == nil {
		t.Fatal("first.at missing")
	}
	if second.Level != "error" || second.Component != "engine" {
		t.Fatalf("second = %+v", second)
	}
	// Sensitive-keyed field collapses to a marker.
	if second.Fields["api_key"] != "[REDACTED]" {
		t.Fatalf("api_key leaked: %+v", second.Fields)
	}
	if first.ID != diagTestDate+":0" {
		t.Fatalf("id = %q", first.ID)
	}
}

func TestDiagnosticsReadTextAndRedaction(t *testing.T) {
	d, dir := newDiagnostics(t)
	writeLogFile(t, dir, FilePrefix, diagTestDate, []string{
		`time=2026-10-02T10:00:00Z level=INFO msg="token sk-abcdef1234567890 visible"`,
		`{"time":"2026-10-02T10:00:02Z","level":"INFO","msg":"fine"}`,
		`{broken json here`,
	})
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 3 {
		t.Fatalf("records = %+v", page.Records)
	}
	if strings.Contains(page.Records[0].Message, "sk-abcdef") {
		t.Fatalf("secret leaked: %q", page.Records[0].Message)
	}
	if page.Records[2].Message != "[malformed log line omitted]" {
		t.Fatalf("malformed placeholder = %q", page.Records[2].Message)
	}
}

func TestDiagnosticsReadValidation(t *testing.T) {
	d, _ := newDiagnostics(t)
	for _, q := range []DiagnosticQuery{
		{Source: ""},
		{Source: "../../etc/passwd"},
		{Source: "runtime", Date: "2026/10/02"},
		{Source: "runtime", Date: "2026-10-02; rm -rf /"},
		{Source: "runtime", Date: "2026-10-02", Level: "nope"},
		{Source: "runtime", Date: "2026-10-02", After: "not-a-cursor"},
	} {
		if _, err := d.Read(context.Background(), q); !errors.Is(err, ErrDiagnosticQuery) {
			t.Fatalf("query %+v err = %v, want ErrDiagnosticQuery", q, err)
		}
	}
}

func TestDiagnosticsReadMissingDateEmptyPage(t *testing.T) {
	d, _ := newDiagnostics(t)
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: "1999-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 0 || page.HasMore || page.Gap {
		t.Fatalf("page = %+v", page)
	}
}

func TestDiagnosticsReadRejectsSymlink(t *testing.T) {
	d, dir := newDiagnostics(t)
	outside := filepath.Join(t.TempDir(), "outside.log")
	if err := os.WriteFile(outside, []byte("secret line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate}); !errors.Is(err, ErrDiagnosticQuery) {
		t.Fatalf("err = %v, want ErrDiagnosticQuery", err)
	}
}

func TestDiagnosticsCursorResumeAndGap(t *testing.T) {
	d, dir := newDiagnostics(t)
	path := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	lines := []string{
		`{"level":"INFO","msg":"one"}`,
		`{"level":"INFO","msg":"two"}`,
		`{"level":"INFO","msg":"three"}`,
	}
	writeLogFile(t, dir, FilePrefix, diagTestDate, lines)

	// Page 1: limit 2 → has_more with cursor.
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 2 || !page.HasMore || page.NextCursor == "" {
		t.Fatalf("page1 = %+v", page)
	}
	page2, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Records) != 1 || page2.Records[0].Message != "three" || page2.Gap || page2.HasMore {
		t.Fatalf("page2 = %+v", page2)
	}

	// Truncate/rotate the file: stale cursor yields gap and a fresh read.
	if err := os.WriteFile(path, []byte(`{"level":"INFO","msg":"new file"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure mtime differs enough for identity mismatch detection.
	os.Chtimes(path, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	page3, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if !page3.Gap || len(page3.Records) != 1 || page3.Records[0].Message != "new file" {
		t.Fatalf("page3 = %+v", page3)
	}
}

func TestDiagnosticsReadFilters(t *testing.T) {
	d, dir := newDiagnostics(t)
	writeLogFile(t, dir, FilePrefix, diagTestDate, []string{
		`{"level":"INFO","msg":"alpha"}`,
		`{"level":"ERROR","msg":"beta failed"}`,
		`{"level":"INFO","msg":"gamma alpha"}`,
	})
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 || page.Records[0].Message != "beta failed" {
		t.Fatalf("level filter = %+v", page.Records)
	}
	page, err = d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, Query: "ALPHA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 2 {
		t.Fatalf("query filter = %+v", page.Records)
	}
}

func TestDiagnosticsRecordClipBound(t *testing.T) {
	d, dir := newDiagnostics(t)
	big := strings.Repeat("x", DiagMaxRecordBytes*2)
	writeLogFile(t, dir, FilePrefix, diagTestDate, []string{
		fmt.Sprintf(`{"level":"INFO","msg":%q}`, big),
	})
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 || !page.Records[0].Truncated {
		t.Fatalf("clip = %+v records", page.Records)
	}
	if len(page.Records[0].Message) > DiagMaxRecordBytes {
		t.Fatalf("message not clipped: %d", len(page.Records[0].Message))
	}
}

func TestDiagnosticsAppendGUIAndReadBack(t *testing.T) {
	d, _ := newDiagnostics(t)
	ack, err := d.AppendGUI(context.Background(), GuiLogBatch{Records: []GuiLogRecord{
		{Level: "INFO", Message: "gui hello", Component: "console"},
		{Level: "bogus", Message: "normalized"},
		{Message: "with secret token=sk-abcdef1234567890"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if ack.Accepted != 3 {
		t.Fatalf("accepted = %d", ack.Accepted)
	}
	date := time.Now().Format("2006-01-02")
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "gui", Date: date})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 3 {
		t.Fatalf("gui records = %+v", page.Records)
	}
	if page.Records[0].Level != "info" || page.Records[0].Component != "console" {
		t.Fatalf("rec0 = %+v", page.Records[0])
	}
	if page.Records[1].Level != "info" {
		t.Fatalf("bogus level normalized: %+v", page.Records[1])
	}
	if strings.Contains(page.Records[2].Message, "sk-abcdef") {
		t.Fatalf("gui secret leaked: %q", page.Records[2].Message)
	}
}

func TestDiagnosticsAppendGUIBounds(t *testing.T) {
	d, _ := newDiagnostics(t)
	overs := GuiLogBatch{Records: make([]GuiLogRecord, DiagMaxRecords+1)}
	for i := range overs.Records {
		overs.Records[i] = GuiLogRecord{Message: "x"}
	}
	if _, err := d.AppendGUI(context.Background(), overs); !errors.Is(err, ErrDiagnosticQuery) {
		t.Fatalf("oversize batch err = %v", err)
	}
	big := GuiLogBatch{Records: []GuiLogRecord{{Message: strings.Repeat("m", DiagMaxRecordBytes)}}}
	if _, err := d.AppendGUI(context.Background(), big); !errors.Is(err, ErrDiagnosticQuery) {
		t.Fatalf("oversize record err = %v", err)
	}
}

func TestDiagnosticsClosedFailsClosed(t *testing.T) {
	d, _ := newDiagnostics(t)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal("repeat close must be safe")
	}
	if _, err := d.AppendGUI(context.Background(), GuiLogBatch{Records: []GuiLogRecord{{Message: "x"}}}); !errors.Is(err, ErrDiagnosticsClosed) {
		t.Fatalf("append after close = %v", err)
	}
	if _, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime"}); !errors.Is(err, ErrDiagnosticsClosed) {
		t.Fatalf("read after close = %v", err)
	}
}
