package tools

import (
	"context"
	"sort"
	"sync"

	"agent-vivy/internal/domain"
)

// ToolActivation is the session-scoped set of deferred tools the model may
// see and call. Seeds are folded from the session's journals
// (tools.exposure_changed plus tool_search results) so activation survives
// restarts and resumes; live updates come from the journal pipeline and the
// tools/activate control path. The zero value is unusable — construct with
// NewToolActivation.
type ToolActivation struct {
	mu    sync.RWMutex
	names map[string]struct{}
}

func NewToolActivation() *ToolActivation {
	return &ToolActivation{names: make(map[string]struct{})}
}

func (a *ToolActivation) Activate(names ...string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, name := range names {
		if name != "" {
			a.names[name] = struct{}{}
		}
	}
}

func (a *ToolActivation) Deactivate(names ...string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, name := range names {
		delete(a.names, name)
	}
}

func (a *ToolActivation) Has(name string) bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.names[name]
	return ok
}

func (a *ToolActivation) Snapshot() []string {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, 0, len(a.names))
	for name := range a.names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

type toolActivationContextKey struct{}

// WithToolActivation binds the run's activation view. Absence means no
// deferred tool is callable: governedTool fails closed with
// toolhost.ErrToolNotActive rather than silently widening visibility.
func WithToolActivation(ctx context.Context, a *ToolActivation) context.Context {
	return context.WithValue(ctx, toolActivationContextKey{}, a)
}

func ToolActivationFromContext(ctx context.Context) *ToolActivation {
	a, _ := ctx.Value(toolActivationContextKey{}).(*ToolActivation)
	return a
}

// fixedVisibleToolNames is the intentionally small, always-disclosed core.
// The names are product surface, not a search catalog: only tools that are
// actually enabled are included by the engine. Everything else active is
// supplied to the dynamic tool-search layer.
var fixedVisibleToolNames = map[string]struct{}{
	AskUserName:     {},
	ListDirName:     {},
	ReadFileName:    {},
	SearchFilesName: {},
	SkillsListName:  {},
	SkillViewName:   {},
	WriteFileName:   {},
	PatchName:       {},
	MultiEditName:   {},
	ExecuteName:     {},
	BashName:        {},
}

// IsFixedVisibleTool reports whether a tool belongs to the always-disclosed
// core by default (used by ResolveToolExposure when a spec declares no
// explicit exposure).
func IsFixedVisibleTool(name string) bool {
	_, ok := fixedVisibleToolNames[name]
	return ok
}

// ResolveToolExposure returns the effective model-visibility level for a
// spec: an explicit declaration wins, otherwise the legacy split applies —
// the fixed-visible core is direct and every other tool is deferred behind
// tool_search (unchanged from the pre-exposure behavior).
func ResolveToolExposure(spec domain.ToolSpec) domain.ToolExposure {
	if spec.Exposure != domain.ToolExposureUnset {
		return spec.Exposure
	}
	if IsFixedVisibleTool(spec.Name) {
		return domain.ToolExposureDirect
	}
	return domain.ToolExposureDeferred
}
