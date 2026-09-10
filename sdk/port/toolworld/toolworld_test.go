package toolworld

import (
	"context"
	"encoding/json"
	"testing"
)

type testProvider struct{}

func (testProvider) Definition() Definition { return Definition{ID: "fixture.world"} }
func (testProvider) Discover(context.Context, Host) ([]ToolDefinition, error) {
	return []ToolDefinition{{ID: "fixture.world.read"}}, nil
}
func (testProvider) Invoke(context.Context, Host, string, json.RawMessage) (Result, error) {
	return Result{Text: "ok"}, nil
}
func (testProvider) Close(context.Context) error { return nil }

func TestToolWorldProviderIsTyped(t *testing.T) {
	var provider Provider = testProvider{}
	if provider.Definition().ID != "fixture.world" {
		t.Fatal("typed definition lost")
	}
}
