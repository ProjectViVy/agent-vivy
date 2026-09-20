package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/channelhost"
	"agent-vivy/internal/rpccontract"
)

type channelCapsResult struct {
	Typing      bool `json:"typing"`
	Edit        bool `json:"edit"`
	Delete      bool `json:"delete"`
	Reaction    bool `json:"reaction"`
	Placeholder bool `json:"placeholder"`
	Media       bool `json:"media"`
	MediaStore  bool `json:"media_store"`
	Webhook     bool `json:"webhook"`
	Listen      bool `json:"listen"`
	Stream      bool `json:"stream"`
	Health      bool `json:"health"`
}

type channelStatusResult struct {
	Name         string            `json:"name"`
	Capabilities channelCapsResult `json:"capabilities"`
	Configured   bool              `json:"configured"`
	Enabled      bool              `json:"enabled"`
	AllowFrom    []string          `json:"allow_from"`
	Started      bool              `json:"started"`
	TokenEnv     string            `json:"token_env"`
	TokenEnvSet  bool              `json:"token_env_set"`
	Note         string            `json:"note"`
}

type channelEnvelopeResult struct {
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	AllowFrom  []string `json:"allow_from"`
	TokenEnv   string   `json:"token_env"`
	Configured bool     `json:"configured"`
}

func (o *owned) RPCBindings() []rpccontract.MethodBinding {
	return []rpccontract.MethodBinding{
		{Method: "channel/inspect", Capability: "channel.inspect", Handler: o.inspectChannel},
		{Method: "channel/get", Capability: "channel.get", Handler: o.getChannel},
		{Method: "channel/update", Capability: "channel.update", Handler: o.updateChannel},
	}
}

func (o *owned) inspectChannel(context.Context, rpccontract.Peer, rpccontract.Request) (any, *rpccontract.Error) {
	o.mu.RLock()
	host := o.host
	o.mu.RUnlock()
	if host == nil {
		return nil, &rpccontract.Error{Code: rpccontract.MethodNotFound, Message: "channel host is not configured"}
	}
	statuses := host.Inspect()
	out := make([]channelStatusResult, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, toChannelStatusResult(status))
	}
	return out, nil
}

func toChannelStatusResult(status channelhost.ChannelStatus) channelStatusResult {
	allowFrom := append([]string(nil), status.AllowFrom...)
	if allowFrom == nil {
		allowFrom = []string{}
	}
	capabilities := status.Capabilities
	return channelStatusResult{
		Name: status.Name,
		Capabilities: channelCapsResult{
			Typing: capabilities.Typing, Edit: capabilities.Edit, Delete: capabilities.Delete,
			Reaction: capabilities.Reaction, Placeholder: capabilities.Placeholder,
			Media: capabilities.Media, MediaStore: capabilities.MediaStore,
			Webhook: capabilities.Webhook, Listen: capabilities.Listen,
			Stream: capabilities.Stream, Health: capabilities.Health,
		},
		Configured:  status.Configured,
		Enabled:     status.Enabled,
		AllowFrom:   allowFrom,
		Started:     status.Started,
		TokenEnv:    status.TokenEnv,
		TokenEnvSet: status.TokenEnvSet,
		Note:        status.Note,
	}
}

func (o *owned) getChannel(ctx context.Context, _ rpccontract.Peer, request rpccontract.Request) (any, *rpccontract.Error) {
	var params struct {
		Name string `json:"name"`
	}
	if rpcErr := decodeParams(request, &params); rpcErr != nil {
		return nil, rpcErr
	}
	if !o.compiledChannelSet()[params.Name] {
		return nil, &rpccontract.Error{Code: rpccontract.CodeNotFound, Message: fmt.Sprintf("channel %q is not compiled into this generation", params.Name)}
	}
	saved, rpcErr := o.readSettings(ctx)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return o.channelEnvelopeView(params.Name, saved), nil
}

func (o *owned) updateChannel(ctx context.Context, _ rpccontract.Peer, request rpccontract.Request) (any, *rpccontract.Error) {
	o.mu.RLock()
	settings := o.settings
	onSettingsChanged := o.onSettingsChanged
	o.mu.RUnlock()
	if settings == nil || !settings.Writable() {
		return nil, &rpccontract.Error{Code: rpccontract.CodeConflict, Message: "settings are read-only in this deployment"}
	}
	if settings.Frozen() {
		return nil, &rpccontract.Error{Code: rpccontract.CodeConflict, Message: "this process is locked to an environment-variable provider session and cannot change channels"}
	}
	var params struct {
		Name      string    `json:"name"`
		Enabled   *bool     `json:"enabled"`
		AllowFrom *[]string `json:"allow_from"`
		TokenEnv  *string   `json:"token_env"`
	}
	if rpcErr := decodeParams(request, &params); rpcErr != nil {
		return nil, rpcErr
	}
	if !o.compiledChannelSet()[params.Name] {
		return nil, &rpccontract.Error{Code: rpccontract.InvalidParams, Message: fmt.Sprintf("channel %q is not compiled into this generation", params.Name)}
	}
	saved, err := settings.Update(ctx, channelcontract.ChannelOverlay{
		Name: params.Name, Enabled: params.Enabled, AllowFrom: params.AllowFrom, TokenEnv: params.TokenEnv,
	})
	if errors.Is(err, channelcontract.ErrInvalidSettings) {
		return nil, &rpccontract.Error{Code: rpccontract.InvalidParams, Message: err.Error()}
	}
	if err != nil {
		return nil, internalRPCError()
	}
	if onSettingsChanged != nil {
		onSettingsChanged()
	}
	return o.channelEnvelopeView(params.Name, saved), nil
}

func (o *owned) compiledChannelSet() map[string]bool {
	o.mu.RLock()
	host := o.host
	o.mu.RUnlock()
	out := make(map[string]bool)
	if host == nil {
		return out
	}
	for _, status := range host.Inspect() {
		out[status.Name] = true
	}
	return out
}

func (o *owned) readSettings(ctx context.Context) (channelcontract.Settings, *rpccontract.Error) {
	o.mu.RLock()
	settings := o.settings
	o.mu.RUnlock()
	if settings == nil {
		return channelcontract.Settings{}, nil
	}
	saved, err := settings.Read(ctx)
	if err != nil {
		return channelcontract.Settings{}, internalRPCError()
	}
	return saved, nil
}

func (o *owned) channelEnvelopeView(name string, saved channelcontract.Settings) channelEnvelopeResult {
	o.mu.RLock()
	envelope, configured := o.config[name]
	o.mu.RUnlock()
	envelope.AllowFrom = append([]string(nil), envelope.AllowFrom...)
	for _, overlay := range saved.Channels {
		if overlay.Name != name {
			continue
		}
		configured = true
		if overlay.Enabled != nil {
			envelope.Enabled = *overlay.Enabled
		}
		if overlay.AllowFrom != nil {
			envelope.AllowFrom = append([]string(nil), (*overlay.AllowFrom)...)
		}
		if overlay.TokenEnv != nil {
			envelope.TokenEnv = *overlay.TokenEnv
		}
	}
	if envelope.AllowFrom == nil {
		envelope.AllowFrom = []string{}
	}
	return channelEnvelopeResult{
		Name: name, Enabled: envelope.Enabled, AllowFrom: envelope.AllowFrom,
		TokenEnv: envelope.TokenEnv, Configured: configured,
	}
}

func decodeParams(request rpccontract.Request, target any) *rpccontract.Error {
	if len(request.Params) == 0 || string(request.Params) == "null" {
		return nil
	}
	if err := json.Unmarshal(request.Params, target); err != nil {
		return &rpccontract.Error{Code: rpccontract.InvalidParams, Message: "params must be a valid JSON object"}
	}
	return nil
}

func internalRPCError() *rpccontract.Error {
	return &rpccontract.Error{Code: rpccontract.InternalError, Message: "internal error"}
}
