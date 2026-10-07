package a2aserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

// fakeChannelHost is the minimal channel.Host for composition checks.
type fakeChannelHost struct {
	settings json.RawMessage
}

func (h *fakeChannelHost) ModuleID() string              { return "projectvivy/a2a-server" }
func (h *fakeChannelHost) Secret(string) (string, error) { return "", nil }
func (h *fakeChannelHost) HTTP() *http.Client            { return http.DefaultClient }
func (h *fakeChannelHost) DialTLS(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("fake host: no tls")
}
func (h *fakeChannelHost) Settings() json.RawMessage { return h.settings }
func (h *fakeChannelHost) PublishInbound(context.Context, channel.InboundMessage) error {
	return nil
}
func (h *fakeChannelHost) Media() channel.MediaStore { return nil }
func (h *fakeChannelHost) Logger() *slog.Logger      { return slog.Default() }

// fakeTaskChannelHost adds the optional TaskHost/TaskServiceInfoHost
// surface the bound channel keys its listener mount on.
type fakeTaskChannelHost struct {
	fakeChannelHost
	*fakeTasks
	fakeInfo
}

var _ channel.Host = (*fakeChannelHost)(nil)
var _ channel.TaskHost = (*fakeTasks)(nil)
var _ channel.TaskServiceInfoHost = fakeInfo{}

// TestA2AModuleComposition pins the selected Module's descriptor,
// provider identity, lifecycle and failure shape (A2A-06 plan Step 1).
func TestA2AModuleComposition(t *testing.T) {
	m := New()
	d := m.Descriptor()
	if d.Module.ID != "projectvivy/a2a-server" || d.Module.Version != "0.1.0" {
		t.Fatalf("descriptor identity = %+v", d.Module)
	}
	if len(d.Provides) != 1 || d.Provides[0].Port != "std/channel@v1" || d.Provides[0].ID != "vivy.a2a" {
		t.Fatalf("provides = %+v", d.Provides)
	}
	if len(d.Requires) != 1 || d.Requires[0].Port != "core/channel-host@v1" {
		t.Fatalf("requires = %+v", d.Requires)
	}
	if len(d.RequestedGrants) != 2 || d.RequestedGrants[0] != module.GrantChannelA2A || d.RequestedGrants[1] != module.GrantSecretRead {
		t.Fatalf("requested grants = %+v", d.RequestedGrants)
	}

	provider := NewProvider()
	if provider == nil || provider.Definition().ID != "vivy.a2a" {
		t.Fatalf("provider definition = %+v", provider.Definition())
	}

	t.Run("host without task surface mounts no partial route", func(t *testing.T) {
		instance, err := provider.Construct(context.Background(), &fakeChannelHost{})
		if err != nil {
			t.Fatalf("construct: %v", err)
		}
		if err := instance.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		lh, ok := instance.(channel.ListenHandler)
		if !ok {
			t.Fatal("instance lacks ListenHandler")
		}
		if lh.ListenHandler() != nil {
			t.Fatal("handler mounted without TaskHost")
		}
		if err := instance.Stop(context.Background()); err != nil {
			t.Fatalf("stop: %v", err)
		}
	})

	t.Run("host with task surface mounts handler", func(t *testing.T) {
		host := &fakeTaskChannelHost{
			fakeTasks: &fakeTasks{submitRef: channel.TaskRef{TaskID: "t1"}},
		}
		instance, err := provider.Construct(context.Background(), host)
		if err != nil {
			t.Fatalf("construct: %v", err)
		}
		if err := instance.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		handler := instance.(channel.ListenHandler).ListenHandler()
		if handler == nil {
			t.Fatal("no handler on task-capable host")
		}
		srv := httptest.NewServer(handler)
		defer srv.Close()
		resp, err := http.Get(srv.URL + "/.well-known/agent-card.json")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("card status=%d", resp.StatusCode)
		}

		if err := instance.Stop(context.Background()); err != nil {
			t.Fatalf("stop: %v", err)
		}
		if instance.(channel.ListenHandler).ListenHandler() != nil {
			t.Fatal("handler survived stop")
		}
	})

	t.Run("send is a typed unsupported", func(t *testing.T) {
		instance, err := provider.Construct(context.Background(), &fakeChannelHost{})
		if err != nil {
			t.Fatalf("construct: %v", err)
		}
		_, err = instance.Send(context.Background(), channel.OutboundMessage{})
		var terr *channel.TaskError
		if !errors.As(err, &terr) || terr.Code != channel.TaskErrUnsupported {
			t.Fatalf("send err=%v", err)
		}
	})

	t.Run("module instance is a no-op code module", func(t *testing.T) {
		mi, err := m.Construct(context.Background(), hostStub{})
		if err != nil {
			t.Fatalf("module construct: %v", err)
		}
		if err := mi.Start(context.Background()); err != nil {
			t.Fatalf("module start: %v", err)
		}
		if err := mi.Ready(context.Background()); err != nil {
			t.Fatalf("module ready: %v", err)
		}
		_ = mi.Stop(context.Background())
		_ = mi.Close(context.Background())
	})
}

type hostStub struct{}

func (hostStub) ModuleID() string { return "projectvivy/a2a-server" }

var _ module.Host = hostStub{}
