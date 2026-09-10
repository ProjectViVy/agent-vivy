package app

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	plugin "agent-vivy/sdk/port/toolworld"
)

const (
	maxLSPStatusProviders     = 8
	maxLSPStatusesPerProvider = 32
	maxLSPStatusInflight      = 64
	lspStatusTimeout          = 250 * time.Millisecond
)

type lspProviderCall struct {
	done     chan struct{}
	statuses []plugin.LanguageServerStatus
	ok       bool
}

type lspProviderCallKey struct {
	provider int
	root     string
}

// buildLanguageServerStatusSource keeps plugin-private process state behind a
// bounded session-to-latest-primary-workspace ownership boundary. It never
// creates a workspace and returns nil when the generation has no owner.
func buildLanguageServerStatusSource(providers []plugin.LanguageServerStatusProvider, runs storage.RunStore, workspaces *runtime.WorkspaceManager) controlrpc.LanguageServerStatusSource {
	if len(providers) > maxLSPStatusProviders {
		providers = providers[:maxLSPStatusProviders]
	}
	latest, ok := runs.(storage.LatestPrimaryRunStore)
	if len(providers) == 0 || !ok || workspaces == nil {
		return nil
	}

	var callsMu sync.Mutex
	inflight := make(map[lspProviderCallKey]*lspProviderCall)
	startCall := func(ctx context.Context, index int, root string) *lspProviderCall {
		key := lspProviderCallKey{provider: index, root: root}
		callsMu.Lock()
		if call := inflight[key]; call != nil {
			callsMu.Unlock()
			return call
		}
		if len(inflight) >= maxLSPStatusInflight {
			callsMu.Unlock()
			return nil
		}
		call := &lspProviderCall{done: make(chan struct{})}
		inflight[key] = call
		callsMu.Unlock()
		go func() {
			defer func() {
				if recover() != nil {
					call.ok = false
				}
				close(call.done)
				callsMu.Lock()
				delete(inflight, key)
				callsMu.Unlock()
			}()
			call.statuses = providers[index].LanguageServerStatuses(ctx, root)
			if len(call.statuses) > maxLSPStatusesPerProvider {
				call.statuses = call.statuses[:maxLSPStatusesPerProvider]
			}
			call.ok = true
		}()
		return call
	}

	return func(ctx context.Context, sessionID domain.SessionID) (controlrpc.LanguageServerSnapshot, error) {
		run, err := latest.LatestPrimaryRunBySession(ctx, sessionID)
		if errors.Is(err, storage.ErrNotFound) {
			return controlrpc.LanguageServerSnapshot{Known: true}, nil
		}
		if err != nil {
			return controlrpc.LanguageServerSnapshot{}, err
		}
		workspace, exists, err := workspaces.Existing(ctx, run.ID)
		if err != nil {
			return controlrpc.LanguageServerSnapshot{}, err
		}
		if !exists {
			return controlrpc.LanguageServerSnapshot{Known: true}, nil
		}

		statusCtx, cancel := context.WithTimeout(ctx, lspStatusTimeout)
		defer cancel()
		calls := make([]*lspProviderCall, len(providers))
		for index := range providers {
			calls[index] = startCall(statusCtx, index, workspace.Path)
			if calls[index] == nil {
				return controlrpc.LanguageServerSnapshot{}, nil
			}
		}
		states := make(map[string]string)
		for _, call := range calls {
			select {
			case <-call.done:
				if !call.ok {
					return controlrpc.LanguageServerSnapshot{}, nil
				}
				for _, status := range call.statuses {
					if status.Language == "" || (status.State != "starting" && status.State != "initialized") {
						continue
					}
					if previous := states[status.Language]; previous != "initialized" || status.State == "initialized" {
						states[status.Language] = status.State
					}
				}
			case <-statusCtx.Done():
				return controlrpc.LanguageServerSnapshot{}, nil
			}
		}
		out := make([]controlrpc.LanguageServerStatus, 0, len(states))
		for language, state := range states {
			out = append(out, controlrpc.LanguageServerStatus{Language: language, State: state})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Language < out[j].Language })
		return controlrpc.LanguageServerSnapshot{Known: true, Servers: out}, nil
	}
}
