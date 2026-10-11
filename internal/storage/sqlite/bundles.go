package sqlite

import (
	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/bundlesql"
	"context"
	"database/sql"
)

type bundleDB struct{ db *sql.DB }

func (d bundleDB) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	return d.db.ExecContext(c, q, a...)
}
func (d bundleDB) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	return d.db.QueryContext(c, q, a...)
}
func (d bundleDB) BeginBundleTx(c context.Context) (bundlesql.Transaction, error) {
	return d.db.BeginTx(c, nil)
}
func (b *Backend) BundleStore() storage.BundleStore                 { return bundlesql.Store{DB: bundleDB{b.db}} }
func (b *Backend) StoreBundles(context.Context) storage.BundleStore { return b.BundleStore() }
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
func (b *Backend) CommitBundleConsent(c context.Context, r storage.ConsentCommit) (bundlecontract.Operation, error) {
	return b.BundleStore().CommitBundleConsent(c, r)
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
