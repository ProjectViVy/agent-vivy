package postgres

import (
	"context"
	"fmt"
	"time"
)

// Acquire grants the lease when the key is free, expired, or already
// owned by the caller. It reports acquired=false when another owner holds
// a live lease.
func (b *Backend) Acquire(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl).UnixMilli()

	res, err := b.db.ExecContext(ctx,
		`UPDATE leases SET owner = ?, expires_at = ?
		 WHERE key = ? AND (owner = ? OR expires_at <= ?)`,
		owner, expiresAt, key, owner, time.Now().UnixMilli())
	if err != nil {
		return false, fmt.Errorf("storage: lease take %q: %w", key, err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}

	res, err = b.db.ExecContext(ctx,
		`INSERT INTO leases (key, owner, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT (key) DO NOTHING`,
		key, owner, expiresAt)
	if err != nil {
		return false, fmt.Errorf("storage: lease insert %q: %w", key, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Release drops the lease only if the caller still owns it; releasing a
// lease you do not own is a no-op, not an error.
func (b *Backend) Release(ctx context.Context, key, owner string) error {
	if _, err := b.db.ExecContext(ctx,
		`DELETE FROM leases WHERE key = ? AND owner = ?`, key, owner); err != nil {
		return fmt.Errorf("storage: lease release %q: %w", key, err)
	}
	return nil
}
