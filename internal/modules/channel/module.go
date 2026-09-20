// Package channel implements the canonical build-owned Channel Host Module.
package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/channelhost"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type factory struct{}

// NewFactory returns the canonical Channel owner factory. Construction is
// inert; provider networking starts only when the returned owner is started.
func NewFactory() channelcontract.Factory { return factory{} }

type owned struct {
	mu                sync.RWMutex
	host              *channelhost.Host
	config            channelcontract.Config
	settings          channelcontract.SettingsAccess
	maintenance       storage.ChannelMaintenanceStore
	logger            *slog.Logger
	onSettingsChanged func()
	processAvailable  bool
	startOnce         sync.Once
	stopOnce          sync.Once
	closeOnce         sync.Once
	startErr          error
	closeErr          error
}

func (factory) Construct(_ context.Context, deps channelcontract.Dependencies, selection channelcontract.Selection) (channelcontract.Owned, error) {
	if deps.Journal == nil || deps.Messages == nil || deps.Sessions == nil {
		return nil, errors.New("channel module: journal, messages, and sessions stores are required")
	}
	if deps.Run == nil {
		return nil, errors.New("channel module: run callback is required")
	}
	if selection.ProcessAvailable && deps.Deliveries == nil {
		return nil, errors.New("channel module: durable delivery store is required when process availability is enabled")
	}

	configured, err := hostConfig(selection.Config)
	if err != nil {
		return nil, err
	}
	providers, err := bindProviders(selection.Providers, selection.Grants, selection.Config)
	if err != nil {
		return nil, err
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	host := channelhost.New(channelhost.Deps{
		Journal:    deps.Journal,
		Messages:   deps.Messages,
		Sessions:   deps.Sessions,
		Deliveries: deps.Deliveries,
		RunPrepared: func(ctx context.Context, sessionID domain.SessionID, text string, attachments []domain.Attachment, provenance *domain.Provenance, prepare channelhost.PrepareRunFunc) (domain.RunID, error) {
			return deps.Run(ctx, sessionID, text, attachments, provenance, channelcontract.PrepareRunCallback(prepare))
		},
		Approvals:      deps.Approvals,
		Runs:           deps.Runs,
		DecideApproval: channelhost.DecideApprovalFunc(deps.DecideApproval),
		Channels:       providers,
		Config:         configured,
		Credentials:    deps.Credentials,
		Logger:         logger,
	})
	return &owned{
		host:              host,
		config:            cloneContractConfig(selection.Config),
		settings:          deps.Settings,
		maintenance:       deps.Maintenance,
		logger:            logger,
		onSettingsChanged: deps.OnSettingsChanged,
		processAvailable:  selection.ProcessAvailable,
	}, nil
}

func cloneContractConfig(input channelcontract.Config) channelcontract.Config {
	out := make(channelcontract.Config, len(input))
	for name, envelope := range input {
		envelope.AllowFrom = append([]string(nil), envelope.AllowFrom...)
		envelope.Settings = append(json.RawMessage(nil), envelope.Settings...)
		out[name] = envelope
	}
	return out
}

func hostConfig(input channelcontract.Config) (config.Channels, error) {
	out := make(config.Channels, len(input))
	for name, envelope := range input {
		hostEnvelope := config.ChannelEnvelope{
			Enabled:   envelope.Enabled,
			AllowFrom: append([]string(nil), envelope.AllowFrom...),
			TokenEnv:  envelope.TokenEnv,
		}
		if len(envelope.Settings) > 0 {
			if !json.Valid(envelope.Settings) {
				return nil, fmt.Errorf("channel module: decode channels.%s settings: invalid JSON", name)
			}
			var document yaml.Node
			if err := yaml.Unmarshal(envelope.Settings, &document); err != nil {
				return nil, fmt.Errorf("channel module: decode channels.%s settings: %w", name, err)
			}
			if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
				return nil, fmt.Errorf("channel module: decode channels.%s settings: expected one JSON value", name)
			}
			hostEnvelope.Settings = *document.Content[0]
		}
		out[name] = hostEnvelope
	}
	return out, nil
}

func (o *owned) Start(ctx context.Context) error {
	o.startOnce.Do(func() {
		if !o.processAvailable {
			return
		}
		o.mu.RLock()
		host := o.host
		o.mu.RUnlock()
		if host == nil {
			o.startErr = errors.New("channel module: owner is closed")
			return
		}
		if o.maintenance != nil {
			cutoff := time.Now().Add(-storage.ChannelInboundRetention)
			if rows, err := o.maintenance.PruneChannelInboundEvents(ctx, cutoff); err != nil {
				o.logger.Warn("channel inbound event prune failed", "err", err)
			} else if rows > 0 {
				o.logger.Info("channel inbound events pruned", "rows", rows)
			}
		}
		o.startErr = host.StartAll(ctx)
	})
	return o.startErr
}

func (o *owned) Ready(context.Context) error { return o.startErr }

func (o *owned) Stop(ctx context.Context) error {
	o.stopOnce.Do(func() {
		o.mu.RLock()
		host := o.host
		o.mu.RUnlock()
		if host != nil {
			host.StopAll(ctx)
		}
	})
	return nil
}

func (o *owned) Close(ctx context.Context) error {
	o.closeOnce.Do(func() {
		o.closeErr = o.Stop(ctx)
		o.mu.Lock()
		o.host = nil
		o.config = nil
		o.settings = nil
		o.maintenance = nil
		o.logger = nil
		o.onSettingsChanged = nil
		o.mu.Unlock()
	})
	return o.closeErr
}

func (o *owned) OnRunEvent(ctx context.Context, event domain.RunEvent) {
	o.mu.RLock()
	host := o.host
	o.mu.RUnlock()
	if host != nil {
		host.OnRunEvent(ctx, event)
	}
}

func (o *owned) Deliver(ctx context.Context, channelName, to, content string) error {
	o.mu.RLock()
	host := o.host
	o.mu.RUnlock()
	if host == nil {
		return errors.New("channel module: owner is closed")
	}
	return host.Deliver(ctx, channelName, to, content)
}

func (o *owned) Inspect() channelcontract.State {
	state := channelcontract.State{Compiled: true, ProcessAvailable: o.processAvailable}
	o.mu.RLock()
	host := o.host
	o.mu.RUnlock()
	if host == nil {
		return state
	}
	statuses := host.Inspect()
	state.Providers = make([]channelcontract.ProviderState, 0, len(statuses))
	for _, status := range statuses {
		state.Providers = append(state.Providers, channelcontract.ProviderState{
			Name:       status.Name,
			Compiled:   true,
			Configured: status.Configured,
			Running:    status.Started,
			Error:      status.Note,
		})
	}
	return state
}
