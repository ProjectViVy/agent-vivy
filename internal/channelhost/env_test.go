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

// TestSecretResolvesSettingsDeclaredEnvKeys (CH-C6/D2): a channel envelope
// may declare additional env_key names through top-level `<name>_env`
// entries of its opaque settings block — the multi-credential pattern
// (dingtalk needs client_id_env + client_secret_env) that a single
// token_env slot cannot carry. The envelope token_env may stay empty; only
// settings-declared names resolve then. Unknown names still fail closed
// and errors never carry values (D-010).
func TestSecretResolvesSettingsDeclaredEnvKeys(t *testing.T) {
	t.Setenv("VIVY_TEST_CLIENT_ID", "id-value")
	t.Setenv("VIVY_TEST_CLIENT_SECRET", "secret-value")
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte("enabled: true\nsettings:\n  client_id_env: VIVY_TEST_CLIENT_ID\n  client_secret_env: VIVY_TEST_CLIENT_SECRET\n"), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	_, env := envHostWithEnvelope(t, "dingtalk-like", envelope)

	for _, name := range []string{"VIVY_TEST_CLIENT_ID", "VIVY_TEST_CLIENT_SECRET"} {
		value, err := env.Secret(name)
		if err != nil {
			t.Fatalf("settings-declared env_key %q: %v", name, err)
		}
		if value == "" {
			t.Fatalf("settings-declared env_key %q resolved empty", name)
		}
	}

	if _, err := env.Secret("VIVY_TEST_CLIENT_OTHER"); err == nil {
		t.Fatal("undeclared env_key must fail closed")
	} else if strings.Contains(err.Error(), "id-value") || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error message leaks a secret value: %v", err)
	}
}

// TestSecretIgnoresNonStringAndNestedEnvEntries (CH-C6/D2): only top-level
// string-valued `<name>_env` settings entries declare env_key names.
// Non-string scalars (number, bool), nested mappings, and keys buried one
// level deeper declare nothing — with the variables set, a wrong
// declaration would make Secret succeed, so every case below must fail.
// The one top-level string sibling proves the walk itself works.
func TestSecretIgnoresNonStringAndNestedEnvEntries(t *testing.T) {
	t.Setenv("VIVY_TEST_TRUE", "v")
	t.Setenv("VIVY_TEST_DEEP", "v")
	t.Setenv("VIVY_TEST_INNER", "v")
	t.Setenv("VIVY_TEST_STRING", "v")
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte(
		"enabled: true\n"+
			"settings:\n"+
			"  numeric_env: 123\n"+
			"  bool_env: true\n"+
			"  nested_env:\n"+
			"    inner_env: VIVY_TEST_INNER\n"+
			"  deep:\n"+
			"    deep_env: VIVY_TEST_DEEP\n"+
			"  string_env: VIVY_TEST_STRING\n"), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	_, env := envHostWithEnvelope(t, "mixed", envelope)

	if _, err := env.Secret("VIVY_TEST_STRING"); err != nil {
		t.Fatalf("top-level string *_env sibling must resolve: %v", err)
	}
	for _, name := range []string{"123", "true", "VIVY_TEST_INNER", "VIVY_TEST_DEEP"} {
		if _, err := env.Secret(name); err == nil {
			t.Fatalf("env_key %q comes from a non-string or nested *_env entry and must fail closed", name)
		}
	}
}

// TestSecretFailsWhenSettingsDeclareNothing (CH-C6/D2): a settings block
// without any *_env entry grants no secret beyond the envelope token_env;
// with the token_env absent too, everything fails closed.
func TestSecretFailsWhenSettingsDeclareNothing(t *testing.T) {
	t.Setenv("VIVY_TEST_BOT_TOKEN", "tok-secret-value")
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte("enabled: true\nsettings:\n  plain: value\n"), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	_, env := envHostWithEnvelope(t, "nothing", envelope)
	if _, err := env.Secret("VIVY_TEST_BOT_TOKEN"); err == nil {
		t.Fatal("secret with no declared name anywhere must fail closed")
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
