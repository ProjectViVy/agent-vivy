// Package bundlesql implements the dialect-neutral Core Storage transactions
// for Toolkits. The dialect migrations remain owned by storage/migrations.
package bundlesql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type Executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
type Transaction interface {
	Executor
	Commit() error
	Rollback() error
}
type DB interface {
	Executor
	BeginBundleTx(context.Context) (Transaction, error)
}

func code(c string) error { return fmt.Errorf("storage: toolkit %s", c) }
func ownsLease(ctx context.Context, q Executor, owner string) bool {
	var found string
	var expiry int64
	return owner != "" && scanOne(ctx, q, `SELECT owner,expires_at FROM leases WHERE key='vivy/organism'`, nil, &found, &expiry) == nil && found == owner && expiry > time.Now().UnixMilli()
}
func scanOne(ctx context.Context, q Executor, query string, args []any, dests ...any) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dests...); err != nil {
		return err
	}
	if rows.Next() {
		return errors.New("storage: toolkit duplicate singleton")
	}
	return rows.Err()
}
func marshal(v any) string { b, _ := json.Marshal(v); return string(b) }
func parseOperation(b []byte) (bundlecontract.Operation, error) {
	var v bundlecontract.Operation
	e := json.Unmarshal(b, &v)
	return v, e
}
func opReceipt(ctx context.Context, q Executor, id string) (bundlecontract.Operation, string, error) {
	var digest string
	var payload []byte
	e := scanOne(ctx, q, `SELECT input_digest,payload_json FROM bundle_operations WHERE operation_id = ?`, []any{id}, &digest, &payload)
	if e != nil {
		return bundlecontract.Operation{}, "", e
	}
	op, e := parseOperation(payload)
	return op, digest, e
}
func begin(ctx context.Context, tx Transaction, r storage.BeginOperation, now int64) (bundlecontract.Operation, error) {
	if !bundlecontract.ValidInstallID(r.ID) || len(r.InputDigest) != 64 || r.Kind == "" {
		return bundlecontract.Operation{}, code("invalid_operation")
	}
	if _, err := hex.DecodeString(r.InputDigest); err != nil {
		return bundlecontract.Operation{}, code("invalid_operation")
	}
	op := bundlecontract.Operation{ID: r.ID, Kind: r.Kind, State: "pending", Revision: 1, ExpectedEpoch: r.ExpectedEpoch, Blockers: []bundlecontract.Blocker{}, Prepared: clonePreparedOperation(r.Prepared)}
	_, err := tx.ExecContext(ctx, `INSERT INTO bundle_operations(operation_id,input_digest,kind,state,revision,expected_epoch,payload_json,created_at,updated_at) VALUES(?,?,?,'pending',1,?,?,?,?) ON CONFLICT(operation_id) DO NOTHING`, r.ID, r.InputDigest, r.Kind, r.ExpectedEpoch, marshal(op), now, now)
	if err != nil {
		return op, err
	}
	prior, digest, err := opReceipt(ctx, tx, r.ID)
	if err != nil {
		return op, err
	}
	if digest != r.InputDigest {
		return op, code("revision_conflict")
	}
	if op.Kind != r.Kind || op.ExpectedEpoch != r.ExpectedEpoch {
		return op, code("revision_conflict")
	}
	return prior, nil
}

func clonePreparedOperation(value *bundlecontract.PreparedOperation) *bundlecontract.PreparedOperation {
	if value == nil {
		return nil
	}
	out := *value
	out.Releases = append([]bundlecontract.ReleaseRef(nil), value.Releases...)
	out.NewReleases = append([]bundlecontract.ReleaseRef(nil), value.NewReleases...)
	out.Installations = append([]bundlecontract.InstallationChange(nil), value.Installations...)
	out.Changes = append([]bundlecontract.ComponentChange(nil), value.Changes...)
	return &out
}
func Begin(ctx context.Context, db DB, r storage.BeginOperation) (bundlecontract.Operation, error) {
	tx, e := db.BeginBundleTx(ctx)
	if e != nil {
		return bundlecontract.Operation{}, e
	}
	defer tx.Rollback()
	op, e := begin(ctx, tx, r, time.Now().UnixMilli())
	if e != nil {
		return op, e
	}
	if e = tx.Commit(); e != nil {
		return op, e
	}
	return op, nil
}

func ReadState(ctx context.Context, db Executor) (storage.BundleState, error) {
	var out storage.BundleState
	err := scanOne(ctx, db, `SELECT epoch,config_revision,effective_digest,generation_id,revocation_revision FROM bundle_catalog_state WHERE id=1`, nil, &out.Catalog.Epoch, &out.Catalog.ConfigRevision, &out.Catalog.EffectiveDigest, &out.Catalog.GenerationID, &out.RevocationRevision)
	if err != nil {
		return out, err
	}
	out.Installations = []storage.BundleInstallation{}
	rows, err := db.QueryContext(ctx, `SELECT install_id,bundle_id,state,source_json,release_digest,revision,tombstone FROM bundle_installations ORDER BY bundle_id,install_id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var x storage.BundleInstallation
		var source []byte
		if err := rows.Scan(&x.InstallID, &x.BundleID, &x.State, &source, &x.ReleaseDigest, &x.Revision, &x.Tombstone); err != nil {
			rows.Close()
			return out, err
		}
		if err := json.Unmarshal(source, &x.Source); err != nil {
			rows.Close()
			return out, err
		}
		out.Installations = append(out.Installations, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	out.Releases = []bundlecontract.ReleaseRef{}
	rows, err = db.QueryContext(ctx, `SELECT payload_json FROM bundle_releases ORDER BY install_id,release_sequence`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var raw []byte
		var r bundlecontract.ReleaseRef
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return out, err
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			rows.Close()
			return out, err
		}
		out.Releases = append(out.Releases, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Bindings = []storage.ToolkitBinding{}
	rows, err = db.QueryContext(ctx, `SELECT install_id,kind,local_id,name,reference FROM bundle_bindings ORDER BY install_id,kind,local_id,name`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var binding storage.ToolkitBinding
		if err := rows.Scan(&binding.InstallID, &binding.Kind, &binding.LocalID, &binding.Name, &binding.Reference); err != nil {
			rows.Close()
			return out, err
		}
		out.Bindings = append(out.Bindings, binding)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	var raw []byte
	err = scanOne(ctx, db, `SELECT payload_json FROM bundle_operations WHERE state='pending' ORDER BY created_at,operation_id LIMIT 1`, nil, &raw)
	if err == nil {
		op, e := parseOperation(raw)
		if e != nil {
			return out, e
		}
		out.Pending = &op
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	return out, nil
}
func SaveInstall(ctx context.Context, db DB, r storage.InstallCommit) (bundlecontract.Operation, error) {
	tx, e := db.BeginBundleTx(ctx)
	if e != nil {
		return bundlecontract.Operation{}, e
	}
	defer tx.Rollback()
	now := r.NowMS
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	op, e := begin(ctx, tx, storage.BeginOperation{ID: r.OperationID, InputDigest: r.InputDigest, Kind: "install", ExpectedEpoch: r.Expected.Epoch}, now)
	if e != nil {
		return op, e
	}
	if op.State != "pending" {
		return op, nil
	}
	var epoch, config int64
	var effective, generation string
	if e = scanOne(ctx, tx, `SELECT epoch,config_revision,effective_digest,generation_id FROM bundle_catalog_state WHERE id=1`, nil, &epoch, &config, &effective, &generation); e != nil {
		return op, e
	}
	if epoch != r.Expected.Epoch || config != r.Expected.ConfigRevision || effective != r.Expected.EffectiveDigest || generation != r.Expected.GenerationID {
		return op, code("revision_conflict")
	}
	var oldID string
	err := scanOne(ctx, tx, `SELECT install_id FROM bundle_installations WHERE bundle_id=? AND tombstone=0`, []any{r.Installation.BundleID}, &oldID)
	if err == nil && oldID != r.Installation.InstallID {
		return op, code("bundle_id_conflict")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return op, err
	}
	x := r.Installation
	x.State = "disabled"
	x.Tombstone = false
	x.Revision = 1
	src := marshal(x.Source)
	if !bundlecontract.ValidInstallID(x.InstallID) || r.Release.InstallID != x.InstallID || r.Release.BundleID != x.BundleID || r.Release.ReleaseDigest != x.ReleaseDigest {
		return op, code("invalid_installation")
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO bundle_installations(install_id,bundle_id,state,source_json,release_digest,revision,tombstone,updated_at) VALUES(?,?,?,?,?,1,FALSE,?) ON CONFLICT(install_id) DO NOTHING`, x.InstallID, x.BundleID, x.State, src, x.ReleaseDigest, now)
	if e != nil {
		return op, e
	}
	var storedID, storedDigest string
	err = scanOne(ctx, tx, `SELECT bundle_id,release_digest FROM bundle_installations WHERE install_id=?`, []any{x.InstallID}, &storedID, &storedDigest)
	if err != nil {
		return op, err
	}
	if storedID != x.BundleID || storedDigest != x.ReleaseDigest {
		return op, code("revision_conflict")
	}
	rr := r.Release
	rr.ReleaseSequence = 1
	_, e = tx.ExecContext(ctx, `INSERT INTO bundle_releases(install_id,release_sequence,bundle_id,release_digest,payload_json,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(install_id,release_sequence) DO NOTHING`, rr.InstallID, rr.ReleaseSequence, rr.BundleID, rr.ReleaseDigest, marshal(rr), now)
	if e != nil {
		return op, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO bundle_artifact_refs(owner_kind,owner_id,digest) VALUES('installation',?,?) ON CONFLICT DO NOTHING`, x.InstallID, rr.ReleaseDigest)
	if e != nil {
		return op, e
	}
	op.State = "committed"
	op.Revision++
	op.Progress = "installed_disabled"
	return finishOperation(ctx, tx, op, r.InputDigest, now)
}
func finishOperation(ctx context.Context, tx Transaction, op bundlecontract.Operation, digest string, now int64) (bundlecontract.Operation, error) {
	_, e := tx.ExecContext(ctx, `UPDATE bundle_operations SET state=?,revision=?,committed_epoch=?,payload_json=?,updated_at=? WHERE operation_id=? AND input_digest=? AND state='pending'`, op.State, op.Revision, op.CommittedEpoch, marshal(op), now, op.ID, digest)
	if e != nil {
		return op, e
	}
	if e = tx.Commit(); e != nil {
		return op, e
	}
	return op, nil
}
func GetOperation(ctx context.Context, db Executor, id string) (bundlecontract.Operation, error) {
	op, _, e := opReceipt(ctx, db, id)
	return op, e
}
func Cancel(ctx context.Context, db DB, r storage.CancelOperation) (bundlecontract.Operation, error) {
	tx, e := db.BeginBundleTx(ctx)
	if e != nil {
		return bundlecontract.Operation{}, e
	}
	defer tx.Rollback()
	op, digest, e := opReceipt(ctx, tx, r.ID)
	if e != nil {
		return op, e
	}
	if op.Revision != r.ExpectedRevision || op.State != "pending" {
		return op, code("revision_conflict")
	}
	op.State = "cancelled"
	op.Revision++
	op, e = finishOperation(ctx, tx, op, digest, time.Now().UnixMilli())
	return op, e
}

func Blockers(ctx context.Context, db Executor, q storage.BlockerQuery) (storage.BlockerPage, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	offset := 0
	if q.Cursor != "" {
		v, e := strconv.Atoi(q.Cursor)
		if e != nil || v < 0 {
			return storage.BlockerPage{}, code("invalid_cursor")
		}
		offset = v
	}
	rows, e := db.QueryContext(ctx, `SELECT id,session_id,status FROM runs WHERE status IN ('accepted','queued','active') ORDER BY created_at,id LIMIT ? OFFSET ?`, limit+1, offset)
	if e != nil {
		return storage.BlockerPage{}, e
	}
	defer rows.Close()
	out := storage.BlockerPage{Items: []bundlecontract.Blocker{}}
	for rows.Next() {
		var b bundlecontract.Blocker
		if e := rows.Scan(&b.RunID, &b.SessionID, &b.Status); e != nil {
			return out, e
		}
		out.Items = append(out.Items, b)
	}
	if e := rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.NextCursor = strconv.Itoa(offset + limit)
	}
	return out, nil
}

// ValidateCapture compares a pending run's immutable catalog token while its
// admission transaction owns the session write lock. The exact executable
// references make a newer unrelated revocation harmless to this run.
func ValidateCapture(ctx context.Context, q Executor, expected *bundlecontract.CaptureCheck) error {
	if expected == nil {
		return nil
	}
	if err := storage.ValidateCaptureCheck(expected); err != nil {
		return err
	}
	var epoch, config, revocation int64
	var effective, generation string
	if err := scanOne(ctx, q, `SELECT epoch,config_revision,effective_digest,generation_id,revocation_revision FROM bundle_catalog_state WHERE id=1`, nil,
		&epoch, &config, &effective, &generation, &revocation); err != nil {
		return storage.AdmissionUnavailable("read toolkit capture state", err)
	}
	identity := bundlecontract.CatalogIdentity{Epoch: epoch, ConfigRevision: config, EffectiveDigest: effective, GenerationID: generation}
	if identity != expected.Catalog || revocation != expected.RevocationRevision {
		return storage.ErrBundleCaptureConflict
	}
	for _, expectedConsent := range expected.Consents {
		currentConsent, found, err := ReadConsent(ctx, q, expectedConsent.InstallID, expectedConsent.Kind, expectedConsent.LocalID)
		if err != nil {
			return storage.AdmissionUnavailable("read toolkit executable consent", err)
		}
		if found != expectedConsent.Present || (found && (currentConsent.DeclarationDigest != expectedConsent.DeclarationDigest ||
			currentConsent.Allowed != expectedConsent.Allowed || currentConsent.Revision != expectedConsent.Revision)) {
			return storage.ErrBundleCaptureConflict
		}
	}
	for _, ref := range expected.Executables {
		var revokedAt int64
		err := scanOne(ctx, q, `SELECT revocation_revision FROM bundle_revocations WHERE install_id=? AND kind=? AND local_id=? AND declaration_digest=?`,
			[]any{ref.InstallID, string(ref.Kind), ref.LocalID, ref.DeclarationDigest}, &revokedAt)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return storage.AdmissionUnavailable("read toolkit revocation fence", err)
		}
		if revokedAt > expected.RevocationRevision {
			return storage.ErrBundleRevoked
		}
	}
	return nil
}

// InsertSnapshotTx stores run evidence and artifact reachability inside the
// caller's admission transaction; it never repairs an already committed run.
func InsertSnapshotTx(ctx context.Context, q Executor, snapshot storage.RunBundleSnapshot) error {
	if _, err := storage.ValidateRunBundleSnapshot(snapshot); err != nil {
		return code("invalid_snapshot")
	}
	_, err := q.ExecContext(ctx, `INSERT INTO run_bundle_snapshots(run_id,schema_version,payload,payload_sha256,created_at) VALUES(?,?,?,?,?) ON CONFLICT(run_id) DO NOTHING`,
		snapshot.RunID, snapshot.SchemaVersion, snapshot.Payload, snapshot.PayloadSHA256, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	var priorDigest string
	if err := scanOne(ctx, q, `SELECT payload_sha256 FROM run_bundle_snapshots WHERE run_id=?`, []any{snapshot.RunID}, &priorDigest); err != nil {
		return err
	}
	if priorDigest != snapshot.PayloadSHA256 {
		return code("revision_conflict")
	}
	for _, release := range snapshot.Releases {
		if _, err := q.ExecContext(ctx, `INSERT INTO bundle_artifact_refs(owner_kind,owner_id,digest) VALUES('run',?,?) ON CONFLICT DO NOTHING`, string(snapshot.RunID), release.ReleaseDigest); err != nil {
			return err
		}
	}
	return nil
}

func Activation(ctx context.Context, db DB, r storage.ActivationCommit) (bundlecontract.Operation, error) {
	tx, e := db.BeginBundleTx(ctx)
	if e != nil {
		return bundlecontract.Operation{}, e
	}
	defer tx.Rollback()
	op, digest, e := opReceipt(ctx, tx, r.OperationID)
	if e != nil {
		return op, e
	}
	if digest != r.InputDigest {
		return op, code("revision_conflict")
	}
	if op.State == "committed" {
		return op, nil
	}
	if op.State != "pending" {
		return op, code("revision_conflict")
	}
	var epoch, config, rev int64
	var effective, generation string
	err := scanOne(ctx, tx, `SELECT epoch,config_revision,effective_digest,generation_id,revocation_revision FROM bundle_catalog_state WHERE id=1`, nil, &epoch, &config, &effective, &generation, &rev)
	if err != nil {
		return op, err
	}
	if epoch != r.Expected.Epoch || config != r.Expected.ConfigRevision || effective != r.Expected.EffectiveDigest || generation != r.Expected.GenerationID {
		return op, code("revision_conflict")
	}
	if !ownsLease(ctx, tx, r.LeaseOwner) {
		return op, code("lease_lost")
	}
	var active int
	if err := scanOne(ctx, tx, `SELECT COUNT(*) FROM runs WHERE status IN ('accepted','queued','active')`, nil, &active); err != nil {
		return op, err
	}
	if active > 0 {
		return op, code("catalog_update_pending")
	}
	for _, mask := range r.RemovedMasks {
		var count int
		if err := scanOne(ctx, tx, `SELECT COUNT(*) FROM session_mask_selections WHERE mask_id=?`, []any{mask}, &count); err != nil {
			return op, err
		}
		if count > 0 {
			return op, code("mask_in_use")
		}
	}
	for _, x := range r.Installations {
		res, err := tx.ExecContext(ctx, `UPDATE bundle_installations SET state=?,release_digest=?,revision=revision+1,tombstone=?,updated_at=? WHERE install_id=? AND revision=? AND tombstone=FALSE`, x.State, x.ReleaseDigest, x.Tombstone, r.NowMS, x.InstallID, x.Revision)
		if err != nil {
			return op, err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return op, code("revision_conflict")
		}
	}
	now := r.NowMS
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	next := epoch + 1
	for _, release := range r.Releases {
		if !bundlecontract.ValidInstallID(release.InstallID) || release.ReleaseSequence <= 0 || release.ReleaseDigest == "" {
			return op, code("invalid_release")
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO bundle_releases(install_id,release_sequence,bundle_id,release_digest,payload_json,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(install_id,release_sequence) DO NOTHING`, release.InstallID, release.ReleaseSequence, release.BundleID, release.ReleaseDigest, marshal(release), now)
		if err != nil {
			return op, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO bundle_artifact_refs(owner_kind,owner_id,digest) VALUES('release',?,?) ON CONFLICT DO NOTHING`, release.InstallID+"/"+strconv.FormatInt(release.ReleaseSequence, 10), release.ReleaseDigest)
		if err != nil {
			return op, err
		}
	}
	identity := bundlecontract.CatalogIdentity{Epoch: next, ConfigRevision: config, EffectiveDigest: r.EffectiveDigest, GenerationID: r.GenerationID}
	_, e = tx.ExecContext(ctx, `INSERT INTO bundle_catalog_epochs(epoch,effective_digest,generation_id,payload_json,created_at) VALUES(?,?,?,?,?)`, next, r.EffectiveDigest, r.GenerationID, marshal(identity), now)
	if e != nil {
		return op, e
	}
	res, e := tx.ExecContext(ctx, `UPDATE bundle_catalog_state SET epoch=?,effective_digest=?,generation_id=? WHERE id=1 AND epoch=? AND config_revision=?`, next, r.EffectiveDigest, r.GenerationID, epoch, config)
	if e != nil {
		return op, e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return op, code("revision_conflict")
	}
	op.State = "committed"
	op.Revision++
	op.CommittedEpoch = next
	op.Progress = "published"
	return finishOperation(ctx, tx, op, digest, now)
}

// CommitBindings replaces one component's reference map and advances the
// catalog configuration revision in the same transaction as its durable
// operation receipt. References are environment-variable names only.
func CommitBindings(ctx context.Context, db DB, r storage.BindingCommit) (bundlecontract.Operation, error) {
	if !bundlecontract.ValidInstallID(r.OperationID) || !bundlecontract.ValidInstallID(r.InstallID) ||
		(r.Kind != string(bundlecontract.Hook) && r.Kind != string(bundlecontract.MCP)) || !bundlecontract.ValidLocalID(r.LocalID) ||
		!bundlecontract.ValidDigest(r.InputDigest) || !bundlecontract.ValidDigest(r.EffectiveDigest) || strings.TrimSpace(r.Subject) == "" {
		return bundlecontract.Operation{}, code("invalid_bindings")
	}
	seen := map[string]bool{}
	for _, binding := range r.Bindings {
		if binding.InstallID != r.InstallID || binding.Kind != r.Kind || binding.LocalID != r.LocalID ||
			!bundlecontract.ValidBindingName(binding.Name) || !bundlecontract.ValidBindingReference(binding.Reference) || seen[binding.Name] {
			return bundlecontract.Operation{}, code("invalid_bindings")
		}
		seen[binding.Name] = true
	}
	tx, err := db.BeginBundleTx(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	defer tx.Rollback()
	now := r.NowMS
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	op, digest, err := opReceipt(ctx, tx, r.OperationID)
	if errors.Is(err, sql.ErrNoRows) {
		op, err = begin(ctx, tx, storage.BeginOperation{ID: r.OperationID, InputDigest: r.InputDigest, Kind: "bindings", ExpectedEpoch: r.Expected.Epoch}, now)
		digest = r.InputDigest
	}
	if err != nil {
		return op, err
	}
	if digest != r.InputDigest || op.Kind != "bindings" || op.ExpectedEpoch != r.Expected.Epoch {
		return op, code("revision_conflict")
	}
	if op.State == "committed" {
		return op, nil
	}
	if op.State != "pending" {
		return op, code("revision_conflict")
	}
	var epoch, config int64
	var effective, generation string
	if err := scanOne(ctx, tx, `SELECT epoch,config_revision,effective_digest,generation_id FROM bundle_catalog_state WHERE id=1`, nil,
		&epoch, &config, &effective, &generation); err != nil {
		return op, err
	}
	if (bundlecontract.CatalogIdentity{Epoch: epoch, ConfigRevision: config, EffectiveDigest: effective, GenerationID: generation}) != r.Expected {
		return op, code("revision_conflict")
	}
	if !ownsLease(ctx, tx, r.LeaseOwner) {
		return op, code("lease_lost")
	}
	var active int
	if err := scanOne(ctx, tx, `SELECT COUNT(*) FROM runs WHERE status IN ('accepted','queued','active')`, nil, &active); err != nil {
		return op, err
	}
	if active > 0 {
		return op, code("catalog_update_pending")
	}
	var tombstone bool
	if err := scanOne(ctx, tx, `SELECT tombstone FROM bundle_installations WHERE install_id=?`, []any{r.InstallID}, &tombstone); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return op, code("not_found")
		}
		return op, err
	}
	if tombstone {
		return op, code("not_found")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM bundle_bindings WHERE install_id=? AND kind=? AND local_id=?`, r.InstallID, r.Kind, r.LocalID); err != nil {
		return op, err
	}
	for _, binding := range r.Bindings {
		if _, err := tx.ExecContext(ctx, `INSERT INTO bundle_bindings(install_id,kind,local_id,name,reference) VALUES(?,?,?,?,?)`, binding.InstallID, binding.Kind, binding.LocalID, binding.Name, binding.Reference); err != nil {
			return op, err
		}
	}
	nextConfig := config + 1
	res, err := tx.ExecContext(ctx, `UPDATE bundle_catalog_state SET config_revision=?,effective_digest=? WHERE id=1 AND epoch=? AND config_revision=? AND effective_digest=? AND generation_id=?`, nextConfig, r.EffectiveDigest, epoch, config, effective, generation)
	if err != nil {
		return op, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return op, code("revision_conflict")
	}
	op.State = "committed"
	op.Revision++
	op.CommittedEpoch = epoch
	op.Progress = "bindings_updated"
	return finishOperation(ctx, tx, op, r.InputDigest, now)
}

func Consent(ctx context.Context, db DB, r storage.ConsentCommit) (bundlecontract.Operation, error) {
	return consent(ctx, db, r.OperationID, r.InputDigest, r.LeaseOwner, r.InstallID, r.Kind, r.LocalID, r.DeclarationDigest, r.Subject, r.ExpectedEpoch, r.ExpectedRevision, r.Allowed, false, r.NowMS)
}

func ReadConsent(ctx context.Context, q Executor, installID string, kind bundlecontract.ComponentKind, localID string) (storage.BundleConsent, bool, error) {
	if !bundlecontract.ValidInstallID(installID) || !bundlecontract.ValidLocalID(localID) || (kind != bundlecontract.Hook && kind != bundlecontract.MCP) {
		return storage.BundleConsent{}, false, code("invalid_consent_identity")
	}
	var row storage.BundleConsent
	var allowed bool
	err := scanOne(ctx, q, `SELECT declaration_digest,allowed,revision,subject,updated_at FROM bundle_consents WHERE install_id=? AND kind=? AND local_id=?`,
		[]any{installID, string(kind), localID}, &row.DeclarationDigest, &allowed, &row.Revision, &row.Subject, &row.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.BundleConsent{}, false, nil
	}
	if err != nil {
		return storage.BundleConsent{}, false, err
	}
	row.InstallID, row.Kind, row.LocalID = installID, string(kind), localID
	row.Allowed = allowed
	if !bundlecontract.ValidDigest(row.DeclarationDigest) || row.Revision <= 0 || strings.TrimSpace(row.Subject) == "" || row.UpdatedAt < 0 {
		return storage.BundleConsent{}, false, code("corrupt_consent")
	}
	return row, true, nil
}

func Revoke(ctx context.Context, db DB, r storage.RevocationCommit) (bundlecontract.Operation, error) {
	return consent(ctx, db, r.OperationID, r.InputDigest, r.LeaseOwner, r.InstallID, r.Kind, r.LocalID, r.DeclarationDigest, r.Subject, r.ExpectedEpoch, r.ExpectedRevision, false, true, r.NowMS)
}
func consent(ctx context.Context, db DB, operationID, inputDigest, leaseOwner, installID, kind, localID, decl, subject string, expectedEpoch, expectedRev int64, allowed, revoke bool, now int64) (bundlecontract.Operation, error) {
	tx, e := db.BeginBundleTx(ctx)
	if e != nil {
		return bundlecontract.Operation{}, e
	}
	defer tx.Rollback()
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	k := "consent"
	if revoke {
		k = "revoke"
	}
	op, e := begin(ctx, tx, storage.BeginOperation{ID: operationID, InputDigest: inputDigest, Kind: k, ExpectedEpoch: expectedEpoch}, now)
	if e != nil {
		return op, e
	}
	if op.State != "pending" {
		return op, nil
	}
	var epoch, revision int64
	if e = scanOne(ctx, tx, `SELECT epoch,revocation_revision FROM bundle_catalog_state WHERE id=1`, nil, &epoch, &revision); e != nil {
		return op, e
	}
	if epoch != expectedEpoch || subject == "" || decl == "" || !ownsLease(ctx, tx, leaseOwner) {
		return op, code("revision_conflict")
	}
	if revoke {
		if revision != expectedRev {
			return op, code("revision_conflict")
		}
		next := revision + 1
		_, e = tx.ExecContext(ctx, `INSERT INTO bundle_revocations(install_id,kind,local_id,declaration_digest,revocation_revision,subject,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, installID, kind, localID, decl, next, subject, now)
		if e != nil {
			return op, e
		}
		res, updateErr := tx.ExecContext(ctx, `UPDATE bundle_catalog_state SET revocation_revision=? WHERE id=1 AND revocation_revision=?`, next, revision)
		e = updateErr
		if e == nil {
			n, _ := res.RowsAffected()
			if n != 1 {
				e = code("revision_conflict")
			}
		}
	} else {
		var current int64
		queryErr := scanOne(ctx, tx, `SELECT revision FROM bundle_consents WHERE install_id=? AND kind=? AND local_id=?`, []any{installID, kind, localID}, &current)
		if errors.Is(queryErr, sql.ErrNoRows) {
			current = 0
		} else if queryErr != nil {
			return op, queryErr
		}
		if current != expectedRev {
			return op, code("revision_conflict")
		}
		next := current + 1
		_, e = tx.ExecContext(ctx, `INSERT INTO bundle_consents(install_id,kind,local_id,declaration_digest,allowed,revision,subject,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(install_id,kind,local_id) DO UPDATE SET declaration_digest=excluded.declaration_digest,allowed=excluded.allowed,revision=excluded.revision,subject=excluded.subject,updated_at=excluded.updated_at`, installID, kind, localID, decl, allowed, next, subject, now)
	}
	if e != nil {
		return op, e
	}
	op.State = "committed"
	op.Revision++
	return finishOperation(ctx, tx, op, inputDigest, now)
}

func SaveSnapshot(ctx context.Context, db DB, s storage.RunBundleSnapshot) error {
	if _, err := storage.ValidateRunBundleSnapshot(s); err != nil {
		return code("invalid_snapshot")
	}
	tx, e := db.BeginBundleTx(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if err := InsertSnapshotTx(ctx, tx, s); err != nil {
		return err
	}
	return tx.Commit()
}
func LoadSnapshot(ctx context.Context, db Executor, id domain.RunID) (storage.RunBundleSnapshot, error) {
	var s storage.RunBundleSnapshot
	s.RunID = id
	err := scanOne(ctx, db, `SELECT schema_version,payload,payload_sha256 FROM run_bundle_snapshots WHERE run_id=?`, []any{id}, &s.SchemaVersion, &s.Payload, &s.PayloadSHA256)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s, storage.ErrNotFound
		}
		return s, err
	}
	payload, err := bundlecontract.DecodeRunSnapshotPayload(s.Payload)
	if err != nil {
		return storage.RunBundleSnapshot{}, code("artifact_corrupt")
	}
	s.Releases = payload.Releases
	if _, err := storage.ValidateRunBundleSnapshot(s); err != nil {
		return storage.RunBundleSnapshot{}, code("artifact_corrupt")
	}
	return s, nil
}
func Reachability(ctx context.Context, db Executor) (storage.ArtifactReachability, error) {
	rows, e := db.QueryContext(ctx, `SELECT DISTINCT digest FROM bundle_artifact_refs ORDER BY digest`)
	if e != nil {
		return storage.ArtifactReachability{}, e
	}
	defer rows.Close()
	out := storage.ArtifactReachability{Digests: []string{}}
	for rows.Next() {
		var d string
		if e := rows.Scan(&d); e != nil {
			return out, e
		}
		out.Digests = append(out.Digests, d)
	}
	return out, rows.Err()
}

func CheckLease(ctx context.Context, db Executor, owner string) error {
	if owner == "" {
		return code("lease_lost")
	}
	var current string
	var expiry int64
	if err := scanOne(ctx, db, `SELECT owner,expires_at FROM leases WHERE key='vivy/organism'`, nil, &current, &expiry); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return code("lease_lost")
		}
		return err
	}
	if current != owner || expiry <= time.Now().UnixMilli() {
		return code("lease_lost")
	}
	return nil
}

func CheckRevocations(ctx context.Context, db Executor, check bundlecontract.CaptureCheck) error {
	if err := storage.ValidateCaptureCheck(&check); err != nil {
		return err
	}
	var current int64
	if err := scanOne(ctx, db, `SELECT revocation_revision FROM bundle_catalog_state WHERE id=1`, nil, &current); err != nil {
		return storage.AdmissionUnavailable("read toolkit revocation revision", err)
	}
	if current < check.RevocationRevision {
		return code("snapshot_corrupt")
	}
	for _, ref := range check.Executables {
		if (ref.Kind == bundlecontract.Hook || ref.Kind == bundlecontract.MCP) && ref.ConsentDigest != "" {
			consent, found, err := ReadConsent(ctx, db, ref.InstallID, ref.Kind, ref.LocalID)
			if err != nil {
				return storage.AdmissionUnavailable("read captured executable consent", err)
			}
			if !found || !consent.Allowed || consent.DeclarationDigest != ref.ConsentDigest || consent.Revision != ref.ConsentRevision {
				return storage.ErrBundleRevoked
			}
		}
		var revision int64
		err := scanOne(ctx, db, `SELECT revocation_revision FROM bundle_revocations WHERE install_id=? AND kind=? AND local_id=? AND declaration_digest=?`,
			[]any{ref.InstallID, string(ref.Kind), ref.LocalID, ref.DeclarationDigest}, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return storage.AdmissionUnavailable("read toolkit revocation fence", err)
		}
		if revision > check.RevocationRevision {
			return storage.ErrBundleRevoked
		}
	}
	return nil
}

var _ storage.BundleStore = (*Store)(nil)

type Store struct{ DB DB }

func (s Store) ReadBundleState(c context.Context) (storage.BundleState, error) {
	return ReadState(c, s.DB)
}
func (s Store) CheckBundleLease(c context.Context, owner string) error {
	return CheckLease(c, s.DB, owner)
}
func (s Store) SaveBundleInstallation(c context.Context, r storage.InstallCommit) (bundlecontract.Operation, error) {
	return SaveInstall(c, s.DB, r)
}
func (s Store) BeginBundleOperation(c context.Context, r storage.BeginOperation) (bundlecontract.Operation, error) {
	return Begin(c, s.DB, r)
}
func (s Store) GetBundleOperation(c context.Context, id string) (bundlecontract.Operation, error) {
	return GetOperation(c, s.DB, id)
}
func (s Store) CancelBundleOperation(c context.Context, r storage.CancelOperation) (bundlecontract.Operation, error) {
	return Cancel(c, s.DB, r)
}
func (s Store) ListBundleBlockers(c context.Context, r storage.BlockerQuery) (storage.BlockerPage, error) {
	return Blockers(c, s.DB, r)
}
func (s Store) CommitBundleActivation(c context.Context, r storage.ActivationCommit) (bundlecontract.Operation, error) {
	return Activation(c, s.DB, r)
}
func (s Store) CommitBundleBindings(c context.Context, r storage.BindingCommit) (bundlecontract.Operation, error) {
	return CommitBindings(c, s.DB, r)
}
func (s Store) CommitBundleConsent(c context.Context, r storage.ConsentCommit) (bundlecontract.Operation, error) {
	return Consent(c, s.DB, r)
}
func (s Store) ReadBundleConsent(c context.Context, installID string, kind bundlecontract.ComponentKind, localID string) (storage.BundleConsent, bool, error) {
	return ReadConsent(c, s.DB, installID, kind, localID)
}
func (s Store) CommitBundleRevocation(c context.Context, r storage.RevocationCommit) (bundlecontract.Operation, error) {
	return Revoke(c, s.DB, r)
}
func (s Store) SaveRunBundleSnapshot(c context.Context, r storage.RunBundleSnapshot) error {
	return SaveSnapshot(c, s.DB, r)
}
func (s Store) LoadRunBundleSnapshot(c context.Context, id domain.RunID) (storage.RunBundleSnapshot, error) {
	return LoadSnapshot(c, s.DB, id)
}
func (s Store) ReadBundleReachability(c context.Context) (storage.ArtifactReachability, error) {
	return Reachability(c, s.DB)
}
func (s Store) CheckBundleRevocations(c context.Context, check bundlecontract.CaptureCheck) error {
	return CheckRevocations(c, s.DB, check)
}

var _ = strings.Builder{}
