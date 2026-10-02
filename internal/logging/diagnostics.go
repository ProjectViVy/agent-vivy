// Diagnostics is the bounded log-read/GUI-capture service behind the
// diagnostics/* control RPCs (D5). It exposes only the two owned rotating
// families — the runtime sink (FilePrefix, read-only) and a distinct GUI
// sink (GUIFilePrefix, append through this service) — resolved purely from
// the configured log directory. Callers pick source + date; no path, glob,
// or filename ever crosses the boundary, and symlinks fail closed.
package logging

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
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

// DiagnosticRecord is one bounded, redacted log line (D5). ID is the
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

// diagFileStat is the cursor-verifiable file identity: device+inode plus
// size and mtime, so rotation/rename/truncation cannot pass as the same
// file.
type diagFileStat struct {
	ino   uint64
	size  int64
	modNs int64
}

func statIdentity(info os.FileInfo) diagFileStat {
	s := diagFileStat{size: info.Size(), modNs: info.ModTime().UnixNano()}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		s.ino = st.Ino
	}
	return s
}

// diagCursor is the server-issued opaque token `<date>.<ino>.<modns>.<size>.<offset>`.
func diagCursorEncode(date string, st diagFileStat, off int64) string {
	return fmt.Sprintf("%s.%d.%d.%d.%d", date, st.ino, st.modNs, st.size, off)
}

func diagCursorDecode(cursor string) (date string, st diagFileStat, off int64, err error) {
	parts := strings.Split(cursor, ".")
	if len(parts) != 5 {
		return "", diagFileStat{}, 0, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	date = parts[0]
	ino, e1 := strconv.ParseUint(parts[1], 10, 64)
	modNs, e2 := strconv.ParseInt(parts[2], 10, 64)
	size, e3 := strconv.ParseInt(parts[3], 10, 64)
	offset, e4 := strconv.ParseInt(parts[4], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || offset < 0 {
		return "", diagFileStat{}, 0, fmt.Errorf("%w: malformed cursor", ErrDiagnosticQuery)
	}
	return date, diagFileStat{ino: ino, size: size, modNs: modNs}, offset, nil
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
// keep bounded sanitized attributes; everything else becomes a raw
// sanitized message. Malformed JSON is reported as a placeholder, never
// an unredacted raw fragment.
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
		record.Message = Redact(trimmed)
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
				record.Message = Redact(s)
			}
		case "component", "logger", "source":
			if s, ok := value.(string); ok {
				record.Component = s
			}
		default:
			if sensitiveAttrKey(key) {
				fields[key] = valueMarker
				continue
			}
			if s, ok := value.(string); ok {
				fields[key] = Redact(s)
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

// clipDiagRecord enforces the serialized 8KiB record bound; on overflow
// fields drop first, then the message is clipped and Truncated is set.
func clipDiagRecord(record DiagnosticRecord) DiagnosticRecord {
	encoded, err := json.Marshal(record)
	if err != nil || len(encoded) <= DiagMaxRecordBytes {
		return record
	}
	record.Fields = nil
	if encoded, err = json.Marshal(record); err == nil && len(encoded) <= DiagMaxRecordBytes {
		record.Truncated = true
		return record
	}
	// Reserve room for the envelope; clip on bytes, never mid-escape.
	overhead := len(encoded) - len(record.Message)
	budget := DiagMaxRecordBytes - overhead
	if budget < 0 {
		budget = 0
	}
	record.Message = record.Message[:budget]
	record.Truncated = true
	return record
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
	var cDate string
	var cStat diagFileStat
	var cOff int64
	if q.After != "" {
		var err error
		cDate, cStat, cOff, err = diagCursorDecode(q.After)
		if err != nil {
			return page, err
		}
	}

	path, info, err := d.resolveFile(q.Source, date)
	if err != nil {
		return page, err
	}
	if info == nil {
		return page, nil // empty page for a date/family that does not exist
	}
	st := statIdentity(info)
	var offset int64
	if q.After != "" {
		if cDate != date || cStat.ino != st.ino || cStat.modNs != st.modNs || cOff > st.size {
			page.Gap = true
		} else {
			offset = cOff
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return page, err
	}
	defer func() { _ = f.Close() }()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return page, err
		}
	}
	reader := bufio.NewReaderSize(f, 64*1024)
	scanned := 0
	page.Records = []DiagnosticRecord{}
	for {
		if err := ctx.Err(); err != nil {
			return page, err
		}
		lineStart := offset
		line, err := reader.ReadBytes('\n')
		lineBytes := len(line)
		offset += int64(lineBytes)
		scanned += lineBytes
		if len(strings.TrimSpace(string(line))) > 0 {
			record := parseDiagLine(fmt.Sprintf("%s:%d", date, lineStart), line)
			if (levelFilter == "" || record.Level == levelFilter) &&
				(queryFilter == "" || strings.Contains(strings.ToLower(record.Message), queryFilter)) {
				page.Records = append(page.Records, clipDiagRecord(record))
			}
		}
		if len(page.Records) >= limit || scanned >= DiagMaxScanBytes {
			page.HasMore = lineBytes > 0 && err == nil
			if page.HasMore {
				// Bound hit mid-file: peek whether anything remains.
				if _, err := reader.Peek(1); err != nil {
					page.HasMore = false
				}
			}
			page.NextCursor = diagCursorEncode(date, st, offset)
			return page, nil
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				page.NextCursor = diagCursorEncode(date, st, offset)
				return page, nil
			}
			return page, err
		}
	}
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
// are redacted, sensitive-keyed fields collapse to a marker.
func marshalGuiRecord(record GuiLogRecord, nowMs int64) ([]byte, error) {
	level, _ := normalizeDiagLevel(record.Level)
	at := nowMs
	if record.At != nil {
		at = *record.At
	}
	obj := map[string]any{
		"time":  time.UnixMilli(at).Format(time.RFC3339Nano),
		"level": level,
		"msg":   Redact(record.Message),
	}
	if record.Component != "" {
		obj["component"] = record.Component
	}
	for key, value := range record.Fields {
		if sensitiveAttrKey(key) {
			obj[key] = valueMarker
			continue
		}
		if s, ok := value.(string); ok {
			obj[key] = Redact(s)
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
