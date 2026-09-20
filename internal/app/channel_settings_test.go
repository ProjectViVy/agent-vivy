package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/config"
)

func TestChannelSettingsAccessPreservesUnrelatedDocumentFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	enabled := true
	_, err := settings.Save(path, settings.Settings{
		Locale:   "zh",
		Channels: []settings.ChannelOverlay{{Name: "dormant", Enabled: &enabled}},
	})
	if err != nil {
		t.Fatal(err)
	}

	access := newChannelSettingsAccess(path, false)
	allow := []string{"alice"}
	_, err = access.Update(context.Background(), channelcontract.ChannelOverlay{Name: "active", AllowFrom: &allow})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Locale != "zh" || len(doc.Channels) != 2 {
		t.Fatalf("document = %+v", doc)
	}
}

func TestChannelSettingsAccessPolicyAndProjection(t *testing.T) {
	readOnly := newChannelSettingsAccess("", true)
	if readOnly.Writable() || !readOnly.Frozen() {
		t.Fatalf("read-only policy writable=%v frozen=%v", readOnly.Writable(), readOnly.Frozen())
	}
	got, err := readOnly.Read(context.Background())
	if err != nil || len(got.Channels) != 0 {
		t.Fatalf("read-only read=%+v err=%v", got, err)
	}

	path := filepath.Join(t.TempDir(), "settings.yaml")
	access := newChannelSettingsAccess(path, false)
	if !access.Writable() || access.Frozen() {
		t.Fatalf("writable policy writable=%v frozen=%v", access.Writable(), access.Frozen())
	}
	empty := []string{}
	token := "CHANNEL_TOKEN"
	saved, err := access.Update(context.Background(), channelcontract.ChannelOverlay{
		Name: "fake", AllowFrom: &empty, TokenEnv: &token,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Channels) != 1 || saved.Channels[0].AllowFrom == nil || len(*saved.Channels[0].AllowFrom) != 0 || saved.Channels[0].TokenEnv == nil || *saved.Channels[0].TokenEnv != token {
		t.Fatalf("saved projection=%+v", saved)
	}
}

func TestChannelSettingsAccessMarksValidationAndHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	access := newChannelSettingsAccess(path, false)
	wildcard := []string{"*"}
	_, err := access.Update(context.Background(), channelcontract.ChannelOverlay{Name: "fake", AllowFrom: &wildcard})
	if !errors.Is(err, channelcontract.ErrInvalidSettings) {
		t.Fatalf("validation error=%v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := access.Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read error=%v", err)
	}
	if _, err := access.Update(ctx, channelcontract.ChannelOverlay{Name: "fake"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled update error=%v", err)
	}
}

func TestChannelSelectionConfigCopiesOpaqueSettings(t *testing.T) {
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte("enabled: true\nallow_from: [alice]\ntoken_env: TOKEN\nsettings:\n  nested:\n    value: 7\n"), &envelope); err != nil {
		t.Fatal(err)
	}
	input := config.Channels{"fake": envelope}
	got, err := channelSelectionConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	selected := got["fake"]
	if !selected.Enabled || selected.TokenEnv != "TOKEN" || string(selected.Settings) != `{"nested":{"value":7}}` {
		t.Fatalf("selection=%+v settings=%s", selected, selected.Settings)
	}
	input["fake"].AllowFrom[0] = "mutated"
	if selected.AllowFrom[0] != "alice" {
		t.Fatal("selection retained caller slice alias")
	}
}
