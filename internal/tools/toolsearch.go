package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"agent-vivy/internal/domain"
)

const ToolSearchName = "tool_search"

type ToolSearchResult struct {
	Query   string            `json:"query"`
	Matches []ToolSearchMatch `json:"matches"`
	Cached  bool              `json:"cached"`
}

type ToolSearchMatch struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Readonly    bool                        `json:"readonly"`
	Params      map[string]domain.ToolParam `json:"params,omitempty"`
}

type toolSearchTool struct {
	catalog map[string]ToolSearchMatch
	mu      sync.Mutex
	cache   map[string][]ToolSearchMatch
	allowed map[string]struct{}
}

func NewToolSearch(ts []Tool) Tool {
	catalog := make(map[string]ToolSearchMatch, len(ts))
	for _, item := range ts {
		spec := item.Spec()
		catalog[spec.Name] = ToolSearchMatch{Name: spec.Name, Description: spec.Description, Readonly: spec.Readonly, Params: spec.Params}
	}
	return &toolSearchTool{catalog: catalog, cache: make(map[string][]ToolSearchMatch)}
}

func (t *toolSearchTool) restrict(names []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.allowed = make(map[string]struct{}, len(names))
	for _, name := range names {
		t.allowed[name] = struct{}{}
	}
	clear(t.cache)
}

func (t *toolSearchTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: ToolSearchName, Description: "Searches the allowlisted Vivy tool catalog and reveals bounded schemas.", Readonly: true,
		Keywords: []string{"tool_search", "discover", "capability", "catalog"}, Params: map[string]domain.ToolParam{
			"query":       {Desc: "Keywords or select:tool_name,tool_name.", Required: true},
			"max_results": {Desc: "Maximum matches, bounded to 10.", Type: "integer", Required: false},
		}}
}

func (t *toolSearchTool) InvokableRun(_ context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("tool_search: invalid arguments: %w", err)
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return "", errors.New("tool_search: query is required")
	}
	limit := input.MaxResults
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	cacheKey := strings.ToLower(query) + "\x00" + fmt.Sprint(limit)
	t.mu.Lock()
	if cached, ok := t.cache[cacheKey]; ok {
		result := ToolSearchResult{Query: query, Matches: cloneToolMatches(cached), Cached: true}
		t.mu.Unlock()
		return marshalToolResult(result)
	}
	t.mu.Unlock()
	matches := t.search(query, limit)
	t.mu.Lock()
	t.cache[cacheKey] = cloneToolMatches(matches)
	t.mu.Unlock()
	return marshalToolResult(ToolSearchResult{Query: query, Matches: matches})
}

func (t *toolSearchTool) search(query string, limit int) []ToolSearchMatch {
	if strings.HasPrefix(strings.ToLower(query), "select:") {
		var out []ToolSearchMatch
		for _, raw := range strings.Split(strings.TrimPrefix(strings.ToLower(query), "select:"), ",") {
			name := strings.TrimSpace(raw)
			for catalogName, item := range t.catalog {
				if !t.isAllowed(catalogName) {
					continue
				}
				if strings.ToLower(catalogName) == name {
					out = append(out, item)
				}
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out
	}
	terms := tokenSet(query)
	type scored struct {
		item  ToolSearchMatch
		score int
	}
	var scoredItems []scored
	for _, item := range t.catalog {
		if !t.isAllowed(item.Name) {
			continue
		}
		score := 0
		name := strings.ToLower(item.Name)
		desc := strings.ToLower(item.Description)
		for term := range terms {
			if strings.Contains(name, term) {
				score += 10
			} else if strings.Contains(desc, term) {
				score += 2
			}
		}
		if score > 0 {
			scoredItems = append(scoredItems, scored{item: item, score: score})
		}
	}
	sort.Slice(scoredItems, func(i, j int) bool {
		if scoredItems[i].score != scoredItems[j].score {
			return scoredItems[i].score > scoredItems[j].score
		}
		return scoredItems[i].item.Name < scoredItems[j].item.Name
	})
	if len(scoredItems) > limit {
		scoredItems = scoredItems[:limit]
	}
	out := make([]ToolSearchMatch, 0, len(scoredItems))
	for _, item := range scoredItems {
		out = append(out, item.item)
	}
	return out
}

func (t *toolSearchTool) isAllowed(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.allowed == nil {
		return true
	}
	_, ok := t.allowed[name]
	return ok
}

func cloneToolMatches(in []ToolSearchMatch) []ToolSearchMatch {
	out := make([]ToolSearchMatch, len(in))
	copy(out, in)
	for i := range out {
		if in[i].Params != nil {
			out[i].Params = make(map[string]domain.ToolParam, len(in[i].Params))
			for name, param := range in[i].Params {
				param.Enum = append([]string(nil), param.Enum...)
				out[i].Params[name] = param
			}
		}
	}
	return out
}
