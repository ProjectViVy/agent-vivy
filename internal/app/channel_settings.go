package app

import (
	"context"
	"fmt"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/config"
)

type channelSettingsAccess struct {
	path   string
	frozen bool
}

func newChannelSettingsAccess(path string, frozen bool) channelcontract.SettingsAccess {
	return channelSettingsAccess{path: path, frozen: frozen}
}

func (access channelSettingsAccess) Read(ctx context.Context) (channelcontract.Settings, error) {
	if err := ctx.Err(); err != nil {
		return channelcontract.Settings{}, err
	}
	if !access.Writable() {
		return channelcontract.Settings{}, nil
	}
	document, err := settings.Load(access.path)
	if err != nil {
		return channelcontract.Settings{}, err
	}
	return channelSettingsProjection(document), nil
}

func (access channelSettingsAccess) Update(ctx context.Context, overlay channelcontract.ChannelOverlay) (channelcontract.Settings, error) {
	if err := ctx.Err(); err != nil {
		return channelcontract.Settings{}, err
	}
	if !access.Writable() {
		return channelcontract.Settings{}, fmt.Errorf("settings are read-only in this deployment")
	}
	document, err := settings.Update(access.path, func(current settings.Settings) (settings.Settings, error) {
		return current.UpsertChannelOverlay(settings.ChannelOverlay{
			Name:      overlay.Name,
			Enabled:   cloneBoolPointer(overlay.Enabled),
			AllowFrom: cloneStringSlicePointer(overlay.AllowFrom),
			TokenEnv:  cloneStringPointer(overlay.TokenEnv),
		}), nil
	})
	if settings.IsValidationError(err) {
		return channelcontract.Settings{}, channelcontract.MarkInvalidSettings(err)
	}
	if err != nil {
		return channelcontract.Settings{}, err
	}
	return channelSettingsProjection(document), nil
}

func (access channelSettingsAccess) Writable() bool { return access.path != "" }
func (access channelSettingsAccess) Frozen() bool   { return access.frozen }

func channelSettingsProjection(document settings.Settings) channelcontract.Settings {
	out := channelcontract.Settings{Channels: make([]channelcontract.ChannelOverlay, 0, len(document.Channels))}
	for _, overlay := range document.Channels {
		out.Channels = append(out.Channels, channelcontract.ChannelOverlay{
			Name:      overlay.Name,
			Enabled:   cloneBoolPointer(overlay.Enabled),
			AllowFrom: cloneStringSlicePointer(overlay.AllowFrom),
			TokenEnv:  cloneStringPointer(overlay.TokenEnv),
		})
	}
	return out
}

func channelSelectionConfig(input config.Channels) (channelcontract.Config, error) {
	out := make(channelcontract.Config, len(input))
	for name, envelope := range input {
		opaque, err := config.OpaqueYAMLToJSON(envelope.Settings)
		if err != nil {
			return nil, fmt.Errorf("app: serialize channels.%s settings: %w", name, err)
		}
		out[name] = channelcontract.ProviderConfig{
			Enabled: envelope.Enabled, AllowFrom: append([]string(nil), envelope.AllowFrom...),
			TokenEnv: envelope.TokenEnv, Settings: append([]byte(nil), opaque...),
		}
	}
	return out, nil
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneStringSlicePointer(value *[]string) *[]string {
	if value == nil {
		return nil
	}
	copy := append([]string(nil), (*value)...)
	if copy == nil {
		copy = []string{}
	}
	return &copy
}
