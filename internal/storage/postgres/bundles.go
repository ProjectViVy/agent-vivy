package postgres

import (
	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/bundlesql"
	"context"
	"database/sql"
)

type bundleDB struct{ db *DB }
type bundleTx struct{ tx *Tx }

func (b *Backend) OrganismLeaseOwner() string { return b.leaseOwner }

func (d bundleDB) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	return d.db.ExecContext(c, q, a...)
}
func (d bundleDB) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	return d.db.QueryContext(c, q, a...)
}
func (d bundleDB) BeginBundleTx(c context.Context) (bundlesql.Transaction, error) {
	t, e := d.db.BeginTx(c, nil)
	if e != nil {
		return nil, e
	}
	return bundleTx{t}, nil
}
func (t bundleTx) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	return t.tx.ExecContext(c, q, a...)
}
func (t bundleTx) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(c, q, a...)
}
func (t bundleTx) Commit() error                                    { return t.tx.Commit() }
func (t bundleTx) Rollback() error                                  { return t.tx.Rollback() }
func (b *Backend) BundleStore() storage.BundleStore                 { return bundlesql.Store{DB: bundleDB{b.db}} }
func (b *Backend) StoreBundles(context.Context) storage.BundleStore { return b.BundleStore() }
func (b *Backend) CheckBundleLease(c context.Context, owner string) error {
	return b.BundleStore().CheckBundleLease(c, owner)
}
func (b *Backend) ReadBundleState(c context.Context) (storage.BundleState, error) {
	return b.BundleStore().ReadBundleState(c)
}
func (b *Backend) SaveBundleInstallation(c context.Context, r storage.InstallCommit) (bundlecontract.Operation, error) {
	return b.BundleStore().SaveBundleInstallation(c, r)
}
func (b *Backend) BeginBundleOperation(c context.Context, r storage.BeginOperation) (bundlecontract.Operation, error) {
	return b.BundleStore().BeginBundleOperation(c, r)
}
func (b *Backend) GetBundleOperation(c context.Context, id string) (bundlecontract.Operation, error) {
	return b.BundleStore().GetBundleOperation(c, id)
}
func (b *Backend) CancelBundleOperation(c context.Context, r storage.CancelOperation) (bundlecontract.Operation, error) {
	return b.BundleStore().CancelBundleOperation(c, r)
}
func (b *Backend) ListBundleBlockers(c context.Context, r storage.BlockerQuery) (storage.BlockerPage, error) {
	return b.BundleStore().ListBundleBlockers(c, r)
}
func (b *Backend) CommitBundleActivation(c context.Context, r storage.ActivationCommit) (bundlecontract.Operation, error) {
	return b.BundleStore().CommitBundleActivation(c, r)
}
func (b *Backend) CommitBundleBindings(c context.Context, r storage.BindingCommit) (bundlecontract.Operation, error) {
	return b.BundleStore().CommitBundleBindings(c, r)
}
func (b *Backend) CommitBundleConsent(c context.Context, r storage.ConsentCommit) (bundlecontract.Operation, error) {
	return b.BundleStore().CommitBundleConsent(c, r)
}
func (b *Backend) ReadBundleConsent(c context.Context, installID string, kind bundlecontract.ComponentKind, localID string) (storage.BundleConsent, bool, error) {
	return b.BundleStore().ReadBundleConsent(c, installID, kind, localID)
}
func (b *Backend) CommitBundleRevocation(c context.Context, r storage.RevocationCommit) (bundlecontract.Operation, error) {
	return b.BundleStore().CommitBundleRevocation(c, r)
}
func (b *Backend) SaveRunBundleSnapshot(c context.Context, r storage.RunBundleSnapshot) error {
	return b.BundleStore().SaveRunBundleSnapshot(c, r)
}
func (b *Backend) LoadRunBundleSnapshot(c context.Context, id domain.RunID) (storage.RunBundleSnapshot, error) {
	return b.BundleStore().LoadRunBundleSnapshot(c, id)
}
func (b *Backend) ReadBundleReachability(c context.Context) (storage.ArtifactReachability, error) {
	return b.BundleStore().ReadBundleReachability(c)
}
func (b *Backend) CheckBundleRevocations(c context.Context, check bundlecontract.CaptureCheck) error {
	return b.BundleStore().CheckBundleRevocations(c, check)
}
