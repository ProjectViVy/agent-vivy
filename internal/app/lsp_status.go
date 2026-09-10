package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/statushost"
	"agent-vivy/internal/storage"
	statusport "agent-vivy/sdk/port/status"
	plugin "agent-vivy/sdk/port/toolworld"
)

const (
	maxLSPStatusProviders     = 8
	maxLSPStatusesPerProvider = 32
	lspStatusTimeout          = 250 * time.Millisecond
)

type lspStatusAdapter struct {
	id       string
	provider plugin.LanguageServerStatusProvider
}

func (adapter lspStatusAdapter) ID() string { return adapter.id }

func (adapter lspStatusAdapter) Status(ctx context.Context, request statusport.Request) (statusport.Snapshot, error) {
	statuses := adapter.provider.LanguageServerStatuses(ctx, request.Scope)
	items := make([]statusport.Item, 0, len(statuses))
	for _, status := range statuses {
		if status.Language == "" || (status.State != "starting" && status.State != "initialized") {
			continue
		}
		items = append(items, statusport.Item{ID: status.Language, State: status.State})
	}
	return statusport.NewSnapshot("", "", items), nil
}

// buildLanguageServerStatusSource adapts the legacy ToolWorld LSP status
// capability into the sole read-only StatusHost. The session lookup only uses
// an already-existing primary workspace; no status read can create, start,
// probe, revive, or reconfigure a language server.
func buildLanguageServerStatusSource(providers []plugin.LanguageServerStatusProvider, runs storage.RunStore, workspaces *runtime.WorkspaceManager) controlrpc.LanguageServerStatusSource {
	latest, ok := runs.(storage.LatestPrimaryRunStore)
	if len(providers) == 0 || !ok || workspaces == nil {
		return nil
	}
	if len(providers) > maxLSPStatusProviders {
		providers = providers[:maxLSPStatusProviders]
	}
	statusProviders := make([]statusport.Provider, 0, len(providers))
	for index, provider := range providers {
		if provider == nil {
			continue
		}
		statusProviders = append(statusProviders, lspStatusAdapter{
			id:       fmt.Sprintf("lsp.%02d", index),
			provider: provider,
		})
	}
	host, err := statushost.New(statushost.Config{
		Providers:           statusProviders,
		Timeout:             lspStatusTimeout,
		MaxProviders:        maxLSPStatusProviders,
		MaxItemsPerProvider: maxLSPStatusesPerProvider,
	})
	if err != nil || len(statusProviders) == 0 {
		return nil
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
		results := host.Read(statusCtx, statusport.Request{InstanceID: string(sessionID), Scope: workspace.Path})
		states := make(map[string]string)
		for _, result := range results {
			if !result.Snapshot.Available {
				return controlrpc.LanguageServerSnapshot{}, nil
			}
			prefix := result.Namespace + "/"
			for _, item := range result.Snapshot.Items {
				language := strings.TrimPrefix(item.ID, prefix)
				if language == "" || (item.State != "starting" && item.State != "initialized") {
					continue
				}
				if previous := states[language]; previous != "initialized" || item.State == "initialized" {
					states[language] = item.State
				}
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
