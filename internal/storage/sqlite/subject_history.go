package sqlite

import (
	"context"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/subjectsql"
)

func (b *Backend) subjectStore() subjectsql.Store { return subjectsql.Store{DB: b.db, Postgres: false} }
func (b *Backend) SealSubjectInteraction(ctx context.Context, bnd storage.SubjectHistoryBinding, in storage.SubjectInteraction) (storage.SubjectInteraction, error) {
	var out storage.SubjectInteraction
	var err error
	err = retrySQLiteBusy(ctx, func() error { out, err = b.subjectStore().SealSubjectInteraction(ctx, bnd, in); return err })
	return out, err
}
func (b *Backend) SealSubjectExclusion(ctx context.Context, bnd storage.SubjectHistoryBinding, in storage.SubjectInteraction) (storage.SubjectInteraction, error) {
	var out storage.SubjectInteraction
	var err error
	err = retrySQLiteBusy(ctx, func() error { out, err = b.subjectStore().SealSubjectExclusion(ctx, bnd, in); return err })
	return out, err
}
func (b *Backend) ListSubjectInteractions(ctx context.Context, bnd storage.SubjectHistoryBinding, after, through uint64, limit int) ([]storage.SubjectInteraction, error) {
	return b.subjectStore().ListSubjectInteractions(ctx, bnd, after, through, limit)
}
func (b *Backend) ReadSubjectInteraction(ctx context.Context, bnd storage.SubjectHistoryBinding, id string) ([]domain.Message, error) {
	return b.subjectStore().ReadSubjectInteraction(ctx, bnd, id)
}
func (b *Backend) ArchiveRetainedSession(ctx context.Context, sid domain.SessionID) error {
	return retrySQLiteBusy(ctx, func() error { return b.subjectStore().ArchiveRetainedSession(ctx, sid) })
}
func (b *Backend) GetSubjectSource(ctx context.Context, sid domain.SessionID) (storage.SubjectSourceBinding, error) {
	return b.subjectStore().GetSubjectSource(ctx, sid)
}
func (b *Backend) GetRunSubjectSource(ctx context.Context, rid domain.RunID) (storage.SubjectSourceBinding, error) {
	return b.subjectStore().GetRunSubjectSource(ctx, rid)
}
func (b *Backend) CaptureSubjectHistory(ctx context.Context, bnd storage.SubjectHistoryBinding) (uint64, error) {
	return b.subjectStore().CaptureSubjectHistory(ctx, bnd)
}

var _ storage.SubjectSourceStore = (*Backend)(nil)

func (b *Backend) ReadSubjectRunEvidence(ctx context.Context, bnd storage.SubjectHistoryBinding, rid domain.RunID) ([]domain.Message, storage.SubjectInteraction, error) {
	return b.subjectStore().ReadSubjectRunEvidence(ctx, bnd, rid)
}

func (b *Backend) CaptureSubjectContext(ctx context.Context, bnd storage.SubjectHistoryBinding) (storage.SubjectContextSnapshot, error) {
	return b.subjectStore().CaptureSubjectContext(ctx, bnd)
}
func (b *Backend) ListSubjectPendingRuns(ctx context.Context, bnd storage.SubjectHistoryBinding) ([]storage.SubjectPendingRun, error) {
	return b.subjectStore().ListSubjectPendingRuns(ctx, bnd)
}
