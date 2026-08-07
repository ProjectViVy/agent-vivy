package runtime

import (
	"context"

	"github.com/cloudwego/eino/adk"
)

// EinoCheckpointAdapter exposes the VersionedCheckpointStore to the eino
// runner as its CheckPointStore (plus the optional deleter). It exists so
// the eino type surface stays inside the runtime boundary (D-007).
type EinoCheckpointAdapter struct {
	store *VersionedCheckpointStore
}

var (
	_ adk.CheckPointStore   = (*EinoCheckpointAdapter)(nil)
	_ adk.CheckPointDeleter = (*EinoCheckpointAdapter)(nil)
)

// NewEinoCheckpointAdapter wires the adapter over a versioned store.
func NewEinoCheckpointAdapter(store *VersionedCheckpointStore) *EinoCheckpointAdapter {
	return &EinoCheckpointAdapter{store: store}
}

// Get satisfies adk.CheckPointStore; the versioned store verifies the
// envelope before any byte reaches the engine.
func (a *EinoCheckpointAdapter) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	return a.store.Get(ctx, checkPointID)
}

// Set satisfies adk.CheckPointStore; bytes are enveloped and committed as
// a new blob generation before the runner considers them durable.
func (a *EinoCheckpointAdapter) Set(ctx context.Context, checkPointID string, checkpoint []byte) error {
	return a.store.Set(ctx, checkPointID, checkpoint)
}

// Delete satisfies adk.CheckPointDeleter so stale checkpoints can be
// reclaimed explicitly (cleanup itself joins in E2/E4).
func (a *EinoCheckpointAdapter) Delete(ctx context.Context, checkPointID string) error {
	return a.store.Delete(ctx, checkPointID)
}
