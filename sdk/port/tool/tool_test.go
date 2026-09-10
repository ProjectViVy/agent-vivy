package tool

import (
	"context"
	"encoding/json"
	"testing"
)

type testProvider struct{}

func (testProvider) Definition() Definition { return Definition{ID: "fixture.read"} }
func (testProvider) Invoke(context.Context, Host, json.RawMessage) (Result, error) {
	return Result{Text: "ok"}, nil
}

func TestToolProviderIsTyped(t *testing.T) {
	var provider ToolProvider = testProvider{}
	if provider.Definition().ID != "fixture.read" {
		t.Fatal("typed definition lost")
	}
}
