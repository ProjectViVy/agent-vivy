package postgres

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (b *Backend) ReadDeliverablePresentation(ctx context.Context, sessionID domain.SessionID, itemID string) ([]byte, error) {
	var content []byte
	err := b.db.SQL.QueryRowContext(ctx, `SELECT content FROM deliverable_presentations WHERE session_id = $1 AND item_id = $2`, sessionID, itemID).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("storage: read private presentation: %w", err)
	}
	if content == nil {
		content = []byte{}
	}
	return content, nil
}
func postgresInsertPresentations(ctx context.Context, tx *sql.Tx, m storage.ContinuityMutation) error {
	if err := m.ValidatePresentations(); err != nil {
		return err
	}
	if m.Event.Type != domain.EventDeliverablesPresented {
		return nil
	}
	hidden, err := storage.HiddenPresentationRuns(ctx, tx, m.SessionID, "$1")
	if err != nil {
		return err
	}
	if hidden[m.RunID] {
		return storage.ErrSourceChanged
	}
	for _, presentation := range m.Presentations {
		content := presentation.Content
		if content == nil {
			content = []byte{}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO deliverable_presentations(session_id,item_id,run_id,content) VALUES($1,$2,$3,$4)`, m.SessionID, presentation.ItemID, m.RunID, content); err != nil {
			return fmt.Errorf("storage: insert private presentation: %w", err)
		}
	}
	return nil
}
