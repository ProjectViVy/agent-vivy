package contexthost

import (
	"context"
	"unicode/utf8"

	"agent-vivy/sdk/port/contextsource"
)

// ResolvedSnapshot is a memory-only candidate body that was already
// authorized and rendered upstream. The source that carries it performs no
// filesystem, network, or history work at query time.
type ResolvedSnapshot struct {
	ContentID string
	MediaType string
	Content   string
	Version   string
	Metadata  map[string]string
}

// ResolvedSnapshotSource adapts already-resolved snapshots into the public
// Context Source shape so they pass through the same request validation,
// authorization, and budget accounting as every other source.
type ResolvedSnapshotSource struct {
	id        string
	snapshots []ResolvedSnapshot
}

func NewResolvedSnapshotSource(id string, snapshots []ResolvedSnapshot) *ResolvedSnapshotSource {
	copied := make([]ResolvedSnapshot, len(snapshots))
	copy(copied, snapshots)
	for index := range copied {
		if copied[index].Metadata == nil {
			continue
		}
		metadata := make(map[string]string, len(copied[index].Metadata))
		for key, value := range copied[index].Metadata {
			metadata[key] = value
		}
		copied[index].Metadata = metadata
	}
	return &ResolvedSnapshotSource{id: id, snapshots: copied}
}

func (source *ResolvedSnapshotSource) ID() string { return source.id }

func (source *ResolvedSnapshotSource) Query(ctx context.Context, request contextsource.Request) (contextsource.Page, error) {
	if err := ctx.Err(); err != nil {
		return contextsource.Page{}, err
	}
	limit := request.Limit
	if limit <= 0 || limit > len(source.snapshots) {
		limit = len(source.snapshots)
	}
	candidates := make([]contextsource.Candidate, 0, limit)
	for _, snapshot := range source.snapshots {
		if err := ctx.Err(); err != nil {
			return contextsource.Page{}, err
		}
		if len(candidates) >= limit {
			break
		}
		if snapshot.ContentID == "" || !utf8.ValidString(snapshot.Content) {
			continue
		}
		candidates = append(candidates, contextsource.NewCandidate(contextsource.Candidate{
			SourceID:  source.id,
			ContentID: snapshot.ContentID,
			MediaType: snapshot.MediaType,
			Content:   snapshot.Content,
			Version:   snapshot.Version,
			Treatment: contextsource.TreatmentCompetitive,
			Metadata:  snapshot.Metadata,
		}))
	}
	return contextsource.NewPage(candidates, ""), nil
}
