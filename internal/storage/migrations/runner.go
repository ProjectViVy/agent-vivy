package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strconv"
	"time"
)

// Apply admits only pristine stores or complete known target formats. Finite
// normalization of historical stores belongs to the separately delivered tool.
func Apply(ctx context.Context, db *sql.DB, dialect Dialect) error {
	manifest, err := Embedded()
	if err != nil {
		return err
	}
	return ApplyManifest(ctx, db, dialect, manifest)
}

// ApplyManifest uses the same format barrier as Apply, including for callers
// supplying a catalog. It is not an offline normalization escape hatch.
func ApplyManifest(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if err := preflight(ctx, db, dialect, manifest); err != nil {
		return err
	}
	return applyManifest(ctx, db, dialect, manifest)
}

// applyManifest is reached only after production admission. Package-private
// tests may use it to construct synthetic prefixes and inject DDL failures.
func applyManifest(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) error {
	if db == nil {
		return fmt.Errorf("storage migrations: nil database")
	}
	if dialect != SQLite && dialect != Postgres {
		return fmt.Errorf("storage migrations: unsupported dialect %q", dialect)
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	tables, err := sourceTables(ctx, db, dialect)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return initialize(ctx, db, dialect, manifest)
	}
	if !tables["schema_migrations"] {
		return fmt.Errorf("%w: application tables without metadata", ErrUnsupportedSourceFormat)
	}
	columns, err := metadataColumns(ctx, db, dialect)
	if err != nil {
		return err
	}
	if !finalMetadataColumns(columns) {
		return fmt.Errorf("%w: expected final version/name/applied_at metadata; use the finite upgrade artifact", ErrUpgradeRequired)
	}
	applied, err := readApplied(ctx, db, dialect, manifest)
	if err != nil {
		return err
	}
	// The exported owner has already validated the exact target shape. Repeating
	// all catalog queries here adds no authority; this executor owns only DDL
	// ordering and transactions after admission.

	for _, item := range manifest.Migrations(dialect)[len(applied):] {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("storage migrations: begin migration %d: %w", item.Version, err)
		}
		if err = applyOne(ctx, tx, dialect, item); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("storage migrations: commit migration %d: %w", item.Version, err)
		}
	}
	return nil
}

func initialize(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) error {
	data, err := fs.ReadFile(embeddedFiles, "metadata/"+string(dialect)+".sql")
	if err != nil {
		return fmt.Errorf("storage migrations: read metadata SQL: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage migrations: begin initialization: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, string(data)); err != nil {
		return fmt.Errorf("storage migrations: init metadata: %w", err)
	}
	for _, item := range manifest.Migrations(dialect) {
		if err := applyOne(ctx, tx, dialect, item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage migrations: commit initialization: %w", err)
	}
	return nil
}

func finalMetadataColumns(columns map[string]bool) bool {
	return len(columns) == 3 && columns["version"] && columns["name"] && columns["applied_at"]
}

func readApplied(ctx context.Context, db *sql.DB, dialect Dialect, manifest Manifest) (map[int64]bool, error) {
	items := manifest.Migrations(dialect)
	rows, err := db.QueryContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("storage migrations: read migration markers: %w", err)
	}
	defer rows.Close()
	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return nil, fmt.Errorf("storage migrations: scan migration marker: %w", err)
		}
		if version != int64(len(applied)+1) || version > int64(len(items)) {
			return nil, fmt.Errorf("%w: unknown or noncontiguous migration version %d", ErrUnsupportedSourceFormat, version)
		}
		if name != items[version-1].Name {
			return nil, fmt.Errorf("%w: migration %d name drift", ErrUnsupportedSourceFormat, version)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage migrations: read migration markers: %w", err)
	}
	if len(applied) == 0 {
		return nil, fmt.Errorf("%w: empty migration metadata", ErrUnsupportedSourceFormat)
	}
	return applied, nil
}

func applyOne(ctx context.Context, tx *sql.Tx, dialect Dialect, item Migration) error {
	if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
		return fmt.Errorf("storage migrations: apply migration %d: %w", item.Version, err)
	}
	query := `INSERT INTO schema_migrations (version, name, applied_at) VALUES (` + placeholder(dialect, 1) + `, ` + placeholder(dialect, 2) + `, ` + placeholder(dialect, 3) + `)`
	if _, err := tx.ExecContext(ctx, query, item.Version, item.Name, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("storage migrations: record migration %d: %w", item.Version, err)
	}
	return nil
}

func placeholder(dialect Dialect, index int) string {
	if dialect == Postgres {
		return "$" + strconv.Itoa(index)
	}
	return "?"
}
