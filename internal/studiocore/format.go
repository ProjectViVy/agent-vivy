package studiocore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// These errors identify the independent Studio format, never the species Journal.
var (
	ErrUpgradeRequired         = errors.New("studiocore: upgrade_required: convert a quiescent ledger copy with the finite Studio upgrade artifact")
	ErrUnsupportedSourceFormat = errors.New("studiocore: unsupported_source_format: restore a supported Studio ledger backup")
)

var ledgerColumns = map[string]string{
	"worktrees":     "id,path,kind,dirty,created_at",
	"generations":   "id,parent_id,source_ref,recipe_json,phase,created_at",
	"eval_runs":     "id,candidate_id,baseline_id,suite,verdict,journal_ref,created_at",
	"releases":      "id,generation_id,eval_id,actor,phase,created_at",
	"installs":      "id,release_id,target,phase,created_at",
	"studio_events": "seq,type,object_id,created_at,payload",
}

func inspectExistingLedger(ctx context.Context, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("studiocore: inspect ledger: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupportedSourceFormat, err)
	}
	defer db.Close()
	return ValidateLedgerFormat(ctx, db)
}

// ValidateLedgerFormat reads only schema metadata. An empty database is pristine;
// every nonempty database needs an exact known table shape. Legacy recognition
// is solely a refusal path: the runtime never upgrades it.
func ValidateLedgerFormat(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupportedSourceFormat, err)
	}
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnsupportedSourceFormat, err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		tables = append(tables, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(tables) == 0 && version == 0 {
		return nil
	}
	expected := make([]string, 0, len(ledgerColumns))
	for name := range ledgerColumns {
		expected = append(expected, name)
	}
	sort.Strings(expected)
	if strings.Join(tables, ",") != strings.Join(expected, ",") || (version != 0 && version != 1) {
		return ErrUnsupportedSourceFormat
	}
	legacy := false
	for _, table := range tables {
		columns, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
		if err != nil {
			return err
		}
		var names []string
		validShape := true
		for columns.Next() {
			var cid, notnull, pk int
			var name, typ string
			var def any
			if err := columns.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
				columns.Close()
				return err
			}
			names = append(names, name)
			wantType := "TEXT"
			if name == "created_at" || name == "dirty" || name == "seq" {
				wantType = "INTEGER"
			}
			wantPK := 0
			wantNull := 1
			if name == "id" || name == "seq" {
				wantPK = 1
				wantNull = 0
			}
			wantDefault := any(nil)
			switch name {
			case "dirty":
				wantDefault = "0"
			case "parent_id", "baseline_id", "journal_ref", "eval_id":
				wantDefault = "''"
			case "recipe_json", "payload":
				wantDefault = "'{}'"
			}
			if typ != wantType || notnull != wantNull || pk != wantPK || fmt.Sprint(def) != fmt.Sprint(wantDefault) {
				validShape = false
			}
		}
		err = columns.Err()
		columns.Close()
		if err != nil {
			return err
		}
		if !validShape {
			return ErrUnsupportedSourceFormat
		}
		want := ledgerColumns[table]
		if table == "generations" && strings.Join(names, ",") == "id,parent_id,artifact_sha256,source_ref,recipe_json,phase,created_at" {
			legacy = true
			continue
		}
		if strings.Join(names, ",") != want {
			return ErrUnsupportedSourceFormat
		}
	}
	var extras int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type IN ('trigger','view') OR (type='index' AND sql IS NOT NULL)").Scan(&extras); err != nil {
		return err
	}
	if extras != 0 {
		return ErrUnsupportedSourceFormat
	}
	if legacy && version == 0 {
		return ErrUpgradeRequired
	}
	if legacy || version != 1 {
		return ErrUnsupportedSourceFormat
	}
	return nil
}
