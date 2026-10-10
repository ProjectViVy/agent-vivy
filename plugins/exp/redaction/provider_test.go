package redaction

import (
	"context"
	"encoding/json"
	"testing"
)

func TestExplicitRedactionTool(t *testing.T) {
	provider := NewProvider()
	input := json.RawMessage(`{"text":"mail alice@example.com password=hunter2 sk-live-abcdefghijkl"}`)
	result, err := provider.Invoke(context.Background(), nil, input)
	if err != nil || result.Text != "mail [REDACTED_EMAIL] password=[REDACTED] [REDACTED_SECRET]" {
		t.Fatalf("explicit result=%q err=%v", result.Text, err)
	}
	for _, invalid := range []string{`{}`, `{"text":1}`, `{"text":"ok","extra":true}`} {
		if _, err := provider.Invoke(context.Background(), nil, json.RawMessage(invalid)); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
	result, err = provider.Invoke(context.Background(), nil, json.RawMessage(`{"text":""}`))
	if err != nil || result.Text != "" {
		t.Fatalf("empty text=%q err=%v", result.Text, err)
	}
}
