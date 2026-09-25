package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

	"agent-vivy/internal/domain"
	mask "agent-vivy/internal/maskcontract"
	"agent-vivy/internal/storage"
)

var _ storage.RunAdmissionStore = (*Backend)(nil)

// CommitRunAdmission writes the user turn, optional edit marker, run,
// immutable prompt envelope, and run.started event as one transaction. The
// session writer lock is acquired before reading the selection so a catalog
// switch cannot race the capture token.
func (b *Backend) CommitRunAdmission(ctx context.Context, in storage.RunAdmission) (domain.RunEvent, error) {
	if err := storage.ValidateRunAdmissionInput(in); err != nil {
		return in.Started, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return in.Started, storage.AdmissionUnavailable("begin run admission", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := sqliteLockAdmissionSession(ctx, tx, in.Run.SessionID); err != nil {
		return in.Started, err
	}
	if existing, found, err := sqliteReadAdmissionRun(ctx, tx, in.Run.ID); err != nil {
		return in.Started, err
	} else if found {
		if err := sqliteCompareAdmission(ctx, tx, in, existing); err != nil {
			return in.Started, err
		}
		if err := tx.Commit(); err != nil {
			return in.Started, storage.AdmissionUnavailable("commit idempotent run admission", err)
		}
		return existing.started, nil
	}

	if err := sqliteValidateAdmissionCapture(ctx, tx, in.ExpectedMask); err != nil {
		return in.Started, err
	}
	if exists, err := sqliteMessageExists(ctx, tx, in.Message.ID); err != nil {
		return in.Started, err
	} else if exists {
		return in.Started, storage.AdmissionConflict()
	}
	in.Message.WorkSeq, err = currentMessageWorkSeq(ctx, tx, in.Message.SessionID)
	if err != nil {
		return in.Started, err
	}
	if in.Edit != nil {
		marker := *in.Edit
		if marker.RunID == "" {
			marker.RunID = in.Run.ID
		}
		if err := sqliteInsertMarker(ctx, tx, marker); err != nil {
			return in.Started, storage.AdmissionUnavailable("insert admission edit marker", err)
		}
	}
	if err := sqliteInsertAdmissionMessage(ctx, tx, in.Message); err != nil {
		return in.Started, err
	}
	if err := sqliteInsertAdmissionRun(ctx, tx, in.Run); err != nil {
		return in.Started, err
	}
	if in.Prompt != nil {
		if err := sqliteInsertAdmissionPrompt(ctx, tx, *in.Prompt); err != nil {
			return in.Started, err
		}
	}
	started := in.Started
	started.Seq = 1
	if err := sqliteInsertHistoryEvent(ctx, tx, &started); err != nil {
		return in.Started, storage.AdmissionUnavailable("insert admission start event", err)
	}
	if err := tx.Commit(); err != nil {
		return in.Started, storage.AdmissionUnavailable("commit run admission", err)
	}
	return started, nil
}

// LoadRunPrompt reads and verifies the immutable envelope before exposing it
// to Runtime. The database row is never treated as trusted JSON merely
// because it was written by an earlier process.
func (b *Backend) LoadRunPrompt(ctx context.Context, runID domain.RunID) (storage.RunPromptSnapshot, error) {
	if runID == "" {
		return storage.RunPromptSnapshot{}, mask.NewError(mask.CodeInvalidMask, errors.New("run id is required"))
	}
	var snapshot storage.RunPromptSnapshot
	var payload []byte
	err := b.db.QueryRowContext(ctx, `
		SELECT run_id, schema_version, composer_version, generation_id, payload, payload_sha256
		FROM run_prompt_snapshots WHERE run_id = ?`, runID).
		Scan((*string)(&snapshot.RunID), &snapshot.SchemaVersion, &snapshot.ComposerVersion,
			&snapshot.GenerationID, &payload, &snapshot.PayloadSHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.RunPromptSnapshot{}, mask.NewError(mask.CodeSnapshotMissing, storage.ErrNotFound)
	}
	if err != nil {
		return storage.RunPromptSnapshot{}, storage.AdmissionUnavailable("load run prompt", err)
	}
	snapshot.Payload = append([]byte(nil), payload...)
	if _, err := storage.ValidateRunPromptSnapshot(snapshot); err != nil {
		return storage.RunPromptSnapshot{}, err
	}
	return snapshot, nil
}

func sqliteLockAdmissionSession(ctx context.Context, tx *sql.Tx, sessionID domain.SessionID) error {
	// SQLite serializes writers. A no-op update obtains that writer lock before
	// the selection and definition reads below.
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET id = id WHERE id = ?`, sessionID); err != nil {
		return storage.AdmissionUnavailable("lock admission session", err)
	}
	var lockedID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id = ?`, sessionID).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return mask.NewError(mask.CodeNotFound, storage.ErrNotFound)
	} else if err != nil {
		return storage.AdmissionUnavailable("read admission session", err)
	}
	return nil
}

func sqliteValidateAdmissionCapture(ctx context.Context, tx *sql.Tx, expected *storage.MaskCaptureCheck) error {
	if expected == nil {
		return nil
	}
	selection, err := sqliteReadSelection(ctx, tx, expected.SessionID)
	if err != nil {
		return err
	}
	if selection.Revision != expected.SelectionRevision || selection.MaskID != expected.MaskID {
		return mask.NewRevisionError(mask.CodeRevisionConflict, selection.Revision, nil)
	}
	if expected.MaskID == "" || mask.IsBuiltinID(expected.MaskID) {
		return nil
	}
	definition, err := sqliteGetCustomMaskForUpdate(ctx, tx, expected.MaskID)
	if err != nil {
		return err
	}
	if definition.Revision != expected.DefinitionRevision || definition.Digest != expected.DefinitionDigest {
		return mask.NewRevisionError(mask.CodeRevisionConflict, definition.Revision, nil)
	}
	return nil
}

func sqliteMessageExists(ctx context.Context, tx *sql.Tx, id string) (bool, error) {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE id = ?`, id).Scan(&exists); err != nil {
		return false, storage.AdmissionUnavailable("check admission message id", err)
	}
	return exists > 0, nil
}

func sqliteInsertAdmissionMessage(ctx context.Context, tx *sql.Tx, m domain.Message) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages
			(id,session_id,run_id,role,created_at,work_seq,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.SessionID, m.RunID, string(m.Role), m.CreatedAt, int64(m.WorkSeq), m.Content,
		m.ToolCallID, m.ToolName, toolArgsBlob(m.ToolArgs), m.Source, m.Channel,
		m.ChatID, m.ChannelMessageID); err != nil {
		return storage.AdmissionUnavailable("insert admission message", err)
	}
	for position, attachment := range m.Attachments {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO message_attachments (message_id,position,name,mime_type,data)
			VALUES (?,?,?,?,?)`, m.ID, position, attachment.Name, attachment.MimeType, attachment.Data); err != nil {
			return storage.AdmissionUnavailable("insert admission attachment", err)
		}
	}
	for position, file := range m.FileContexts {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO message_file_contexts (message_id,position,path,name,size,content)
			VALUES (?,?,?,?,?,?)`, m.ID, position, file.Path, file.Name, file.Size, file.Content); err != nil {
			return storage.AdmissionUnavailable("insert admission file context", err)
		}
	}
	at := messageActivityAt(m.CreatedAt)
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions SET updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END
		WHERE id = ?`, at, at, m.SessionID); err != nil {
		return storage.AdmissionUnavailable("touch admission session", err)
	}
	return nil
}

func sqliteInsertAdmissionRun(ctx context.Context, tx *sql.Tx, r domain.Run) error {
	kind := r.Kind
	if kind == "" {
		kind = domain.RunKindPrimary
	}
	rootID := r.RootID
	if rootID == "" {
		rootID = r.ID
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO runs (id,session_id,status,created_at,kind,parent_run_id,root_run_id,depth)
		VALUES (?,?,?,?,?,?,?,?)`, r.ID, r.SessionID, string(domain.RunActive), r.CreatedAt,
		string(kind), r.ParentID, rootID, r.Depth); err != nil {
		return storage.AdmissionUnavailable("insert admission run", err)
	}
	return nil
}

func sqliteInsertAdmissionPrompt(ctx context.Context, tx *sql.Tx, prompt storage.RunPromptSnapshot) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO run_prompt_snapshots
			(run_id,schema_version,composer_version,generation_id,payload,payload_sha256)
		VALUES (?,?,?,?,?,?)`, prompt.RunID, prompt.SchemaVersion, prompt.ComposerVersion,
		prompt.GenerationID, prompt.Payload, prompt.PayloadSHA256); err != nil {
		return storage.AdmissionUnavailable("insert admission prompt", err)
	}
	return nil
}

type sqliteAdmissionRun struct {
	run     domain.Run
	started domain.RunEvent
	prompt  *storage.RunPromptSnapshot
	edit    *storage.SessionTruncation
}

func sqliteReadAdmissionRun(ctx context.Context, tx *sql.Tx, runID domain.RunID) (sqliteAdmissionRun, bool, error) {
	var existing sqliteAdmissionRun
	var rid, sid, status, kind, parentID, rootID string
	err := tx.QueryRowContext(ctx, `
		SELECT id,session_id,status,created_at,kind,parent_run_id,root_run_id,depth
		FROM runs WHERE id = ?`, runID).
		Scan(&rid, &sid, &status, &existing.run.CreatedAt, &kind, &parentID, &rootID, &existing.run.Depth)
	if errors.Is(err, sql.ErrNoRows) {
		return sqliteAdmissionRun{}, false, nil
	}
	if err != nil {
		return sqliteAdmissionRun{}, false, storage.AdmissionUnavailable("read existing admission run", err)
	}
	existing.run.ID = domain.RunID(rid)
	existing.run.SessionID = domain.SessionID(sid)
	existing.run.Status = domain.RunStatus(status)
	existing.run.Kind = domain.RunKind(kind)
	existing.run.ParentID = domain.RunID(parentID)
	existing.run.RootID = domain.RunID(rootID)

	var seq int64
	var eventType string
	err = tx.QueryRowContext(ctx, `
		SELECT seq,type,created_at,payload_version,payload
		FROM run_events WHERE run_id = ? AND type = ? ORDER BY seq LIMIT 1`, runID, domain.EventRunStarted).
		Scan(&seq, &eventType, &existing.started.CreatedAt, &existing.started.PayloadVersion, &existing.started.Payload)
	if errors.Is(err, sql.ErrNoRows) {
		return sqliteAdmissionRun{}, false, storage.AdmissionConflict()
	}
	if err != nil {
		return sqliteAdmissionRun{}, false, storage.AdmissionUnavailable("read existing admission start", err)
	}
	existing.started.RunID = runID
	existing.started.Seq = domain.EventSeq(seq)
	existing.started.Type = domain.EventType(eventType)

	var marker storage.SessionTruncation
	var markerSessionID, markerRunID, markerForkSessionID string
	err = tx.QueryRowContext(ctx, `
		SELECT session_id, run_id, cutoff_message_id, tail_message_id, work_seq, reason, fork_session_id, created_at
		FROM session_truncations WHERE run_id = ? AND reason = ? ORDER BY id LIMIT 1`, runID, storage.TruncationEdit).
		Scan(&markerSessionID, &markerRunID, &marker.CutoffMessageID, &marker.TailMessageID, &marker.WorkSeq, &marker.Reason, &markerForkSessionID, &marker.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// No admission edit marker is normal for an ordinary run. Continue
		// reading the optional prompt snapshot below.
	} else if err != nil {
		return sqliteAdmissionRun{}, false, storage.AdmissionUnavailable("read existing admission edit marker", err)
	} else {
		marker.SessionID = domain.SessionID(markerSessionID)
		marker.RunID = domain.RunID(markerRunID)
		marker.ForkSessionID = markerForkSessionID
		existing.edit = &marker
	}

	var prompt storage.RunPromptSnapshot
	var payload []byte
	err = tx.QueryRowContext(ctx, `
		SELECT run_id,schema_version,composer_version,generation_id,payload,payload_sha256
		FROM run_prompt_snapshots WHERE run_id = ?`, runID).
		Scan((*string)(&prompt.RunID), &prompt.SchemaVersion, &prompt.ComposerVersion,
			&prompt.GenerationID, &payload, &prompt.PayloadSHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return existing, true, nil
	}
	if err != nil {
		return sqliteAdmissionRun{}, false, storage.AdmissionUnavailable("read existing admission prompt", err)
	}
	prompt.Payload = append([]byte(nil), payload...)
	if _, err := storage.ValidateRunPromptSnapshot(prompt); err != nil {
		return sqliteAdmissionRun{}, false, err
	}
	existing.prompt = &prompt
	return existing, true, nil
}

func sqliteCompareAdmission(ctx context.Context, tx *sql.Tx, in storage.RunAdmission, existing sqliteAdmissionRun) error {
	wantRun := in.Run
	if wantRun.Kind == "" {
		wantRun.Kind = domain.RunKindPrimary
	}
	if wantRun.RootID == "" {
		wantRun.RootID = wantRun.ID
	}
	if existing.run.SessionID != wantRun.SessionID || existing.run.CreatedAt != wantRun.CreatedAt ||
		existing.run.Kind != wantRun.Kind || existing.run.ParentID != wantRun.ParentID ||
		existing.run.RootID != wantRun.RootID || existing.run.Depth != wantRun.Depth {
		return storage.AdmissionConflict()
	}
	existingMessage, err := sqliteLoadAdmissionMessage(ctx, tx, in.Message.ID)
	if err != nil {
		return err
	}
	if !sqliteSameAdmissionMessage(existingMessage, in.Message) {
		return storage.AdmissionConflict()
	}
	wantStarted := in.Started
	wantStarted.Seq = 1
	if !sqliteSameAdmissionEvent(existing.started, wantStarted) {
		return storage.AdmissionConflict()
	}
	if (in.Prompt == nil) != (existing.prompt == nil) {
		return storage.AdmissionConflict()
	}
	if in.Prompt != nil && !sqliteSamePrompt(*existing.prompt, *in.Prompt) {
		return storage.AdmissionConflict()
	}
	existingEdit := existing.edit
	if existingEdit == nil && in.Edit != nil {
		// Rows written before the run_id migration can still be matched by
		// their complete marker identity. This preserves safe retries while
		// keeping the new run-linked path symmetric.
		found, err := sqliteAdmissionMarkerExists(ctx, tx, *in.Edit)
		if err != nil {
			return err
		}
		if found {
			existingEdit = in.Edit
		}
	}
	if (in.Edit == nil) != (existingEdit == nil) {
		return storage.AdmissionConflict()
	}
	if in.Edit != nil && existingEdit != nil && !sqliteSameAdmissionMarker(*existingEdit, *in.Edit) {
		return storage.AdmissionConflict()
	}
	return nil
}

type sqliteAdmissionQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func sqliteLoadAdmissionMessage(ctx context.Context, q sqliteAdmissionQueryer, id string) (domain.Message, error) {
	var m domain.Message
	var sid, rid, role string
	var args []byte
	err := q.QueryRowContext(ctx, `
		SELECT id,session_id,run_id,role,created_at,content,tool_call_id,tool_name,tool_args,source,channel,chat_id,channel_message_id
		FROM messages WHERE id = ?`, id).
		Scan(&m.ID, &sid, &rid, &role, &m.CreatedAt, &m.Content, &m.ToolCallID, &m.ToolName,
			&args, &m.Source, &m.Channel, &m.ChatID, &m.ChannelMessageID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Message{}, storage.AdmissionConflict()
	}
	if err != nil {
		return domain.Message{}, storage.AdmissionUnavailable("read existing admission message", err)
	}
	m.SessionID, m.RunID, m.Role, m.ToolArgs = domain.SessionID(sid), domain.RunID(rid), domain.Role(role), args

	rows, err := q.QueryContext(ctx, `
		SELECT name,mime_type,data FROM message_attachments WHERE message_id = ? ORDER BY position`, id)
	if err != nil {
		return domain.Message{}, storage.AdmissionUnavailable("read existing admission attachments", err)
	}
	for rows.Next() {
		var attachment domain.Attachment
		if err := rows.Scan(&attachment.Name, &attachment.MimeType, &attachment.Data); err != nil {
			_ = rows.Close()
			return domain.Message{}, storage.AdmissionUnavailable("scan existing admission attachment", err)
		}
		m.Attachments = append(m.Attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return domain.Message{}, storage.AdmissionUnavailable("iterate existing admission attachments", err)
	}
	_ = rows.Close()

	rows, err = q.QueryContext(ctx, `
		SELECT path,name,size,content FROM message_file_contexts WHERE message_id = ? ORDER BY position`, id)
	if err != nil {
		return domain.Message{}, storage.AdmissionUnavailable("read existing admission file contexts", err)
	}
	for rows.Next() {
		var file domain.FileContext
		if err := rows.Scan(&file.Path, &file.Name, &file.Size, &file.Content); err != nil {
			_ = rows.Close()
			return domain.Message{}, storage.AdmissionUnavailable("scan existing admission file context", err)
		}
		m.FileContexts = append(m.FileContexts, file)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return domain.Message{}, storage.AdmissionUnavailable("iterate existing admission file contexts", err)
	}
	_ = rows.Close()
	return m, nil
}

func sqliteSameAdmissionMessage(a, b domain.Message) bool {
	if !storage.SameProjectedMessage(a, b) || len(a.Attachments) != len(b.Attachments) || len(a.FileContexts) != len(b.FileContexts) {
		return false
	}
	for i := range a.Attachments {
		if a.Attachments[i].Name != b.Attachments[i].Name || a.Attachments[i].MimeType != b.Attachments[i].MimeType ||
			!bytes.Equal(a.Attachments[i].Data, b.Attachments[i].Data) {
			return false
		}
	}
	for i := range a.FileContexts {
		if a.FileContexts[i].Path != b.FileContexts[i].Path || a.FileContexts[i].Name != b.FileContexts[i].Name ||
			a.FileContexts[i].Size != b.FileContexts[i].Size || !bytes.Equal(a.FileContexts[i].Content, b.FileContexts[i].Content) {
			return false
		}
	}
	return true
}

func sqliteSameAdmissionEvent(a, b domain.RunEvent) bool {
	return a.RunID == b.RunID && a.Seq == b.Seq && a.Type == b.Type && a.CreatedAt == b.CreatedAt &&
		a.PayloadVersion == b.PayloadVersion && bytes.Equal(a.Payload, b.Payload)
}

func sqliteSamePrompt(a, b storage.RunPromptSnapshot) bool {
	return a.RunID == b.RunID && a.SchemaVersion == b.SchemaVersion && a.ComposerVersion == b.ComposerVersion &&
		a.GenerationID == b.GenerationID && a.PayloadSHA256 == b.PayloadSHA256 && bytes.Equal(a.Payload, b.Payload)
}

func sqliteSameAdmissionMarker(a, b storage.SessionTruncation) bool {
	return a.SessionID == b.SessionID && a.CutoffMessageID == b.CutoffMessageID &&
		a.TailMessageID == b.TailMessageID && a.WorkSeq == b.WorkSeq && a.Reason == b.Reason &&
		a.ForkSessionID == b.ForkSessionID && a.CreatedAt == b.CreatedAt
}

func sqliteAdmissionMarkerExists(ctx context.Context, tx *sql.Tx, marker storage.SessionTruncation) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM session_truncations
		WHERE session_id = ? AND cutoff_message_id = ? AND tail_message_id = ? AND work_seq = ? AND reason = ?
		  AND fork_session_id = ? AND created_at = ?`, marker.SessionID, marker.CutoffMessageID,
		marker.TailMessageID, int64(marker.WorkSeq), marker.Reason, marker.ForkSessionID, marker.CreatedAt).Scan(&count)
	if err != nil {
		return false, storage.AdmissionUnavailable("check admission edit marker", err)
	}
	return count > 0, nil
}
