package rpc

import (
	"context"
	"strings"
	"testing"
)

func validMethodBinding(method, capability string) MethodBinding {
	return MethodBinding{
		Method:     method,
		Capability: capability,
		Handler: func(context.Context, *Peer, Request) (any, *Error) {
			return nil, nil
		},
	}
}

func TestValidateMethodBindings(t *testing.T) {
	tests := []struct {
		name     string
		core     map[string]struct{}
		bindings []MethodBinding
		want     string
	}{
		{name: "valid", bindings: []MethodBinding{validMethodBinding("channel/inspect", "channel.inspect")}},
		{name: "empty method", bindings: []MethodBinding{validMethodBinding("", "channel.inspect")}, want: "empty method"},
		{name: "empty capability", bindings: []MethodBinding{validMethodBinding("channel/inspect", "")}, want: "empty capability"},
		{name: "nil handler", bindings: []MethodBinding{{Method: "channel/inspect", Capability: "channel.inspect"}}, want: "nil handler"},
		{
			name: "duplicate method",
			bindings: []MethodBinding{
				validMethodBinding("channel/inspect", "channel.inspect"),
				validMethodBinding("channel/inspect", "channel.read"),
			},
			want: `duplicate method "channel/inspect"`,
		},
		{
			name: "repeated capability is valid",
			bindings: []MethodBinding{
				validMethodBinding("channel/get", "channel.manage"),
				validMethodBinding("channel/update", "channel.manage"),
			},
		},
		{
			name:     "core collision",
			core:     map[string]struct{}{"initialize": {}},
			bindings: []MethodBinding{validMethodBinding("initialize", "channel.inspect")},
			want:     `core method "initialize"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMethodBindings(tt.core, tt.bindings)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("ValidateMethodBindings() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateMethodBindings() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestValidateMethodBindingsReportsMethodsDeterministically(t *testing.T) {
	core := map[string]struct{}{"z/core": {}, "a/core": {}}
	bindings := []MethodBinding{
		validMethodBinding("z/core", "z"),
		validMethodBinding("a/core", "a"),
		validMethodBinding("z/core", "duplicate"),
	}

	err := ValidateMethodBindings(core, bindings)
	if err == nil {
		t.Fatal("ValidateMethodBindings() error = nil")
	}
	message := err.Error()
	if strings.Index(message, "a/core") > strings.Index(message, "z/core") {
		t.Fatalf("diagnostic is not sorted: %q", message)
	}
}

type emptyContribution struct{}

func (emptyContribution) RPCBindings() []MethodBinding { return nil }

func TestEmptyContributionHasNoBindings(t *testing.T) {
	var contribution Contribution = emptyContribution{}
	bindings := contribution.RPCBindings()
	if len(bindings) != 0 {
		t.Fatalf("bindings = %d, want 0", len(bindings))
	}
	if err := ValidateMethodBindings(nil, bindings); err != nil {
		t.Fatalf("ValidateMethodBindings() error = %v", err)
	}
}
