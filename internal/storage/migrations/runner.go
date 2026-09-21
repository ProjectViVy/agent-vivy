package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

// Apply validates and applies the executable's embedded migration catalog.
func Apply(ctx context.Context, db *sql.DB, dialect Dialect) error {
	manifest, err := Embedded()
	if err != nil {
		return err
	}
	return ApplyManifest(ctx, db, dialect, manifest)
}

// ApplyManifest applies a validated catalog. It is exported for conformance
// tests that need to exercise rollback and failure behavior with a small
// temporary catalog; production callers should use Apply.
func ApplyManifest(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) error {
	if db == nil {
		return fmt.Errorf("storage migrations: nil database")
	}
	if dialect != SQLite && dialect != Postgres {
		return fmt.Errorf("storage migrations: unsupported dialect %q", dialect)
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	legacyMetadata, err := ensureMetadata(ctx, db, dialect)
	if err != nil {
		return err
	}
	if dialect == Postgres && legacyMetadata {
		if err := normalizeLegacyPostgres(ctx, db, manifest); err != nil {
			return err
		}
	} else if err := backfillMetadata(ctx, db, dialect, manifest); err != nil {
		return err
	}

	applied, err := readApplied(ctx, db, dialect, manifest)
	if err != nil {
		return err
	}
	for _, migration := range manifest.Migrations(dialect) {
		if applied[migration.Version] {
			continue
		}
		if err := applyOne(ctx, db, dialect, migration); err != nil {
			return err
		}
	}
	return nil
}

func ensureMetadata(ctx context.Context, db *sql.DB, dialect Dialect) (bool, error) {
	data, err := fs.ReadFile(embeddedFiles, "metadata/"+string(dialect)+".sql")
	if err != nil {
		return false, fmt.Errorf("storage migrations: read %s metadata SQL: %w", dialect, err)
	}
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		return false, fmt.Errorf("storage migrations: init metadata table: %w", err)
	}
	columns, err := metadataColumns(ctx, db, dialect)
	if err != nil {
		return false, err
	}
	hasName, hasChecksum := columns["name"], columns["checksum"]
	if hasName != hasChecksum {
		return false, fmt.Errorf("storage migrations: partial metadata columns: name=%t checksum=%t", hasName, hasChecksum)
	}
	if hasName {
		return false, nil
	}
	legacy, err := fs.ReadFile(embeddedFiles, "metadata/"+string(dialect)+"_add_metadata.sql")
	if err != nil {
		return false, fmt.Errorf("storage migrations: read %s metadata upgrade SQL: %w", dialect, err)
	}
	if _, err := db.ExecContext(ctx, string(legacy)); err != nil {
		return false, fmt.Errorf("storage migrations: upgrade metadata table: %w", err)
	}
	columns, err = metadataColumns(ctx, db, dialect)
	if err != nil {
		return false, err
	}
	if !columns["name"] || !columns["checksum"] {
		return false, fmt.Errorf("storage migrations: metadata upgrade did not create name/checksum columns")
	}
	return true, nil
}

func metadataColumns(ctx context.Context, db *sql.DB, dialect Dialect) (map[string]bool, error) {
	columns := make(map[string]bool)
	var rows *sql.Rows
	var err error
	if dialect == SQLite {
		rows, err = db.QueryContext(ctx, `PRAGMA table_info(schema_migrations)`)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'schema_migrations'`)
	}
	if err != nil {
		return nil, fmt.Errorf("storage migrations: inspect metadata columns: %w", err)
	}
	defer rows.Close()
	if dialect == SQLite {
		for rows.Next() {
			var cid, notNull, pk int
			var name, dataType string
			var defaultValue any
			if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
				return nil, fmt.Errorf("storage migrations: scan sqlite metadata column: %w", err)
			}
			columns[name] = true
		}
	} else {
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, fmt.Errorf("storage migrations: scan postgres metadata column: %w", err)
			}
			columns[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage migrations: inspect metadata columns: %w", err)
	}
	return columns, nil
}

type postgresLegacyShape struct {
	sessionCompactions  bool
	channelProvenance   bool
	cronJobs            bool
	messageAttachments  bool
	fileVersions        bool
	fileReads           bool
	sessionTruncations  bool
	messageFileContexts bool
	sessionUpdatedAt    bool
	workspacePath       bool
}

func legacyPostgresState(maxVersion int64, shape postgresLegacyShape) (int64, error) {
	if maxVersion < 13 || maxVersion > 21 {
		return 0, fmt.Errorf("storage migrations: unsupported legacy postgres migration version %d", maxVersion)
	}
	if shape.workspacePath && !shape.sessionUpdatedAt {
		return 0, fmt.Errorf("storage migrations: legacy postgres workspace_path without sessions.updated_at")
	}
	if shape.sessionUpdatedAt && !shape.messageFileContexts {
		return 0, fmt.Errorf("storage migrations: legacy postgres sessions.updated_at without message_file_contexts")
	}
	if shape.messageFileContexts && !shape.fileVersions {
		return 0, fmt.Errorf("storage migrations: legacy postgres message_file_contexts without file_versions")
	}
	if shape.sessionTruncations && !shape.fileVersions {
		return 0, fmt.Errorf("storage migrations: legacy postgres session_truncations without file_versions")
	}
	if shape.fileVersions && !shape.messageAttachments {
		return 0, fmt.Errorf("storage migrations: legacy postgres file_versions without message_attachments")
	}
	if shape.fileVersions != shape.fileReads {
		return 0, fmt.Errorf("storage migrations: legacy postgres file_versions/file_reads shape is incomplete")
	}
	if shape.messageAttachments && (!shape.channelProvenance || !shape.cronJobs) {
		return 0, fmt.Errorf("storage migrations: legacy postgres message_attachments without channel and cron schema")
	}
	if (shape.channelProvenance || shape.cronJobs) && !shape.sessionCompactions {
		return 0, fmt.Errorf("storage migrations: legacy postgres channel/cron schema without session_compactions")
	}

	// The historical runner recorded compressed PostgreSQL versions. It also
	// had a release window in which the v14 -> v21 upgrade path skipped the
	// truncation table even though the fresh bootstrap already contained it.
	// Marker requirements below reject unrelated partial shapes, while the
	// contiguous state calculation deliberately leaves that one known hole
	// pending so migration 020 can repair it.
	if maxVersion >= 14 && !shape.sessionCompactions {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d without session_compactions", maxVersion)
	}
	if maxVersion >= 16 && (!shape.channelProvenance || !shape.cronJobs) {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d without channel and cron schema", maxVersion)
	}
	if maxVersion >= 17 && !shape.messageAttachments {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d without message_attachments", maxVersion)
	}
	if maxVersion >= 18 && !shape.fileVersions {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d without file_versions", maxVersion)
	}
	if maxVersion >= 19 && !shape.messageFileContexts {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d without message_file_contexts", maxVersion)
	}
	if maxVersion >= 20 && !shape.sessionUpdatedAt {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d without sessions.updated_at", maxVersion)
	}
	if maxVersion >= 21 && !shape.workspacePath {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d without sessions.workspace_path", maxVersion)
	}

	state := int64(13)
	if shape.sessionCompactions {
		state = 15
	}
	if shape.channelProvenance && shape.cronJobs {
		state = 16
	}
	if shape.messageAttachments {
		state = 18
	}
	if shape.fileVersions {
		state = 19
	}
	if shape.sessionTruncations {
		state = 20
	}
	if shape.messageFileContexts && shape.sessionTruncations {
		state = 21
	}
	if shape.sessionUpdatedAt && shape.messageFileContexts && shape.sessionTruncations {
		state = 22
	}
	if shape.workspacePath && shape.sessionUpdatedAt && shape.messageFileContexts && shape.sessionTruncations {
		state = 23
	}
	minimum := int64(13)
	switch maxVersion {
	case 13:
		minimum = 13
	case 14, 15:
		minimum = 15
	case 16:
		minimum = 16
	case 17:
		minimum = 18
	case 18, 19, 20, 21:
		minimum = 19
	}
	if state < minimum {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d is inconsistent with observed schema state %d", maxVersion, state)
	}
	return state, nil
}

func normalizeLegacyPostgres(ctx context.Context, db *sql.DB, manifest Manifest) error {
	var max sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&max); err != nil {
		return fmt.Errorf("storage migrations: read legacy postgres marker: %w", err)
	}
	if !max.Valid {
		return nil
	}
	shape, err := inspectPostgresLegacyShape(ctx, db)
	if err != nil {
		return err
	}
	state, err := legacyPostgresState(max.Int64, shape)
	if err != nil {
		return err
	}
	items := manifest.Migrations(Postgres)
	if state > int64(len(items)) {
		return fmt.Errorf("storage migrations: legacy postgres state %d exceeds manifest length %d", state, len(items))
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage migrations: begin legacy postgres normalization: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations`); err != nil {
		return fmt.Errorf("storage migrations: clear legacy postgres markers: %w", err)
	}
	insert := `INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (` +
		placeholder(Postgres, 1) + `, ` + placeholder(Postgres, 2) + `, ` + placeholder(Postgres, 3) + `, ` + placeholder(Postgres, 4) + `)`
	for _, migration := range items[:state] {
		if _, err := tx.ExecContext(ctx, insert, migration.Version, migration.Name, migration.Checksum, time.Now().UnixMilli()); err != nil {
			return fmt.Errorf("storage migrations: record canonical postgres migration %d: %w", migration.Version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage migrations: commit legacy postgres normalization: %w", err)
	}
	return nil
}

func inspectPostgresLegacyShape(ctx context.Context, db *sql.DB) (postgresLegacyShape, error) {
	shape := postgresLegacyShape{}
	var err error
	if shape.sessionCompactions, err = postgresTableExists(ctx, db, "session_compactions"); err != nil {
		return shape, err
	}
	if shape.cronJobs, err = postgresTableExists(ctx, db, "cron_jobs"); err != nil {
		return shape, err
	}
	if shape.messageAttachments, err = postgresTableExists(ctx, db, "message_attachments"); err != nil {
		return shape, err
	}
	if shape.fileVersions, err = postgresTableExists(ctx, db, "file_versions"); err != nil {
		return shape, err
	}
	if shape.fileReads, err = postgresTableExists(ctx, db, "file_reads"); err != nil {
		return shape, err
	}
	if shape.sessionTruncations, err = postgresTableExists(ctx, db, "session_truncations"); err != nil {
		return shape, err
	}
	if shape.messageFileContexts, err = postgresTableExists(ctx, db, "message_file_contexts"); err != nil {
		return shape, err
	}
	shape.channelProvenance = true
	for _, column := range []string{"source", "channel", "chat_id", "channel_message_id"} {
		exists, columnErr := postgresColumnExists(ctx, db, "messages", column)
		if columnErr != nil {
			return shape, columnErr
		}
		shape.channelProvenance = shape.channelProvenance && exists
	}
	if shape.sessionUpdatedAt, err = postgresColumnExists(ctx, db, "sessions", "updated_at"); err != nil {
		return shape, err
	}
	if shape.workspacePath, err = postgresColumnExists(ctx, db, "sessions", "workspace_path"); err != nil {
		return shape, err
	}
	return shape, nil
}

func postgresTableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ` + placeholder(Postgres, 1)
	if err := db.QueryRowContext(ctx, query, table).Scan(&count); err != nil {
		return false, fmt.Errorf("storage migrations: inspect postgres table %s: %w", table, err)
	}
	return count == 1, nil
}

func postgresColumnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ` + placeholder(Postgres, 1) + ` AND column_name = ` + placeholder(Postgres, 2)
	if err := db.QueryRowContext(ctx, query, table, column).Scan(&count); err != nil {
		return false, fmt.Errorf("storage migrations: inspect postgres column %s.%s: %w", table, column, err)
	}
	return count == 1, nil
}

func backfillMetadata(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) error {
	items := manifest.Migrations(dialect)
	expected := make(map[int64]Migration, len(items))
	for _, item := range items {
		expected[item.Version] = item
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("storage migrations: read applied metadata: %w", err)
	}
	defer rows.Close()
	type legacyRow struct {
		version  int64
		name     string
		checksum string
	}
	var rowsToBackfill []legacyRow
	for rows.Next() {
		var row legacyRow
		if err := rows.Scan(&row.version, &row.name, &row.checksum); err != nil {
			return fmt.Errorf("storage migrations: scan applied metadata: %w", err)
		}
		migration, ok := expected[row.version]
		if !ok {
			return fmt.Errorf("storage migrations: database records unknown migration version %d", row.version)
		}
		if row.name == "" && row.checksum == "" {
			rowsToBackfill = append(rowsToBackfill, legacyRow{version: row.version, name: migration.Name, checksum: migration.Checksum})
			continue
		}
		if row.name == "" || row.checksum == "" {
			return fmt.Errorf("storage migrations: incomplete metadata for migration %d", row.version)
		}
		if row.name != migration.Name {
			return fmt.Errorf("storage migrations: migration %d name drift: database=%q embedded=%q", row.version, row.name, migration.Name)
		}
		if row.checksum != migration.Checksum {
			return fmt.Errorf("storage migrations: migration %d checksum drift: database=%q embedded=%q", row.version, row.checksum, migration.Checksum)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("storage migrations: read applied metadata: %w", err)
	}
	if len(rowsToBackfill) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage migrations: begin metadata backfill: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	query := `UPDATE schema_migrations SET name = ` + placeholder(dialect, 1) + `, checksum = ` + placeholder(dialect, 2) + ` WHERE version = ` + placeholder(dialect, 3)
	for _, row := range rowsToBackfill {
		if _, err := tx.ExecContext(ctx, query, row.name, row.checksum, row.version); err != nil {
			return fmt.Errorf("storage migrations: backfill migration %d: %w", row.version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage migrations: commit metadata backfill: %w", err)
	}
	return nil
}

func readApplied(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) (map[int64]bool, error) {
	expected := make(map[int64]Migration)
	for _, migration := range manifest.Migrations(dialect) {
		expected[migration.Version] = migration
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("storage migrations: read migration markers: %w", err)
	}
	defer rows.Close()
	applied := make(map[int64]bool, len(expected))
	for rows.Next() {
		var version int64
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			return nil, fmt.Errorf("storage migrations: scan migration marker: %w", err)
		}
		migration, ok := expected[version]
		if !ok {
			return nil, fmt.Errorf("storage migrations: database records unknown migration version %d", version)
		}
		if name != migration.Name || checksum != migration.Checksum {
			return nil, fmt.Errorf("storage migrations: migration %d checksum drift: database=%q/%q embedded=%q/%q", version, name, checksum, migration.Name, migration.Checksum)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage migrations: read migration markers: %w", err)
	}
	return applied, nil
}

func applyOne(ctx context.Context, db *sql.DB, dialect Dialect, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage migrations: begin migration %d: %w", migration.Version, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("storage migrations: apply migration %d: %w", migration.Version, err)
	}
	query := `INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (` +
		placeholder(dialect, 1) + `, ` + placeholder(dialect, 2) + `, ` + placeholder(dialect, 3) + `, ` + placeholder(dialect, 4) + `)`
	if _, err := tx.ExecContext(ctx, query, migration.Version, migration.Name, migration.Checksum, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("storage migrations: record migration %d: %w", migration.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage migrations: commit migration %d: %w", migration.Version, err)
	}
	return nil
}

func placeholder(dialect Dialect, index int) string {
	if dialect == Postgres {
		return "$" + strings.TrimSpace(fmt.Sprintf("%d", index))
	}
	return "?"
}
