package channelhost

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/plugin"
)

// grantStub is a channel stub with selectable grants, used to test the
// env surface without the fake's fixed grant set.
type grantStub struct {
	name   string
	grants []plugin.Grant
}

func (s grantStub) Name() string                                   { return s.name }
func (s grantStub) Seam() plugin.Seam                              { return plugin.SeamChannel }
func (s grantStub) Grants() []plugin.Grant                         { return s.grants }
func (s grantStub) Tools() []plugin.Tool                           { return nil }
func (s grantStub) Start(context.Context, plugin.ChannelEnv) error { return nil }
func (s grantStub) Stop(context.Context) error                     { return nil }
func (s grantStub) Send(context.Context, plugin.OutboundMessage) ([]string, error) {
	return nil, nil
}

// Compile-time: the env handed to adapters satisfies the ABI.
var _ plugin.ChannelEnv = (*hostEnv)(nil)

func envHostWithEnvelope(t *testing.T, name string, envelope config.ChannelEnvelope) (*Host, plugin.ChannelEnv) {
	t.Helper()
	host := New(Deps{
		Config:   config.Channels{name: envelope},
		Channels: []plugin.Channel{grantStub{name: name, grants: []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}}},
		Logger:   testLogger(),
	})
	return host, host.envFor(grantStub{name: name, grants: []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}})
}

// TestSecretPinnedToEnvelopeTokenEnv (CH-C3-N2 hardening): Secret resolves
// only the env_key the channel envelope declares. Matching name resolves,
// a different name fails closed, and no path leaks the value.
func TestSecretPinnedToEnvelopeTokenEnv(t *testing.T) {
	t.Setenv("VIVY_TEST_BOT_TOKEN", "tok-secret-value")
	host, env := envHostWithEnvelope(t, "pinned", config.ChannelEnvelope{TokenEnv: "VIVY_TEST_BOT_TOKEN"})

	value, err := env.Secret("VIVY_TEST_BOT_TOKEN")
	if err != nil {
		t.Fatalf("secret with matching token_env: %v", err)
	}
	if value != "tok-secret-value" {
		t.Fatal("secret value mismatch")
	}

	if _, err := env.Secret("VIVY_TEST_OTHER_TOKEN"); err == nil {
		t.Fatal("secret with a foreign env_key must fail closed")
	} else if strings.Contains(err.Error(), "tok-secret-value") {
		t.Fatalf("error message leaks the secret value: %v", err)
	}

	if _, err := env.Secret(""); err == nil {
		t.Fatal("empty env_key must fail closed")
	}
	_ = host
}

// TestSecretFailsWithoutTokenEnvDeclaration: an envelope without token_env
// grants no secret at all, even when an env variable of the same name is
// set and the grant is present.
func TestSecretFailsWithoutTokenEnvDeclaration(t *testing.T) {
	t.Setenv("VIVY_TEST_BOT_TOKEN", "tok-secret-value")
	_, env := envHostWithEnvelope(t, "undeclared", config.ChannelEnvelope{})

	if _, err := env.Secret("VIVY_TEST_BOT_TOKEN"); err == nil {
		t.Fatal("secret without a declared token_env must fail closed")
	}
}

// TestSecretDeniedWithoutGrant: the secret.read grant is still required
// before the pinning check.
func TestSecretDeniedWithoutGrant(t *testing.T) {
	host := New(Deps{
		Config:   config.Channels{"nogrant": {TokenEnv: "VIVY_TEST_BOT_TOKEN"}},
		Channels: []plugin.Channel{grantStub{name: "nogrant", grants: []plugin.Grant{plugin.GrantChannelPoll}}},
		Logger:   testLogger(),
	})
	env := host.envFor(grantStub{name: "nogrant", grants: []plugin.Grant{plugin.GrantChannelPoll}})
	if _, err := env.Secret("VIVY_TEST_BOT_TOKEN"); err != plugin.ErrDenied {
		t.Fatalf("secret without secret.read grant = %v, want ErrDenied", err)
	}
}

// TestSecretFailsOnUnsetVariable: a matching, declared env_key whose
// variable is unset or empty fails closed.
func TestSecretFailsOnUnsetVariable(t *testing.T) {
	t.Setenv("VIVY_TEST_EMPTY_TOKEN", "")
	_, env := envHostWithEnvelope(t, "unset", config.ChannelEnvelope{TokenEnv: "VIVY_TEST_EMPTY_TOKEN"})
	if _, err := env.Secret("VIVY_TEST_EMPTY_TOKEN"); err == nil {
		t.Fatal("secret for an empty variable must fail closed")
	}
	if _, err := env.Secret("VIVY_TEST_MISSING_TOKEN"); err == nil {
		t.Fatal("secret for an unset variable must fail closed")
	}
	_ = os.Unsetenv // keep os imported for future env manipulations
}

// TestEnvSettingsRoundTrip: the opaque settings yaml node arrives at the
// adapter as JSON, nested shapes included; absent settings arrive as `{}`.
func TestEnvSettingsRoundTrip(t *testing.T) {
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte("enabled: true\nsettings:\n  token_env: TELEGRAM_BOT_TOKEN\n  nested:\n    deep: 7\n"), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	_, env := envHostWithEnvelope(t, "settings", envelope)

	var got struct {
		TokenEnv string `json:"token_env"`
		Nested   struct {
			Deep int `json:"deep"`
		} `json:"nested"`
	}
	if err := json.Unmarshal(env.Settings(), &got); err != nil {
		t.Fatalf("settings are not valid json: %v", err)
	}
	if got.TokenEnv != "TELEGRAM_BOT_TOKEN" || got.Nested.Deep != 7 {
		t.Fatalf("settings round trip = %+v", got)
	}

	_, emptyEnv := envHostWithEnvelope(t, "no-settings", config.ChannelEnvelope{Enabled: true})
	if string(emptyEnv.Settings()) != "{}" {
		t.Fatalf("absent settings = %s, want {}", emptyEnv.Settings())
	}
}

// TestEnvSettingsNullArrivesAsEmptyObject: an explicit null settings block
// must decode as an empty object on the adapter side, not as null.
func TestEnvSettingsNullArrivesAsEmptyObject(t *testing.T) {
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte("enabled: true\nsettings: null\n"), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	_, env := envHostWithEnvelope(t, "null-settings", envelope)
	var obj map[string]any
	if err := json.Unmarshal(env.Settings(), &obj); err != nil {
		t.Fatalf("null settings did not arrive as a json object: %v", err)
	}
	if len(obj) != 0 {
		t.Fatalf("null settings arrived non-empty: %v", obj)
	}
}
