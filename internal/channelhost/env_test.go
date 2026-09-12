package channelhost

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/config"
	credentialmodule "agent-vivy/internal/modules/credential"
	plugin "agent-vivy/sdk/port/channel"
)

// grantStub is a channel stub with selectable grants, used to test the
// env surface without the fake's fixed grant set.
type grantStub struct {
	name   string
	grants []plugin.Grant
}

func (s grantStub) Name() string                                   { return s.name }
func (s grantStub) Grants() []plugin.Grant                         { return s.grants }
func (s grantStub) Start(context.Context, plugin.ChannelEnv) error { return nil }
func (s grantStub) Stop(context.Context) error                     { return nil }
func (s grantStub) Send(context.Context, plugin.OutboundMessage) ([]string, error) {
	return nil, nil
}

// Compile-time: the env handed to adapters satisfies the ABI.
var _ plugin.ChannelEnv = (*hostEnv)(nil)

// TestChannelEnvLoggerFace (CH-C6-N1): the env exposes the optional
// plugin.ChannelLogger face and the returned logger is pre-scoped with
// the channel name, so adapter lifecycle lines land in the kernel log
// under the right channel without the adapter naming itself.
func TestChannelEnvLoggerFace(t *testing.T) {
	var buf bytes.Buffer
	host := New(Deps{Logger: slog.New(slog.NewTextHandler(&buf, nil))})
	env := host.envFor(grantStub{name: "probe"})
	lc, ok := env.(plugin.ChannelLogger)
	if !ok {
		t.Fatal("hostEnv must implement plugin.ChannelLogger")
	}
	logger := lc.Logger()
	if logger == nil {
		t.Fatal("Logger() returned nil")
	}
	logger.Warn("probe line")
	if out := buf.String(); !strings.Contains(out, "channel=probe") || !strings.Contains(out, "probe line") {
		t.Fatalf("channel-scoped logger output = %q", out)
	}
}

func envHostWithEnvelope(t *testing.T, name string, envelope config.ChannelEnvelope) (*Host, plugin.ChannelEnv) {
	t.Helper()
	credentials, err := credentialmodule.Compose(credentialmodule.CompileScopes(nil, config.Channels{name: envelope}))
	if err != nil {
		t.Fatal(err)
	}
	host := New(Deps{
		Config:      config.Channels{name: envelope},
		Channels:    []plugin.Channel{grantStub{name: name, grants: []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}}},
		Logger:      testLogger(),
		Credentials: credentials,
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

// TestSecretRefusesMalformedSettingsEnvName (CH-C6-N2): a top-level string
// `*_env` entry whose declared name is not a well-formed environment
// variable name grants no secret — even when a variable under that exact
// malformed name is set, so the refusal is observable. A valid sibling
// still resolves.
func TestSecretRefusesMalformedSettingsEnvName(t *testing.T) {
	t.Setenv("bad-name", "leak-attempt")
	t.Setenv("VIVY_TEST_GOOD_NAME", "good-value")
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte(
		"enabled: true\n"+
			"settings:\n"+
			"  client_id_env: bad-name\n"+
			"  client_secret_env: VIVY_TEST_GOOD_NAME\n"), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	_, env := envHostWithEnvelope(t, "malformed", envelope)

	if _, err := env.Secret("bad-name"); err == nil {
		t.Fatal("malformed declared name must grant no secret, even when the variable is set")
	} else if strings.Contains(err.Error(), "leak-attempt") {
		t.Fatalf("error message leaks the value: %v", err)
	}
	if value, err := env.Secret("VIVY_TEST_GOOD_NAME"); err != nil || value != "good-value" {
		t.Fatalf("valid sibling *_env declaration must still resolve: %q %v", value, err)
	}
}

// TestStartAllWarnsMalformedSettingsEnvName (CH-C6-N2): the start path
// surfaces each malformed top-level settings `*_env` declaration as a
// warning naming the channel and settings key (names only, no values).
func TestStartAllWarnsMalformedSettingsEnvName(t *testing.T) {
	var buf bytes.Buffer
	var envelope config.ChannelEnvelope
	if err := yaml.Unmarshal([]byte(
		"enabled: true\n"+
			"allow_from: [alice]\n"+
			"settings:\n"+
			"  client_id_env: bad-name\n"+
			"  client_secret_env: VIVY_TEST_GOOD_NAME\n"), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	host := New(Deps{
		Journal:  &recordingJournal{Journal: backend},
		Messages: backend,
		Sessions: backend,
		Run:      runs.run,
		Channels: []plugin.Channel{grantStub{name: "audit", grants: []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}}},
		Config:   config.Channels{"audit": envelope},
		Logger:   slog.New(slog.NewTextHandler(&buf, nil)),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "malformed environment variable name") ||
		!strings.Contains(out, "client_id_env") || !strings.Contains(out, "bad-name") {
		t.Fatalf("start log lacks the malformed *_env warning: %s", out)
	}
	if strings.Count(out, "malformed environment variable name") != 1 {
		t.Fatalf("valid sibling must not be warned: %s", out)
	}
}
