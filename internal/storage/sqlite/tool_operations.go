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

var _ storage.ToolOperationStore = (*Backend)(nil)

func (b *Backend) AdmitToolOperation(ctx context.Context, admission domain.ToolOperation) (domain.ToolOperation, bool, domain.RunEvent, error) {
	if err := storage.ValidateToolOperationAdmission(admission); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, err
	}
	if admission.CreatedAt <= 0 {
		admission.CreatedAt = time.Now().UnixMilli()
	}
	admission.UpdatedAt = admission.CreatedAt
	admission.State = domain.ToolOperationAdmitted

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: begin tool operation admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET status = status WHERE id = ?`, admission.RunID); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: lock tool operation run: %w", err)
	}
	if existing, found, err := sqliteGetToolOperation(ctx, tx, admission.RunID, admission.OperationID); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, err
	} else if found {
		if !sameToolOperationBinding(existing, admission) {
			return domain.ToolOperation{}, false, domain.RunEvent{}, storage.ErrToolOperationConflict
		}
		return existing, false, domain.RunEvent{}, nil
	}
	if err := sqliteCheckToolOperationRunOpen(ctx, tx, admission.RunID); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tool_operations
		(run_id, operation_id, tool_name, request_digest, middleware_input_arguments, arguments_digest, effective_arguments, state, claim_owner, result, failure, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', '', '', ?, ?)`, admission.RunID, admission.OperationID, admission.ToolName,
		admission.RequestDigest, admission.MiddlewareInputArguments, admission.ArgumentsDigest, admission.EffectiveArguments, string(admission.State), admission.CreatedAt, admission.UpdatedAt); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: insert tool operation admission: %w", err)
	}
	event, err := sqliteAppendToolOperationEvent(ctx, tx, admission)
	if err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: commit tool operation admission: %w", err)
	}
	return admission, true, event, nil
}

func (b *Backend) GetToolOperation(ctx context.Context, runID domain.RunID, operationID string) (domain.ToolOperation, error) {
	op, found, err := sqliteGetToolOperation(ctx, b.db, runID, operationID)
	if err != nil {
		return domain.ToolOperation{}, err
	}
	if !found {
		return domain.ToolOperation{}, storage.ErrNotFound
	}
	return op, nil
}

func (b *Backend) ClaimToolOperation(ctx context.Context, runID domain.RunID, operationID, owner string) (domain.ToolOperation, bool, domain.RunEvent, error) {
	if owner == "" {
		return domain.ToolOperation{}, false, domain.RunEvent{}, errors.New("storage: tool operation claim owner is empty")
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: begin tool operation claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET status = status WHERE id = ?`, runID); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: lock tool operation run: %w", err)
	}
	op, found, err := sqliteGetToolOperation(ctx, tx, runID, operationID)
	if err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, err
	}
	if !found {
		return domain.ToolOperation{}, false, domain.RunEvent{}, storage.ErrNotFound
	}
	if op.State != domain.ToolOperationAdmitted {
		return op, false, domain.RunEvent{}, nil
	}
	if err := sqliteCheckToolOperationRunOpen(ctx, tx, runID); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, err
	}
	op.State = domain.ToolOperationClaimed
	op.ClaimOwner = owner
	op.UpdatedAt = time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx, `UPDATE tool_operations SET state = ?, claim_owner = ?, updated_at = ?
		WHERE run_id = ? AND operation_id = ? AND state = ?`, string(op.State), owner, op.UpdatedAt, runID, operationID, string(domain.ToolOperationAdmitted)); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: claim tool operation: %w", err)
	}
	event, err := sqliteAppendToolOperationEvent(ctx, tx, op)
	if err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ToolOperation{}, false, domain.RunEvent{}, fmt.Errorf("storage: commit tool operation claim: %w", err)
	}
	return op, true, event, nil
}

func (b *Backend) CompleteToolOperation(ctx context.Context, runID domain.RunID, operationID, owner, result, failure string) (domain.ToolOperation, domain.RunEvent, error) {
	if owner == "" {
		return domain.ToolOperation{}, domain.RunEvent{}, errors.New("storage: tool operation completion owner is empty")
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ToolOperation{}, domain.RunEvent{}, fmt.Errorf("storage: begin tool operation completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET status = status WHERE id = ?`, runID); err != nil {
		return domain.ToolOperation{}, domain.RunEvent{}, fmt.Errorf("storage: lock tool operation run: %w", err)
	}
	op, found, err := sqliteGetToolOperation(ctx, tx, runID, operationID)
	if err != nil {
		return domain.ToolOperation{}, domain.RunEvent{}, err
	}
	if !found {
		return domain.ToolOperation{}, domain.RunEvent{}, storage.ErrNotFound
	}
	if op.State == domain.ToolOperationCompleted {
		if op.Result != result || op.Failure != failure {
			return domain.ToolOperation{}, domain.RunEvent{}, storage.ErrToolOperationConflict
		}
		return op, domain.RunEvent{}, nil
	}
	if op.State != domain.ToolOperationClaimed || op.ClaimOwner != owner {
		return domain.ToolOperation{}, domain.RunEvent{}, storage.ErrToolOperationConflict
	}
	if err := sqliteCheckToolOperationRunOpen(ctx, tx, runID); err != nil {
		return domain.ToolOperation{}, domain.RunEvent{}, err
	}
	op.State = domain.ToolOperationCompleted
	op.Result = result
	op.Failure = failure
	op.UpdatedAt = time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx, `UPDATE tool_operations SET state = ?, result = ?, failure = ?, updated_at = ?
		WHERE run_id = ? AND operation_id = ? AND state = ? AND claim_owner = ?`, string(op.State), result, failure, op.UpdatedAt,
		runID, operationID, string(domain.ToolOperationClaimed), owner); err != nil {
		return domain.ToolOperation{}, domain.RunEvent{}, fmt.Errorf("storage: complete tool operation: %w", err)
	}
	event, err := sqliteAppendToolOperationEvent(ctx, tx, op)
	if err != nil {
		return domain.ToolOperation{}, domain.RunEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ToolOperation{}, domain.RunEvent{}, fmt.Errorf("storage: commit tool operation completion: %w", err)
	}
	return op, event, nil
}

type sqliteToolOperationQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func sqliteGetToolOperation(ctx context.Context, q sqliteToolOperationQuery, runID domain.RunID, operationID string) (domain.ToolOperation, bool, error) {
	var op domain.ToolOperation
	var state string
	err := q.QueryRowContext(ctx, `SELECT run_id, operation_id, tool_name, request_digest, middleware_input_arguments, arguments_digest,
		effective_arguments, state, claim_owner, result, failure, created_at, updated_at
		FROM tool_operations WHERE run_id = ? AND operation_id = ?`, runID, operationID).Scan(
		&op.RunID, &op.OperationID, &op.ToolName, &op.RequestDigest, &op.MiddlewareInputArguments, &op.ArgumentsDigest,
		&op.EffectiveArguments, &state, &op.ClaimOwner, &op.Result, &op.Failure, &op.CreatedAt, &op.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ToolOperation{}, false, nil
	}
	if err != nil {
		return domain.ToolOperation{}, false, fmt.Errorf("storage: read tool operation: %w", err)
	}
	op.State = domain.ToolOperationState(state)
	return op, true, nil
}

func sameToolOperationBinding(existing, request domain.ToolOperation) bool {
	return existing.RunID == request.RunID && existing.OperationID == request.OperationID &&
		existing.ToolName == request.ToolName && existing.RequestDigest == request.RequestDigest &&
		existing.ArgumentsDigest == request.ArgumentsDigest &&
		string(existing.MiddlewareInputArguments) == string(request.MiddlewareInputArguments) &&
		string(existing.EffectiveArguments) == string(request.EffectiveArguments)
}

func sqliteCheckToolOperationRunOpen(ctx context.Context, tx *sql.Tx, runID domain.RunID) error {
	var closed int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM run_events WHERE run_id = ? AND type IN `+terminalTypes, runID).Scan(&closed); err != nil {
		return fmt.Errorf("storage: check tool operation run status: %w", err)
	}
	if closed > 0 {
		return storage.ErrRunClosed
	}
	return nil
}

func sqliteAppendToolOperationEvent(ctx context.Context, tx *sql.Tx, op domain.ToolOperation) (domain.RunEvent, error) {
	event, err := storage.NewToolOperationEvent(op)
	if err != nil {
		return domain.RunEvent{}, err
	}
	if err := sqliteCheckToolOperationRunOpen(ctx, tx, op.RunID); err != nil {
		return domain.RunEvent{}, err
	}
	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(seq) FROM run_events WHERE run_id = ?`, op.RunID).Scan(&maxSeq); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: read tool operation Journal sequence: %w", err)
	}
	event.Seq = domain.EventSeq(maxSeq.Int64 + 1)
	if _, err := tx.ExecContext(ctx, `INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload)
		VALUES (?, ?, ?, ?, ?, ?)`, event.RunID, event.Seq, string(event.Type), event.CreatedAt, event.PayloadVersion, event.Payload); err != nil {
		return domain.RunEvent{}, fmt.Errorf("storage: append tool operation event: %w", err)
	}
	return event, nil
}
