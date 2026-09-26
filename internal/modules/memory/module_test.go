package memory_test

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/modules/memory"
	"github.com/ProjectViVy/agent-vivy/bml"
)

func testConfig(dir string) config.Config {
	return config.Config{Storage: config.Storage{DataDir: dir}}
}

func TestOpenStoresActiveService(t *testing.T) {
	service, err := memory.Open(context.Background(), testConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	if service == nil {
		t.Fatal("Open() returned a nil service")
	}
	if got := memory.Active(); got != service {
		t.Fatalf("Active() = %p, want %p", got, service)
	}
}

func TestServiceListsEmptyStore(t *testing.T) {
	service, err := memory.Open(context.Background(), testConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	outcome := service.List(context.Background(), bml.MemoryListRequest{})
	if outcome.Status != bml.CrudOutcomeListed {
		t.Fatalf("List() status = %q, want %q", outcome.Status, bml.CrudOutcomeListed)
	}
	if len(outcome.Entries) != 0 {
		t.Fatalf("List() entries = %d, want 0 on a fresh store", len(outcome.Entries))
	}
}

func TestCloseClearsActiveAndIsIdempotent(t *testing.T) {
	if _, err := memory.Open(context.Background(), testConfig(t.TempDir())); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := memory.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := memory.Active(); got != nil {
		t.Fatalf("Active() = %p after Close, want nil", got)
	}
	if err := memory.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestClosedServiceReportsUnavailableOutcome(t *testing.T) {
	service, err := memory.Open(context.Background(), testConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := memory.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	outcome := service.Add(context.Background(), bml.MemoryAddRequest{Content: "fact"})
	if outcome.Status != bml.CrudOutcomeFailed {
		t.Fatalf("Add() on closed service status = %q, want %q", outcome.Status, bml.CrudOutcomeFailed)
	}
	if outcome.Reason != bml.HomeCodeBmlUnavailable {
		t.Fatalf("Add() on closed service reason = %q, want %q", outcome.Reason, bml.HomeCodeBmlUnavailable)
	}
}

func TestActionProvidersReportUnavailableWithoutOpenService(t *testing.T) {
	if err := memory.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	providers := memory.ActionProviders()
	if len(providers) == 0 {
		t.Fatal("ActionProviders() returned no providers")
	}
	for _, provider := range providers {
		raw, err := provider.Invoke(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("%s Invoke() error = %v", provider.Definition().ID, err)
		}
		var outcome bml.MemoryCrudOutcome
		if err := json.Unmarshal(raw, &outcome); err != nil {
			t.Fatalf("%s Invoke() output is not an outcome: %v", provider.Definition().ID, err)
		}
		if outcome.Status != bml.CrudOutcomeFailed {
			t.Fatalf("%s Invoke() status = %q, want %q", provider.Definition().ID, outcome.Status, bml.CrudOutcomeFailed)
		}
	}
}
