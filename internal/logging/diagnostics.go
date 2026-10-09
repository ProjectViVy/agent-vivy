// Diagnostics is the bounded log-read/GUI-capture service behind the
// diagnostics/* control RPCs (D5). It exposes only the two owned rotating
// families — the runtime sink (FilePrefix, read-only) and a distinct GUI
// sink (GUIFilePrefix, append through this service) — resolved purely from
// the configured log directory. Callers pick source + date; no path, glob,
// or filename ever crosses the boundary, and symlinks fail closed.
package logging

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// GUIFilePrefix names the rotating GUI family <prefix>.YYYY-MM-DD,
// appended through Diagnostics.AppendGUI.
const GUIFilePrefix = "vivy-gui.log"

// D5 resource bounds. These are guards, not performance targets.
const (
	// DiagMaxRecords is the default and ceiling for page records and GUI
	// batch/queue records.
	DiagMaxRecords = 500
	// DiagMaxRecordBytes is the serialized ceiling per record.
	DiagMaxRecordBytes = 8 * 1024
	// DiagMaxScanBytes is the per-request scan/batch byte ceiling:
	// 500 records × 8KiB.
	DiagMaxScanBytes = DiagMaxRecords * DiagMaxRecordBytes
)

// DiagnosticSourceRuntime and DiagnosticSourceGUI enumerate the readable
// families; every other value is rejected.
const (
	DiagnosticSourceRuntime = "runtime"
	DiagnosticSourceGUI     = "gui"
)

var (
	// ErrDiagnosticQuery marks query-validation failures; the control plane
	// maps it to InvalidParams.
	ErrDiagnosticQuery = errors.New("logging: invalid diagnostic query")
	// ErrDiagnosticsClosed fails append/read after Close.
	ErrDiagnosticsClosed = errors.New("logging: diagnostics closed")
)

// DiagnosticRecord is one bounded log line (D5). ID is the
// stable file-offset identity `<date>:<offset>`; Truncated marks a record
// clipped to DiagMaxRecordBytes.
type DiagnosticRecord struct {
	ID        string         `json:"id"`
	At        *int64         `json:"at,omitempty"`
	Level     string         `json:"level,omitempty"`
	Component string         `json:"component,omitempty"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	Truncated bool           `json:"truncated"`
}

// DiagnosticQuery is diagnostics/logs input. Source is required; Date is a
// strict YYYY-MM-DD log-family date (default: today); After is a
// server-issued cursor; Limit defaults to and caps at DiagMaxRecords;
// Level and Query filter within the bounded scan.
type DiagnosticQuery struct {
	Source string `json:"source"`
	Date   string `json:"date,omitempty"`
	After  string `json:"after,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Level  string `json:"level,omitempty"`
	Query  string `json:"query,omitempty"`
}

// DiagnosticPage is diagnostics/logs output. Gap marks rotation/retention/
// truncation invalidation of the supplied cursor; HasMore means the scan
// stopped at a bound and NextCursor resumes it.
type DiagnosticPage struct {
	Source     string             `json:"source"`
	Records    []DiagnosticRecord `json:"records"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Gap        bool               `json:"gap"`
	HasMore    bool               `json:"has_more"`
}

// GuiLogRecord is one diagnostics/gui/append record (no id/truncated —
// both are server-owned).
type GuiLogRecord struct {
	At        *int64         `json:"at,omitempty"`
	Level     string         `json:"level,omitempty"`
	Component string         `json:"component,omitempty"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// GuiLogBatch is diagnostics/gui/append input.
type GuiLogBatch struct {
	Records []GuiLogRecord `json:"records"`
}

// GuiLogAck reports accepted records. Any write failure is an explicit
// error carrying the accepted prefix count, never a silent ack.
type GuiLogAck struct {
	Accepted int `json:"accepted"`
}

// GuiAppendError reports a partial GUI write: Accepted records reached
// disk before Err.
type GuiAppendError struct {
	Accepted int
	Err      error
}

func (e *GuiAppendError) Error() string {
	return fmt.Sprintf("logging: gui append failed after %d accepted record(s): %v", e.Accepted, e.Err)
}

func (e *GuiAppendError) Unwrap() error { return e.Err }

// Diagnostics serves bounded reads over the owned log families and owns
// the GUI family's daily writer. It logs nothing itself — recursive
// logging through this service is prohibited.
type Diagnostics struct {
	dir string
	now func() time.Time

	mu     sync.Mutex
	gui    *dailyFile
	closed bool
}

// NewDiagnostics binds the service to dir (the configured log directory).
// The GUI writer opens lazily on first append so a read-only deployment
// costs nothing.
func NewDiagnostics(dir string) (*Diagnostics, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("logging: diagnostics dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("logging: create %s: %w", dir, err)
	}
	return &Diagnostics{dir: dir, now: time.Now, gui: newDailyFile(dir, GUIFilePrefix, time.Now)}, nil
}

// diagFileStat retains the v1 cursor fields so old cursors can be validated
// before their one-time reset. V2 uses stable file ID, size and an anchor.
type diagFileStat struct {
	ino   uint64
	size  int64
	modNs int64
}

func statIdentity(info os.FileInfo) diagFileStat {
	return diagFileStat{ino: fileIdentity(info), size: info.Size(), modNs: info.ModTime().UnixNano()}
}

// diagCursorV2 is a continuation contract over one physical file snapshot.
// Anchor detects same-inode truncate/regrow when mtime and size are preserved.
type diagCursorV2 struct {
	Date        string `json:"date"`
	FileID      uint64 `json:"file_id"`
	Size        int64  `json:"size"`
	Offset      int64  `json:"offset"`
	Anchor      string `json:"anchor"`
	DiscardLine bool   `json:"discard_line"`
}

func diagCursorV2Encode(c diagCursorV2) string {
	payload, _ := json.Marshal(c)
	return "v2." + base64.RawURLEncoding.EncodeToString(payload)
}

func diagCursorV2Decode(raw string) (diagCursorV2, error) {
	var c diagCursorV2
	if len(raw) == 0 || len(raw) > 2048 || !strings.HasPrefix(raw, "v2.") {
		return c, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "v2."))
	if err != nil || len(payload) > 2048 {
		return c, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil || len(fields) != 6 {
		return c, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	for _, name := range []string{"date", "file_id", "size", "offset", "anchor", "discard_line"} {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return c, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return diagCursorV2{}, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return diagCursorV2{}, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	if validateDiagnosticDate(c.Date) != nil || c.Size < 0 || c.Offset < 0 || c.Offset > c.Size {
		return diagCursorV2{}, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	if (c.Offset == 0 && c.Anchor != "") || (c.Offset > 0 && len(c.Anchor) != sha256.Size*2) {
		return diagCursorV2{}, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	if c.Anchor != "" {
		if _, err := hex.DecodeString(c.Anchor); err != nil || strings.ToLower(c.Anchor) != c.Anchor {
			return diagCursorV2{}, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
		}
	}
	return c, nil
}

// diagLegacyCursorDecode recognizes the retired five-part cursor solely so
// callers can receive one explicit gap/reset into the v2 contract.
func diagLegacyCursorDecode(raw string) (string, diagFileStat, int64, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 5 {
		return "", diagFileStat{}, 0, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	if err := validateDiagnosticDate(parts[0]); err != nil {
		return "", diagFileStat{}, 0, err
	}
	ino, e1 := strconv.ParseUint(parts[1], 10, 64)
	modNs, e2 := strconv.ParseInt(parts[2], 10, 64)
	size, e3 := strconv.ParseInt(parts[3], 10, 64)
	offset, e4 := strconv.ParseInt(parts[4], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || size < 0 || offset < 0 || offset > size {
		return "", diagFileStat{}, 0, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	return parts[0], diagFileStat{ino: ino, size: size, modNs: modNs}, offset, nil
}

// diagAnchor hashes at most the 64 bytes immediately preceding offset. Its
// read is charged to the caller's physical-read budget.
func diagAnchor(f *os.File, offset int64, budget *int64) (string, error) {
	if offset == 0 {
		return "", nil
	}
	if offset < 0 || budget == nil {
		return "", fmt.Errorf("%w: invalid cursor anchor", ErrDiagnosticQuery)
	}
	count := min(int64(64), offset)
	if *budget < count {
		return "", fmt.Errorf("%w: diagnostic read budget exhausted", ErrDiagnosticQuery)
	}
	buf := make([]byte, count)
	n, err := f.ReadAt(buf, offset-count)
	*budget -= int64(n)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if int64(n) != count {
		return "", io.ErrUnexpectedEOF
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:]), nil
}

func validateDiagnosticDate(date string) error {
	if _, err := time.ParseInLocation("2006-01-02", date, time.Local); err != nil || len(date) != len("2006-01-02") {
		return fmt.Errorf("%w: date must be YYYY-MM-DD", ErrDiagnosticQuery)
	}
	return nil
}

func diagPrefix(source string) (string, error) {
	switch source {
	case DiagnosticSourceRuntime:
		return FilePrefix, nil
	case DiagnosticSourceGUI:
		return GUIFilePrefix, nil
	default:
		return "", fmt.Errorf("%w: unknown source %q", ErrDiagnosticQuery, source)
	}
}

// resolveFile maps (source, date) to the owned file inside d.dir. Any
// non-regular or symlinked path is refused — containment is structural:
// the only inputs are the enumerated source and a strict date.
func (d *Diagnostics) resolveFile(source, date string) (string, os.FileInfo, error) {
	prefix, err := diagPrefix(source)
	if err != nil {
		return "", nil, err
	}
	if err := validateDiagnosticDate(date); err != nil {
		return "", nil, err
	}
	path := filepath.Join(d.dir, fmt.Sprintf("%s.%s", prefix, date))
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil
		}
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%w: log file is not a regular file", ErrDiagnosticQuery)
	}
	return path, info, nil
}

// normalizeDiagLevel maps a level to the closed vocabulary; ok is false
// for unrecognized values.
func normalizeDiagLevel(level string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "dbg":
		return "debug", true
	case "", "info", "inf", "information":
		return "info", true
	case "warn", "warning", "wrn":
		return "warn", true
	case "error", "err", "fatal", "panic":
		return "error", true
	default:
		return "", false
	}
}

// parseDiagLine decodes one raw line into a bounded record. JSON lines
// keep authorized attributes; everything else becomes a bounded message.
// Malformed JSON is reported as a placeholder.
func parseDiagLine(id string, raw []byte) DiagnosticRecord {
	trimmed := strings.TrimSpace(string(raw))
	record := DiagnosticRecord{ID: id}
	var obj map[string]any
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			record.Message = "[malformed log line omitted]"
			return record
		}
	} else {
		record.Message = trimmed
		return record
	}
	fields := map[string]any{}
	for key, value := range obj {
		switch key {
		case "time", "ts", "at":
			switch v := value.(type) {
			case string:
				if ts, err := time.Parse(time.RFC3339Nano, v); err == nil {
					ms := ts.UnixMilli()
					record.At = &ms
				}
			case float64:
				ms := int64(v)
				record.At = &ms
			}
		case "level", "level_name":
			if s, ok := value.(string); ok {
				if normalized, ok := normalizeDiagLevel(s); ok {
					record.Level = normalized
				}
			}
		case "msg", "message":
			if s, ok := value.(string); ok {
				record.Message = s
			}
		case "component", "logger", "source":
			if s, ok := value.(string); ok {
				record.Component = s
			}
		default:
			if s, ok := value.(string); ok {
				fields[key] = s
				continue
			}
			fields[key] = value
		}
	}
	if len(fields) > 0 {
		record.Fields = fields
	}
	if record.Message == "" {
		record.Message = "[empty log line]"
	}
	return record
}

// clipDiagRecord enforces the serialized 8KiB record bound, including JSON
// escaping and the complete envelope. Structured fields drop before text.
func clipDiagRecord(record DiagnosticRecord) DiagnosticRecord {
	record.ID = strings.ToValidUTF8(record.ID, "\uFFFD")
	record.Level = strings.ToValidUTF8(record.Level, "\uFFFD")
	record.Component = strings.ToValidUTF8(record.Component, "\uFFFD")
	record.Message = strings.ToValidUTF8(record.Message, "\uFFFD")
	if encoded, err := json.Marshal(record); err == nil && len(encoded) <= DiagMaxRecordBytes {
		return record
	}
	record.Truncated = true
	record.Fields = nil
	// Cap envelope strings first. Besides bounding output, this leaves room
	// for the message even when every byte needs a six-byte JSON escape.
	record.ID = diagUTF8Prefix(record.ID, 128)
	record.Level = diagUTF8Prefix(record.Level, 64)
	record.Component = diagUTF8Prefix(record.Component, 128)
	if _, err := json.Marshal(record); err == nil {
		low, high := 0, len(record.Message)
		for low < high {
			mid := (low + high + 1) / 2
			candidate := record
			candidate.Message = diagUTF8Prefix(record.Message, mid)
			encoded, marshalErr := json.Marshal(candidate)
			if marshalErr == nil && len(encoded) <= DiagMaxRecordBytes {
				low = mid
			} else {
				high = mid - 1
			}
		}
		record.Message = diagUTF8Prefix(record.Message, low)
	}
	return record
}

func diagUTF8Prefix(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

// diagBudgetReader bounds and counts bytes physically returned by the file
// reader. bufio read-ahead therefore consumes the same per-call budget.
type diagBudgetReader struct {
	r         io.Reader
	remaining int64
	read      int64
}

func (r *diagBudgetReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	r.read += int64(n)
	return n, err
}

// Read serves diagnostics/logs over the owned runtime/GUI families.
// Missing files yield an empty page; a stale/mismatched cursor yields
// gap:true with the file re-read from offset 0 so the client reloads
// rather than claiming continuity.
func (d *Diagnostics) Read(ctx context.Context, q DiagnosticQuery) (DiagnosticPage, error) {
	page := DiagnosticPage{Source: q.Source}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return page, ErrDiagnosticsClosed
	}
	d.mu.Unlock()

	if _, err := diagPrefix(q.Source); err != nil {
		return page, err
	}
	date := strings.TrimSpace(q.Date)
	if date == "" {
		date = d.now().Format("2006-01-02")
	}
	limit := q.Limit
	if limit <= 0 || limit > DiagMaxRecords {
		limit = DiagMaxRecords
	}
	levelFilter := ""
	if q.Level != "" {
		var ok bool
		levelFilter, ok = normalizeDiagLevel(q.Level)
		if !ok {
			return page, fmt.Errorf("%w: unknown level %q", ErrDiagnosticQuery, q.Level)
		}
	}
	queryFilter := strings.ToLower(q.Query)
	// A malformed cursor is InvalidParams regardless of whether the
	// referenced file still exists, so decode before resolution.
	var cursor diagCursorV2
	legacyCursor := false
	if q.After != "" {
		if strings.HasPrefix(q.After, "v2.") {
			var err error
			cursor, err = diagCursorV2Decode(q.After)
			if err != nil {
				return page, err
			}
		} else if _, _, _, err := diagLegacyCursorDecode(q.After); err != nil {
			return page, err
		} else {
			legacyCursor = true
		}
	}

	path, info, err := d.resolveFile(q.Source, date)
	if err != nil {
		return page, err
	}
	if info == nil {
		return page, nil // empty page for a date/family that does not exist
	}
	f, err := os.Open(path)
	if err != nil {
		return page, err
	}
	defer func() { _ = f.Close() }()
	fInfo, err := f.Stat()
	if err != nil {
		return page, err
	}
	if !fInfo.Mode().IsRegular() || !os.SameFile(info, fInfo) {
		return page, fmt.Errorf("%w: opened log file changed during resolution", ErrDiagnosticQuery)
	}
	st := statIdentity(fInfo)
	var offset int64
	discardLine := false
	budget := int64(DiagMaxScanBytes)
	if q.After != "" {
		if legacyCursor {
			page.Gap = true
		} else if cursor.Date != date || cursor.FileID != st.ino || cursor.Size > st.size {
			page.Gap = true
		} else {
			anchor, anchorErr := diagAnchor(f, cursor.Offset, &budget)
			if anchorErr != nil {
				return page, anchorErr
			}
			if anchor != cursor.Anchor {
				page.Gap = true
			} else {
				offset = cursor.Offset
				discardLine = cursor.DiscardLine
			}
		}
	}

	scanStart := offset
	// Reserve up to 64 bytes for the next cursor's anchor. The incoming
	// cursor anchor, scan and outgoing anchor share one physical-read budget.
	scanBudget := budget - min(int64(64), budget)
	limited := &diagBudgetReader{
		r:         io.NewSectionReader(f, scanStart, max(int64(0), st.size-scanStart)),
		remaining: scanBudget,
	}
	reader := bufio.NewReaderSize(limited, 64*1024)
	logicalOffset := func() int64 {
		return scanStart + limited.read - int64(reader.Buffered())
	}
	page.Records = []DiagnosticRecord{}
	incompleteLine := false
	appendRecord := func(record DiagnosticRecord) {
		if (levelFilter == "" || record.Level == levelFilter) &&
			(queryFilter == "" || strings.Contains(strings.ToLower(record.Message), queryFilter)) {
			page.Records = append(page.Records, clipDiagRecord(record))
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return page, err
		}
		lineStart := logicalOffset()
		if discardLine {
			lineComplete := false
			for {
				fragment, readErr := reader.ReadSlice('\n')
				if len(fragment) > 0 && fragment[len(fragment)-1] == '\n' {
					lineComplete = true
					break
				}
				if errors.Is(readErr, bufio.ErrBufferFull) {
					continue
				}
				if readErr != nil {
					break
				}
			}
			offset = logicalOffset()
			if !lineComplete {
				break
			}
			discardLine = false
			continue
		}

		prefix := make([]byte, 0, DiagMaxRecordBytes)
		lineBytes := int64(0)
		lineComplete := false
		var lineErr error
		for {
			fragment, readErr := reader.ReadSlice('\n')
			terminated := len(fragment) > 0 && fragment[len(fragment)-1] == '\n'
			lineBytes += int64(len(fragment))
			if room := DiagMaxRecordBytes - len(prefix); room > 0 {
				if len(fragment) > room {
					fragment = fragment[:room]
				}
				prefix = append(prefix, fragment...)
			}
			if terminated {
				lineComplete = true
				break
			}
			if errors.Is(readErr, bufio.ErrBufferFull) {
				continue
			}
			if readErr != nil {
				lineErr = readErr
				break
			}
		}
		consumedOffset := logicalOffset()
		if lineComplete {
			offset = consumedOffset
			if len(strings.TrimSpace(string(prefix))) > 0 {
				record := parseDiagLine(fmt.Sprintf("%s:%d", date, lineStart), prefix)
				record.Truncated = record.Truncated || lineBytes > int64(len(prefix))
				appendRecord(record)
			}
			if len(page.Records) >= limit {
				break
			}
			continue
		}
		if lineErr != nil && !errors.Is(lineErr, io.EOF) {
			return page, lineErr
		}
		if limited.remaining == 0 && consumedOffset < st.size {
			// The scan budget stopped inside this physical line. Emit only its
			// bounded prefix and persist discard state so no tail fragment can
			// be mistaken for another record on the next page.
			if lineBytes > 0 {
				if len(strings.TrimSpace(string(prefix))) > 0 {
					record := parseDiagLine(fmt.Sprintf("%s:%d", date, lineStart), prefix)
					record.Truncated = true
					appendRecord(record)
				}
				discardLine = true
				offset = consumedOffset
			} else {
				offset = lineStart
			}
			break
		}
		// The snapshot ends in an incomplete line. Leave the cursor at that
		// line's start; a later append rereads the now complete physical line.
		offset = lineStart
		incompleteLine = lineBytes > 0
		break
	}
	page.HasMore = discardLine || incompleteLine || offset < st.size
	remainingForAnchor := budget - limited.read
	anchor, err := diagAnchor(f, offset, &remainingForAnchor)
	if err != nil {
		return page, err
	}
	page.NextCursor = diagCursorV2Encode(diagCursorV2{
		Date: date, FileID: st.ino, Size: st.size, Offset: offset,
		Anchor: anchor, DiscardLine: discardLine,
	})
	return page, nil
}

// AppendGUI validates, sanitizes and persists one bounded GUI batch to the
// owned GUI daily family. Accepted counts only records that actually
// reached disk; a mid-batch failure returns GuiAppendError, never a
// successful ack with hidden loss.
func (d *Diagnostics) AppendGUI(ctx context.Context, batch GuiLogBatch) (GuiLogAck, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return GuiLogAck{}, ErrDiagnosticsClosed
	}
	if len(batch.Records) > DiagMaxRecords {
		return GuiLogAck{}, fmt.Errorf("%w: batch exceeds %d records", ErrDiagnosticQuery, DiagMaxRecords)
	}
	nowMs := d.now().UnixMilli()
	lines := make([][]byte, len(batch.Records))
	var total int
	for i := range batch.Records {
		line, err := marshalGuiRecord(batch.Records[i], nowMs)
		if err != nil {
			return GuiLogAck{Accepted: i}, err
		}
		lines[i] = line
		total += len(line)
	}
	if total > DiagMaxScanBytes {
		return GuiLogAck{}, fmt.Errorf("%w: batch exceeds %d bytes", ErrDiagnosticQuery, DiagMaxScanBytes)
	}
	ack := GuiLogAck{}
	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return ack, &GuiAppendError{Accepted: ack.Accepted, Err: err}
		}
		if _, err := d.gui.Write(line); err != nil {
			return ack, &GuiAppendError{Accepted: ack.Accepted, Err: err}
		}
		ack.Accepted++
	}
	return ack, nil
}

// marshalGuiRecord renders one GUI record as the JSON line the reader
// parses back. Level/time are normalized, the message and string fields
// retain their original values within the structural payload budget.
func marshalGuiRecord(record GuiLogRecord, nowMs int64) ([]byte, error) {
	level, _ := normalizeDiagLevel(record.Level)
	at := nowMs
	if record.At != nil {
		at = *record.At
	}
	obj := map[string]any{
		"time":  time.UnixMilli(at).Format(time.RFC3339Nano),
		"level": level,
		"msg":   record.Message,
	}
	if record.Component != "" {
		obj["component"] = record.Component
	}
	for key, value := range record.Fields {
		if s, ok := value.(string); ok {
			obj[key] = s
			continue
		}
		obj[key] = value
	}
	line, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	if len(line)+1 > DiagMaxRecordBytes {
		return nil, fmt.Errorf("%w: record exceeds %d bytes", ErrDiagnosticQuery, DiagMaxRecordBytes)
	}
	return append(line, '\n'), nil
}

// Close releases the GUI writer. Repeated Close is safe; append and read
// after close fail closed.
func (d *Diagnostics) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	return d.gui.Close()
}
