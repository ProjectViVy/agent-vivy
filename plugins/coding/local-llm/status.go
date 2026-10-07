package localllm

import (
	"context"
	"sort"
	"strconv"
	"strings"

	statusport "agent-vivy/sdk/port/status"
)

// ID returns the status source identity declared in the Descriptor.
func (*Provider) ID() string { return StatusSourceID }

// Status reports the last-known truth only: the cached discover snapshot.
// A status read must never probe, start, or reconfigure a server —
// operators refresh via the manage action's status/discover ops, which
// update the cache this view reads.
func (p *Provider) Status(ctx context.Context, _ statusport.Request) (statusport.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return statusport.Snapshot{}, err
	}
	p.registry.mu.Lock()
	items := make([]statusport.Item, 0, len(serverSpecs))
	seen := p.registry.lastSeen
	for _, spec := range serverSpecs {
		item := statusport.Item{ID: spec.ID, Fields: map[string]string{"base_url": spec.BaseURL}}
		if st, ok := seen[spec.ID]; ok {
			item.State = "unreachable"
			if st.Reachable {
				item.State = "reachable"
			}
			if st.Error != "" {
				item.Fields["error"] = st.Error
			}
			item.Fields["models"] = strconv.Itoa(len(st.Models))
		} else {
			item.State = "unprobed"
		}
		items = append(items, item)
	}
	revisionParts := make([]string, 0, len(seen))
	for id := range seen {
		revisionParts = append(revisionParts, id)
	}
	p.registry.mu.Unlock()
	sort.Strings(revisionParts)
	revision := "probes:" + strings.Join(revisionParts, ",")
	return statusport.NewSnapshot(revision, "", items), nil
}
