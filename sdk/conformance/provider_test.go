package conformance

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

func TestRunProviderSuiteRecordsEveryCommonCheck(t *testing.T) {
	definition, ok := port.PublicCatalog().Lookup(module.PortRef{Port: "std/tool@v1"})
	if !ok {
		t.Fatal("std/tool@v1 is not in the public Port catalog")
	}
	wantFailure := errors.New("representative provider failure")
	checks := make(map[string]Check)
	for _, name := range RequiredProviderChecks() {
		checks[name] = func(context.Context) error { return nil }
	}
	checks[CheckRealFailure] = func(context.Context) error { return wantFailure }

	results, err := RunProviderSuite(context.Background(), definition, "fixture/provider", "sdk/conformance/provider_test.go#TestRunProviderSuiteRecordsEveryCommonCheck", checks)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(RequiredProviderChecks()) {
		t.Fatalf("results = %d, want %d", len(results), len(RequiredProviderChecks()))
	}
	for index, name := range RequiredProviderChecks() {
		result := results[index]
		if result.Port.Port != definition.Ref.Port || result.ProviderID != "fixture/provider" || result.Suite != name {
			t.Fatalf("result %d = %#v", index, result)
		}
		if result.EvidenceID == "" {
			t.Fatalf("result %s has no evidence ID", name)
		}
		if result.Passed != (name != CheckRealFailure) {
			t.Fatalf("result %s passed = %v", name, result.Passed)
		}
	}

	again, err := RunProviderSuite(context.Background(), definition, "fixture/provider", "sdk/conformance/provider_test.go#TestRunProviderSuiteRecordsEveryCommonCheck", checks)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(results, again) {
		t.Fatalf("provider evidence is not deterministic:\nfirst: %#v\nagain: %#v", results, again)
	}
}

func TestRunProviderSuiteRejectsMissingCommonCheck(t *testing.T) {
	definition, ok := port.PublicCatalog().Lookup(module.PortRef{Port: "std/channel@v1"})
	if !ok {
		t.Fatal("std/channel@v1 is not in the public Port catalog")
	}
	checks := make(map[string]Check)
	for _, name := range RequiredProviderChecks() {
		checks[name] = func(context.Context) error { return nil }
	}
	delete(checks, CheckCleanup)

	if _, err := RunProviderSuite(context.Background(), definition, "fixture/channel", "sdk/conformance/provider_test.go#TestRunProviderSuiteRejectsMissingCommonCheck", checks); err == nil {
		t.Fatal("RunProviderSuite accepted a suite without cleanup coverage")
	}
}

func TestRunProviderSuiteRejectsMissingEvidencePath(t *testing.T) {
	definition, _ := port.PublicCatalog().Lookup(module.PortRef{Port: "std/channel@v1"})
	checks := make(map[string]Check)
	for _, name := range RequiredProviderChecks() {
		checks[name] = func(context.Context) error { return nil }
	}
	for _, evidence := range []string{"", "missing-anchor.go", "../outside.go#Test", "/absolute.go#Test", "C:/absolute.go#Test", "C:\\absolute.go#Test"} {
		if _, err := RunProviderSuite(context.Background(), definition, "fixture/channel", evidence, checks); err == nil {
			t.Fatalf("invalid evidence path accepted: %q", evidence)
		}
	}
}
