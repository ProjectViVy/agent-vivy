package bml

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func opaqueID(prefix string) string { return prefix + "-" + rand.Text() }

func formatError(code StoreErrorCode) *StoreError {
	return &StoreError{Code: code, Message: "BML " + string(code) + ": use the finite upgrade artifact on a copy; source left untouched"}
}

// inspectStore opens existing data read-only before any WAL pragma, CREATE or
// repair. Only a genuinely empty database can enter fresh initialization.
func inspectStore(ctx context.Context, path, expected string) (owner string, pristine bool, err error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", true, nil
	}
	if err != nil {
		return "", false, ioErr(path, err)
	}
	if info.IsDir() {
		return "", false, formatError(ErrUnsupportedSourceFormat)
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(%d)", path, busyTimeoutMS))
	if err != nil {
		return "", false, ioErr(path, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	rows, err := db.QueryContext(ctx, "SELECT name, type FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'")
	if err != nil {
		return "", false, formatError(ErrUnsupportedSourceFormat)
	}
	tables := map[string]string{}
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			rows.Close()
			return "", false, persistenceErr(path, err)
		}
		tables[name] = kind
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", false, persistenceErr(path, err)
	}
	if len(tables) == 0 {
		return "", true, nil
	}
	if tables["schema_meta"] != "table" {
		return "", false, formatError(ErrUnsupportedSourceFormat)
	}
	var version, count int64
	if err := db.QueryRowContext(ctx, "SELECT schema_version, workspace_id FROM schema_meta WHERE component = ?", component).Scan(&version, &owner); err != nil {
		return "", false, formatError(ErrUnsupportedSourceFormat)
	}
	if (version != 1 && version != schemaVersion) || strings.TrimSpace(owner) == "" {
		return "", false, formatError(ErrUnsupportedSourceFormat)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_meta").Scan(&count); err != nil || count != 1 {
		return "", false, formatError(ErrUnsupportedSourceFormat)
	}
	required := map[string]string{
		"schema_meta":       "component,schema_version,store_revision,record_count,content_bytes,workspace_id",
		"memory_records":    "memory_id,record_revision,kind,tenant_id,workspace_id,session_id,trust,sensitivity,created_at,effective_at,expires_at,tombstone,content_bytes,record_json",
		"memory_supersedes": "memory_id,superseded_id",
		"memory_fts":        "memory_id,tenant_id,workspace_id,session_id,content",
	}
	if version == 1 {
		required["memory_apply_journal"] = "idempotency_key,proposal_id,request_id,content_digest,record_id,store_revision,record_revision,actor_id,applied_at"
	}
	for name, columns := range required {
		if tables[name] != "table" {
			return "", false, formatError(ErrUnsupportedSourceFormat)
		}
		if err := inspectColumns(ctx, db, name, strings.Split(columns, ",")); err != nil {
			return "", false, err
		}
	}
	shadow := map[string]bool{"memory_fts_data": true, "memory_fts_idx": true, "memory_fts_content": true, "memory_fts_docsize": true, "memory_fts_config": true}
	for name, kind := range tables {
		if _, ok := required[name]; !ok && !shadow[name] && kind != "index" {
			return "", false, formatError(ErrUnsupportedSourceFormat)
		}
	}
	if version == 1 {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM memory_apply_journal").Scan(&count); err != nil || count != 0 {
			return "", false, formatError(ErrUnsupportedSourceFormat)
		}
		return "", false, formatError(ErrUpgradeRequired)
	}
	if expected != "" && owner != expected {
		return "", false, databaseWorkspaceMismatchErr(expected, owner)
	}
	return owner, false, nil
}

func inspectColumns(ctx context.Context, db *sql.DB, table string, expected []string) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return formatError(ErrUnsupportedSourceFormat)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var ordinal, nonnull, primary int
		var name, typ string
		var fallback any
		if err := rows.Scan(&ordinal, &name, &typ, &nonnull, &fallback, &primary); err != nil {
			return formatError(ErrUnsupportedSourceFormat)
		}
		if i >= len(expected) || name != expected[i] {
			return formatError(ErrUnsupportedSourceFormat)
		}
		i++
	}
	if rows.Err() != nil || i != len(expected) {
		return formatError(ErrUnsupportedSourceFormat)
	}
	return nil
}

// rejectRetiredFields prevents legacy JSON from being silently admitted as v2.
func rejectRetiredFields(data []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	for _, field := range fields {
		if _, ok := object[field]; ok {
			return formatError(ErrUpgradeRequired)
		}
	}
	return nil
}
