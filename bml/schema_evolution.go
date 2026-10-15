package bml

import (
	"context"
	"database/sql"
	"errors"
)

// Schema 2 keeps released record JSON and FTS intact. Old rows have no owner;
// the upgrade never infers subject/domain ownership or historical outcomes.
// These tables are created with the existing schema/identity transaction.
var boundSchemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS memory_ownership (
	    memory_id TEXT PRIMARY KEY REFERENCES memory_records(memory_id),
	    binding_json TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS memory_operations (
	    operation_id TEXT PRIMARY KEY,
	    binding_json TEXT NOT NULL,
	    payload_digest TEXT NOT NULL,
	    request_fingerprint TEXT NOT NULL,
	    outcome_json TEXT NOT NULL
	)`,
}

// Workspace-identity rewriting cannot alter the canonical scope used by any
// retained native fingerprint. Check historical operations as well as records.
func (s *Store) hasBoundState(ctx context.Context) (bool, error) {
	if s.schema < 2 {
		return false, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM memory_ownership)+(SELECT count(*) FROM memory_operations)`).Scan(&count); err != nil {
		return false, persistenceErr(s.path, err)
	}
	return count > 0, nil
}

// Version 2 never repairs a missing receipt table by inventing an empty one.
// Checking required columns also rejects reserved-name collisions during v1
// upgrade before the version change commits.
func (s *Store) checkBoundSchema(ctx context.Context, q rowQuerier) error {
	for _, query := range []string{
		`SELECT memory_id,binding_json FROM memory_ownership LIMIT 0`,
		`SELECT operation_id,binding_json,payload_digest,request_fingerprint,outcome_json FROM memory_operations LIMIT 0`,
	} {
		if err := q.QueryRowContext(ctx, query).Scan(); !errors.Is(err, sql.ErrNoRows) {
			if err == nil {
				return corruptRecordErr()
			}
			return persistenceErr(s.path, err)
		}
	}
	return nil
}
