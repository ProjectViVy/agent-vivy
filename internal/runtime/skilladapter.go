package runtime

import (
	"context"

	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"

	"agent-vivy/internal/skillhost"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/skillsource"
)

const localSkillSourceID = "vivy.local-skills"

// localSkillSource adapts the existing trusted filesystem/CAS backend into
// data-only SkillSource records. Mutation authority remains on
// EinoSkillBackend behind protected Skill tools; this source is read-only.
type localSkillSource struct {
	backend *EinoSkillBackend
}

func (source *localSkillSource) ID() string { return localSkillSourceID }

func (source *localSkillSource) List(ctx context.Context, _ skillsource.Request) ([]skillsource.Summary, error) {
	items, err := source.backend.loadSkillsAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]skillsource.Summary, 0, len(items))
	for _, item := range items {
		out = append(out, localLoadedSkill(item).Summary())
	}
	return out, nil
}

func (source *localSkillSource) Get(ctx context.Context, _ skillsource.Request, id string) (skillsource.Skill, error) {
	item, err := source.backend.loadSkill(ctx, id)
	if err != nil {
		return skillsource.Skill{}, err
	}
	return localLoadedSkill(item), nil
}

func localLoadedSkill(item loadedSkill) skillsource.Skill {
	disabledReason := ""
	if !item.enabled {
		disabledReason = "disabled"
	}
	origin := item.origin
	if origin == "" {
		origin = tools.SkillOriginUser
	}
	return skillsource.Skill{
		ID:             item.name,
		Name:           item.front.Name,
		Description:    item.front.Description,
		Version:        item.hash,
		SourceHash:     item.hash,
		Content:        item.content,
		Available:      item.enabled,
		DisabledReason: disabledReason,
		Always:         item.always,
		UserInvocable:  item.userInvocable,
		DeclaredTools:  append([]string(nil), item.declaredTools...),
		Context:        string(item.front.Context),
		Agent:          item.front.Agent,
		Model:          item.front.Model,
		Metadata:       map[string]string{"origin": origin},
	}
}

// HostedSkillBackend is the only SkillHost -> pinned Eino skill.Backend
// adapter. It carries no mutation methods, so Eino receives instruction data
// without inheriting EinoSkillBackend's filesystem/CAS authority.
type HostedSkillBackend struct {
	host  *skillhost.Host
	local *EinoSkillBackend
}

var _ einoskill.Backend = (*HostedSkillBackend)(nil)
var _ AlwaysSkillsSource = (*HostedSkillBackend)(nil)

func NewHostedSkillBackend(local *EinoSkillBackend, additional ...skillsource.Provider) (*HostedSkillBackend, error) {
	return newHostedSkillBackend(local, skillhost.Config{}, additional...)
}

func newHostedSkillBackend(local *EinoSkillBackend, hostConfig skillhost.Config, additional ...skillsource.Provider) (*HostedSkillBackend, error) {
	sources := make([]skillsource.Provider, 0, len(additional)+1)
	if local != nil {
		sources = append(sources, &localSkillSource{backend: local})
	}
	sources = append(sources, additional...)
	hostConfig.Sources = sources
	host, err := skillhost.New(hostConfig)
	if err != nil {
		return nil, err
	}
	return &HostedSkillBackend{host: host, local: local}, nil
}

func (backend *HostedSkillBackend) List(ctx context.Context) ([]einoskill.FrontMatter, error) {
	items, err := backend.host.List(ctx, hostedSkillRequest(ctx))
	if err != nil {
		return nil, err
	}
	items = skillhost.StableSummaries(items)
	out := make([]einoskill.FrontMatter, 0, len(items))
	for _, item := range items {
		out = append(out, einoskill.FrontMatter{
			// Eino's pinned Backend contract has only a Name lookup key.
			// Emit the stable Host ID as that key; Get below resolves the
			// exact hosted record and can retain the display name in the
			// returned projection.
			Name:        item.ID,
			Description: item.Description,
			Context:     einoskill.ContextMode(item.Context),
			Agent:       item.Agent,
			Model:       item.Model,
		})
	}
	return out, nil
}

func (backend *HostedSkillBackend) Get(ctx context.Context, name string) (einoskill.Skill, error) {
	resolved, err := backend.host.Get(ctx, hostedSkillRequest(ctx), name)
	if err != nil {
		return einoskill.Skill{}, err
	}
	baseDirectory := ""
	if resolved.SourceID == localSkillSourceID && backend.local != nil {
		baseDirectory, err = backend.local.skillDir(resolved.ID)
		if err != nil {
			return einoskill.Skill{}, err
		}
	}
	return einoskill.Skill{
		FrontMatter: einoskill.FrontMatter{
			Name:        resolved.Name,
			Description: resolved.Description,
			Context:     einoskill.ContextMode(resolved.Context),
			Agent:       resolved.Agent,
			Model:       resolved.Model,
		},
		Content:       resolved.Content,
		BaseDirectory: baseDirectory,
	}, nil
}

// AlwaysSkills keeps the existing always-injection semantics while making
// both catalog selection and body resolution traverse SkillHost.
func (backend *HostedSkillBackend) AlwaysSkills(ctx context.Context) (string, error) {
	return backend.host.Always(ctx, hostedSkillRequest(ctx))
}

func hostedSkillRequest(ctx context.Context) skillhost.Request {
	sessionID := contextSessionID(ctx)
	if sessionID == "" {
		sessionID = tools.SessionIDFromContext(ctx)
	}
	workspaceID := contextWorkspaceID(ctx)
	if workspaceID == "" {
		workspaceID = tools.WorkspaceIDFromContext(ctx)
	}
	return skillhost.Request{
		SessionID:   string(sessionID),
		WorkspaceID: workspaceID,
	}
}
