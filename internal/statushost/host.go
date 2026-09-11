package statushost

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"agent-vivy/internal/tools"
	statusport "agent-vivy/sdk/port/status"
)

const (
	defaultTimeout             = 250 * time.Millisecond
	defaultMaxProviders        = 16
	defaultMaxItemsPerProvider = 32
)

var ErrInvalidProvider = errors.New("statushost: invalid provider")

type Config struct {
	Providers           []statusport.Provider
	Timeout             time.Duration
	MaxProviders        int
	MaxItemsPerProvider int
}

type Host struct {
	providers           []statusport.Provider
	timeout             time.Duration
	maxItemsPerProvider int
}

type Result struct {
	Namespace string
	Snapshot  statusport.Snapshot
}

type providerResult struct {
	snapshot statusport.Snapshot
	err      error
}

func New(cfg Config) (*Host, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxProviders := cfg.MaxProviders
	if maxProviders <= 0 {
		maxProviders = defaultMaxProviders
	}
	maxItems := cfg.MaxItemsPerProvider
	if maxItems <= 0 {
		maxItems = defaultMaxItemsPerProvider
	}
	seen := make(map[string]struct{})
	providers := make([]statusport.Provider, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		if provider == nil || strings.TrimSpace(provider.ID()) == "" {
			return nil, ErrInvalidProvider
		}
		id := strings.TrimSpace(provider.ID())
		if _, duplicate := seen[id]; duplicate {
			return nil, ErrInvalidProvider
		}
		seen[id] = struct{}{}
		providers = append(providers, provider)
	}
	sort.SliceStable(providers, func(i, j int) bool {
		return strings.TrimSpace(providers[i].ID()) < strings.TrimSpace(providers[j].ID())
	})
	if len(providers) > maxProviders {
		providers = providers[:maxProviders]
	}
	return &Host{providers: providers, timeout: timeout, maxItemsPerProvider: maxItems}, nil
}

func (host *Host) Read(ctx context.Context, request statusport.Request) []Result {
	out := make([]Result, 0, len(host.providers))
	for _, provider := range host.providers {
		id := strings.TrimSpace(provider.ID())
		callCtx, cancel := context.WithTimeout(ctx, host.timeout)
		resultCh := make(chan providerResult, 1)
		go func(provider statusport.Provider) {
			result := providerResult{}
			defer func() {
				if recover() != nil {
					result.err = errors.New("provider panic")
				}
				resultCh <- result
			}()
			result.snapshot, result.err = provider.Status(callCtx, request)
		}(provider)

		var snapshot statusport.Snapshot
		select {
		case <-callCtx.Done():
			snapshot = statusport.Unavailable("timeout")
		case result := <-resultCh:
			if result.err != nil {
				snapshot = statusport.Unavailable("provider_error")
			} else {
				snapshot = sanitizeSnapshot(id, result.snapshot, host.maxItemsPerProvider)
			}
		}
		cancel()
		out = append(out, Result{Namespace: id, Snapshot: snapshot})
	}
	return out
}

func sanitizeSnapshot(namespace string, snapshot statusport.Snapshot, maxItems int) statusport.Snapshot {
	if !snapshot.Available {
		return statusport.Unavailable(tools.RedactSensitive(snapshot.UnavailableReason))
	}
	items := snapshot.Items
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	cleaned := make([]statusport.Item, 0, len(items))
	for _, item := range items {
		item.ID = namespace + "/" + strings.Trim(strings.TrimSpace(item.ID), "/")
		item.State = tools.RedactSensitive(item.State)
		item.Message = tools.RedactSensitive(item.Message)
		if item.Fields != nil {
			fields := make(map[string]string, len(item.Fields))
			for key, value := range item.Fields {
				fields[key] = tools.RedactSensitive(value)
			}
			item.Fields = fields
		}
		cleaned = append(cleaned, item)
	}
	return statusport.NewSnapshot(
		tools.RedactSensitive(snapshot.Revision),
		tools.RedactSensitive(snapshot.Cursor),
		cleaned,
	)
}
