package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var (
	_ storage.ChildSessionStore = (*Backend)(nil)
	_ storage.ChildMailboxStore = (*Backend)(nil)
)

func (b *Backend) CommitChildSessionAdmission(ctx context.Context, in storage.ChildSessionAdmission) (storage.ChildSessionAdmissionResult, error) {
	result := storage.ChildSessionAdmissionResult{Binding: in.Binding, Run: in.Admission.Run, Started: in.Admission.Started}
	if err := storage.ValidateChildSessionAdmission(in); err != nil {
		return result, err
	}
	in.Binding.AuthorityCeiling.ToolNames, _ = domain.CanonicalToolNames(in.Binding.AuthorityCeiling.ToolNames)
	in.Binding.ActivationToolNames, _ = domain.CanonicalToolNames(in.Binding.ActivationToolNames)
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("storage: begin child admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := sqliteLockAdmissionSession(ctx, tx, in.Binding.OriginParentSessionID); err != nil {
		return result, err
	}
	if existing, found, err := sqliteReadChildBindingByOperation(ctx, tx, in.Binding.OriginParentSessionID, in.Binding.OperationKey); err != nil {
		return result, err
	} else if found {
		if existing.RequestDigest != in.Binding.RequestDigest || existing.OriginParentRunID != in.Binding.OriginParentRunID || existing.AuthorityCeilingDigest != in.Binding.AuthorityCeilingDigest {
			return result, storage.ErrChildAdmissionConflict
		}
		stored, found, err := sqliteReadAdmissionRun(ctx, tx, existing.InitialActivationRunID)
		if err != nil || !found {
			if err == nil {
				err = storage.ErrChildAdmissionConflict
			}
			return result, err
		}
		if stored.run.SessionID != existing.ChildSessionID || stored.run.Kind != domain.RunKindChild || stored.run.EffectiveChildMode() != domain.ChildModeContinuable {
			return result, storage.ErrChildAdmissionConflict
		}
		if err := tx.Commit(); err != nil {
			return result, fmt.Errorf("storage: commit idempotent child admission: %w", err)
		}
		return storage.ChildSessionAdmissionResult{Binding: existing, Run: stored.run, Started: stored.started, Created: false}, nil
	}
	parent, err := sqliteReadActiveAuthorizer(ctx, tx, in.Binding.OriginParentRunID, in.Binding.OriginParentSessionID)
	if err != nil {
		return result, err
	}
	if !sqliteChildLineageMatches(in.Admission.Run, parent) {
		return result, storage.ErrChildAdmissionConflict
	}
	if err := sqliteCheckChildConcurrency(ctx, tx, parent.ID); err != nil {
		return result, err
	}
	if err := sqliteInsertChildSession(ctx, tx, in.Session); err != nil {
		return result, err
	}
	if err := sqliteValidateAdmissionCapture(ctx, tx, in.Admission.ExpectedMask); err != nil {
		return result, err
	}
	if exists, err := sqliteMessageExists(ctx, tx, in.Admission.Message.ID); err != nil {
		return result, err
	} else if exists {
		return result, storage.ErrChildAdmissionConflict
	}
	if err := sqliteInsertAdmissionMessage(ctx, tx, in.Admission.Message); err != nil {
		return result, err
	}
	if err := sqliteInsertAdmissionRun(ctx, tx, in.Admission.Run); err != nil {
		return result, err
	}
	if in.Admission.Prompt != nil {
		if err := sqliteInsertAdmissionPrompt(ctx, tx, *in.Admission.Prompt); err != nil {
			return result, err
		}
	}
	started := in.Admission.Started
	started.Seq = 1
	if err := sqliteInsertHistoryEvent(ctx, tx, &started); err != nil {
		return result, storage.AdmissionUnavailable("insert child start event", err)
	}
	binding := in.Binding
	if binding.State == "" {
		binding.State = domain.ChildSessionOpen
	}
	binding.NextMessageSequence = 1
	binding.NextParentMessageSequence = 1
	ceilingJSON, ceilingDigest, err := storage.EncodeChildAuthorityCeiling(binding.AuthorityCeiling)
	if err != nil || ceilingDigest != binding.AuthorityCeilingDigest {
		return result, errors.New("storage: invalid child authority ceiling")
	}
	activationToolsJSON, err := storage.EncodeChildToolNames(binding.ActivationToolNames)
	if err != nil {
		return result, err
	}
	if binding.CreatedAt <= 0 {
		binding.CreatedAt = in.Admission.Run.CreatedAt
	}
	if binding.UpdatedAt <= 0 {
		binding.UpdatedAt = binding.CreatedAt
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO child_sessions
			(child_session_id,origin_parent_session_id,origin_parent_run_id,authorizer_run_id,initial_activation_run_id,activation_run_id,
			 operation_key,request_digest,authority_ceiling_digest,authority_ceiling_json,activation_operation_key,activation_request_digest,activation_tools_json,
			 state,next_message_sequence,consumed_message_sequence,next_parent_message_sequence,consumed_parent_message_sequence,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, binding.ChildSessionID, binding.OriginParentSessionID, binding.OriginParentRunID,
		binding.AuthorizerRunID, binding.InitialActivationRunID, binding.ActivationRunID, binding.OperationKey,
		binding.RequestDigest, binding.AuthorityCeilingDigest, string(ceilingJSON), binding.ActivationOperationKey, binding.ActivationRequestDigest,
		string(activationToolsJSON), string(binding.State), binding.NextMessageSequence, binding.ConsumedMessageSequence,
		binding.NextParentMessageSequence, binding.ConsumedParentMessageSequence,
		binding.CreatedAt, binding.UpdatedAt); err != nil {
		return result, storage.AdmissionUnavailable("insert child session binding", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO child_session_activations (child_session_id,activation_run_id,authorizer_run_id,operation_key,request_digest,tool_names_json,created_at)
		VALUES (?,?,?,?,?,?,?)`, binding.ChildSessionID, binding.ActivationRunID, binding.AuthorizerRunID, binding.ActivationOperationKey,
		binding.ActivationRequestDigest, string(activationToolsJSON), in.Admission.Run.CreatedAt); err != nil {
		return result, storage.AdmissionUnavailable("insert initial child activation identity", err)
	}
	if err := tx.Commit(); err != nil {
		return result, storage.AdmissionUnavailable("commit child session admission", err)
	}
	run := in.Admission.Run
	run.Status = domain.RunActive
	run.RootID = effectiveRunRootID(run)
	run.ChildMode = domain.ChildModeContinuable
	return storage.ChildSessionAdmissionResult{Binding: binding, Run: run, Started: started, Created: true}, nil
}

func (b *Backend) CommitChildSessionActivation(ctx context.Context, in storage.ChildSessionActivation) (storage.ChildSessionAdmissionResult, error) {
	result := storage.ChildSessionAdmissionResult{Run: in.Admission.Run, Started: in.Admission.Started}
	if err := storage.ValidateChildSessionActivation(in); err != nil {
		return result, err
	}
	in.ToolNames, _ = domain.CanonicalToolNames(in.ToolNames)
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("storage: begin child activation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	binding, err := sqliteReadChildBinding(ctx, tx, in.ChildSessionID)
	if err != nil {
		return result, err
	}
	if err := sqliteLockAdmissionSession(ctx, tx, binding.OriginParentSessionID); err != nil {
		return result, err
	}
	binding, err = sqliteReadChildBinding(ctx, tx, in.ChildSessionID)
	if err != nil {
		return result, err
	}
	if existing, found, err := sqliteReadChildActivation(ctx, tx, in.ChildSessionID, in.OperationKey); err != nil {
		return result, err
	} else if found {
		if existing.requestDigest != in.RequestDigest || existing.authorizerRunID != in.AuthorizerRunID {
			return result, storage.ErrChildAdmissionConflict
		}
		stored, found, err := sqliteReadAdmissionRun(ctx, tx, existing.runID)
		if err != nil || !found {
			if err == nil {
				err = storage.ErrChildAdmissionConflict
			}
			return result, err
		}
		if stored.run.SessionID != in.ChildSessionID || stored.run.Kind != domain.RunKindChild || stored.run.EffectiveChildMode() != domain.ChildModeContinuable {
			return result, storage.ErrChildAdmissionConflict
		}
		if err := tx.Commit(); err != nil {
			return result, fmt.Errorf("storage: commit idempotent child activation: %w", err)
		}
		return storage.ChildSessionAdmissionResult{Binding: binding, Run: stored.run, Started: stored.started, Created: false}, nil
	}
	if stored, found, err := sqliteReadAdmissionRun(ctx, tx, in.Admission.Run.ID); err != nil {
		return result, err
	} else if found {
		_ = stored
		return result, storage.ErrChildAdmissionConflict
	}
	if binding.State != domain.ChildSessionOpen {
		return result, storage.ErrChildSessionClosed
	}
	currentActivation, found, err := sqliteReadAdmissionRun(ctx, tx, binding.ActivationRunID)
	if err != nil {
		return result, err
	}
	if !found || !currentActivation.run.Status.Terminal() {
		return result, storage.ErrChildAdmissionConflict
	}
	if !sqliteChildToolsSubset(in.ToolNames, binding.AuthorityCeiling.ToolNames) {
		return result, storage.ErrChildAdmissionConflict
	}
	parent, err := sqliteReadActiveAuthorizer(ctx, tx, in.AuthorizerRunID, binding.OriginParentSessionID)
	if err != nil {
		return result, err
	}
	if !sqliteChildLineageMatches(in.Admission.Run, parent) {
		return result, storage.ErrChildAdmissionConflict
	}
	if err := sqliteCheckChildConcurrency(ctx, tx, parent.ID); err != nil {
		return result, err
	}
	if err := sqliteValidateAdmissionCapture(ctx, tx, in.Admission.ExpectedMask); err != nil {
		return result, err
	}
	if exists, err := sqliteMessageExists(ctx, tx, in.Admission.Message.ID); err != nil {
		return result, err
	} else if exists {
		return result, storage.ErrChildAdmissionConflict
	}
	if err := sqliteInsertAdmissionMessage(ctx, tx, in.Admission.Message); err != nil {
		return result, err
	}
	if err := sqliteInsertAdmissionRun(ctx, tx, in.Admission.Run); err != nil {
		return result, err
	}
	if in.Admission.Prompt != nil {
		if err := sqliteInsertAdmissionPrompt(ctx, tx, *in.Admission.Prompt); err != nil {
			return result, err
		}
	}
	started := in.Admission.Started
	started.Seq = 1
	if err := sqliteInsertHistoryEvent(ctx, tx, &started); err != nil {
		return result, storage.AdmissionUnavailable("insert child activation event", err)
	}
	toolsJSON, err := storage.EncodeChildToolNames(in.ToolNames)
	if err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO child_session_activations (child_session_id,activation_run_id,authorizer_run_id,operation_key,request_digest,tool_names_json,created_at)
		VALUES (?,?,?,?,?,?,?)`, in.ChildSessionID, in.Admission.Run.ID, in.AuthorizerRunID, in.OperationKey, in.RequestDigest, string(toolsJSON), in.Admission.Run.CreatedAt); err != nil {
		return result, storage.AdmissionUnavailable("insert child activation identity", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE child_sessions
		SET authorizer_run_id = ?, activation_run_id = ?, activation_operation_key = ?, activation_request_digest = ?, activation_tools_json = ?, updated_at = ?
		WHERE child_session_id = ? AND state = 'open'`,
		in.AuthorizerRunID, in.Admission.Run.ID, in.OperationKey, in.RequestDigest, string(toolsJSON), in.Admission.Run.CreatedAt, in.ChildSessionID); err != nil {
		return result, storage.AdmissionUnavailable("update child session authorizer", err)
	}
	if err := tx.Commit(); err != nil {
		return result, storage.AdmissionUnavailable("commit child activation", err)
	}
	binding.AuthorizerRunID = in.AuthorizerRunID
	binding.ActivationRunID = in.Admission.Run.ID
	binding.ActivationOperationKey = in.OperationKey
	binding.ActivationRequestDigest = in.RequestDigest
	binding.ActivationToolNames = append([]string(nil), in.ToolNames...)
	binding.UpdatedAt = in.Admission.Run.CreatedAt
	run := in.Admission.Run
	run.Status = domain.RunActive
	run.RootID = effectiveRunRootID(run)
	run.ChildMode = domain.ChildModeContinuable
	return storage.ChildSessionAdmissionResult{Binding: binding, Run: run, Started: started, Created: true}, nil
}

func (b *Backend) GetChildSessionBinding(ctx context.Context, childSessionID domain.SessionID) (domain.ChildSessionBinding, error) {
	return sqliteReadChildBinding(ctx, b.db, childSessionID)
}

func sqliteCheckChildConcurrency(ctx context.Context, tx *sql.Tx, parentRunID domain.RunID) error {
	if parentRunID == "" {
		return storage.ErrChildAdmissionConflict
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE parent_run_id = ? AND status IN ('accepted','queued','active')`, parentRunID).Scan(&count); err != nil {
		return storage.AdmissionUnavailable("count active child runs", err)
	}
	if count >= storage.MaxActiveChildrenPerRun {
		return storage.ErrChildConcurrencyLimit
	}
	return nil
}

func (b *Backend) ListChildSessions(ctx context.Context, parentSessionID domain.SessionID) ([]domain.ChildSessionBinding, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT child_session_id FROM child_sessions WHERE origin_parent_session_id = ? ORDER BY child_session_id`, parentSessionID)
	if err != nil {
		return nil, fmt.Errorf("storage: list child sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []domain.SessionID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("storage: scan child session id: %w", err)
		}
		ids = append(ids, domain.SessionID(id))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	bindings := make([]domain.ChildSessionBinding, 0, len(ids))
	for _, id := range ids {
		binding, err := sqliteReadChildBinding(ctx, b.db, id)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

type sqliteChildBindingReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func sqliteReadChildBinding(ctx context.Context, q sqliteChildBindingReader, childID domain.SessionID) (domain.ChildSessionBinding, error) {
	var binding domain.ChildSessionBinding
	var authorityJSON, toolsJSON string
	err := q.QueryRowContext(ctx, `
		SELECT child_session_id,origin_parent_session_id,origin_parent_run_id,authorizer_run_id,initial_activation_run_id,activation_run_id,
		       operation_key,request_digest,authority_ceiling_digest,authority_ceiling_json,activation_operation_key,activation_request_digest,activation_tools_json,
		       state,next_message_sequence,consumed_message_sequence,next_parent_message_sequence,consumed_parent_message_sequence,created_at,updated_at
		FROM child_sessions WHERE child_session_id = ?`, childID).Scan(
		(*string)(&binding.ChildSessionID), (*string)(&binding.OriginParentSessionID), (*string)(&binding.OriginParentRunID),
		(*string)(&binding.AuthorizerRunID), (*string)(&binding.InitialActivationRunID), (*string)(&binding.ActivationRunID),
		&binding.OperationKey, &binding.RequestDigest, &binding.AuthorityCeilingDigest, &authorityJSON, &binding.ActivationOperationKey,
		&binding.ActivationRequestDigest, &toolsJSON, (*string)(&binding.State),
		&binding.NextMessageSequence, &binding.ConsumedMessageSequence, &binding.NextParentMessageSequence,
		&binding.ConsumedParentMessageSequence, &binding.CreatedAt, &binding.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChildSessionBinding{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.ChildSessionBinding{}, fmt.Errorf("storage: read child session binding: %w", err)
	}
	var decodeErr error
	binding.AuthorityCeiling, decodeErr = storage.DecodeChildAuthorityCeiling([]byte(authorityJSON), binding.AuthorityCeilingDigest)
	if decodeErr != nil {
		return domain.ChildSessionBinding{}, decodeErr
	}
	binding.ActivationToolNames, decodeErr = storage.DecodeChildToolNames([]byte(toolsJSON))
	if decodeErr != nil {
		return domain.ChildSessionBinding{}, decodeErr
	}
	return binding, nil
}

func sqliteReadChildBindingByOperation(ctx context.Context, tx *sql.Tx, parentID domain.SessionID, operationKey string) (domain.ChildSessionBinding, bool, error) {
	var binding domain.ChildSessionBinding
	var authorityJSON, toolsJSON string
	err := tx.QueryRowContext(ctx, `
		SELECT child_session_id,origin_parent_session_id,origin_parent_run_id,authorizer_run_id,initial_activation_run_id,activation_run_id,
		       operation_key,request_digest,authority_ceiling_digest,authority_ceiling_json,activation_operation_key,activation_request_digest,activation_tools_json,
		       state,next_message_sequence,consumed_message_sequence,next_parent_message_sequence,consumed_parent_message_sequence,created_at,updated_at
		FROM child_sessions WHERE origin_parent_session_id = ? AND operation_key = ?`, parentID, operationKey).Scan(
		(*string)(&binding.ChildSessionID), (*string)(&binding.OriginParentSessionID), (*string)(&binding.OriginParentRunID),
		(*string)(&binding.AuthorizerRunID), (*string)(&binding.InitialActivationRunID), (*string)(&binding.ActivationRunID),
		&binding.OperationKey, &binding.RequestDigest, &binding.AuthorityCeilingDigest, &authorityJSON, &binding.ActivationOperationKey,
		&binding.ActivationRequestDigest, &toolsJSON, (*string)(&binding.State),
		&binding.NextMessageSequence, &binding.ConsumedMessageSequence, &binding.NextParentMessageSequence,
		&binding.ConsumedParentMessageSequence, &binding.CreatedAt, &binding.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChildSessionBinding{}, false, nil
	}
	if err != nil {
		return domain.ChildSessionBinding{}, false, fmt.Errorf("storage: read child operation binding: %w", err)
	}
	binding.AuthorityCeiling, err = storage.DecodeChildAuthorityCeiling([]byte(authorityJSON), binding.AuthorityCeilingDigest)
	if err != nil {
		return domain.ChildSessionBinding{}, false, err
	}
	binding.ActivationToolNames, err = storage.DecodeChildToolNames([]byte(toolsJSON))
	if err != nil {
		return domain.ChildSessionBinding{}, false, err
	}
	return binding, true, nil
}

type sqliteChildActivation struct {
	runID           domain.RunID
	authorizerRunID domain.RunID
	requestDigest   string
}

func sqliteReadChildActivation(ctx context.Context, tx *sql.Tx, childID domain.SessionID, operationKey string) (sqliteChildActivation, bool, error) {
	var activation sqliteChildActivation
	err := tx.QueryRowContext(ctx, `SELECT activation_run_id,authorizer_run_id,request_digest FROM child_session_activations WHERE child_session_id = ? AND operation_key = ?`, childID, operationKey).
		Scan((*string)(&activation.runID), (*string)(&activation.authorizerRunID), &activation.requestDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return sqliteChildActivation{}, false, nil
	}
	if err != nil {
		return sqliteChildActivation{}, false, fmt.Errorf("storage: read child activation identity: %w", err)
	}
	return activation, true, nil
}

func sqliteChildToolsSubset(requested, ceiling []string) bool {
	allowed := make(map[string]struct{}, len(ceiling))
	for _, name := range ceiling {
		allowed[name] = struct{}{}
	}
	for _, name := range requested {
		if _, ok := allowed[name]; !ok {
			return false
		}
	}
	return true
}

func sqliteReadActiveAuthorizer(ctx context.Context, tx *sql.Tx, runID domain.RunID, sessionID domain.SessionID) (domain.Run, error) {
	var run domain.Run
	var id, sid, status, kind, childMode, parentID, rootID string
	err := tx.QueryRowContext(ctx, `SELECT id,session_id,status,created_at,kind,child_mode,parent_run_id,root_run_id,depth FROM runs WHERE id = ?`, runID).
		Scan(&id, &sid, &status, &run.CreatedAt, &kind, &childMode, &parentID, &rootID, &run.Depth)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Run{}, fmt.Errorf("storage: read child authorizer: %w", err)
	}
	run.ID, run.SessionID, run.Status = domain.RunID(id), domain.SessionID(sid), domain.RunStatus(status)
	run.Kind, run.ChildMode, run.ParentID, run.RootID = domain.RunKind(kind), domain.ChildMode(childMode), domain.RunID(parentID), domain.RunID(rootID)
	if run.SessionID != sessionID || run.Status != domain.RunActive {
		return domain.Run{}, storage.ErrChildAdmissionConflict
	}
	return run, nil
}

func sqliteChildLineageMatches(child, parent domain.Run) bool {
	root := parent.RootID
	if root == "" {
		root = parent.ID
	}
	return child.ParentID == parent.ID && child.RootID == root && child.Depth == parent.Depth+1
}

func effectiveRunRootID(run domain.Run) domain.RunID {
	if run.RootID != "" {
		return run.RootID
	}
	return run.ID
}

func sqliteInsertChildSession(ctx context.Context, tx *sql.Tx, session domain.Session) error {
	mode, policy := session.EffectiveSandbox()
	if session.UpdatedAt <= 0 {
		session.UpdatedAt = session.CreatedAt
	}
	if session.UpdatedAt <= 0 {
		session.UpdatedAt = time.Now().UnixMilli()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id,title,created_at,updated_at,sandbox_mode,approval_policy,workspace_path) VALUES (?,?,?,?,?,?,?)`,
		session.ID, session.Title, session.CreatedAt, session.UpdatedAt, string(mode), string(policy), session.WorkspacePath); err != nil {
		return storage.AdmissionUnavailable("insert child session", err)
	}
	return nil
}
