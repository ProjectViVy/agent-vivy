package lsp

import (
	"context"
	"sort"
	"strings"
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

// statuses returns only live, initialized servers for one exact workspace.
// Inspection is side-effect free: it never starts or probes a process.
func (m *manager) statuses(root string) []plugin.LanguageServerStatus {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	statuses := make([]plugin.LanguageServerStatus, 0)
	for key := range m.starting {
		if key.root == root {
			statuses = append(statuses, plugin.LanguageServerStatus{Language: key.lang, State: "starting"})
		}
	}
	for key, server := range m.servers {
		if key.root != root || server.exited() {
			continue
		}
		statuses = append(statuses, plugin.LanguageServerStatus{Language: key.lang, State: "initialized"})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Language < statuses[j].Language })
	return statuses
}

// manager owns every live language-server connection. It outlives single
// tool calls (the plugin value is created once per generation) and reaps
// servers idle longer than idleMax.
type manager struct {
	mu       sync.Mutex
	servers  map[serverKey]*server
	starting map[serverKey]*serverStart
}

type serverStart struct {
	done   chan struct{}
	server *server
	err    error
}

func newManager() *manager {
	return &manager{servers: map[serverKey]*server{}, starting: map[serverKey]*serverStart{}}
}

// get returns a healthy connection for the key, spawning and initializing
// one on first use and replacing one whose process died.
func (m *manager) get(ctx context.Context, env plugin.Env, lang language, root string) (*server, error) {
	key := serverKey{lang: lang.Name, root: root}
	m.mu.Lock()
	if s, ok := m.servers[key]; ok {
		if !s.exited() {
			s.touch()
			m.mu.Unlock()
			return s, nil
		}
		_ = s.Close()
		delete(m.servers, key)
	}
	if pending := m.starting[key]; pending != nil {
		m.mu.Unlock()
		select {
		case <-pending.done:
			return pending.server, pending.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	pending := &serverStart{done: make(chan struct{})}
	m.starting[key] = pending
	m.mu.Unlock()

	s, err := startServer(ctx, env, lang, root)
	m.mu.Lock()
	pending.server = s
	pending.err = err
	if err == nil {
		m.servers[key] = s
	}
	delete(m.starting, key)
	close(pending.done)
	m.mu.Unlock()
	return s, err
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
