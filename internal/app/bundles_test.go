package app

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/runtime"
)

type scriptedToolkitSettingsDrain struct {
	begin  func(context.Context, string) error
	wait   func(context.Context, string) error
	active bool
	ends   int
}

func (d *scriptedToolkitSettingsDrain) BeginDrain(ctx context.Context, id string) error {
	err := d.begin(ctx, id)
	if err == nil {
		d.active = true
	}
	return err
}

func (d *scriptedToolkitSettingsDrain) WaitCatalogIdle(ctx context.Context, id string) error {
	if d.wait != nil {
		return d.wait(ctx, id)
	}
	return nil
}

func (d *scriptedToolkitSettingsDrain) EndDrain(string) {
	d.active = false
	d.ends++
}

func TestBeginToolkitDrainRetriesWhileAnotherPublicationIsActive(t *testing.T) {
	attempts := 0
	err := beginToolkitDrainWhenAvailable(context.Background(), "settings", func(context.Context, string) error {
		attempts++
		if attempts < 3 {
			return runtime.CatalogUpdatePendingError{OperationID: "toolkit-publish"}
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("begin drain attempts = %d, err = %v; want retry through the active publication", attempts, err)
	}
}

func TestReplaceMCPServersWithDrainBuildsLatestSettingsAfterAcquiringGate(t *testing.T) {
	latestName := "user-old"
	backend := runtime.NewMCPBackend(nil, nil)
	t.Cleanup(func() { _ = backend.Close() })
	backend.ReplaceServers([]runtime.MCPServerConfig{{Name: "user-old", Endpoint: "https://old.example.invalid/mcp"}})
	drain := &scriptedToolkitSettingsDrain{
		begin: func(context.Context, string) error { return nil },
		wait: func(context.Context, string) error {
			// Model a settings save that occurs before this update obtains the
			// exclusive publication gate.
			latestName = "user-new"
			return nil
		},
	}
	buildCalls := 0
	afterCalls := 0
	release, err := replaceMCPServersWithDrain(drain, backend, func(context.Context) ([]runtime.MCPServerConfig, error) {
		buildCalls++
		return []runtime.MCPServerConfig{{Name: latestName, Endpoint: "https://new.example.invalid/mcp"}}, nil
	}, func(configs []runtime.MCPServerConfig) {
		afterCalls++
		if !drain.active {
			t.Error("registry refresh ran after the publication gate was released")
		}
		if len(configs) != 1 || configs[0].Name != "user-new" {
			t.Errorf("post-replace configs = %+v, want latest settings", configs)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	configured := backend.ConfiguredServers()
	if buildCalls != 1 || afterCalls != 1 || drain.ends != 0 || !drain.active || len(configured) != 1 || configured[0].Name != "user-new" {
		t.Fatalf("settings publication result: build=%d after=%d ends=%d configs=%+v", buildCalls, afterCalls, drain.ends, configured)
	}
	release()
	if drain.ends != 1 || drain.active {
		t.Fatalf("settings publication did not release its gate: ends=%d active=%v", drain.ends, drain.active)
	}
}

func TestBeginToolkitDrainStopsRetryWhenContextExpires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := beginToolkitDrainWhenAvailable(ctx, "settings", func(context.Context, string) error {
		attempts++
		cancel()
		return runtime.CatalogUpdatePendingError{OperationID: "toolkit-publish"}
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("retry after context cancellation: attempts=%d err=%v", attempts, err)
	}
}
