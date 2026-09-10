package mcphost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"agent-vivy/sdk/port/contextsource"
)

const mcpResourceSourceID = "mcp.resources"

// ResourceSource is an explicit MCP Resource -> Context Source bridge. Merely
// configuring an MCP instance does not enable it; each instance must opt in via
// ResourceBridge. Prompts and binary resource bodies are intentionally absent
// from this surface.
type ResourceSource struct {
	host *Host
}

func NewResourceSource(host *Host) *ResourceSource { return &ResourceSource{host: host} }
func (*ResourceSource) ID() string                 { return mcpResourceSourceID }

func (source *ResourceSource) Query(ctx context.Context, request contextsource.Request) (contextsource.Page, error) {
	if source == nil || source.host == nil {
		return contextsource.Page{}, nil
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 16
	}
	query := strings.ToLower(strings.TrimSpace(request.Query))
	out := make([]contextsource.Candidate, 0, limit)
	for _, status := range source.host.Status() {
		if len(out) >= limit {
			break
		}
		if !status.ResourceBridge || status.State == StateUnconfigured || status.State == StateDeferred || status.CircuitOpen {
			continue
		}
		resources, err := source.host.ListResources(ctx, status.ID)
		if err != nil {
			if ctx.Err() != nil {
				return contextsource.Page{}, ctx.Err()
			}
			continue
		}
		for _, resource := range resources {
			if len(out) >= limit {
				break
			}
			if query != "" {
				haystack := strings.ToLower(resource.URI + "\n" + resource.Name + "\n" + resource.Description)
				if !strings.Contains(haystack, query) {
					continue
				}
			}
			content, err := source.host.ReadResource(ctx, status.ID, resource.URI)
			if err != nil {
				if ctx.Err() != nil {
					return contextsource.Page{}, ctx.Err()
				}
				continue
			}
			// Binary MCP resources do not silently become prompt text. A future
			// media-aware bridge can define a separate bounded representation.
			if strings.TrimSpace(content.Text) == "" || len(content.Blob) > 0 {
				continue
			}
			mediaType := strings.TrimSpace(content.MediaType)
			if mediaType == "" {
				mediaType = strings.TrimSpace(resource.MediaType)
			}
			if mediaType == "" {
				mediaType = "text/plain"
			}
			sum := sha256.Sum256([]byte(status.ID + "\x00" + resource.URI + "\x00" + content.Text))
			out = append(out, contextsource.NewCandidate(contextsource.Candidate{
				SourceID:   mcpResourceSourceID,
				ContentID:  status.ID + ":" + resource.URI,
				MediaType:  mediaType,
				Content:    content.Text,
				SizeHint:   len(content.Text),
				Confidence: 0.5,
				Version:    hex.EncodeToString(sum[:]),
				Metadata: map[string]string{
					"instance": status.ID,
					"uri":      resource.URI,
					"name":     resource.Name,
				},
			}))
		}
	}
	return contextsource.NewPage(out, ""), nil
}

var _ contextsource.Provider = (*ResourceSource)(nil)
