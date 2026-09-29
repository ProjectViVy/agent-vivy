package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.WorkflowStepStore = (*Backend)(nil)

// CommitWorkflowStep applies one INOFY commit as a single Postgres
// transaction. It mirrors the SQLite driver statement-for-statement: same
// invariants, same receipt shape, `?` placeholders rebound by the Tx wrapper.
func (b *Backend) CommitWorkflowStep(ctx context.Context, c storage.WorkflowStepCommit) (storage.WorkflowStepReceipt, error) {
	if err := storage.ValidateWorkflowStepCommit(c); err != nil {
		return storage.WorkflowStepReceipt{}, err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("begin workflow step", err)
	}
	defer func() { _ = tx.Rollback() }()

	run, err := postgresReadRunForUpdate(ctx, tx, c.RunID)
	if err != nil {
		return storage.WorkflowStepReceipt{}, err
	}
	revision, err := postgresReadWorkflowRevision(ctx, tx, c.RunID)
	if err != nil {
		return storage.WorkflowStepReceipt{}, err
	}
	if revision.SchemaVersion != 2 {
		return storage.WorkflowStepReceipt{}, storage.ErrNotFound
	}
	if revision.ProgramDigest != c.ProgramDigest ||
		(c.HostBindingID != "" && revision.HostBindingID != c.HostBindingID) {
		return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepIdentity
	}

	var digest string
	var firstSeq, lastSeq, rev int64
	err = tx.QueryRowContext(ctx,
		`SELECT digest,first_seq,last_seq,revision FROM workflow_commits WHERE workflow_run_id = ? AND commit_id = ?`,
		c.RunID, c.CommitID).Scan(&digest, &firstSeq, &lastSeq, &rev)
	if err == nil {
		if digest != c.Digest {
			return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepIdempotency
		}
		if err := tx.Commit(); err != nil {
			return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("commit step replay", err)
		}
		return storage.WorkflowStepReceipt{
			FirstSequence: domain.EventSeq(firstSeq),
			LastSequence:  domain.EventSeq(lastSeq),
			Revision:      uint64(rev),
			Replayed:      true,
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("read workflow commit", err)
	}

	proj, err := postgresReadWorkflowExecution(ctx, tx, c.RunID)
	if err != nil {
		return storage.WorkflowStepReceipt{}, err
	}
	storedStatus := storage.WorkflowStepStatus("")
	var storedEpoch uint64
	if proj != nil {
		storedStatus = proj.Status
		storedEpoch = proj.Epoch
	}
	if storedStatus.Terminal() {
		return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepTerminal
	}
	if c.Epoch < storedEpoch {
		return storage.WorkflowStepReceipt{}, storage.ErrStaleWriter
	}
	if storedStatus != c.Expected {
		return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepState
	}

	var closed int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM run_events WHERE run_id = ? AND type IN `+terminalTypes,
		c.RunID).Scan(&closed); err != nil {
		return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("check workflow terminal", err)
	}
	if closed > 0 {
		return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepTerminal
	}

	current := storage.WorkflowStepProjection{}
	if proj != nil {
		current = *proj
	}
	next, err := storage.WorkflowStepDerive(&current, c, revision)
	if err != nil {
		return storage.WorkflowStepReceipt{}, err
	}

	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(seq) FROM run_events WHERE run_id = ?`, c.RunID).Scan(&maxSeq); err != nil {
		return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("read workflow seq", err)
	}
	seq := maxSeq.Int64
	for _, e := range c.Events {
		seq++
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO run_events (run_id,seq,type,created_at,payload_version,payload) VALUES (?,?,?,?,?,?)`,
			c.RunID, seq, string(e.Type), e.CreatedAt, e.PayloadVersion, e.Payload); err != nil {
			return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("insert workflow event", err)
		}
	}

	now := time.Now().UnixMilli()
	for _, r := range c.Results {
		var existing string
		err := tx.QueryRowContext(ctx,
			`INSERT INTO workflow_results (workflow_run_id,path,attempt,digest,blob_id,bytes,created_at)
			 VALUES (?,?,?,?,?,?,?) ON CONFLICT (workflow_run_id,path,attempt) DO NOTHING
			 RETURNING digest`,
			c.RunID, r.Path, r.Attempt, r.Digest, r.BlobID, r.Bytes, now).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			// Conflict: the row already exists; compare digests.
			if err := tx.QueryRowContext(ctx,
				`SELECT digest FROM workflow_results WHERE workflow_run_id = ? AND path = ? AND attempt = ?`,
				c.RunID, r.Path, r.Attempt).Scan(&existing); err != nil {
				return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("read workflow result", err)
			}
			if existing != r.Digest {
				return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepIdempotency
			}
			continue
		}
		if err != nil {
			return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("insert workflow result", err)
		}
	}

	if proj == nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_executions
				(workflow_run_id,status,epoch,revision,usage,checkpoint,checkpoint_blob_id,checkpoint_digest,
				 waits,interrupts,gates,resume_key,resume_answers_digest,unresolved,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			c.RunID, string(next.Status), int64(next.Epoch), int64(next.Revision), next.UsageJSON,
			next.CheckpointJSON, next.CheckpointBlobID, next.CheckpointDigest,
			next.WaitsJSON, next.InterruptsJSON, next.GatesJSON,
			next.ResumeKey, next.ResumeAnswersDigest, next.UnresolvedJSON, now); err != nil {
			return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("insert workflow execution", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE workflow_executions SET status=?,epoch=?,revision=?,usage=?,checkpoint=?,checkpoint_blob_id=?,
				checkpoint_digest=?,waits=?,interrupts=?,gates=?,resume_key=?,resume_answers_digest=?,unresolved=?,updated_at=?
			WHERE workflow_run_id = ?`,
			string(next.Status), int64(next.Epoch), int64(next.Revision), next.UsageJSON,
			next.CheckpointJSON, next.CheckpointBlobID, next.CheckpointDigest,
			next.WaitsJSON, next.InterruptsJSON, next.GatesJSON,
			next.ResumeKey, next.ResumeAnswersDigest, next.UnresolvedJSON, now, c.RunID); err != nil {
			return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("update workflow execution", err)
		}
	}

	if native, ok := c.Target.NativeRunStatus(); ok {
		if run.Status.Terminal() {
			return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepTerminal
		}
		if _, err := tx.ExecContext(ctx, `UPDATE runs SET status = ? WHERE id = ?`,
			string(native), c.RunID); err != nil {
			return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("set workflow run terminal", err)
		}
	} else {
		if run.Status.Terminal() {
			return storage.WorkflowStepReceipt{}, storage.ErrWorkflowStepTerminal
		}
		if c.Target != storage.WorkflowStepAdmitted && run.Status == domain.RunAccepted {
			if _, err := tx.ExecContext(ctx, `UPDATE runs SET status = ? WHERE id = ?`,
				string(domain.RunActive), c.RunID); err != nil {
				return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("activate workflow run", err)
			}
		}
	}

	firstSeqOut := seq
	if len(c.Events) > 0 {
		firstSeqOut = seq - int64(len(c.Events)) + 1
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO workflow_commits (workflow_run_id,commit_id,digest,first_seq,last_seq,revision,created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		c.RunID, c.CommitID, c.Digest, firstSeqOut, seq, int64(next.Revision), now); err != nil {
		return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("record workflow commit", err)
	}
	if err := tx.Commit(); err != nil {
		return storage.WorkflowStepReceipt{}, storage.AdmissionUnavailable("commit workflow step", err)
	}
	return storage.WorkflowStepReceipt{
		FirstSequence: domain.EventSeq(firstSeqOut),
		LastSequence:  domain.EventSeq(seq),
		Revision:      next.Revision,
	}, nil
}

// LoadWorkflowStep reads the admitted revision plus the current projection
// and result index for one INOFY execution.
func (b *Backend) LoadWorkflowStep(ctx context.Context, runID domain.RunID) (storage.WorkflowStepState, error) {
	run, err := b.GetRun(ctx, runID)
	if err != nil {
		return storage.WorkflowStepState{}, err
	}
	revision, err := postgresReadWorkflowRevision(ctx, b.db, runID)
	if err != nil {
		return storage.WorkflowStepState{}, err
	}
	if revision.SchemaVersion != 2 {
		return storage.WorkflowStepState{}, storage.ErrNotFound
	}
	proj, err := postgresScanWorkflowExecution(b.db.QueryRowContext(ctx, `
		SELECT status,epoch,revision,usage,checkpoint,checkpoint_blob_id,checkpoint_digest,
			waits,interrupts,gates,resume_key,resume_answers_digest,unresolved,updated_at
		FROM workflow_executions WHERE workflow_run_id = ?`, runID))
	if err != nil {
		return storage.WorkflowStepState{}, err
	}
	results, err := postgresListWorkflowResults(ctx, b.db, runID)
	if err != nil {
		return storage.WorkflowStepState{}, err
	}
	return storage.WorkflowStepState{Revision: revision, Run: run, Projection: proj, Results: results}, nil
}

type postgresStepScanner interface {
	Scan(dest ...any) error
}

func postgresScanWorkflowExecution(row postgresStepScanner) (*storage.WorkflowStepProjection, error) {
	var p storage.WorkflowStepProjection
	var status string
	var epoch, revision, updatedAt int64
	err := row.Scan(&status, &epoch, &revision, &p.UsageJSON, &p.CheckpointJSON,
		&p.CheckpointBlobID, &p.CheckpointDigest, &p.WaitsJSON, &p.InterruptsJSON,
		&p.GatesJSON, &p.ResumeKey, &p.ResumeAnswersDigest, &p.UnresolvedJSON, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, storage.AdmissionUnavailable("read workflow execution", err)
	}
	p.Status = storage.WorkflowStepStatus(status)
	p.Epoch = uint64(epoch)
	p.Revision = uint64(revision)
	p.UpdatedAt = updatedAt
	return &p, nil
}

func postgresReadWorkflowExecution(ctx context.Context, tx *Tx, runID domain.RunID) (*storage.WorkflowStepProjection, error) {
	return postgresScanWorkflowExecution(tx.QueryRowContext(ctx, `
		SELECT status,epoch,revision,usage,checkpoint,checkpoint_blob_id,checkpoint_digest,
			waits,interrupts,gates,resume_key,resume_answers_digest,unresolved,updated_at
		FROM workflow_executions WHERE workflow_run_id = ? FOR UPDATE`, runID))
}

func postgresReadRunForUpdate(ctx context.Context, tx *Tx, runID domain.RunID) (domain.Run, error) {
	var r domain.Run
	var id, sid, status, kind, parentID, rootID, childMode string
	err := tx.QueryRowContext(ctx, `
		SELECT id,session_id,status,created_at,kind,child_mode,parent_run_id,root_run_id,depth
		FROM runs WHERE id = ? FOR UPDATE`, runID).
		Scan(&id, &sid, &status, &r.CreatedAt, &kind, &childMode, &parentID, &rootID, &r.Depth)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Run{}, storage.AdmissionUnavailable("read workflow run", err)
	}
	r.ID = domain.RunID(id)
	r.SessionID = domain.SessionID(sid)
	r.Status = domain.RunStatus(status)
	r.Kind = domain.RunKind(kind)
	r.ChildMode = domain.ChildMode(childMode)
	r.ParentID = domain.RunID(parentID)
	r.RootID = domain.RunID(rootID)
	return r, nil
}

func postgresListWorkflowResults(ctx context.Context, q postgresWorkflowQueryer, runID domain.RunID) ([]storage.WorkflowStepResult, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT path,attempt,digest,blob_id,bytes FROM workflow_results
		WHERE workflow_run_id = ? ORDER BY path, attempt`, runID)
	if err != nil {
		return nil, storage.AdmissionUnavailable("list workflow results", err)
	}
	defer func() { _ = rows.Close() }()
	var out []storage.WorkflowStepResult
	for rows.Next() {
		var r storage.WorkflowStepResult
		if err := rows.Scan(&r.Path, &r.Attempt, &r.Digest, &r.BlobID, &r.Bytes); err != nil {
			return nil, storage.AdmissionUnavailable("scan workflow result", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, storage.AdmissionUnavailable("iterate workflow results", err)
	}
	return out, nil
}
