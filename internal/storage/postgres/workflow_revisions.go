package postgres

import (
	"context"
	"database/sql"
	"errors"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.WorkflowRevisionStore = (*Backend)(nil)

func (b *Backend) CommitWorkflowAdmission(ctx context.Context, in storage.WorkflowAdmission) (storage.WorkflowAdmissionResult, error) {
	result := storage.WorkflowAdmissionResult{Revision: in.Revision, Run: in.Run, Started: in.Started}
	if err := storage.ValidateWorkflowAdmission(in); err != nil {
		return result, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return result, storage.AdmissionUnavailable("begin workflow admission", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := postgresLockAdmissionSession(ctx, tx, in.Revision.ParentSessionID); err != nil {
		return result, err
	}
	namespace := in.Revision.AdmissionNamespace
	if namespace == "" {
		namespace = string(in.Revision.ParentRunID)
	}
	if existing, found, err := postgresReadWorkflowRevisionByOperation(ctx, tx, namespace, in.Revision.OperationKey); err != nil {
		return result, err
	} else if found {
		if existing.DescriptorDigest != in.Revision.DescriptorDigest || existing.AuthorityDigest != in.Revision.AuthorityDigest ||
			existing.ParentSessionID != in.Revision.ParentSessionID || string(existing.DescriptorJSON) != string(in.Revision.DescriptorJSON) ||
			string(existing.AuthorityJSON) != string(in.Revision.AuthorityJSON) || existing.RootPurpose != in.Revision.RootPurpose ||
			existing.RequestDigest != in.Revision.RequestDigest || existing.TargetKey != in.Revision.TargetKey {
			return result, storage.ErrWorkflowRevisionConflict
		}
		stored, err := postgresReadWorkflowAdmissionRun(ctx, tx, existing.RunID)
		if err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, storage.AdmissionUnavailable("commit idempotent workflow admission", err)
		}
		return storage.WorkflowAdmissionResult{Revision: existing, Run: stored.run, Started: stored.started, Created: false}, nil
	}
	if in.Revision.RootPurpose == "" {
		parent, err := postgresValidateWorkflowParent(ctx, tx, in.Revision, in.Run)
		if err != nil {
			return result, err
		}
		if err := postgresCheckChildConcurrency(ctx, tx, parent.ID); err != nil {
			return result, err
		}
	} else {
		if err := postgresValidateWorkflowRoot(ctx, tx, in.Revision, in.Run); err != nil {
			return result, err
		}
		if busy, err := postgresWorkflowBusyTarget(ctx, tx, namespace, in.Revision.TargetKey); err != nil {
			return result, err
		} else if busy != "" {
			existing, err := postgresReadWorkflowRevision(ctx, tx, busy)
			if err != nil {
				return result, err
			}
			stored, err := postgresReadWorkflowAdmissionRun(ctx, tx, busy)
			if err != nil {
				return result, err
			}
			if err := tx.Commit(); err != nil {
				return result, storage.AdmissionUnavailable("commit busy workflow admission", err)
			}
			return storage.WorkflowAdmissionResult{Revision: existing, Run: stored.run, Started: stored.started, Created: false, Busy: true}, nil
		}
	}
	if err := postgresInsertAdmissionRun(ctx, tx, in.Run); err != nil {
		return result, err
	}
	started := in.Started
	if err := postgresInsertAdmissionEvent(ctx, tx, &started); err != nil {
		return result, storage.AdmissionUnavailable("insert workflow start event", err)
	}
	r := in.Revision
	if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_revisions
		(workflow_run_id,parent_run_id,parent_session_id,root_run_id,operation_key,root_purpose,admission_namespace,request_digest,target_key,descriptor_digest,authority_digest,descriptor_json,authority_json,schema_version,created_at,program_digest,catalog_digest,compiler_version,eino_build,input_digest,effective_limits,host_binding_id,definition_id,definition_revision,input_json)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.RunID, r.ParentRunID, r.ParentSessionID, r.RootRunID, r.OperationKey,
		r.RootPurpose, namespace, r.RequestDigest, r.TargetKey,
		r.DescriptorDigest, r.AuthorityDigest, r.DescriptorJSON, r.AuthorityJSON, r.SchemaVersion, r.CreatedAt,
		nullString(r.ProgramDigest), nullString(r.CatalogDigest), nullString(r.CompilerVersion),
		nullString(r.EinoBuild), nullString(r.InputDigest), nullBytes(r.EffectiveLimits), nullString(r.HostBindingID),
		nullString(r.DefinitionID), nullInt64U(r.DefinitionRevision), nullBytes(r.InputJSON)); err != nil {
		return result, storage.AdmissionUnavailable("insert workflow revision", err)
	}
	if err := tx.Commit(); err != nil {
		return result, storage.AdmissionUnavailable("commit workflow admission", err)
	}
	run := in.Run
	run.Status = domain.RunActive
	return storage.WorkflowAdmissionResult{Revision: r, Run: run, Started: started, Created: true}, nil
}

func (b *Backend) GetWorkflowRevision(ctx context.Context, runID domain.RunID) (domain.WorkflowRevision, error) {
	return postgresReadWorkflowRevision(ctx, b.db, runID)
}

func (b *Backend) GetWorkflowRevisionByOperation(ctx context.Context, parentRunID domain.RunID, operationKey string) (domain.WorkflowRevision, error) {
	return b.GetWorkflowRevisionByOperationInNamespace(ctx, string(parentRunID), operationKey)
}

func (b *Backend) GetWorkflowRevisionByOperationInNamespace(ctx context.Context, namespace, operationKey string) (domain.WorkflowRevision, error) {
	r, found, err := postgresReadWorkflowRevisionByOperation(ctx, b.db, namespace, operationKey)
	if err != nil {
		return domain.WorkflowRevision{}, err
	}
	if !found {
		return domain.WorkflowRevision{}, storage.ErrNotFound
	}
	return r, nil
}

func (b *Backend) ListWorkflowRevisions(ctx context.Context, parentRunID domain.RunID) ([]domain.WorkflowRevision, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT workflow_run_id FROM workflow_revisions WHERE parent_run_id = ? ORDER BY created_at, workflow_run_id`, parentRunID)
	if err != nil {
		return nil, storage.AdmissionUnavailable("list workflow revisions", err)
	}
	defer func() { _ = rows.Close() }()
	ids := make([]domain.RunID, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, storage.AdmissionUnavailable("scan workflow revision id", err)
		}
		ids = append(ids, domain.RunID(id))
	}
	if err := rows.Err(); err != nil {
		return nil, storage.AdmissionUnavailable("iterate workflow revisions", err)
	}
	result := make([]domain.WorkflowRevision, 0, len(ids))
	for _, id := range ids {
		revision, err := postgresReadWorkflowRevision(ctx, b.db, id)
		if err != nil {
			return nil, err
		}
		result = append(result, revision)
	}
	return result, nil
}

type postgresWorkflowQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *Row
}

func postgresReadWorkflowRevision(ctx context.Context, q postgresWorkflowQueryer, runID domain.RunID) (domain.WorkflowRevision, error) {
	var r domain.WorkflowRevision
	var workflowRun, parentRun, parentSession, rootRun string
	var programDigest, catalogDigest, compilerVersion, einoBuild, inputDigest, hostBindingID, definitionID sql.NullString
	var effectiveLimits []byte
	var definitionRevision sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT workflow_run_id,parent_run_id,parent_session_id,root_run_id,operation_key,root_purpose,admission_namespace,request_digest,target_key,descriptor_digest,authority_digest,descriptor_json,authority_json,schema_version,created_at,program_digest,catalog_digest,compiler_version,eino_build,input_digest,effective_limits,host_binding_id,definition_id,definition_revision,input_json
		FROM workflow_revisions WHERE workflow_run_id = ?`, runID).Scan(&workflowRun, &parentRun, &parentSession, &rootRun, &r.OperationKey,
		&r.RootPurpose, &r.AdmissionNamespace, &r.RequestDigest, &r.TargetKey,
		&r.DescriptorDigest, &r.AuthorityDigest, &r.DescriptorJSON, &r.AuthorityJSON, &r.SchemaVersion, &r.CreatedAt,
		&programDigest, &catalogDigest, &compilerVersion, &einoBuild, &inputDigest, &effectiveLimits, &hostBindingID,
		&definitionID, &definitionRevision, &r.InputJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowRevision{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.WorkflowRevision{}, storage.AdmissionUnavailable("read workflow revision", err)
	}
	r.RunID, r.ParentRunID, r.ParentSessionID, r.RootRunID = domain.RunID(workflowRun), domain.RunID(parentRun), domain.SessionID(parentSession), domain.RunID(rootRun)
	r.DescriptorJSON = append([]byte(nil), r.DescriptorJSON...)
	r.AuthorityJSON = append([]byte(nil), r.AuthorityJSON...)
	r.ProgramDigest = programDigest.String
	r.CatalogDigest = catalogDigest.String
	r.CompilerVersion = compilerVersion.String
	r.EinoBuild = einoBuild.String
	r.InputDigest = inputDigest.String
	r.EffectiveLimits = effectiveLimits
	r.HostBindingID = hostBindingID.String
	r.DefinitionID = definitionID.String
	r.DefinitionRevision = uint64(definitionRevision.Int64)
	r.InputJSON = append([]byte(nil), r.InputJSON...)
	return r, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt64U(v uint64) any {
	if v == 0 {
		return nil
	}
	return int64(v)
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func postgresReadWorkflowRevisionByOperation(ctx context.Context, q postgresWorkflowQueryer, namespace, operationKey string) (domain.WorkflowRevision, bool, error) {
	var runID string
	err := q.QueryRowContext(ctx, `SELECT workflow_run_id FROM workflow_revisions WHERE admission_namespace = ? AND operation_key = ?`, namespace, operationKey).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowRevision{}, false, nil
	}
	if err != nil {
		return domain.WorkflowRevision{}, false, storage.AdmissionUnavailable("read workflow operation", err)
	}
	r, err := postgresReadWorkflowRevision(ctx, q, domain.RunID(runID))
	return r, err == nil, err
}

type postgresWorkflowAdmissionRun struct {
	run     domain.Run
	started domain.RunEvent
}

func postgresReadWorkflowAdmissionRun(ctx context.Context, q postgresWorkflowQueryer, runID domain.RunID) (postgresWorkflowAdmissionRun, error) {
	var result postgresWorkflowAdmissionRun
	var id, sessionID, status, kind, childMode, parentID, rootID, purpose string
	if err := q.QueryRowContext(ctx, `SELECT id,session_id,status,created_at,kind,child_mode,parent_run_id,root_run_id,depth,purpose FROM runs WHERE id = ?`, runID).
		Scan(&id, &sessionID, &status, &result.run.CreatedAt, &kind, &childMode, &parentID, &rootID, &result.run.Depth, &purpose); errors.Is(err, sql.ErrNoRows) {
		return result, storage.ErrNotFound
	} else if err != nil {
		return result, storage.AdmissionUnavailable("read workflow Run", err)
	}
	result.run.ID, result.run.SessionID, result.run.Status = domain.RunID(id), domain.SessionID(sessionID), domain.RunStatus(status)
	result.run.Kind, result.run.ChildMode, result.run.ParentID, result.run.RootID = domain.RunKind(kind), domain.ChildMode(childMode), domain.RunID(parentID), domain.RunID(rootID)
	result.run.Purpose = domain.RunPurpose(purpose)
	var seq int64
	var eventType string
	if err := q.QueryRowContext(ctx, `SELECT seq,type,created_at,payload_version,payload FROM run_events WHERE run_id = ? AND type = ? ORDER BY seq LIMIT 1`, runID, domain.EventRunStarted).
		Scan(&seq, &eventType, &result.started.CreatedAt, &result.started.PayloadVersion, &result.started.Payload); err != nil {
		return result, storage.AdmissionUnavailable("read workflow run.started", err)
	}
	result.started.RunID, result.started.Seq, result.started.Type = runID, domain.EventSeq(seq), domain.EventType(eventType)
	return result, nil
}

// postgresValidateWorkflowRoot enforces the trusted-root invariants inside
// the admission tx: the namespace session exists with the report-control
// purpose and the Run is self-rooted at depth 0.
func postgresValidateWorkflowRoot(ctx context.Context, tx *Tx, revision domain.WorkflowRevision, workflowRun domain.Run) error {
	var purpose string
	if err := tx.QueryRowContext(ctx, `SELECT purpose FROM sessions WHERE id = ?`, revision.ParentSessionID).
		Scan(&purpose); errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	} else if err != nil {
		return storage.AdmissionUnavailable("read admission session", err)
	}
	if domain.SessionPurpose(purpose) != domain.SessionPurposeReportControl {
		return storage.ErrWorkflowRevisionConflict
	}
	if workflowRun.RootID != workflowRun.ID || workflowRun.Depth != 0 {
		return storage.ErrWorkflowRevisionConflict
	}
	return nil
}

// postgresWorkflowBusyTarget returns the non-terminal Run owning targetKey
// inside namespace, or "" when free. Called under the session row lock.
func postgresWorkflowBusyTarget(ctx context.Context, tx *Tx, namespace, targetKey string) (domain.RunID, error) {
	var runID string
	err := tx.QueryRowContext(ctx, `SELECT r.workflow_run_id FROM workflow_revisions r
		JOIN runs ru ON ru.id = r.workflow_run_id
		WHERE r.admission_namespace = ? AND r.target_key = ? AND ru.status NOT IN ('completed','failed','cancelled')
		ORDER BY r.created_at LIMIT 1`, namespace, targetKey).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", storage.AdmissionUnavailable("check workflow target", err)
	}
	return domain.RunID(runID), nil
}

func postgresValidateWorkflowParent(ctx context.Context, tx *Tx, revision domain.WorkflowRevision, workflowRun domain.Run) (domain.Run, error) {
	var parent domain.Run
	var id, sessionID, status, kind, rootID, childMode string
	if err := tx.QueryRowContext(ctx, `SELECT id,session_id,status,created_at,kind,child_mode,parent_run_id,root_run_id,depth FROM runs WHERE id = ? FOR UPDATE`, revision.ParentRunID).
		Scan(&id, &sessionID, &status, &parent.CreatedAt, &kind, &childMode, &parent.ParentID, &rootID, &parent.Depth); errors.Is(err, sql.ErrNoRows) {
		return parent, storage.ErrNotFound
	} else if err != nil {
		return parent, storage.AdmissionUnavailable("read workflow authorizer", err)
	}
	parent.ID, parent.SessionID, parent.Status, parent.Kind = domain.RunID(id), domain.SessionID(sessionID), domain.RunStatus(status), domain.RunKind(kind)
	parent.ChildMode, parent.RootID = domain.ChildMode(childMode), domain.RunID(rootID)
	if (parent.Status != domain.RunActive && parent.Status != domain.RunAccepted) || parent.Kind == domain.RunKindWorkflow || parent.SessionID != revision.ParentSessionID {
		return parent, storage.ErrWorkflowRevisionConflict
	}
	root := parent.RootID
	if root == "" {
		root = parent.ID
	}
	if workflowRun.RootID != root || workflowRun.Depth != parent.Depth+1 {
		return parent, storage.ErrWorkflowRevisionConflict
	}
	return parent, nil
}
