package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrUpgradeRequired         = errors.New("storage migrations: upgrade_required")
	ErrUnsupportedSourceFormat = errors.New("storage migrations: unsupported_source_format")
)

// preflight reads only. Old normalization/conversion belongs to an explicit
// disposable copy; startup must never CREATE or backfill an old source.
func preflight(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) error {
	if db == nil {
		return fmt.Errorf("storage migrations: nil database")
	}
	if dialect != SQLite && dialect != Postgres {
		return fmt.Errorf("storage migrations: unsupported dialect %q", dialect)
	}
	tables, err := sourceTables(ctx, db, dialect)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return nil
	}
	if !tables["schema_migrations"] {
		return fmt.Errorf("%w: application tables without metadata", ErrUnsupportedSourceFormat)
	}
	columns, err := metadataColumns(ctx, db, dialect)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupportedSourceFormat, err)
	}
	if !columns["version"] || !columns["applied_at"] || columns["name"] != columns["checksum"] {
		return fmt.Errorf("%w: partial migration metadata", ErrUnsupportedSourceFormat)
	}
	if !columns["name"] {
		return preflightLegacy(ctx, db, dialect, tables, manifest)
	}
	applied, err := readApplied(ctx, db, dialect, manifest)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupportedSourceFormat, err)
	}
	if len(applied) == 0 {
		return fmt.Errorf("%w: metadata without initialized schema", ErrUnsupportedSourceFormat)
	}
	for i := 1; i <= len(applied); i++ {
		if !applied[int64(i)] {
			return fmt.Errorf("%w: noncontiguous migration metadata", ErrUnsupportedSourceFormat)
		}
	}
	items := manifest.Migrations(dialect)
	if err := verifySourceTables(tables, items[:len(applied)]); err != nil {
		return err
	}
	if len(applied) < len(items) {
		// Only the exact accepted D03/D08 manifests are finite intermediate
		// sources. Historical lineages with other names/checksums were
		// rejected by readApplied before any shape check or write.
		if len(applied) == 36 || len(applied) == 37 {
			if err := validateOperationShape(ctx, db, dialect, true); err != nil {
				return err
			}
			if err := validateSnapshotColumns(ctx, db, dialect, len(applied) == 36); err != nil {
				return err
			}
			if err := validateGenerationShape(ctx, db, dialect, false); err != nil {
				return err
			}
			return ErrUpgradeRequired
		}
		if len(applied) > 35 {
			return fmt.Errorf("%w: unlisted post-baseline source", ErrUnsupportedSourceFormat)
		}
		if tables["generations"] {
			if err := validateGenerationShape(ctx, db, dialect, false); err != nil {
				return err
			}
		}
		if len(applied) >= 31 {
			if err := validateOperationShape(ctx, db, dialect, false); err != nil {
				return err
			}
		}
		return ErrUpgradeRequired
	}
	// A migration number alone is never evidence of the converted target.
	if err := validateGenerationShape(ctx, db, dialect, true); err != nil {
		return err
	}
	if err := validateOperationShape(ctx, db, dialect, true); err != nil {
		return err
	}
	return validateSnapshotColumns(ctx, db, dialect, false)
}

func validateSnapshotColumns(ctx context.Context, db *sql.DB, dialect Dialect, legacy bool) error {
	for table, column := range map[string]string{"run_prompt_snapshots": "payload_sha256", "checkpoints": "checksum"} {
		exists, err := sourceColumnExists(ctx, db, dialect, table, column)
		if err != nil {
			return err
		}
		if exists != legacy {
			return fmt.Errorf("%w: unexpected admitted snapshot shape", ErrUnsupportedSourceFormat)
		}
	}
	return nil
}

func validateOperationShape(ctx context.Context, db *sql.DB, dialect Dialect, bound bool) error {
	required := []string{"run_id", "operation_id", "tool_name", "middleware_input_arguments", "effective_arguments", "state", "claim_owner", "result", "failure", "created_at", "updated_at"}
	old := []string{"request_digest", "arguments_digest"}
	current := []string{"request_arguments", "outcome_status", "outcome_reason", "outcome_effects"}
	forbidden := current
	if bound {
		required = append(required, current...)
		forbidden = old
	} else {
		required = append(required, old...)
	}
	for _, column := range required {
		exists, err := sourceColumnExists(ctx, db, dialect, "tool_operations", column)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: missing tool operation column %s", ErrUnsupportedSourceFormat, column)
		}
	}
	for _, column := range forbidden {
		exists, err := sourceColumnExists(ctx, db, dialect, "tool_operations", column)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%w: unexpected tool operation column %s", ErrUnsupportedSourceFormat, column)
		}
	}
	return nil
}

func sourceTables(ctx context.Context, db *sql.DB, dialect Dialect) (map[string]bool, error) {
	query := `SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name NOT GLOB 'sqlite_*'`
	if dialect == Postgres {
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema()`
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("storage migrations: inspect source tables: %w", err)
	}
	defer rows.Close()
	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables[name] = true
	}
	return tables, rows.Err()
}

var createdTablePattern = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)

func verifySourceTables(tables map[string]bool, items []Migration) error {
	expected := map[string]bool{"schema_migrations": true}
	for _, item := range items {
		for _, match := range createdTablePattern.FindAllStringSubmatch(item.SQL, -1) {
			expected[match[1]] = true
			if !tables[match[1]] {
				// Released-017 repair sources are normalized only by the old
				// explicit converter. Startup recognizes them without repair.
				head := len(items)
				if match[1] == "cron_jobs" && (head == 16 || head == 17 || head == 23 || head == 35) {
					continue
				}
				return fmt.Errorf("%w: missing application table %s", ErrUnsupportedSourceFormat, match[1])
			}
		}
	}
	for table := range tables {
		if !expected[table] {
			return fmt.Errorf("%w: unexplained application table %s", ErrUnsupportedSourceFormat, table)
		}
	}
	return nil
}

func preflightLegacy(ctx context.Context, db *sql.DB, dialect Dialect, tables map[string]bool, manifest Manifest) error {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupportedSourceFormat, err)
	}
	var versions []int64
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			_ = rows.Close()
			return err
		}
		versions = append(versions, version)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("%w: empty legacy metadata", ErrUnsupportedSourceFormat)
	}
	for _, name := range []string{"sessions", "runs", "run_events"} {
		if !tables[name] {
			return fmt.Errorf("%w: incomplete legacy schema", ErrUnsupportedSourceFormat)
		}
	}
	max := versions[len(versions)-1]
	if dialect == Postgres {
		for _, version := range versions {
			if version <= 0 {
				return fmt.Errorf("%w: invalid legacy version", ErrUnsupportedSourceFormat)
			}
		}
		shape, err := inspectPostgresLegacyShape(ctx, db)
		if err != nil {
			return err
		}
		if _, err := legacyPostgresState(max, shape); err != nil {
			return fmt.Errorf("%w: %w", ErrUnsupportedSourceFormat, err)
		}
		// PostgreSQL 001 is an accumulated bootstrap, broader than compressed
		// historical stores. Their required shape is checked above.
		items := manifest.Migrations(dialect)[:23]
		base := map[string]bool{}
		for _, match := range createdTablePattern.FindAllStringSubmatch(items[0].SQL, -1) {
			base[match[1]] = true
		}
		for _, item := range items[13:] {
			for _, match := range createdTablePattern.FindAllStringSubmatch(item.SQL, -1) {
				delete(base, match[1])
			}
		}
		for table := range base {
			if !tables[table] {
				return fmt.Errorf("%w: missing legacy table %s", ErrUnsupportedSourceFormat, table)
			}
		}
		allowed := map[string]bool{"schema_migrations": true}
		for _, item := range items {
			for _, match := range createdTablePattern.FindAllStringSubmatch(item.SQL, -1) {
				allowed[match[1]] = true
			}
		}
		for table := range tables {
			if !allowed[table] {
				return fmt.Errorf("%w: unexplained legacy table %s", ErrUnsupportedSourceFormat, table)
			}
		}
		return ErrUpgradeRequired
	} else {
		if max > 35 {
			return fmt.Errorf("%w: unknown legacy version", ErrUnsupportedSourceFormat)
		}
		for i, version := range versions {
			if version != int64(i+1) {
				return fmt.Errorf("%w: incomplete legacy history", ErrUnsupportedSourceFormat)
			}
		}
	}
	if err := verifySourceTables(tables, manifest.Migrations(dialect)[:max]); err != nil {
		return err
	}
	return ErrUpgradeRequired
}

func sourceColumnExists(ctx context.Context, db *sql.DB, dialect Dialect, table, column string) (bool, error) {
	if dialect == Postgres {
		return postgresColumnExists(ctx, db, table, column)
	}
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("storage migrations: inspect source column: %w", err)
	}
	return count == 1, nil
}

func validateGenerationShape(ctx context.Context, db *sql.DB, dialect Dialect, current bool) error {
	for _, column := range []string{"id", "parent_id", "source_ref", "recipe_json", "phase", "created_at"} {
		exists, err := sourceColumnExists(ctx, db, dialect, "generations", column)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: missing Generation column %s", ErrUnsupportedSourceFormat, column)
		}
	}
	exists, err := sourceColumnExists(ctx, db, dialect, "generations", "artifact_sha256")
	if err != nil {
		return err
	}
	if current && exists {
		return fmt.Errorf("%w: retired Generation artifact hash column", ErrUnsupportedSourceFormat)
	}
	if !current && !exists {
		return fmt.Errorf("%w: missing legacy Generation artifact column", ErrUnsupportedSourceFormat)
	}
	return nil
}
