// Package skillsource defines the public, read-only Skill Source Port. Skill
// text is instruction data only; loading it grants no Tool, filesystem,
// network, Secret, Agent, or Model authority.
package skillsource

import (
	"context"
	"strings"
)

type Request struct {
	SessionID   string
	WorkspaceID string
}

type Summary struct {
	ID             string
	Name           string
	Description    string
	Version        string
	SourceHash     string
	Available      bool
	DisabledReason string
	Dependencies   []string
	Always         bool
	UserInvocable  bool
	DeclaredTools  []string
	Context        string
	Agent          string
	Model          string
	Metadata       map[string]string
}

type Skill struct {
	ID             string
	Name           string
	Description    string
	Version        string
	SourceHash     string
	Content        string
	Available      bool
	DisabledReason string
	Dependencies   []string
	Always         bool
	UserInvocable  bool
	DeclaredTools  []string
	Context        string
	Agent          string
	Model          string
	Metadata       map[string]string
}

func (skill Skill) Clone() Skill {
	skill.ID = strings.TrimSpace(skill.ID)
	skill.Name = strings.TrimSpace(skill.Name)
	skill.Version = strings.TrimSpace(skill.Version)
	skill.SourceHash = strings.TrimSpace(skill.SourceHash)
	skill.Context = strings.TrimSpace(skill.Context)
	skill.Agent = strings.TrimSpace(skill.Agent)
	skill.Model = strings.TrimSpace(skill.Model)
	skill.Dependencies = append([]string(nil), skill.Dependencies...)
	skill.DeclaredTools = append([]string(nil), skill.DeclaredTools...)
	if skill.Metadata != nil {
		metadata := make(map[string]string, len(skill.Metadata))
		for key, value := range skill.Metadata {
			metadata[strings.TrimSpace(key)] = value
		}
		skill.Metadata = metadata
	}
	return skill
}

func (skill Skill) Summary() Summary {
	clone := skill.Clone()
	return Summary{
		ID: clone.ID, Name: clone.Name, Description: clone.Description,
		Version: clone.Version, SourceHash: clone.SourceHash,
		Available: clone.Available, DisabledReason: clone.DisabledReason,
		Dependencies: clone.Dependencies, Always: clone.Always,
		UserInvocable: clone.UserInvocable, DeclaredTools: clone.DeclaredTools,
		Context: clone.Context, Agent: clone.Agent, Model: clone.Model,
		Metadata: clone.Metadata,
	}
}

type Provider interface {
	ID() string
	List(context.Context, Request) ([]Summary, error)
	Get(context.Context, Request, string) (Skill, error)
}
