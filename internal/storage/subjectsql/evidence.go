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

// evidence validates the existing Journal against its deterministic message
// projection, then expands owned exact tool artifacts. The index stores only
// identities and the digest of these canonical originals.
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
	byID := make(map[string]int, len(messages))
	for i, m := range messages {
		byID[m.ID] = i
	}
	var admitted sql.NullString
	if err := s.one(ctx, tx, `SELECT admitted_message_id FROM run_subject_sources WHERE run_id=?`, []any{in.RunID}, &admitted); err != nil {
		return nil, "", err
	}
	if admitted.Valid {
		i, ok := byID[admitted.String]
		if !ok || messages[i].Role != domain.RoleUser {
			return nil, "", storage.ErrSubjectEvidenceIncomplete
		}
		delete(byID, admitted.String)
	}
	rows, err := tx.QueryContext(ctx, s.query(`SELECT seq,type,created_at,payload_version,payload FROM run_events WHERE run_id=? ORDER BY seq`), in.RunID)
	if err != nil {
		return nil, "", err
	}
	var events []domain.RunEvent
	for rows.Next() {
		var e domain.RunEvent
		if err := rows.Scan(&e.Seq, &e.Type, &e.CreatedAt, &e.PayloadVersion, &e.Payload); err != nil {
			rows.Close()
			return nil, "", err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, "", err
	}
	var pending strings.Builder
	var provenance *storage.InputProvenance
	var last domain.EventSeq
	terminal := ""
	started := false
	calls := map[string]bool{}
	type completedOperation struct {
		result string
		seq    domain.EventSeq
	}
	completedOps := map[string]completedOperation{}
	consume := func(id string, role domain.Role, content string) (int, error) {
		i, ok := byID[id]
		if !ok || messages[i].Role != role || messages[i].Content != content {
			return 0, storage.ErrSubjectEvidenceIncomplete
		}
		if role == domain.RoleAssistant {
			if provenance != nil {
				if messages[i].ContentOrigin != provenance.ContentOrigin || messages[i].ExcludeAutomaticIngest != provenance.ExcludeAutomaticIngest {
					return 0, storage.ErrSubjectEvidenceIncomplete
				}
			} else if messages[i].ContentOrigin != "" || messages[i].ExcludeAutomaticIngest {
				return 0, storage.ErrSubjectEvidenceIncomplete
			}
		}
		delete(byID, id)
		return i, nil
	}
	flush := func(e domain.RunEvent, partial bool) error {
		if pending.Len() == 0 && partial {
			return nil
		}
		id := fmt.Sprintf("msgp_%s_%020d_0", in.RunID, e.Seq)
		i, err := consume(id, domain.RoleAssistant, pending.String())
		if err != nil {
			return err
		}
		if partial && messages[i].Source != "journal_partial" {
			return storage.ErrSubjectEvidenceIncomplete
		}
		pending.Reset()
		return nil
	}
	for _, e := range events {
		if e.Seq != last+1 || terminal != "" {
			return nil, "", storage.ErrSubjectEvidenceIncomplete
		}
		last = e.Seq
		switch e.Type {
		case domain.EventRunStarted:
			var p struct {
				NoContext bool `json:"no_context"`
			}
			if e.Seq != 1 || json.Unmarshal(e.Payload, &p) != nil || p.NoContext {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			started = true
			if !admitted.Valid {
				a := storage.RunAdmission{Subject: &source, OmitMessage: true, Run: domain.Run{ID: in.RunID, SessionID: in.SessionID, Kind: domain.RunKindPrimary}, Started: domain.RunEvent{Payload: e.Payload}}
				if !storage.IsSubjectShellAdmission(a) {
					return nil, "", storage.ErrSubjectEvidenceIncomplete
				}
			}
		case domain.EventModelRequest:
			var p struct {
				Source string `json:"source"`
			}
			if json.Unmarshal(e.Payload, &p) != nil {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			if p.Source == "maintenance" {
				continue
			}
			if pending.Len() != 0 {
				if err := flush(e, true); err != nil {
					return nil, "", err
				}
			}
			if e.PayloadVersion >= 4 {
				p, err := storage.DecodeModelInputProvenance(e)
				if err != nil {
					return nil, "", err
				}
				provenance = &p
			} else {
				provenance = nil
			}
		case domain.EventModelDelta:
			var p struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(e.Payload, &p) != nil {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			pending.WriteString(p.Delta)
		case domain.EventModelCompleted:
			var p struct {
				ContentSHA256 string `json:"content_sha256"`
				ByteLen       int    `json:"byte_len"`
			}
			var fields map[string]json.RawMessage
			if e.PayloadVersion != 2 || json.Unmarshal(e.Payload, &p) != nil || json.Unmarshal(e.Payload, &fields) != nil || len(fields) != 2 {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			sum := sha256.Sum256([]byte(pending.String()))
			if p.ByteLen != pending.Len() || p.ContentSHA256 != hex.EncodeToString(sum[:]) {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			if err := flush(e, false); err != nil {
				return nil, "", err
			}
		case domain.EventToolRequested:
			if pending.Len() != 0 {
				if err := flush(e, false); err != nil {
					return nil, "", err
				}
			}
			var p struct {
				ToolCallID                   string                          `json:"tool_call_id"`
				ToolName                     string                          `json:"tool_name"`
				Args                         map[string]any                  `json:"args"`
				OriginalArgumentsBytes       []byte                          `json:"original_arguments_bytes"`
				OriginalArgumentsInvalidUTF8 bool                            `json:"original_arguments_invalid_utf8"`
				OriginalArguments            string                          `json:"original_arguments"`
				ArgumentsReference           *storage.ToolArgumentsReference `json:"arguments_reference"`
			}
			if json.Unmarshal(e.Payload, &p) != nil {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			if e.PayloadVersion >= 2 || p.OriginalArgumentsInvalidUTF8 || len(p.OriginalArgumentsBytes) > 0 {
				// The received bytes cannot become an exact original; this never heals.
				return nil, "", storage.ErrSubjectEvidenceUnresolvable
			}
			if p.ToolCallID == "" || calls[p.ToolCallID] {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			id := fmt.Sprintf("msgp_%s_%020d_1", in.RunID, e.Seq)
			i, err := consume(id, domain.RoleAssistant, "")
			if err != nil {
				return nil, "", err
			}
			args, _ := json.Marshal(p.Args)
			if messages[i].ToolCallID != p.ToolCallID || messages[i].ToolName != p.ToolName || string(messages[i].ToolArgs) != string(args) {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			if p.OriginalArguments != "" {
				if !json.Valid([]byte(p.OriginalArguments)) {
					return nil, "", storage.ErrSubjectEvidenceIncomplete
				}
				messages[i].ToolArgs = []byte(p.OriginalArguments)
			}
			if p.ArgumentsReference != nil {
				op := domain.ToolOperation{RunID: in.RunID, OperationID: p.ToolCallID, RequestDigest: p.ArgumentsReference.SHA256}
				raw, err := s.blob(ctx, tx, p.ArgumentsReference.ID)
				if err != nil {
					return nil, "", err
				}
				if err := storage.VerifyToolArgumentsReference(op, *p.ArgumentsReference, raw); err != nil {
					return nil, "", err
				}
				messages[i].ToolArgs = raw
			}
			calls[p.ToolCallID] = true
		case domain.EventToolOperation:
			var p struct {
				OperationID string `json:"operation_id"`
				State       string `json:"state"`
				Result      string `json:"result"`
			}
			if json.Unmarshal(e.Payload, &p) != nil {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			if p.State == string(domain.ToolOperationCompleted) {
				completedOps[p.OperationID] = completedOperation{result: p.Result, seq: e.Seq}
			}
		case domain.EventToolFinished:
			var p struct {
				ToolCallID string `json:"tool_call_id"`
				ToolName   string `json:"tool_name"`
				Result     string `json:"result"`
				Error      string `json:"error"`
			}
			if json.Unmarshal(e.Payload, &p) != nil || !calls[p.ToolCallID] {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			content := p.Result
			if p.Error != "" {
				content = p.Error
			}
			id := fmt.Sprintf("msgp_%s_%020d_0", in.RunID, e.Seq)
			i, err := consume(id, domain.RoleTool, content)
			if err != nil {
				return nil, "", err
			}
			if messages[i].ToolCallID != p.ToolCallID || messages[i].ToolName != p.ToolName {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			op, found, err := s.toolOperation(ctx, tx, in.RunID, p.ToolCallID)
			if err != nil {
				return nil, "", err
			}
			if found {
				if op.State != domain.ToolOperationCompleted || op.ToolName != p.ToolName || completedOps[op.OperationID].result != op.Result {
					return nil, "", storage.ErrSubjectEvidenceIncomplete
				}
				if err := s.expandToolMessage(ctx, tx, messages, i, op); err != nil {
					return nil, "", err
				}
				delete(completedOps, p.ToolCallID)
			} else if p.Error == "" {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			delete(calls, p.ToolCallID)
		case domain.EventTurnSteered:
			var p struct {
				QueueID string `json:"queue_id"`
				Text    string `json:"text"`
			}
			if json.Unmarshal(e.Payload, &p) != nil {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			if _, err := consume("steer-"+p.QueueID, domain.RoleUser, p.Text); err != nil {
				return nil, "", err
			}
		case domain.EventRunCompleted, domain.EventRunFailed, domain.EventRunCancelled:
			terminal = string(e.Type)
			expected := map[domain.EventType]domain.RunStatus{domain.EventRunCompleted: domain.RunCompleted, domain.EventRunFailed: domain.RunFailed, domain.EventRunCancelled: domain.RunCancelled}
			if e.Seq != in.TerminalSeq || expected[e.Type] != domain.RunStatus(status) {
				return nil, "", storage.ErrSubjectEvidenceIncomplete
			}
			for call, completed := range completedOps {
				op, found, err := s.toolOperation(ctx, tx, in.RunID, call)
				if err != nil {
					return nil, "", err
				}
				if !found || op.State != domain.ToolOperationCompleted || op.Result != completed.result {
					return nil, "", storage.ErrSubjectEvidenceIncomplete
				}
				ref, err := storage.DecodeToolEvidenceReference(op)
				if err != nil {
					return nil, "", err
				}
				content := ref.Projection
				if op.Failure != "" {
					content = op.Failure
				}
				id := fmt.Sprintf("msgp_%s_%020d_0", in.RunID, completed.seq)
				i, err := consume(id, domain.RoleTool, content)
				if err != nil {
					return nil, "", err
				}
				if messages[i].Source != "journal_partial" || messages[i].ToolCallID != call || !calls[call] {
					return nil, "", storage.ErrSubjectEvidenceIncomplete
				}
				if err := s.expandToolMessage(ctx, tx, messages, i, op); err != nil {
					return nil, "", err
				}
				delete(calls, call)
			}
			if pending.Len() != 0 {
				if e.Type == domain.EventRunCompleted {
					return nil, "", storage.ErrSubjectEvidenceIncomplete
				}
				if err := flush(e, true); err != nil {
					return nil, "", err
				}
			}
		}
	}
	if !started || terminal == "" || last != in.TerminalSeq || len(byID) != 0 || (len(calls) != 0 && domain.RunStatus(status) == domain.RunCompleted) {
		return nil, "", storage.ErrSubjectEvidenceIncomplete
	}
	var unresolved int
	if err := s.one(ctx, tx, `SELECT COUNT(*) FROM tool_operations WHERE run_id=? AND state<>?`, []any{in.RunID, domain.ToolOperationCompleted}, &unresolved); err != nil {
		return nil, "", err
	}
	if unresolved > 0 {
		// Terminal runs cannot complete a claimed operation, so this never heals.
		return nil, "", storage.ErrSubjectEvidenceUnresolvable
	}
	return messages, terminal, nil
}

func (s Store) toolOperation(ctx context.Context, tx *sql.Tx, run domain.RunID, call string) (domain.ToolOperation, bool, error) {
	op := domain.ToolOperation{RunID: run, OperationID: call}
	err := s.one(ctx, tx, `SELECT tool_name,request_digest,state,result,failure,content_origin,exclude_automatic_ingest FROM tool_operations WHERE run_id=? AND operation_id=?`, []any{run, call}, &op.ToolName, &op.RequestDigest, &op.State, &op.Result, &op.Failure, &op.ContentOrigin, &op.ExcludeAutomaticIngest)
	if err == sql.ErrNoRows {
		return op, false, nil
	}
	return op, err == nil, err
}
func (s Store) blob(ctx context.Context, tx *sql.Tx, id string) ([]byte, error) {
	var raw []byte
	err := s.one(ctx, tx, `SELECT g.blob FROM checkpoints c JOIN checkpoint_generations g ON g.id=c.id AND g.generation=c.generation WHERE c.id=?`, []any{id}, &raw)
	if err == sql.ErrNoRows {
		return nil, storage.ErrSubjectEvidenceIncomplete
	}
	return raw, err
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

func (s Store) expandToolMessage(ctx context.Context, tx *sql.Tx, messages []domain.Message, i int, op domain.ToolOperation) error {
	if messages[i].ContentOrigin != op.ContentOrigin || messages[i].ExcludeAutomaticIngest != op.ExcludeAutomaticIngest {
		return storage.ErrSubjectEvidenceIncomplete
	}
	ref, err := storage.DecodeToolEvidenceReference(op)
	if err != nil {
		return err
	}
	raw, err := s.blob(ctx, tx, ref.ID)
	if err != nil {
		return err
	}
	full, err := storage.VerifyToolEvidence(ref, raw)
	if err != nil {
		return err
	}
	if op.Failure != full.ModelFailure {
		return storage.ErrSubjectEvidenceIncomplete
	}
	args, err := s.blob(ctx, tx, storage.ToolEvidenceArgumentsID(op))
	if err != nil {
		return err
	}
	if err := storage.VerifyToolEvidenceArguments(op, args); err != nil {
		return err
	}
	for j := range messages {
		if messages[j].Role == domain.RoleAssistant && messages[j].ToolCallID == op.OperationID {
			messages[j].ToolArgs = append([]byte(nil), args...)
		}
	}
	messages[i].Content = full.Result
	messages[i].ToolError = full.Failure
	if full.Failure != "" && messages[i].Source != "journal_partial" {
		messages[i].Source = "tool_error"
	}
	return nil
}
