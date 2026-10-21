// Package studioledger is the finite D06 transition artifact. Final runtimes
// never import it; D14 archives this source and removes it from the release tree.
package studioledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"agent-vivy/internal/studiocore"
)

type Options struct {
	SourceLedger string
	// TargetHome must not exist. The caller switches its configured ledger home
	// only after this function has verified and atomically published the copy.
	TargetHome string
}
type Result struct {
	Artifacts map[string]string `json:"artifacts"`
}

// Convert requires a quiescent source and never changes it or an installed body.
// Artifact allocation is the deterministic sorted source-path map owned by this
// one target; failed attempts publish nothing and may safely be retried.
func Convert(ctx context.Context, opt Options) (Result, error) { return convert(ctx, opt, nil) }

func convert(ctx context.Context, opt Options, check func(string) error) (result Result, err error) {
	if opt.SourceLedger == "" || opt.TargetHome == "" {
		return result, errors.New("studio transition: source ledger and new target home are required")
	}
	source, err := filepath.Abs(opt.SourceLedger)
	if err != nil {
		return result, err
	}
	target, err := filepath.Abs(opt.TargetHome)
	if err != nil {
		return result, err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return result, fmt.Errorf("studio transition: source_binding_unavailable: %w", err)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return result, fmt.Errorf("studio transition: target parent must exist: %w", err)
	}
	target = filepath.Join(parent, filepath.Base(target))
	if inside(target, filepath.Dir(source)) || inside(filepath.Dir(source), target) {
		return result, errors.New("studio transition: source and target homes overlap")
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("studio transition: target must not exist")
	}
	uriPath := filepath.ToSlash(source)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	ro := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}
	src, err := sql.Open("sqlite", ro.String())
	if err != nil {
		return result, err
	}
	defer src.Close()
	if err := studiocore.ValidateLedgerFormat(ctx, src); !errors.Is(err, studiocore.ErrUpgradeRequired) {
		if err == nil {
			err = errors.New("studio transition: unsupported_source_format: expected exact legacy Studio ledger")
		}
		return result, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), "."+filepath.Base(target)+"-upgrade-")
	if err != nil {
		return result, err
	}
	defer func() {
		if stage != "" {
			_ = os.RemoveAll(stage)
		}
	}()
	ledgerPath := filepath.Join(stage, "studio.db")
	ledger, err := studiocore.OpenLedger(ctx, ledgerPath)
	if err != nil {
		return result, err
	}
	if err := ledger.Close(); err != nil {
		return result, err
	}
	db, err := sql.Open("sqlite", ledgerPath)
	if err != nil {
		return result, err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.ExecContext(ctx, "ATTACH DATABASE ? AS legacy", ro.String()); err != nil {
		return result, fmt.Errorf("studio transition: attach source: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	result.Artifacts, err = copyArtifacts(ctx, tx, stage, target)
	if err != nil {
		return result, err
	}
	if err := checkpoint(check, "copy"); err != nil {
		return result, err
	}
	if err := copyRollback(filepath.Join(filepath.Dir(source), "rollback"), filepath.Join(stage, "rollback")); err != nil {
		return result, err
	}
	for _, table := range []string{"worktrees", "eval_runs", "releases", "installs", "studio_events"} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO "+table+" SELECT * FROM legacy."+table); err != nil {
			return result, fmt.Errorf("studio transition: copy %s: %w", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO generations(id,parent_id,source_ref,recipe_json,phase,created_at) SELECT id,parent_id,source_ref,recipe_json,phase,created_at FROM legacy.generations`); err != nil {
		return result, err
	}
	for from, to := range result.Artifacts {
		if _, err := tx.ExecContext(ctx, "UPDATE generations SET source_ref=? WHERE source_ref=?", "file:"+filepath.ToSlash(to), "file:"+from); err != nil {
			return result, err
		}
	}
	// Keep the historical AUTOINCREMENT position, including deleted events.
	if _, err := tx.ExecContext(ctx, `DELETE FROM sqlite_sequence WHERE name='studio_events'; INSERT INTO sqlite_sequence(name,seq) SELECT name,seq FROM legacy.sqlite_sequence WHERE name='studio_events'`); err != nil {
		return result, err
	}
	if err := verifyReferences(ctx, tx); err != nil {
		return result, err
	}
	if err := checkpoint(check, "sql"); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	if err := studiocore.ValidateLedgerFormat(ctx, db); err != nil {
		return result, err
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return result, fmt.Errorf("studio transition: incomplete_conversion: target integrity: %w", err)
	}
	if integrity != "ok" {
		return result, fmt.Errorf("studio transition: incomplete_conversion: target integrity %q", integrity)
	}
	if err := checkpoint(check, "verify"); err != nil {
		return result, err
	}
	if err := db.Close(); err != nil {
		return result, err
	}
	if err := checkpoint(check, "publish"); err != nil {
		return result, err
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("studio transition: target appeared before publication")
	}
	if err := os.Rename(stage, target); err != nil {
		return result, err
	}
	stage = ""
	return result, nil
}

func copyArtifacts(ctx context.Context, tx *sql.Tx, stage, target string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT DISTINCT source_ref FROM legacy.generations WHERE source_ref GLOB 'file:*' ORDER BY source_ref")
	if err != nil {
		return nil, err
	}
	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			rows.Close()
			return nil, err
		}
		refs = append(refs, ref)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	mapping := map[string]string{}
	for i, ref := range refs {
		source := strings.TrimPrefix(ref, "file:")
		if !filepath.IsAbs(source) {
			return nil, errors.New("studio transition: source_binding_unavailable: artifact reference must be absolute")
		}
		name := fmt.Sprintf("artifact-%06d.exe", i+1)
		mapping[source] = filepath.Join(target, "artifacts", name)
	}
	// Persist allocations before copying; final references use the published home.
	encoded, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stage, "artifact-map.json"), encoded, 0600); err != nil {
		return nil, err
	}
	if len(mapping) > 0 {
		if err := os.Mkdir(filepath.Join(stage, "artifacts"), 0700); err != nil {
			return nil, err
		}
	}
	for source, destination := range mapping {
		if err := copyRegular(source, filepath.Join(stage, "artifacts", filepath.Base(destination)), false); err != nil {
			return nil, fmt.Errorf("studio transition: source_binding_unavailable: %w", err)
		}
	}
	return mapping, nil
}

func copyRollback(source, target string) error {
	info, err := os.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("studio transition: unsupported_source_format: rollback root is not a directory")
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("studio transition: unsupported_source_format: rollback symlink")
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		if entry.Name() != "vivy.exe" && entry.Name() != "install.json" {
			return errors.New("studio transition: unsupported_source_format: unknown rollback file")
		}
		return copyRegular(path, destination, entry.Name() == "install.json")
	})
}

func copyRegular(source, target string, manifest bool) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("artifact is not a regular file")
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	mode := os.FileMode(0700)
	if manifest {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return fmt.Errorf("studio transition: incomplete_conversion: rollback manifest: %w", err)
		}
		delete(fields, "artifact_sha256")
		raw, err = json.MarshalIndent(fields, "", "  ")
		if err != nil {
			return err
		}
		mode = 0600
	}
	if err := os.WriteFile(target, raw, mode); err != nil {
		return err
	}
	copied, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if string(copied) != string(raw) {
		return errors.New("studio transition: incomplete_conversion: copied bytes differ")
	}
	return nil
}

func verifyReferences(ctx context.Context, tx *sql.Tx) error {
	for _, query := range []string{
		`SELECT count(*) FROM generations g LEFT JOIN generations p ON p.id=g.parent_id WHERE g.parent_id<>'' AND p.id IS NULL`,
		`SELECT count(*) FROM eval_runs e LEFT JOIN generations g ON g.id=e.candidate_id LEFT JOIN generations b ON b.id=e.baseline_id WHERE g.id IS NULL OR (e.baseline_id<>'' AND b.id IS NULL)`,
		`SELECT count(*) FROM releases r LEFT JOIN generations g ON g.id=r.generation_id LEFT JOIN eval_runs e ON e.id=r.eval_id WHERE g.id IS NULL OR e.id IS NULL OR e.candidate_id<>r.generation_id`,
		`SELECT count(*) FROM installs i LEFT JOIN releases r ON r.id=i.release_id WHERE r.id IS NULL`,
	} {
		var count int
		if err := tx.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return errors.New("studio transition: incomplete_conversion: broken lifecycle references")
		}
	}
	return nil
}
func checkpoint(check func(string) error, at string) error {
	if check != nil {
		return check(at)
	}
	return nil
}
func inside(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
