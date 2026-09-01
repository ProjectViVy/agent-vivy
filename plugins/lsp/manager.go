package lsp

import (
	"context"
	"sync"
	"time"

	"agent-vivy/sdk/plugin"
)

// serverKey scopes one connection: per language per workspace root. Two
// different run workspaces never share a language server.
type serverKey struct {
	lang string
	root string
}

// manager owns every live language-server connection. It outlives single
// tool calls (the plugin value is created once per generation) and reaps
// servers idle longer than idleMax.
type manager struct {
	mu      sync.Mutex
	servers map[serverKey]*server
}

func newManager() *manager {
	return &manager{servers: map[serverKey]*server{}}
}

// get returns a healthy connection for the key, spawning and initializing
// one on first use and replacing one whose process died.
func (m *manager) get(ctx context.Context, env plugin.Env, lang language, root string) (*server, error) {
	key := serverKey{lang: lang.Name, root: root}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.servers[key]; ok {
		if !s.exited() {
			s.touch()
			return s, nil
		}
		_ = s.Close()
		delete(m.servers, key)
	}
	s, err := startServer(ctx, env, lang, root)
	if err != nil {
		return nil, err
	}
	m.servers[key] = s
	return s, nil
}

const (
	reapInterval = time.Minute
	idleMax      = 10 * time.Minute
)

// startReaper closes connections idle longer than idleMax, once per
// minute, for the lifetime of the process. The plugin contract has no
// Stop hook, so the reaper is the shutdown story for idle servers.
func (m *manager) startReaper() {
	go func() {
		ticker := time.NewTicker(reapInterval)
		defer ticker.Stop()
		for range ticker.C {
			m.reap(idleMax)
		}
	}()
}

func (m *manager) reap(maxIdle time.Duration) {
	cutoff := time.Now().Add(-maxIdle)
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, s := range m.servers {
		if s.exited() || s.idleSince().Before(cutoff) {
			_ = s.Close()
			delete(m.servers, key)
		}
	}
}
