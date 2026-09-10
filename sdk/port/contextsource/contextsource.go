// Package contextsource defines the public, data-only Context Source Port.
// Sources return bounded candidate data. They cannot construct model messages,
// write prompts, call a model, start a Run, or execute Tools through this API.
package contextsource

import (
	"context"
	"strings"
)

type Request struct {
	Query       string
	SessionID   string
	WorkspaceID string
	Cursor      string
	Limit       int
}

type Candidate struct {
	SourceID   string
	ContentID  string
	MediaType  string
	Content    string
	SizeHint   int
	Confidence float64
	Version    string
	UpdatedAt  int64
	Metadata   map[string]string
}

func NewCandidate(candidate Candidate) Candidate {
	candidate.SourceID = strings.TrimSpace(candidate.SourceID)
	candidate.ContentID = strings.TrimSpace(candidate.ContentID)
	candidate.MediaType = strings.TrimSpace(candidate.MediaType)
	candidate.Version = strings.TrimSpace(candidate.Version)
	if candidate.SizeHint <= 0 {
		candidate.SizeHint = len(candidate.Content)
	}
	if candidate.Metadata != nil {
		metadata := make(map[string]string, len(candidate.Metadata))
		for key, value := range candidate.Metadata {
			metadata[strings.TrimSpace(key)] = value
		}
		candidate.Metadata = metadata
	}
	return candidate
}

func (candidate Candidate) Clone() Candidate {
	return NewCandidate(candidate)
}

type Page struct {
	Candidates []Candidate
	NextCursor string
}

func NewPage(candidates []Candidate, nextCursor string) Page {
	out := Page{Candidates: make([]Candidate, 0, len(candidates)), NextCursor: strings.TrimSpace(nextCursor)}
	for _, candidate := range candidates {
		out.Candidates = append(out.Candidates, candidate.Clone())
	}
	return out
}

type Provider interface {
	ID() string
	Query(context.Context, Request) (Page, error)
}
