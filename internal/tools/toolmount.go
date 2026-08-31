package tools

import (
	"context"
	"sync"
)

// MountedTools is the run-scoped record of skill-declared tools activated
// mid-run. Viewing a SKILL.md that declares tools mounts them for the
// remainder of the run: the surface middleware advertises them to the model
// on the next generation and the adapter lets their calls through. Mounts
// never outlive the run — the next run re-views the skill if needed.
type MountedTools struct {
	mu    sync.Mutex
	order []string
	names map[string]struct{}
}

// NewMountedTools builds an empty mount registry.
func NewMountedTools() *MountedTools {
	return &MountedTools{names: make(map[string]struct{})}
}

// Mount records the given tool names in first-seen order (deduplicated).
// A nil receiver is a no-op so callers can mount without checking whether
// the run wired a mount set.
func (m *MountedTools) Mount(names ...string) {
	if m == nil || len(names) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := m.names[name]; ok {
			continue
		}
		m.names[name] = struct{}{}
		m.order = append(m.order, name)
	}
}

// Mounted returns the mounted names in first-seen order.
func (m *MountedTools) Mounted() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.order...)
}

// Has reports whether name is mounted.
func (m *MountedTools) Has(name string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.names[name]
	return ok
}

type mountedToolsContextKey struct{}

// WithMountedTools binds the run's mount registry. Absence means the run
// carries no mount surface: skill_view then reports declarations without
// mounting anything.
func WithMountedTools(ctx context.Context, m *MountedTools) context.Context {
	return context.WithValue(ctx, mountedToolsContextKey{}, m)
}

// MountedToolsFromContext returns the run's mount registry, or nil.
func MountedToolsFromContext(ctx context.Context) *MountedTools {
	m, _ := ctx.Value(mountedToolsContextKey{}).(*MountedTools)
	return m
}

// MountSkillTools records the tools a viewed skill declares as mounted for
// this run. Missing registry is a no-op.
func MountSkillTools(ctx context.Context, names []string) {
	if m := MountedToolsFromContext(ctx); m != nil {
		m.Mount(names...)
	}
}
