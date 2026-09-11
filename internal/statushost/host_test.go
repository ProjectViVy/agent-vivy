package statushost

import (
	"context"
	"errors"
	"testing"
	"time"

	statusport "agent-vivy/sdk/port/status"
)

type fixtureStatusProvider struct {
	id       string
	started  int
	probed   int
	status   int
	snapshot statusport.Snapshot
	wait     bool
	err      error
}

func (provider *fixtureStatusProvider) ID() string { return provider.id }
func (provider *fixtureStatusProvider) Start()     { provider.started++ }
func (provider *fixtureStatusProvider) Probe()     { provider.probed++ }
func (provider *fixtureStatusProvider) Status(ctx context.Context, _ statusport.Request) (statusport.Snapshot, error) {
	provider.status++
	if provider.wait {
		<-ctx.Done()
		return statusport.Snapshot{}, ctx.Err()
	}
	return provider.snapshot, provider.err
}

func TestStatusReadDoesNotStartOrProbeProvider(t *testing.T) {
	provider := &fixtureStatusProvider{id: "lsp", snapshot: statusport.NewSnapshot("r1", "c1", []statusport.Item{{ID: "python", State: "ready"}})}
	host, err := New(Config{Providers: []statusport.Provider{provider}})
	if err != nil {
		t.Fatal(err)
	}
	result := host.Read(context.Background(), statusport.Request{InstanceID: "workspace-1"})
	if provider.started != 0 || provider.probed != 0 || provider.status != 1 {
		t.Fatalf("start=%d probe=%d status=%d", provider.started, provider.probed, provider.status)
	}
	if len(result) != 1 || result[0].Namespace != "lsp" || !result[0].Snapshot.Available {
		t.Fatalf("status result = %#v", result)
	}
}

func TestStatusHostBoundsItemsAndNamespaces(t *testing.T) {
	items := make([]statusport.Item, 10)
	for index := range items {
		items[index] = statusport.Item{ID: "item", State: "ready"}
	}
	provider := &fixtureStatusProvider{id: "acme/plugin", snapshot: statusport.NewSnapshot("r1", "c1", items)}
	host, err := New(Config{Providers: []statusport.Provider{provider}, MaxItemsPerProvider: 3})
	if err != nil {
		t.Fatal(err)
	}
	result := host.Read(context.Background(), statusport.Request{})
	if len(result) != 1 || len(result[0].Snapshot.Items) != 3 {
		t.Fatalf("bounded result = %#v", result)
	}
	for _, item := range result[0].Snapshot.Items {
		if item.ID != "acme/plugin/item" {
			t.Fatalf("item namespace = %q", item.ID)
		}
	}
}

func TestStatusTimeoutProjectsUnavailable(t *testing.T) {
	provider := &fixtureStatusProvider{id: "slow", wait: true}
	host, err := New(Config{Providers: []statusport.Provider{provider}, Timeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	result := host.Read(context.Background(), statusport.Request{})
	if len(result) != 1 || result[0].Snapshot.Available || result[0].Snapshot.UnavailableReason != "timeout" {
		t.Fatalf("timeout result = %#v", result)
	}
}

func TestStatusProviderFailureIsRedactedUnavailable(t *testing.T) {
	provider := &fixtureStatusProvider{id: "bad", err: errors.New("dial tcp secret.internal:9443 failed")}
	host, err := New(Config{Providers: []statusport.Provider{provider}})
	if err != nil {
		t.Fatal(err)
	}
	result := host.Read(context.Background(), statusport.Request{})
	if len(result) != 1 || result[0].Snapshot.Available || result[0].Snapshot.UnavailableReason != "provider_error" {
		t.Fatalf("failure result = %#v", result)
	}
}
