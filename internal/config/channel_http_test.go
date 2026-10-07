package config

import (
	"strings"
	"testing"
)

func TestChannelHTTPValidation(t *testing.T) {
	cfg := func(http *ChannelHTTPConfig, from []string) error {
		return validateChannelHTTP("a2a", ChannelEnvelope{Enabled: true, AllowFrom: from, HTTP: http})
	}
	good := &ChannelHTTPConfig{Listen: "127.0.0.1:8790", PublicBaseURL: "http://127.0.0.1:8790",
		Principal: ChannelHTTPPrincipal{ID: "pens-local", TokenEnv: "VIVY_A2A_PENS_TOKEN"}}
	if err := cfg(good, []string{"pens-local"}); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := []struct {
		name string
		http *ChannelHTTPConfig
		from []string
		want string
	}{
		{"missing listen", &ChannelHTTPConfig{PublicBaseURL: "http://127.0.0.1:8790", Principal: ChannelHTTPPrincipal{ID: "p", TokenEnv: "X"}}, []string{"p"}, "listen is required"},
		{"missing principal", &ChannelHTTPConfig{Listen: "127.0.0.1:8790", Principal: ChannelHTTPPrincipal{TokenEnv: "X"}}, []string{"p"}, "principal.id is required"},
		{"missing token_env", &ChannelHTTPConfig{Listen: "127.0.0.1:8790", Principal: ChannelHTTPPrincipal{ID: "p"}}, []string{"p"}, "token_env"},
		{"nonloopback cleartext", &ChannelHTTPConfig{Listen: "0.0.0.0:8790", Principal: ChannelHTTPPrincipal{ID: "p", TokenEnv: "X"}}, []string{"p"}, "loopback"},
		{"url path", &ChannelHTTPConfig{Listen: "127.0.0.1:8790", PublicBaseURL: "https://h/x", Principal: ChannelHTTPPrincipal{ID: "p", TokenEnv: "X"}}, []string{"p"}, "path"},
		{"url userinfo", &ChannelHTTPConfig{Listen: "127.0.0.1:8790", PublicBaseURL: "https://u:p@h", Principal: ChannelHTTPPrincipal{ID: "p", TokenEnv: "X"}}, []string{"p"}, "userinfo"},
		{"url query", &ChannelHTTPConfig{Listen: "127.0.0.1:8790", PublicBaseURL: "https://h?x=1", Principal: ChannelHTTPPrincipal{ID: "p", TokenEnv: "X"}}, []string{"p"}, "query"},
		{"principal not allowlisted", &ChannelHTTPConfig{Listen: "127.0.0.1:8790", Principal: ChannelHTTPPrincipal{ID: "p", TokenEnv: "X"}}, []string{"other"}, "allow_from"},
		{"disabled channel", good, []string{"pens-local"}, "enabled"},
	}
	for _, tc := range cases {
		env := ChannelEnvelope{Enabled: true, AllowFrom: tc.from, HTTP: tc.http}
		if tc.name == "disabled channel" {
			env.Enabled = false
		}
		err := validateChannelHTTP("a2a", env)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err=%v want %q", tc.name, err, tc.want)
		}
	}
	// IPv6 loopback is valid.
	if err := cfg(&ChannelHTTPConfig{Listen: "[::1]:8790", Principal: ChannelHTTPPrincipal{ID: "p", TokenEnv: "X"}}, []string{"p"}); err != nil {
		t.Fatalf("ipv6 loopback rejected: %v", err)
	}
}
