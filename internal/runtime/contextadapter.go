package runtime

import (
	"context"
	"fmt"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"

	"github.com/cloudwego/eino/schema"
)

// contextCandidatesToUserParts is the only generic ContextHost -> Eino
// projection. Context Sources and ContextHost themselves never import Eino.
func contextCandidatesToUserParts(candidates []contexthost.Candidate) []schema.MessageInputPart {
	parts := make([]schema.MessageInputPart, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.SourceID == "" || candidate.ContentID == "" || candidate.Content == "" {
			continue
		}
		label := fmt.Sprintf("\n\n[context: %s/%s provenance=%s]\n", candidate.SourceID, candidate.ContentID, candidate.ProvenanceID)
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: label + candidate.Content,
		})
	}
	return parts
}

// hostedFileContextParts preserves the current project-file presentation while
// forcing first-party snapshots through ContextSource and ContextHost before
// they become Eino message parts. The source is memory-only and cannot perform
// filesystem or network work at this stage.
func hostedFileContextParts(files []domain.FileContext) []schema.MessageInputPart {
	if len(files) == 0 {
		return nil
	}
	source := contexthost.NewFileSnapshotSource("vivy.project-files", files)
	host, err := contexthost.New(contexthost.Config{Sources: []contextsourceProvider{source}})
	if err != nil {
		return nil
	}
	result, err := host.Query(context.Background(), contexthost.Request{PerSourceLimit: len(files)})
	if err != nil {
		return nil
	}
	parts := make([]schema.MessageInputPart, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "\n\n[project file: " + candidate.ContentID + "]\n" + candidate.Content,
		})
	}
	return parts
}

// contextsourceProvider keeps the runtime adapter's dependency surface small
// while remaining assignment-compatible with contextsource.Provider.
type contextsourceProvider interface {
	ID() string
	Query(context.Context, contextsource.Request) (contextsource.Page, error)
}
