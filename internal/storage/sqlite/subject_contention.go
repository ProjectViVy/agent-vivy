package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	driversqlite "modernc.org/sqlite"
)

// modernc's long SQLite busy handler ignores context cancellation until its
// timeout expires. Connections therefore wait at most 25ms; these bounded local
// retries repeat only rolled-back transactions or atomic CAS statements.
func retrySQLiteBusy(ctx context.Context, operation func() error) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := operation()
		if err == nil {
			return nil
		}
		if errors.Is(err, storage.ErrCommitUncertain) {
			return err
		}
		var sqliteErr *driversqlite.Error
		if !errors.As(err, &sqliteErr) || (sqliteErr.Code()&255 != 5 && sqliteErr.Code()&255 != 6) {
			return err
		}
		if cause := ctx.Err(); cause != nil {
			return cause
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("storage: SQLite contention timeout: %w", err)
		}
		timer := time.NewTimer(5 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (b *Backend) CommitRunAdmission(ctx context.Context, in storage.RunAdmission) (out domain.RunEvent, err error) {
	if in.Subject == nil {
		return b.commitRunAdmission(ctx, in)
	}
	err = retrySQLiteBusy(ctx, func() error { out, err = b.commitRunAdmission(ctx, in); return err })
	return out, err
}
func (b *Backend) CommitPrimaryRun(ctx context.Context, in storage.PrimaryRunCommit) (out domain.RunEvent, err error) {
	if in.Subject == nil {
		return b.commitPrimaryRun(ctx, in)
	}
	err = retrySQLiteBusy(ctx, func() error { out, err = b.commitPrimaryRun(ctx, in); return err })
	return out, err
}
func (b *Backend) CommitGoalRun(ctx context.Context, in storage.GoalRunCommit) (out storage.GoalRunCommitResult, err error) {
	if in.Subject == nil {
		return b.commitGoalRun(ctx, in)
	}
	err = retrySQLiteBusy(ctx, func() error { out, err = b.commitGoalRun(ctx, in); return err })
	return out, err
}
func (b *Backend) CommitContinuityRun(ctx context.Context, in storage.ContinuityAdmission) (out storage.ContinuityResult, err error) {
	if in.Subject == nil {
		return b.commitContinuityRun(ctx, in)
	}
	err = retrySQLiteBusy(ctx, func() error { out, err = b.commitContinuityRun(ctx, in); return err })
	return out, err
}
