package domain

// Note is one persisted notebook entry (MA-3). Content is append-only:
// the notebook has no update or delete path in V1, mirroring the
// message log's immutability (FR-2).
type Note struct {
	ID        string
	Content   string
	CreatedAt int64 // unix milli
}
