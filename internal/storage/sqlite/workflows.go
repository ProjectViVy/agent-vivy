package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var (
	_ storage.WorkflowStore    = (*Backend)(nil)
	_ storage.WorkflowRunStore = (*Backend)(nil)
)

// SaveDefinition appends one revision. Rev zero asks the backend to allocate
// latest+1; an explicit revision must be exactly latest+1. The check and
// insert share a transaction so a stale writer fails closed as a version
// conflict instead of creating a gap.
func (b *Backend) SaveDefinition(ctx context.Context, record storage.WorkflowDefinitionRecord) error {
	if strings.TrimSpace(record.ID) == "" {
		return fmt.Errorf("storage: workflow definition id is required")
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin workflow definition %s: %w", record.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	var latest sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT rev FROM workflow_definitions WHERE id = ? ORDER BY rev DESC LIMIT 1`, record.ID).Scan(&latest); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("storage: read workflow definition head %s: %w", record.ID, err)
	}
	expected := int64(1)
	if latest.Valid {
		expected = latest.Int64 + 1
	}
	if record.Rev == 0 {
		record.Rev = expected
	}
	if record.Rev != expected {
		return fmt.Errorf("%w: workflow definition %s revision %d, want %d", storage.ErrVersionConflict, record.ID, record.Rev, expected)
	}
	definition := append([]byte(nil), record.Definition...)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_definitions (id, rev, content_hash, definition, created_at_ms)
		VALUES (?, ?, ?, ?, ?)`, record.ID, record.Rev, record.Hash, definition, record.CreatedAt); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("%w: workflow definition %s@%d", storage.ErrVersionConflict, record.ID, record.Rev)
		}
		return fmt.Errorf("storage: save workflow definition %s@%d: %w", record.ID, record.Rev, err)
	}
	if err := tx.Commit(); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("%w: workflow definition %s@%d", storage.ErrVersionConflict, record.ID, record.Rev)
		}
		return fmt.Errorf("storage: commit workflow definition %s@%d: %w", record.ID, record.Rev, err)
	}
	return nil
}

func (b *Backend) GetDefinition(ctx context.Context, id string, rev int64) (storage.WorkflowDefinitionRecord, error) {
	var record storage.WorkflowDefinitionRecord
	err := b.db.QueryRowContext(ctx, `
		SELECT id, rev, content_hash, definition, created_at_ms
		FROM workflow_definitions WHERE id = ? AND rev = ?`, id, rev).
		Scan(&record.ID, &record.Rev, &record.Hash, &record.Definition, &record.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowDefinitionRecord{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.WorkflowDefinitionRecord{}, fmt.Errorf("storage: get workflow definition %s@%d: %w", id, rev, err)
	}
	record.Definition = append([]byte(nil), record.Definition...)
	return record, nil
}

func (b *Backend) LatestDefinition(ctx context.Context, id string) (storage.WorkflowDefinitionRecord, error) {
	var record storage.WorkflowDefinitionRecord
	err := b.db.QueryRowContext(ctx, `
		SELECT id, rev, content_hash, definition, created_at_ms
		FROM workflow_definitions WHERE id = ? ORDER BY rev DESC LIMIT 1`, id).
		Scan(&record.ID, &record.Rev, &record.Hash, &record.Definition, &record.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowDefinitionRecord{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.WorkflowDefinitionRecord{}, fmt.Errorf("storage: latest workflow definition %s: %w", id, err)
	}
	record.Definition = append([]byte(nil), record.Definition...)
	return record, nil
}

func (b *Backend) ListDefinitions(ctx context.Context) ([]storage.WorkflowDefinitionRecord, error) {
	rows, err := b.db.QueryContext(ctx, `
		SELECT d.id, d.rev, d.content_hash, d.definition, d.created_at_ms
		FROM workflow_definitions d
		WHERE d.rev = (SELECT MAX(latest.rev) FROM workflow_definitions latest WHERE latest.id = d.id)
		ORDER BY d.id`)
	if err != nil {
		return nil, fmt.Errorf("storage: list workflow definitions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var records []storage.WorkflowDefinitionRecord
	for rows.Next() {
		var record storage.WorkflowDefinitionRecord
		if err := rows.Scan(&record.ID, &record.Rev, &record.Hash, &record.Definition, &record.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan workflow definition: %w", err)
		}
		record.Definition = append([]byte(nil), record.Definition...)
		records = append(records, record)
	}
	return records, rows.Err()
}

func (b *Backend) SaveWorkflowRun(ctx context.Context, record storage.WorkflowRunRecord) error {
	capabilities := append([]byte(nil), record.CapabilityJSON...)
	outcomes := append([]byte(nil), record.NodeOutcomes...)
	if capabilities == nil {
		capabilities = []byte{}
	}
	if outcomes == nil {
		outcomes = []byte{}
	}
	if _, err := b.db.ExecContext(ctx, `
		INSERT INTO workflow_runs
		(run_id, workflow_id, workflow_rev, workflow_hash, capability_json, session_id, status, reason, node_outcomes_json, created_at_ms, updated_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.RunID, record.WorkflowID, record.Rev, record.Hash, capabilities, record.SessionID,
		record.Status, record.Reason, outcomes, record.CreatedAt, record.UpdatedAt); err != nil {
		return fmt.Errorf("storage: save workflow run %s: %w", record.RunID, err)
	}
	return nil
}

func (b *Backend) GetWorkflowRun(ctx context.Context, runID domain.RunID) (storage.WorkflowRunRecord, error) {
	var record storage.WorkflowRunRecord
	var status string
	err := b.db.QueryRowContext(ctx, `
		SELECT run_id, workflow_id, workflow_rev, workflow_hash, capability_json, session_id, status, reason, node_outcomes_json, created_at_ms, updated_at_ms
		FROM workflow_runs WHERE run_id = ?`, runID).
		Scan(&record.RunID, &record.WorkflowID, &record.Rev, &record.Hash, &record.CapabilityJSON, &record.SessionID, &status, &record.Reason, &record.NodeOutcomes, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.WorkflowRunRecord{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.WorkflowRunRecord{}, fmt.Errorf("storage: get workflow run %s: %w", runID, err)
	}
	record.Status = domain.RunStatus(status)
	record.CapabilityJSON = append([]byte(nil), record.CapabilityJSON...)
	record.NodeOutcomes = append([]byte(nil), record.NodeOutcomes...)
	return record, nil
}

func (b *Backend) ListWorkflowRunsByDefinition(ctx context.Context, id string, limit int) ([]storage.WorkflowRunRecord, error) {
	if limit <= 0 {
		return []storage.WorkflowRunRecord{}, nil
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := b.db.QueryContext(ctx, `
		SELECT run_id, workflow_id, workflow_rev, workflow_hash, capability_json, session_id, status, reason, node_outcomes_json, created_at_ms, updated_at_ms
		FROM workflow_runs WHERE workflow_id = ? ORDER BY created_at_ms DESC, run_id DESC LIMIT ?`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list workflow runs for %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()
	var records []storage.WorkflowRunRecord
	for rows.Next() {
		var record storage.WorkflowRunRecord
		var status string
		if err := rows.Scan(&record.RunID, &record.WorkflowID, &record.Rev, &record.Hash, &record.CapabilityJSON, &record.SessionID, &status, &record.Reason, &record.NodeOutcomes, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan workflow run: %w", err)
		}
		record.Status = domain.RunStatus(status)
		record.CapabilityJSON = append([]byte(nil), record.CapabilityJSON...)
		record.NodeOutcomes = append([]byte(nil), record.NodeOutcomes...)
		records = append(records, record)
	}
	return records, rows.Err()
}

func (b *Backend) SetWorkflowRunTerminal(ctx context.Context, runID domain.RunID, status domain.RunStatus, reason string, outcomes []byte) error {
	if !status.Terminal() {
		return fmt.Errorf("storage: workflow terminal status %q is not terminal", status)
	}
	if outcomes == nil {
		outcomes = []byte{}
	}
	res, err := b.db.ExecContext(ctx, `
		UPDATE workflow_runs SET status = ?, reason = ?, node_outcomes_json = ?, updated_at_ms = ?
		WHERE run_id = ? AND status IN ('accepted', 'queued', 'active')`,
		status, reason, append([]byte(nil), outcomes...), time.Now().UnixMilli(), runID)
	if err != nil {
		return fmt.Errorf("storage: set workflow run %s terminal: %w", runID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: set workflow run %s terminal rows: %w", runID, err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
