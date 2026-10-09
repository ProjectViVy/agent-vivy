package logging

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDiagnosticsAppendContinuationStable(t *testing.T) {
	d, dir := newDiagnostics(t)
	path := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate})
	if err != nil || len(first.Records) != 1 || first.Records[0].Message != "one" {
		t.Fatalf("first page = %+v err=%v", first, err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("two\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	continued, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: first.NextCursor})
	if err != nil || continued.Gap || len(continued.Records) != 1 || continued.Records[0].Message != "two" {
		t.Fatalf("append continuation = %+v err=%v", continued, err)
	}
}

func TestDiagnosticsRotationAndTruncateRegrowGap(t *testing.T) {
	d, dir := newDiagnostics(t)
	path := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	original := []byte("one\nsecond\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, Limit: 1})
	if err != nil || len(first.Records) != 1 || !first.HasMore {
		t.Fatalf("first page = %+v err=%v", first, err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("new\n"), 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("fixture did not preserve identity/size/mtime: before=%+v after=%+v", before, after)
	}
	continued, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: first.NextCursor})
	if err != nil || !continued.Gap || len(continued.Records) != 2 {
		t.Fatalf("same-file truncate/regrow should gap: %+v err=%v", continued, err)
	}
}

func TestDiagnosticsOversizeContinuationKeepsLineAlignment(t *testing.T) {
	d, dir := newDiagnostics(t)
	path := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	const oversizedBytes = DiagMaxScanBytes + 128
	if err := os.WriteFile(path, []byte(strings.Repeat("x", oversizedBytes)+"\ntail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate})
	if err != nil || len(first.Records) != 1 || !first.Records[0].Truncated || !first.HasMore {
		t.Fatalf("oversize first page = %+v err=%v", first, err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(first.NextCursor, "v2."))
	if err != nil {
		t.Fatalf("cursor is not v2: %q: %v", first.NextCursor, err)
	}
	var cursor struct {
		Offset      int64 `json:"offset"`
		DiscardLine bool  `json:"discard_line"`
	}
	if err := json.Unmarshal(payload, &cursor); err != nil {
		t.Fatal(err)
	}
	if cursor.Offset+64 != DiagMaxScanBytes || !cursor.DiscardLine {
		t.Fatalf("first page exceeded budget or lost line state: %+v", cursor)
	}
	t.Logf("first page cursor offset=%d, outgoing anchor budget=64, total ceiling=%d", cursor.Offset, DiagMaxScanBytes)
	second, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: first.NextCursor})
	if err != nil || second.Gap || len(second.Records) != 1 || second.Records[0].Message != "tail" {
		t.Fatalf("oversize continuation parsed a fragment or lost tail: %+v err=%v", second, err)
	}
}

func TestDiagnosticsScanBudgetCountsPhysicalReads(t *testing.T) {
	counted := &diagCountingReader{r: strings.NewReader(strings.Repeat("x", DiagMaxScanBytes*2))}
	remaining := int64(DiagMaxScanBytes)
	// The input and output cursors each charge their maximum 64-byte anchor.
	remaining -= 64
	scan := &diagBudgetReader{r: counted, remaining: remaining - 64}
	read, err := io.Copy(io.Discard, scan)
	if err != nil {
		t.Fatal(err)
	}
	physical := int64(64) + counted.read + 64
	if read != remaining-64 || counted.read != read || physical != DiagMaxScanBytes {
		t.Fatalf("physical reads=%d scan=%d budget=%d", physical, read, DiagMaxScanBytes)
	}
}

type diagCountingReader struct {
	r    io.Reader
	read int64
}

func (r *diagCountingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.read += int64(n)
	return n, err
}

func TestDiagnosticsIncompleteLineWaitsForNewline(t *testing.T) {
	d, dir := newDiagnostics(t)
	path := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	partialBytes := append([]byte(`{"level":"INFO","msg":"caf`), []byte("é")[:1]...)
	if err := os.WriteFile(path, partialBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	partial, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate})
	if err != nil || len(partial.Records) != 0 || !partial.HasMore || partial.NextCursor == "" {
		t.Fatalf("partial line page = %+v err=%v", partial, err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append([]byte("é"[1:]), []byte(`"}`+"\n")...)); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	complete, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: partial.NextCursor})
	if err != nil || complete.Gap || len(complete.Records) != 1 || complete.Records[0].Message != "café" {
		t.Fatalf("completed line page = %+v err=%v", complete, err)
	}
}

func TestDiagnosticsEveryEnvelopeFieldBounded(t *testing.T) {
	control := strings.Repeat("\x01", 16*1024)
	record := clipDiagRecord(DiagnosticRecord{
		ID: strings.Repeat("i", 16*1024), Level: strings.Repeat("级", 16*1024),
		Component: strings.Repeat("组件", 16*1024), Message: control + strings.Repeat("🧪", 4096),
		Fields: map[string]any{"nested": map[string]any{"large": strings.Repeat("值", 16*1024)}},
	})
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > DiagMaxRecordBytes || !record.Truncated || !utf8.ValidString(string(encoded)) {
		t.Fatalf("bounded record bytes=%d truncated=%v valid_utf8=%v", len(encoded), record.Truncated, utf8.ValidString(string(encoded)))
	}
}

func TestDiagnosticsLegacyCursorResetsOnce(t *testing.T) {
	d, dir := newDiagnostics(t)
	path := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := diagLegacyCursorForTest(diagTestDate, statIdentity(info), 4)
	reset, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: legacy})
	if err != nil || !reset.Gap || len(reset.Records) != 2 || !strings.HasPrefix(reset.NextCursor, "v2.") {
		t.Fatalf("legacy reset = %+v err=%v", reset, err)
	}
	continued, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: reset.NextCursor})
	if err != nil || continued.Gap || len(continued.Records) != 0 {
		t.Fatalf("v2 continuation after one-time reset = %+v err=%v", continued, err)
	}
}

func TestDiagnosticsRejectsMalformedV2Cursor(t *testing.T) {
	d, _ := newDiagnostics(t)
	for _, payload := range []string{
		`{}`,
		`{"date":"2026-10-02","file_id":1,"size":0,"offset":0,"anchor":""}`,
		`{"date":"2026-10-02","file_id":1,"size":0,"offset":0,"anchor":"","discard_line":null}`,
		`{"date":"2026-10-02","file_id":1,"size":1,"offset":2,"anchor":"","discard_line":false}`,
		`{"date":"2026-10-02","file_id":1,"size":0,"offset":0,"anchor":"","discard_line":false,"extra":1}`,
		`{"date":"2026-10-02","file_id":1,"size":0,"offset":0,"anchor":"","discard_line":false} trailing`,
	} {
		token := "v2." + base64.RawURLEncoding.EncodeToString([]byte(payload))
		if _, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: token}); !errors.Is(err, ErrDiagnosticQuery) {
			t.Fatalf("cursor %q error=%v, want ErrDiagnosticQuery", token, err)
		}
	}
}

func TestDiagnosticsReplacementFileReportsGap(t *testing.T) {
	d, dir := newDiagnostics(t)
	path := filepath.Join(dir, FilePrefix+"."+diagTestDate)
	if err := os.WriteFile(path, []byte("old\nnext\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, Limit: 1})
	if err != nil || len(first.Records) != 1 || !first.HasMore {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(replacement, []byte("new file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	page, err := d.Read(context.Background(), DiagnosticQuery{Source: "runtime", Date: diagTestDate, After: first.NextCursor})
	if err != nil || !page.Gap || len(page.Records) != 1 || page.Records[0].Message != "new file" {
		t.Fatalf("replacement page=%+v err=%v", page, err)
	}
}

func diagLegacyCursorForTest(date string, stat diagFileStat, offset int64) string {
	return strings.Join([]string{
		date,
		strconv.FormatUint(stat.ino, 10),
		strconv.FormatInt(stat.modNs, 10),
		strconv.FormatInt(stat.size, 10),
		strconv.FormatInt(offset, 10),
	}, ".")
}
