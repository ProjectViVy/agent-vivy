package module

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const validSourceHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func validDescriptor() Descriptor {
	return Descriptor{
		APIVersion: APIVersionV1,
		Module: Identity{
			ID:      "example/search-tools",
			Version: "1.2.3",
		},
		Source: Source{
			Ref:    "git:example/search-tools@0123456",
			SHA256: validSourceHash,
		},
		Provides:        []PortRef{{Port: "std/tool@v1", ID: "example.search"}},
		Requires:        []Requirement{{PortRef: PortRef{Port: "core/tool-host@v1"}}},
		RequestedGrants: []Grant{GrantNetClient},
		Lifecycle:       Lifecycle{Scope: ScopeGeneration},
	}
}

func TestDescriptorValidateRejectsV0(t *testing.T) {
	descriptor := validDescriptor()
	legacy := "vivy.plugin/" + "v0"
	descriptor.APIVersion = legacy

	err := descriptor.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported apiVersion "+legacy) {
		t.Fatalf("Validate() error = %v, want unsupported v0 apiVersion", err)
	}
}

func TestDescriptorValidateRejectsInvalidIdentityAndSource(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Descriptor)
		wantErr string
	}{
		{
			name: "empty module id",
			mutate: func(descriptor *Descriptor) {
				descriptor.Module.ID = ""
			},
			wantErr: "module.id is required",
		},
		{
			name: "unnamespaced module id",
			mutate: func(descriptor *Descriptor) {
				descriptor.Module.ID = "search-tools"
			},
			wantErr: "module.id must be lowercase and namespace-qualified",
		},
		{
			name: "invalid semantic version",
			mutate: func(descriptor *Descriptor) {
				descriptor.Module.Version = "v1"
			},
			wantErr: "module.version is not semantic versioning",
		},
		{
			name: "malformed source hash",
			mutate: func(descriptor *Descriptor) {
				descriptor.Source.SHA256 = "ABC123"
			},
			wantErr: "invalid source sha256 for example/search-tools",
		},
		{
			name: "duplicate provided port identity",
			mutate: func(descriptor *Descriptor) {
				descriptor.Provides = append(descriptor.Provides, descriptor.Provides[0])
			},
			wantErr: "duplicate provides port std/tool@v1 id example.search",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor := validDescriptor()
			test.mutate(&descriptor)

			err := descriptor.Validate()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestDescriptorValidateIsSideEffectFree(t *testing.T) {
	descriptor := validDescriptor()
	descriptor.Optional = []Requirement{{PortRef: PortRef{Port: "std/observer/diagnostic@v1"}}}
	descriptor.Conflicts = []Conflict{{Module: "example/legacy-search"}}
	descriptor.Lifecycle.After = []string{"example/bootstrap"}
	before := cloneDescriptor(descriptor)

	if err := descriptor.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !reflect.DeepEqual(descriptor, before) {
		t.Fatalf("Validate() mutated descriptor\n got: %#v\nwant: %#v", descriptor, before)
	}
}

func TestDescriptorUsesCanonicalFlatPortWireShape(t *testing.T) {
	descriptor := validDescriptor()
	descriptor.Requires[0].Provider = "example/tool-host"

	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, `"requires":[{"port":"core/tool-host@v1","provider":"example/tool-host"}]`) {
		t.Fatalf("Descriptor JSON has a non-canonical requirement shape: %s", text)
	}
	for _, forbidden := range []string{`"trust"`, `"support"`, `"state"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Descriptor JSON contains compiler-owned field %s: %s", forbidden, text)
		}
	}
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	descriptor.Provides = append([]PortRef(nil), descriptor.Provides...)
	descriptor.Requires = append([]Requirement(nil), descriptor.Requires...)
	descriptor.Optional = append([]Requirement(nil), descriptor.Optional...)
	descriptor.Conflicts = append([]Conflict(nil), descriptor.Conflicts...)
	descriptor.RequestedGrants = append([]Grant(nil), descriptor.RequestedGrants...)
	descriptor.Lifecycle.After = append([]string(nil), descriptor.Lifecycle.After...)
	return descriptor
}
