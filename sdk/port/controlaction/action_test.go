package controlaction

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type typedProvider struct{}

func (typedProvider) Definition() Definition {
	return Definition{
		ID:           "example.action.read",
		Owner:        "example/module",
		Effect:       EffectRead,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		ResultSchema: json.RawMessage(`{"type":"object"}`),
	}
}

func (typedProvider) Invoke(context.Context, Host, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true}`), nil
}

func TestProviderIsTyped(t *testing.T) {
	var provider Provider = typedProvider{}
	if provider.Definition().ID != "example.action.read" {
		t.Fatalf("definition id = %q", provider.Definition().ID)
	}
}

func TestDefinitionValidateRequiresOwnerSchemasAndEffect(t *testing.T) {
	base := typedProvider{}.Definition()
	tests := []struct {
		name string
		edit func(*Definition)
		want string
	}{
		{name: "missing owner", edit: func(d *Definition) { d.Owner = "" }, want: "owner"},
		{name: "missing input schema", edit: func(d *Definition) { d.InputSchema = nil }, want: "input schema"},
		{name: "missing result schema", edit: func(d *Definition) { d.ResultSchema = nil }, want: "result schema"},
		{name: "unknown effect", edit: func(d *Definition) { d.Effect = Effect("network") }, want: "effect"},
		{name: "invalid schema", edit: func(d *Definition) { d.InputSchema = json.RawMessage(`{"type":`) }, want: "input schema"},
		{name: "action outside owner namespace", edit: func(d *Definition) { d.ID = "other.action" }, want: "namespace"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := base
			test.edit(&definition)
			err := definition.Validate()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDefinitionNormalizeAliases(t *testing.T) {
	definition := Definition{
		ID:           "example.action.read",
		ModuleID:     "example/module",
		Effect:       EffectRead,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Grants:       []Grant{GrantRPCClient},
	}
	normalized, err := definition.Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if normalized.Owner != "example/module" || string(normalized.ResultSchema) != `{"type":"object"}` || len(normalized.RequiredGrants) != 1 {
		t.Fatalf("normalized definition = %+v", normalized)
	}
}

func TestDefinitionRejectsConflictingAliases(t *testing.T) {
	definition := typedProvider{}.Definition()
	definition.ModuleID = "other/module"
	if err := definition.Validate(); err == nil || !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("Validate() error = %v, want ErrInvalidDefinition", err)
	}
}

func TestDefinitionAcceptsLiteralPluginNamespace(t *testing.T) {
	definition := typedProvider{}.Definition()
	definition.ID = "plugin.example/module.read"
	if err := definition.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDefinitionRejectsConflictingTimeoutAliases(t *testing.T) {
	definition := typedProvider{}.Definition()
	definition.Timeout = 2 * time.Second
	definition.TimeoutMS = 1
	if err := definition.Validate(); err == nil || !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("Validate() error = %v, want ErrInvalidDefinition", err)
	}
}
