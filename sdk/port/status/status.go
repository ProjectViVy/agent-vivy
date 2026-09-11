// Package status defines read-only module/runtime status projections. The
// Provider contract intentionally contains no lifecycle, probe, mutation, or
// transport control methods.
package status

import (
	"context"
	"strings"
)

type Request struct {
	InstanceID string
	Scope      string
}

type Item struct {
	ID      string
	State   string
	Message string
	Fields  map[string]string
}

type Snapshot struct {
	Available         bool
	UnavailableReason string
	Revision          string
	Cursor            string
	Items             []Item
}

func NewSnapshot(revision, cursor string, items []Item) Snapshot {
	out := Snapshot{Available: true, Revision: strings.TrimSpace(revision), Cursor: strings.TrimSpace(cursor), Items: make([]Item, 0, len(items))}
	for _, item := range items {
		cloned := item
		if item.Fields != nil {
			cloned.Fields = make(map[string]string, len(item.Fields))
			for key, value := range item.Fields {
				cloned.Fields[key] = value
			}
		}
		out.Items = append(out.Items, cloned)
	}
	return out
}

func Unavailable(reason string) Snapshot {
	return Snapshot{Available: false, UnavailableReason: strings.TrimSpace(reason)}
}

type Provider interface {
	ID() string
	Status(context.Context, Request) (Snapshot, error)
}
