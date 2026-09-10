package discord

import (
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

func TestV1ModuleAndProviderIdentity(t *testing.T) {
	var described module.Module = New()
	if err := described.Descriptor().Validate(); err != nil {
		t.Fatal(err)
	}
	if got := described.Descriptor().Module.ID; got != "vivy/discord" {
		t.Fatalf("module id = %q", got)
	}
	var provider channel.ChannelProvider = NewProvider()
	if got := provider.Definition().ID; got != "vivy.discord" {
		t.Fatalf("provider id = %q", got)
	}
}
