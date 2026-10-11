// Package bundlesql implements the dialect-neutral Core Storage transactions
// for Toolkits. The dialect migrations remain owned by storage/migrations.
package bundlesql

import (
	"context"
	"crypto/sha256"
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
	op := bundlecontract.Operation{ID: r.ID, Kind: r.Kind, State: "pending", Revision: 1, ExpectedEpoch: r.ExpectedEpoch, Blockers: []bundlecontract.Blocker{}}
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
	op, e := begin(ctx, tx, storage.BeginOperation{ID: r.OperationID, InputDigest: r.InputDigest, Kind: "install", ExpectedEpoch: 0}, now)
	if e != nil {
		return op, e
	}
	if op.State != "pending" {
		return op, nil
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

func Consent(ctx context.Context, db DB, r storage.ConsentCommit) (bundlecontract.Operation, error) {
	return consent(ctx, db, r.OperationID, r.InputDigest, r.LeaseOwner, r.InstallID, r.Kind, r.LocalID, r.DeclarationDigest, r.Subject, r.ExpectedEpoch, r.ExpectedRevision, r.Allowed, false, r.NowMS)
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
	if s.RunID == "" || s.SchemaVersion != 1 {
		return code("invalid_snapshot")
	}
	h := sha256.Sum256(s.Payload)
	digest := hex.EncodeToString(h[:])
	if digest != s.PayloadSHA256 {
		return code("invalid_snapshot")
	}
	tx, e := db.BeginBundleTx(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `INSERT INTO run_bundle_snapshots(run_id,schema_version,payload,payload_sha256,created_at) VALUES(?,?,?,?,?) ON CONFLICT(run_id) DO NOTHING`, s.RunID, s.SchemaVersion, s.Payload, digest, time.Now().UnixMilli())
	if e != nil {
		return e
	}
	var prior string
	if e = scanOne(ctx, tx, `SELECT payload_sha256 FROM run_bundle_snapshots WHERE run_id=?`, []any{s.RunID}, &prior); e != nil {
		return e
	}
	if prior != digest {
		return code("revision_conflict")
	}
	for _, r := range s.Releases {
		if !bundlecontract.ValidInstallID(r.InstallID) || r.ReleaseDigest == "" {
			return code("invalid_snapshot")
		}
		owner := string(s.RunID)
		_, e = tx.ExecContext(ctx, `INSERT INTO bundle_artifact_refs(owner_kind,owner_id,digest) VALUES('run',?,?) ON CONFLICT DO NOTHING`, owner, r.ReleaseDigest)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}
func LoadSnapshot(ctx context.Context, db Executor, id domain.RunID) (storage.RunBundleSnapshot, error) {
	var s storage.RunBundleSnapshot
	s.RunID = id
	err := scanOne(ctx, db, `SELECT schema_version,payload,payload_sha256 FROM run_bundle_snapshots WHERE run_id=?`, []any{id}, &s.SchemaVersion, &s.Payload, &s.PayloadSHA256)
	if err != nil {
		return s, err
	}
	h := sha256.Sum256(s.Payload)
	if hex.EncodeToString(h[:]) != s.PayloadSHA256 {
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

var _ storage.BundleStore = (*Store)(nil)

type Store struct{ DB DB }

func (s Store) ReadBundleState(c context.Context) (storage.BundleState, error) {
	return ReadState(c, s.DB)
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
func (s Store) CommitBundleConsent(c context.Context, r storage.ConsentCommit) (bundlecontract.Operation, error) {
	return Consent(c, s.DB, r)
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

var _ = strings.Builder{}
