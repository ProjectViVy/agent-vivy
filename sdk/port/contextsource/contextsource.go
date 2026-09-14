// Package contextsource defines the public, data-only Context Source Port.
// Sources return bounded candidate data. They cannot construct model messages,
// write prompts, call a model, start a Run, or execute Tools through this API.
package contextsource

import (
	"context"
	"errors"
)

var ErrVersionUnavailable = errors.New("context source: version unavailable")

type Treatment string

const (
	TreatmentCompetitive Treatment = "competitive"
	TreatmentReserved    Treatment = "reserved"
	TreatmentRequired    Treatment = "required"
)

type VersionMode string

const (
	VersionBestEffort VersionMode = "best-effort"
	VersionExact      VersionMode = "exact"
)

// Scope is the identity boundary a reference was issued for. A reference is
// not a bearer permission: ContextHost rechecks this scope at resolve time.
type Scope struct {
	TenantID    string
	WorkspaceID string
	SessionID   string
}

// ResourceReference identifies provider-owned bytes without moving them into
// the candidate page or Journal. Exact references must resolve to Version or
// fail with ErrVersionUnavailable; returning current bytes under an old label
// is a contract violation.
type ResourceReference struct {
	URI         string
	Version     string
	MediaType   string
	SizeHint    int
	Scope       Scope
	VersionMode VersionMode
	Replayable  bool
}

func (reference ResourceReference) Clone() ResourceReference { return reference }

type Request struct {
	Query       string
	TenantID    string
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
	ValidUntil int64
	Treatment  Treatment
	Resource   *ResourceReference
	Metadata   map[string]string
}

func NewCandidate(candidate Candidate) Candidate {
	if candidate.SizeHint <= 0 {
		candidate.SizeHint = len(candidate.Content)
	}
	if candidate.Metadata != nil {
		metadata := make(map[string]string, len(candidate.Metadata))
		for key, value := range candidate.Metadata {
			metadata[key] = value
		}
		candidate.Metadata = metadata
	}
	if candidate.Resource != nil {
		reference := candidate.Resource.Clone()
		candidate.Resource = &reference
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
	out := Page{Candidates: make([]Candidate, 0, len(candidates)), NextCursor: nextCursor}
	for _, candidate := range candidates {
		out.Candidates = append(out.Candidates, candidate.Clone())
	}
	return out
}

type Provider interface {
	ID() string
	Query(context.Context, Request) (Page, error)
}

type ResolveRequest struct {
	Reference   ResourceReference
	TenantID    string
	SessionID   string
	WorkspaceID string
	MaxBytes    int
}

type Resource struct {
	Reference ResourceReference
	Content   []byte
}

func NewResource(reference ResourceReference, content []byte) Resource {
	return Resource{Reference: reference.Clone(), Content: append([]byte(nil), content...)}
}

func (resource Resource) Clone() Resource {
	return NewResource(resource.Reference, resource.Content)
}

// Resolver is an optional companion implemented by a Provider that returns
// reference candidates. It remains part of std/context-source@v1; ContextHost
// is the sole caller and retains authorization, version, scope, and budget
// authority.
type Resolver interface {
	Resolve(context.Context, ResolveRequest) (Resource, error)
}
