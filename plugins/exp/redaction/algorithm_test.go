package redaction

import "testing"

func TestRedactVocabulary(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"key sk-live-abcdefghijkl end", "key [REDACTED_SECRET] end"},
		{"Authorization: Bearer abcdefghijklmnop", "Authorization: [REDACTED]"},
		{"mail alice@example.com", "mail [REDACTED_EMAIL]"},
		{"password=hunter2 token: abc123 api_key='private value'", "password=[REDACTED] token: [REDACTED] api_key=[REDACTED]"},
		{"access_token=one client_secret=two GH_TOKEN=three", "access_token=[REDACTED] client_secret=[REDACTED] GH_TOKEN=[REDACTED]"},
		// Ordinary fields survive, including near-misses without a token
		// boundary ("task-" has no \b before "sk-").
		{"run r-1 model claude-3 path /tmp/a.txt task-123456789012",
			"run r-1 model claude-3 path /tmp/a.txt task-123456789012"},
	}
	for _, tc := range cases {
		if got := Redact(tc.in); got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
