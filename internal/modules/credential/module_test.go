package credential

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCredentialResolverIsScopedAndNonEnumerable(t *testing.T) {
	const secret = "credential-canary-value"
	t.Setenv("VIVY_MODEL_KEY", secret)
	resolver, err := Compose(map[string][]string{
		"vivy/model":   {"VIVY_MODEL_KEY"},
		"vivy/channel": {"VIVY_CHANNEL_TOKEN"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolver.Resolve("vivy/model", "VIVY_MODEL_KEY")
	if err != nil || got != secret {
		t.Fatalf("Resolve() = %q, %v", got, err)
	}
	if _, err := resolver.Resolve("vivy/channel", "VIVY_MODEL_KEY"); !errors.Is(err, ErrDenied) {
		t.Fatalf("cross-module Resolve() error = %v, want ErrDenied", err)
	}
	if _, ok := reflect.TypeOf(resolver).MethodByName("List"); ok {
		t.Fatal("credential resolver must not expose enumeration")
	}
}

func TestCredentialResolverMissingAndErrorsNeverLeakValues(t *testing.T) {
	const secret = "credential-error-canary"
	t.Setenv("VIVY_SCOPED_KEY", secret)
	resolver, err := Compose(map[string][]string{"vivy/model": {"VIVY_SCOPED_KEY", "VIVY_MISSING_KEY"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct{ moduleID, ref string }{
		{"vivy/other", "VIVY_SCOPED_KEY"},
		{"vivy/model", "VIVY_MISSING_KEY"},
	} {
		_, err := resolver.Resolve(call.moduleID, call.ref)
		if err == nil {
			t.Fatalf("Resolve(%q, %q) succeeded", call.moduleID, call.ref)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked secret: %v", err)
		}
	}
}

func TestCredentialModuleOwnsCanonicalCorePort(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != Port {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}
