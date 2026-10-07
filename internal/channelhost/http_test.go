package channelhost

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/sdk/port/channel"
)

type envCreds map[string]string

func (c envCreds) Resolve(moduleID, ref string) (string, error) {
	if v, ok := c[ref]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown ref %q", ref)
}

func (c envCreds) IsSet(moduleID, ref string) bool {
	_, ok := c[ref]
	return ok
}

func httpHost(t *testing.T, listen string) (*Host, *taskFixture) {
	t.Helper()
	f := newTaskFixture(t)
	deps := Deps{
		Journal:  f.b,
		Messages: f.b,
		Sessions: f.b,
		Config: config.Channels{
			"a2a": {
				Enabled:   true,
				AllowFrom: []string{"pens-local"},
				HTTP: &config.ChannelHTTPConfig{
					Listen:        listen,
					PublicBaseURL: "http://" + listen,
					Principal:     config.ChannelHTTPPrincipal{ID: "pens-local", TokenEnv: "A2A_TOKEN"},
				},
			},
		},
		Credentials: envCreds{"A2A_TOKEN": "s3cret"},
		Tasks:       f.taskDeps(),
	}
	h := New(deps)
	return h, f
}

func (f *taskFixture) taskDeps() *TaskDeps {
	return &TaskDeps{
		Store:    f.b,
		Journal:  f.b,
		Messages: f.b,
		Submit: func(ctx context.Context, in domain.ChannelTaskInput) (domain.ChannelTaskReceipt, error) {
			f.submits = append(f.submits, in)
			return f.admitTask(ctx, in)
		},
		Cancel:    f.depsCancel(),
		Subscribe: events.NewBus(8).Subscribe,
		Authorize: func(context.Context, TaskPrincipal) bool { return true },
		ServiceInfo: channel.TaskServiceInfo{
			Name: "vivy", Description: "task service", Version: "v1",
			Streaming: true, InputContinuation: true,
		},
	}
}

// okHandler is the stand-in ListenHandler: it serves the public card and
// echoes the bound principal on RPC — the only identity the adapter sees.
func okHandler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"vivy"}`))
	})
	mux.HandleFunc("POST /a2a", func(w http.ResponseWriter, r *http.Request) {
		p, ok := taskPrincipalFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Forwarded-For") != "" {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(p.PrincipalID))
	})
	return mux
}

func startTestListener(t *testing.T, h *Host, handler http.Handler) string {
	t.Helper()
	stop, err := h.startTaskHTTP(context.Background(), "a2a", "vivy/a2a", handler)
	if err != nil {
		t.Fatalf("startTaskHTTP: %v", err)
	}
	t.Cleanup(func() { _ = stop(context.Background()) })
	return h.deps.Config["a2a"].HTTP.Listen
}

func TestA2AHTTPIsolationAndLifecycle(t *testing.T) {
	listen := "127.0.0.1:0"
	t.Run("listener serves auth'd RPC and public card only", func(t *testing.T) {
		h, _ := httpHost(t, listen)
		addr := startTestListener(t, h, okHandler(t))
		// Bind picked a port; re-derive the real addr from the state.
		addr = listenerAddr(t, h)
		base := "http://" + addr

		// Card is public.
		resp, err := http.Get(base + "/.well-known/agent-card.json")
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("card: %v %v", resp.Status, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(body), `"name":"vivy"`) {
			t.Fatalf("card = %s", body)
		}

		// RPC without token => 401.
		resp, err = http.Post(base+"/a2a", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"message/send","params":{}}`))
		if err != nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauth'd rpc: %v %v", resp.Status, err)
		}
		resp.Body.Close()

		// Valid token + forged identity headers => adapter sees only the
		// configured principal; forwarding headers stripped.
		req, _ := http.NewRequest("POST", base+"/a2a", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer s3cret")
		req.Header.Set("X-Forwarded-For", "9.9.9.9")
		req.Header.Set("X-Forwarded-User", "mallory")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("rpc: %v", err)
		}
		rbody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || string(rbody) != "pens-local" {
			t.Fatalf("rpc = %d %q", resp.StatusCode, rbody)
		}

		// Wrong token => 401 (a valid-but-unlisted identity cannot exist:
		// exactly one principal per endpoint).
		req, _ = http.NewRequest("POST", base+"/a2a", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer wrong")
		resp, _ = http.DefaultClient.Do(req)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("bad token = %d", resp.StatusCode)
		}
		resp.Body.Close()

		// Unknown paths are absent.
		for _, p := range []string{"/rpc", "/admin", "/a2a/tasks", "/.well-known/other"} {
			req, _ := http.NewRequest("GET", base+p, nil)
			req.Header.Set("Authorization", "Bearer s3cret")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s: %v", p, err)
			}
			resp.Body.Close()
			if resp.StatusCode != 404 {
				t.Fatalf("%s = %d, want 404", p, resp.StatusCode)
			}
		}

		// Oversized body => 413 before admission.
		big := bytes.Repeat([]byte("x"), taskHTTPBodyLimit+1)
		req, _ = http.NewRequest("POST", base+"/a2a", bytes.NewReader(big))
		req.Header.Set("Authorization", "Bearer s3cret")
		resp, err = http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversize = %v %v", resp.StatusCode, err)
		}
		resp.Body.Close()
	})

	t.Run("rate limit bounds bursts; bind failure records failed state", func(t *testing.T) {
		h, _ := httpHost(t, listen)
		addr := listenerAddrAfterStart(t, h)
		base := "http://" + addr
		var hit429 bool
		for i := 0; i < 60; i++ {
			req, _ := http.NewRequest("POST", base+"/a2a", strings.NewReader(`{}`))
			req.Header.Set("Authorization", "Bearer s3cret")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("rpc %d: %v", i, err)
			}
			resp.Body.Close()
			if resp.StatusCode == 429 {
				hit429 = true
				break
			}
		}
		if !hit429 {
			t.Fatal("burst never hit 429")
		}

		// Bind failure: same name second bind or occupied port.
		h2, _ := httpHost(t, "127.0.0.1:1")
		if _, err := h2.startTaskHTTP(context.Background(), "a2a", "vivy/a2a", okHandler(t)); err == nil {
			t.Fatal("bind on port 1 should fail")
		}
		if states := h2.TaskHTTPStates(); states["a2a"] != string(taskHTTPFailed) {
			t.Fatalf("bind failure state = %v", states)
		}
	})
}

func listenerAddr(t *testing.T, h *Host) string {
	t.Helper()
	h.mu.Lock()
	lis := h.taskListeners["a2a"]
	h.mu.Unlock()
	if lis == nil || lis.ln == nil {
		t.Fatal("no listener")
	}
	return lis.ln.Addr().String()
}

func listenerAddrAfterStart(t *testing.T, h *Host) string {
	t.Helper()
	stop, err := h.startTaskHTTP(context.Background(), "a2a", "vivy/a2a", okHandler(t))
	if err != nil {
		t.Fatalf("startTaskHTTP: %v", err)
	}
	t.Cleanup(func() { _ = stop(context.Background()) })
	return listenerAddr(t, h)
}
