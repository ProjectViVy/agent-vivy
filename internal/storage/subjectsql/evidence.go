package subjectsql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// evidence refuses to turn a shortened projection into an original. Tool and
// interrupted-delta repair is the subsequent lossless-evidence task: until that
// owner exists these runs remain retained but explicitly unsealed.
func (s Store) evidence(ctx context.Context, tx *sql.Tx, b storage.SubjectHistoryBinding, in storage.SubjectInteraction) ([]domain.Message, string, error) {
	if in.RunID == "" || in.SessionID == "" || in.TerminalSeq <= 0 || in.ID != storage.SubjectInteractionID(in.RunID, in.TerminalSeq) {
		return nil, "", storage.ErrSubjectEvidenceIncomplete
	}
	source, err := s.runSource(ctx, tx, in.RunID)
	if err != nil {
		return nil, "", err
	}
	if source.SubjectHistoryBinding != b || source.SessionID != in.SessionID {
		return nil, "", storage.ErrSubjectBinding
	}
	var status, kind, purpose, session string
	var depth int
	if err := s.one(ctx, tx, `SELECT session_id,status,kind,purpose,depth FROM runs WHERE id=?`, []any{in.RunID}, &session, &status, &kind, &purpose, &depth); err != nil {
		return nil, "", err
	}
	if session != string(in.SessionID) || (kind != "" && kind != string(domain.RunKindPrimary)) || purpose != "" || depth != 0 || !domain.RunStatus(status).Terminal() {
		return nil, "", storage.ErrSubjectEvidenceIncomplete
	}
	messages, err := s.messages(ctx, tx, in)
	if err != nil {
		return nil, "", err
	}
	if len(messages) == 0 || messages[0].Role != domain.RoleUser {
		return nil, "", storage.ErrSubjectEvidenceIncomplete
	}
	byID := make(map[string]domain.Message, len(messages))
	for _, m := range messages {
		if m.ExcludeAutomaticIngest || m.ContentOrigin != "" || m.ToolCallID != "" || m.Role == domain.RoleTool {
			return nil, "", storage.ErrSubjectEvidenceIncomplete
		}
		byID[m.ID] = m
	}
	var admitted string
	if err := s.one(ctx, tx, `SELECT admitted_message_id FROM run_subject_sources WHERE run_id=?`, []any{in.RunID}, &admitted); err != nil {
		return nil, "", err
	}
	if messages[0].ID != admitted {
		return nil, "", storage.ErrSubjectEvidenceIncomplete
	}
	delete(byID, admitted)
	rows, err := tx.QueryContext(ctx, s.query(`SELECT seq,type,payload_version,payload FROM run_events WHERE run_id=? ORDER BY seq`), in.RunID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var pending strings.Builder
	var last domain.EventSeq
	var terminal string
	started := false
	for rows.Next() {
		var seq domain.EventSeq
		var typ string
		var version int
		var raw []byte
		if err := rows.Scan(&seq, &typ, &version, &raw); err != nil {
			return nil, "", err
		}
		if seq != last+1 || terminal != "" {
			return nil, "", storage.ErrSubjectEvidenceIncomplete
		}
		last = seq
		switch domain.EventType(typ) {
		case domain.EventRunStarted:
			var p struct {
				NoContext bool `json:"no_context"`
			}
			if seq != 1 || json.Unmarshal(raw, &p) != nil || p.NoContext {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			started = true
		case domain.EventModelRequest:
			if pending.Len() != 0 {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
		case domain.EventModelDelta:
			var p struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(raw, &p) != nil {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			pending.WriteString(p.Delta)
		case domain.EventModelCompleted:
			var p struct {
				ContentSHA256 string `json:"content_sha256"`
				ByteLen       int    `json:"byte_len"`
			}
			var fields map[string]json.RawMessage
			if version != 2 || json.Unmarshal(raw, &p) != nil || json.Unmarshal(raw, &fields) != nil || len(fields) != 2 {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			sum := sha256.Sum256([]byte(pending.String()))
			if p.ByteLen != pending.Len() || p.ContentSHA256 != hex.EncodeToString(sum[:]) {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			id := fmt.Sprintf("msgp_%s_%020d_0", in.RunID, seq)
			m, ok := byID[id]
			if !ok || m.Role != domain.RoleAssistant || m.Content != pending.String() {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			delete(byID, id)
			pending.Reset()
		case domain.EventToolRequested, domain.EventToolFinished, domain.EventToolOperation, domain.EventTurnSteered:
			return nil, "", storage.ErrSubjectEvidenceIncomplete
		case domain.EventRunCompleted, domain.EventRunFailed, domain.EventRunCancelled:
			terminal = typ
			expected := map[domain.EventType]domain.RunStatus{domain.EventRunCompleted: domain.RunCompleted, domain.EventRunFailed: domain.RunFailed, domain.EventRunCancelled: domain.RunCancelled}
			if seq != in.TerminalSeq || expected[domain.EventType(typ)] != domain.RunStatus(status) {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if !started || terminal == "" || last != in.TerminalSeq || pending.Len() != 0 || len(byID) != 0 {
		return nil, "", storage.ErrSubjectEvidenceIncomplete
	}
	var excluded int
	if err := s.one(ctx, tx, `SELECT COUNT(*) FROM tool_operations WHERE run_id=? AND exclude_automatic_ingest=TRUE`, []any{in.RunID}, &excluded); err != nil {
		return nil, "", err
	}
	if excluded > 0 {
		return nil, "", storage.ErrSubjectEvidenceIncomplete
	}
	return messages, terminal, nil
}
func (s Store) messages(ctx context.Context, tx *sql.Tx, in storage.SubjectInteraction) ([]domain.Message, error) {
	rows, err := tx.QueryContext(ctx, s.query(`SELECT id,session_id,run_id,role,created_at,work_seq,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id,content_origin,exclude_automatic_ingest FROM messages WHERE session_id=? AND run_id=? ORDER BY position`), in.SessionID, in.RunID)
	if err != nil {
		return nil, err
	}
	var out []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.RunID, &m.Role, &m.CreatedAt, &m.WorkSeq, &m.Content, &m.ToolCallID, &m.ToolName, &m.ToolArgs, &m.Source, &m.Channel, &m.ChatID, &m.ChannelMessageID, &m.ContentOrigin, &m.ExcludeAutomaticIngest); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		rows, err := tx.QueryContext(ctx, s.query(`SELECT name,mime_type,data FROM message_attachments WHERE message_id=? ORDER BY position`), out[i].ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var a domain.Attachment
			if err := rows.Scan(&a.Name, &a.MimeType, &a.Data); err != nil {
				rows.Close()
				return nil, err
			}
			out[i].Attachments = append(out[i].Attachments, a)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		rows, err = tx.QueryContext(ctx, s.query(`SELECT path,name,size,content FROM message_file_contexts WHERE message_id=? ORDER BY position`), out[i].ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var f domain.FileContext
			if err := rows.Scan(&f.Path, &f.Name, &f.Size, &f.Content); err != nil {
				rows.Close()
				return nil, err
			}
			out[i].FileContexts = append(out[i].FileContexts, f)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
