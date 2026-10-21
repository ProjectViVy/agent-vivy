package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

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
	if maxVersion < 13 || maxVersion > 23 {
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
	case 18, 19, 20, 21, 22, 23:
		minimum = 19
	}
	if state < minimum {
		return 0, fmt.Errorf("storage migrations: legacy postgres marker %d is inconsistent with observed schema state %d", maxVersion, state)
	}
	return state, nil
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

// validateFinalMetadata checks the structure as well as its column names.
// A marker in a nullable or keyless lookalike is not conversion evidence.
func validateFinalMetadata(ctx context.Context, db *sql.DB, dialect Dialect) error {
	query := `SELECT COUNT(*) FROM pragma_table_info('schema_migrations') WHERE
 (name='version' AND upper(type)='INTEGER' AND pk=1) OR
 (name='name' AND upper(type)='TEXT' AND "notnull"=1 AND pk=0) OR
 (name='applied_at' AND upper(type)='INTEGER' AND "notnull"=1 AND pk=0)`
	if dialect == Postgres {
		query = `SELECT COUNT(*) FROM information_schema.columns c
  WHERE c.table_schema=current_schema() AND c.table_name='schema_migrations' AND c.is_nullable='NO' AND (
   (c.column_name='version' AND c.udt_name='int8') OR
   (c.column_name='name' AND c.udt_name='text') OR
   (c.column_name='applied_at' AND c.udt_name='int8'))`
	}
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return fmt.Errorf("storage migrations: inspect final metadata: %w", err)
	}
	if count != 3 {
		return fmt.Errorf("%w: invalid final metadata types or constraints", ErrUnsupportedSourceFormat)
	}
	if dialect == Postgres {
		query = `SELECT COUNT(*), COALESCE(MIN(k.column_name),'') FROM information_schema.table_constraints t
  JOIN information_schema.key_column_usage k ON k.constraint_catalog=t.constraint_catalog AND k.constraint_schema=t.constraint_schema AND k.constraint_name=t.constraint_name
  WHERE t.table_schema=current_schema() AND t.table_name='schema_migrations' AND t.constraint_type='PRIMARY KEY'`
		var name string
		if err := db.QueryRowContext(ctx, query).Scan(&count, &name); err != nil {
			return fmt.Errorf("storage migrations: inspect metadata key: %w", err)
		}
		if count != 1 || name != "version" {
			return fmt.Errorf("%w: invalid final metadata key", ErrUnsupportedSourceFormat)
		}
	}
	return nil
}
