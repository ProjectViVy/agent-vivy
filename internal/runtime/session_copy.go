package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// VCP C1: session tree read model, pi-JSONL import, and standalone HTML
// export over the existing Journal/truncation infrastructure.

// ErrSessionCopyNotWired refuses tree/import when the stores the read
// model needs are absent. Export needs only Sessions+Messages+ExportDir.
var ErrSessionCopyNotWired = errors.New("runtime: session copy stores are not wired")

// ErrImportMalformed rejects transcripts that are not pi session JSONL —
// the first line must be the session header object.
var ErrImportMalformed = errors.New("runtime: import data is not a pi session transcript")

// Bounds (VCP C1 contract): the tree is a bounded read model and the
// import/export paths cap how many lines/messages they will process.
const (
	sessionTreeMaxNodes  = 500
	importMaxLines       = 10000
	sessionCopyMaxRows   = 5000
	importSourceName     = "pi-jsonl"
	sessionExportPerms   = 0o600
	sessionExportDirPerm = 0o700
)

// SessionTreeNode is one session in the tree; ParentSessionID and
// ForkPointMessageID are set when the node is a fork/clone child.
type SessionTreeNode struct {
	SessionID          string `json:"session_id"`
	Title              string `json:"title"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
	ParentSessionID    string `json:"parent_session_id,omitempty"`
	ForkPointMessageID string `json:"fork_point_message_id,omitempty"`
}

// SessionTreeEdge is one parent→child fork/clone link.
type SessionTreeEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // always "fork" — clones record the same edge shape
}

// SessionTree is the bounded read model behind session/tree.
type SessionTree struct {
	Nodes []SessionTreeNode `json:"nodes"`
	Edges []SessionTreeEdge `json:"edges"`
}

// SessionTree builds the session tree from fork provenance markers: nodes
// are the newest sessions (durable activity order, capped), edges are the
// reason=fork anchors whose endpoints both resolve. Edge ordering follows
// marker insertion order so the model is stable.
func (s *Service) SessionTree(ctx context.Context) (SessionTree, error) {
	if s.deps.Sessions == nil {
		return SessionTree{}, ErrSessionCopyNotWired
	}
	sessions, err := s.deps.Sessions.ListSessions(ctx)
	if err != nil {
		return SessionTree{}, fmt.Errorf("runtime: list sessions: %w", err)
	}
	if len(sessions) > sessionTreeMaxNodes {
		sessions = sessions[:sessionTreeMaxNodes]
	}
	known := make(map[string]bool, len(sessions))
	nodes := make([]SessionTreeNode, 0, len(sessions))
	index := make(map[string]int, len(sessions))
	for _, session := range sessions {
		id := string(session.ID)
		known[id] = true
		index[id] = len(nodes)
		nodes = append(nodes, SessionTreeNode{
			SessionID: id, Title: session.Title,
			CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
		})
	}
	edges := []SessionTreeEdge{}
	if s.deps.Truncations != nil {
		links, err := s.deps.Truncations.ListSessionForkLinks(ctx)
		if err != nil {
			return SessionTree{}, fmt.Errorf("runtime: list fork links: %w", err)
		}
		seen := make(map[string]bool, len(links))
		for _, link := range links {
			if link.Reason != storage.TruncationFork {
				continue // the forked-from back-anchor duplicates the edge
			}
			from, to := string(link.SessionID), link.ForkSessionID
			if !known[from] || !known[to] {
				continue // endpoint outside the node window (or deleted)
			}
			key := from + "\x00" + to + "\x00" + link.CutoffMessageID
			if seen[key] {
				continue
			}
			seen[key] = true
			edges = append(edges, SessionTreeEdge{From: from, To: to, Kind: "fork"})
			nodes[index[to]].ParentSessionID = from
			nodes[index[to]].ForkPointMessageID = link.CutoffMessageID
		}
	}
	return SessionTree{Nodes: nodes, Edges: edges}, nil
}

// ImportResult reports a pi-JSONL import: the new session and how many
// rows landed vs. were skipped (unknown entry types, unparseable lines).
type ImportResult struct {
	SessionID string `json:"session_id"`
	Imported  int    `json:"imported"`
	Skipped   int    `json:"skipped"`
}

// payloadSessionImported journals the provenance of a transcript import.
type payloadSessionImported struct {
	SessionID       string `json:"session_id"`
	Source          string `json:"source"`
	SourceSessionID string `json:"source_session_id,omitempty"`
	Imported        int    `json:"imported"`
	Skipped         int    `json:"skipped"`
}

// piSessionLine is the shared shape of every JSONL line in a pi session
// transcript (packages/coding-agent SessionManager serialization v3):
// a "session" header first, then parentId-chained entries.
type piSessionLine struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp json.RawMessage `json:"timestamp"` // ISO string on entries
	Message   json.RawMessage `json:"message"`   // set on type="message" entries
}

type piMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Timestamp  int64           `json:"timestamp"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
}

type piContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Data      string          `json:"data"`     // base64 on image blocks
	MimeType  string          `json:"mimeType"` // image blocks
	ID        string          `json:"id"`       // toolCall blocks
	Name      string          `json:"name"`     // toolCall blocks
	Arguments json.RawMessage `json:"arguments"`
}

// ImportSession rebuilds a pi session transcript (JSONL) as a NEW Vivy
// session (VCP C1). It never merges into an existing session. The first
// line must be pi's session header; each "message" entry maps onto the
// Vivy row shapes (user text/attachments, assistant text and tool-call
// rows, tool results). Every other entry type — usage, compaction,
// labels, unknown kinds — is counted in result.skipped.
func (s *Service) ImportSession(ctx context.Context, data, title string) (ImportResult, error) {
	mutations, ok := s.deps.Truncations.(storage.HistoryMutationStore)
	if s.deps.Truncations == nil || !ok {
		return ImportResult{}, ErrSessionCopyNotWired
	}
	lines := strings.Split(data, "\n")
	if len(lines) > importMaxLines {
		return ImportResult{}, fmt.Errorf("runtime: import exceeds %d lines", importMaxLines)
	}
	header := piSessionLine{}
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil || header.Type != "session" {
		return ImportResult{}, ErrImportMalformed
	}
	now := time.Now().UnixMilli()
	newID := domain.SessionID(newPrefixedID("sess_"))
	if title == "" {
		title = "Imported session"
		if header.ID != "" {
			title = "Imported session " + header.ID
		}
	}
	child := domain.Session{ID: newID, Title: title, CreatedAt: now, UpdatedAt: now}
	messages := []domain.Message{}
	skipped := 0
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry := piSessionLine{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			skipped++
			continue
		}
		switch entry.Type {
		case "message":
			rows, drop, err := piMessageToRows(entry, newID)
			if err != nil {
				skipped++
				continue
			}
			skipped += drop
			messages = append(messages, rows...)
			if len(messages) > sessionCopyMaxRows {
				return ImportResult{}, fmt.Errorf("runtime: import exceeds %d messages", sessionCopyMaxRows)
			}
		default:
			skipped++ // usage/compaction/model_change/custom/... entries
		}
	}
	event, err := historyEvent(domain.RunID(newPrefixedID("tr_")), domain.EventSessionImported, payloadSessionImported{
		SessionID:       string(newID),
		Source:          importSourceName,
		SourceSessionID: header.ID,
		Imported:        len(messages),
		Skipped:         skipped,
	})
	if err != nil {
		return ImportResult{}, err
	}
	// The transcript lands through the same atomic multi-row commit as a
	// fork: child row + message copies + provenance event, all or nothing.
	events, err := mutations.CommitSessionFork(ctx, child, messages, nil, []domain.RunEvent{event})
	if err != nil {
		return ImportResult{}, fmt.Errorf("runtime: commit session import: %w", err)
	}
	for _, ev := range events {
		s.publish(ctx, ev)
	}
	return ImportResult{SessionID: string(newID), Imported: len(messages), Skipped: skipped}, nil
}

// piMessageToRows maps one pi message entry onto Vivy message rows. A pi
// assistant message carries text AND toolCall blocks: it becomes one text
// row plus one tool-call row per call (the same shape the projector
// writes). The second return counts dropped content blocks (thinking,
// unsupported kinds) separately from skipped entries.
func piMessageToRows(entry piSessionLine, sessionID domain.SessionID) (rows []domain.Message, dropped int, err error) {
	msg := piMessage{}
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return nil, 0, fmt.Errorf("decode message entry: %w", err)
	}
	created := msg.Timestamp
	if created == 0 {
		created = parseEntryTimestamp(entry.Timestamp)
	}
	base := domain.Message{ID: newMessageID(), SessionID: sessionID, CreatedAt: created, WorkSeq: 0}
	textContent, blocks := piContent(msg.Content)
	switch msg.Role {
	case "user":
		text, attachments, drop := piTextAndAttachments(textContent, blocks)
		base.Role, base.Content, base.Attachments = domain.RoleUser, text, attachments
		return []domain.Message{base}, drop, nil
	case "assistant":
		if blocks != nil {
			text := ""
			for _, block := range blocks {
				switch block.Type {
				case "text":
					if text != "" {
						text += "\n"
					}
					text += block.Text
				case "toolCall":
					row := base
					row.ID = newMessageID()
					row.Role, row.ToolCallID, row.ToolName = domain.RoleAssistant, block.ID, block.Name
					if len(block.Arguments) > 0 {
						row.ToolArgs = append([]byte(nil), block.Arguments...)
					}
					rows = append(rows, row)
				default:
					dropped++ // thinking blocks carry no replayable content
				}
			}
			if text != "" {
				base.Role, base.Content = domain.RoleAssistant, text
				// Text precedes its tool calls in the stored view.
				rows = append([]domain.Message{base}, rows...)
			}
			// ListMessages orders by (created_at, id): give each row a +i ms
			// bump so the transcript order survives the fresh random ids.
			for i := range rows {
				rows[i].CreatedAt = created + int64(i)
			}
			return rows, dropped, nil
		}
		base.Role, base.Content = domain.RoleAssistant, textContent
		return []domain.Message{base}, 0, nil
	case "toolResult":
		text, _, drop := piTextAndAttachments(textContent, blocks)
		base.Role, base.Content = domain.RoleTool, text
		base.ToolCallID, base.ToolName = msg.ToolCallID, msg.ToolName
		return []domain.Message{base}, drop, nil
	default:
		return nil, 0, fmt.Errorf("unknown role %q", msg.Role)
	}
}

// piContent decodes pi message content: a bare string unmarshals to
// text; an array decodes to typed blocks (blocks != nil).
func piContent(raw json.RawMessage) (text string, blocks []piContentBlock) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s, nil
		}
		return "", nil
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", nil
	}
	return "", blocks
}

// piTextAndAttachments flattens a content union into text plus
// attachments; returns the dropped-block count for non-text/non-image
// blocks.
func piTextAndAttachments(textContent string, blocks []piContentBlock) (text string, attachments []domain.Attachment, dropped int) {
	if blocks == nil {
		return textContent, nil, 0
	}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if text != "" {
				text += "\n"
			}
			text += block.Text
		case "image":
			if data, err := base64.StdEncoding.DecodeString(block.Data); err == nil && len(data) > 0 {
				attachments = append(attachments, domain.Attachment{Name: "image", MimeType: block.MimeType, Data: data})
			} else {
				dropped++
			}
		default:
			dropped++
		}
	}
	return text, attachments, dropped
}

// parseEntryTimestamp reads pi's ISO-8601 entry timestamp into unix
// milli; unparseable input falls back to now.
func parseEntryTimestamp(raw json.RawMessage) int64 {
	var iso string
	if err := json.Unmarshal(raw, &iso); err == nil {
		if ts, err := time.Parse(time.RFC3339Nano, iso); err == nil {
			return ts.UnixMilli()
		}
	}
	return time.Now().UnixMilli()
}

// ExportResult reports a session export: the written file, its digest for
// verified download (exports/read), and how many visible messages it
// contains.
type ExportResult struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	MessageCount int    `json:"message_count"`
}

// ExportSession renders the session's VISIBLE view (truncation markers
// applied) as a standalone HTML file in the instance exports directory
// (VCP C1). The page is trusted-readonly: all dynamic text is escaped and
// the CSP blocks scripts and external references.
func (s *Service) ExportSession(ctx context.Context, sessionID domain.SessionID, format string) (ExportResult, error) {
	if format != "" && format != "html" {
		return ExportResult{}, fmt.Errorf("runtime: unsupported export format %q", format)
	}
	if s.deps.ExportDir == "" || s.deps.Sessions == nil || s.deps.Messages == nil {
		return ExportResult{}, ErrSessionCopyNotWired
	}
	session, err := s.deps.Sessions.GetSession(ctx, sessionID)
	if err != nil {
		return ExportResult{}, fmt.Errorf("runtime: get session: %w", err)
	}
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return ExportResult{}, fmt.Errorf("runtime: list session messages: %w", err)
	}
	messages, err := s.effectiveSessionMessages(ctx, sessionID, stored)
	if err != nil {
		return ExportResult{}, err
	}
	if len(messages) > sessionCopyMaxRows {
		return ExportResult{}, fmt.Errorf("runtime: export exceeds %d messages", sessionCopyMaxRows)
	}
	body := renderSessionHTML(session, messages)
	if err := os.MkdirAll(s.deps.ExportDir, sessionExportDirPerm); err != nil {
		return ExportResult{}, fmt.Errorf("runtime: create export dir: %w", err)
	}
	name := fmt.Sprintf("%s-%d.html", sanitizeExportName(string(sessionID)), time.Now().UnixMilli())
	path := filepath.Join(s.deps.ExportDir, name)
	raw := []byte(body)
	if err := os.WriteFile(path, raw, sessionExportPerms); err != nil {
		return ExportResult{}, fmt.Errorf("runtime: write export: %w", err)
	}
	digest := sha256.Sum256(raw)
	return ExportResult{
		Path:         path,
		Name:         name,
		SHA256:       hex.EncodeToString(digest[:]),
		Size:         int64(len(raw)),
		MessageCount: len(messages),
	}, nil
}

// renderSessionHTML emits the standalone transcript document: inline
// styles only, no scripts, no external references.
func renderSessionHTML(session domain.Session, messages []domain.Message) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">" +
		"<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; style-src 'unsafe-inline'\">" +
		"<title>" + html.EscapeString(session.Title) + " — Vivy session export</title>" +
		"<style>body{font-family:system-ui,sans-serif;max-width:52rem;margin:2rem auto;padding:0 1rem;color:#1c1c1e;background:#fff}" +
		"h1{font-size:1.25rem} .m{margin:1rem 0;padding:.6rem .8rem;border-radius:8px}" +
		".u{background:#eef4ff} .a{background:#f6f6f8} .t{background:#fff7e6} .role{font-size:.75rem;text-transform:uppercase;letter-spacing:.05em;color:#666;margin-bottom:.3rem}" +
		"pre{white-space:pre-wrap;word-break:break-word;margin:0;font-family:ui-monospace,monospace;font-size:.85rem}" +
		".tool{font-size:.8rem;color:#7a4d00;margin-bottom:.3rem} footer{margin-top:2rem;font-size:.75rem;color:#888}</style>" +
		"</head><body><h1>" + html.EscapeString(session.Title) + "</h1>")
	counts := map[domain.Role]int{}
	for _, m := range messages {
		counts[m.Role]++
		switch {
		case m.Role == domain.RoleTool:
			b.WriteString("<div class=\"m t\"><div class=\"tool\">tool result: " + html.EscapeString(m.ToolName) + "</div><pre>" +
				html.EscapeString(m.Content) + "</pre></div>")
		case m.ToolName != "":
			b.WriteString("<div class=\"m a\"><div class=\"tool\">tool call: " + html.EscapeString(m.ToolName) + "</div><pre>" +
				html.EscapeString(string(m.ToolArgs)) + "</pre></div>")
		default:
			class := "a"
			if m.Role == domain.RoleUser {
				class = "u"
			}
			b.WriteString("<div class=\"m " + class + "\"><div class=\"role\">" + html.EscapeString(string(m.Role)) + "</div><pre>" +
				html.EscapeString(m.Content) + "</pre></div>")
		}
	}
	fmt.Fprintf(&b, "<footer>session %s · %d messages (%d user, %d assistant, %d tool) · exported %s</footer>",
		html.EscapeString(string(session.ID)), len(messages),
		counts[domain.RoleUser], counts[domain.RoleAssistant], counts[domain.RoleTool],
		html.EscapeString(time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("</body></html>")
	return b.String()
}

// sanitizeExportName keeps export file names path-safe.
func sanitizeExportName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}
