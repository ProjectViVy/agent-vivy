package acp

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	acp "github.com/eino-contrib/acp"
)

// fullCapsHost returns a fakeHost whose initialize response advertises every
// capability the pilot requires plus code_mode_available.
func fullCapsHost() *fakeHost {
	return &fakeHost{callFn: func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		switch method {
		case "initialize":
			return json.RawMessage(`{"protocol_version":1,"capabilities":["session","turn","run","run.subscribe","approval","question","review"],"code_mode_available":true}`), nil
		case "session/create":
			return json.RawMessage(`{"id":"sess_test","title":"","created_at":1,"updated_at":1,"sandbox_mode":"smart","approval_policy":"smart","permission_preset":"smart","workspace_path":"/tmp"}`), nil
		}
		return nil, errors.New("unexpected call: " + method)
	}}
}

func initReq() acp.InitializeRequest {
	return acp.InitializeRequest{
		ProtocolVersion:    1,
		ClientCapabilities: &acp.ClientCapabilities{},
		ClientInfo:         &acp.Implementation{Name: "probe", Version: "0.0.1"},
	}
}

func mustInitialize(t *testing.T, a *agent) {
	t.Helper()
	if _, err := a.Initialize(context.Background(), initReq()); err != nil {
		t.Fatalf("initialize: %v", err)
	}
}

func TestInitializeRestrictedPilot(t *testing.T) {
	t.Run("negotiates the restricted surface", func(t *testing.T) {
		a := newAgent(fullCapsHost())
		resp, err := a.Initialize(context.Background(), initReq())
		if err != nil {
			t.Fatalf("initialize: %v", err)
		}
		if resp.ProtocolVersion != 1 {
			t.Fatalf("protocolVersion = %v", resp.ProtocolVersion)
		}
		if resp.AgentCapabilities == nil || resp.AgentCapabilities.LoadSession {
			t.Fatalf("loadSession must be false: %+v", resp.AgentCapabilities)
		}
		pc := resp.AgentCapabilities.PromptCapabilities
		if pc == nil || pc.Image || pc.Audio || pc.EmbeddedContext {
			t.Fatalf("promptCapabilities = %+v", pc)
		}
		mc := resp.AgentCapabilities.MCPCapabilities
		if mc == nil || mc.HTTP || mc.SSE {
			t.Fatalf("mcpCapabilities = %+v", mc)
		}
		if resp.AgentCapabilities.SessionCapabilities == nil {
			t.Fatal("sessionCapabilities must be present and empty")
		}
		if resp.AuthMethods == nil || len(resp.AuthMethods) != 0 {
			t.Fatalf("authMethods = %+v", resp.AuthMethods)
		}
		if resp.AgentInfo == nil || resp.AgentInfo.Name == "" || resp.AgentInfo.Version == "" {
			t.Fatalf("agentInfo = %+v", resp.AgentInfo)
		}
		// Wire shape: the response object must carry exactly the negotiated
		// fields, with sessionCapabilities={} and authMethods=[].
		wire, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(wire, &obj); err != nil {
			t.Fatalf("wire decode: %v", err)
		}
		var sessionCaps map[string]any
		var capsObj struct {
			SessionCapabilities map[string]any `json:"sessionCapabilities"`
		}
		_ = json.Unmarshal(obj["agentCapabilities"], &capsObj)
		sessionCaps = capsObj.SessionCapabilities
		if sessionCaps == nil {
			t.Fatal("agentCapabilities.sessionCapabilities missing on wire")
		}
		if len(sessionCaps) != 0 {
			t.Fatalf("sessionCapabilities must be empty: %v", sessionCaps)
		}
	})

	t.Run("missing code_mode_available fails", func(t *testing.T) {
		h := &fakeHost{callFn: func(_ context.Context, method string, _ any) (json.RawMessage, error) {
			return json.RawMessage(`{"protocol_version":1,"capabilities":["session","turn","run","run.subscribe","approval","question","review"],"code_mode_available":false}`), nil
		}}
		a := newAgent(h)
		_, err := a.Initialize(context.Background(), initReq())
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32603 {
			t.Fatalf("want -32603 REQUIRED_CAPABILITY_UNAVAILABLE, got %v", err)
		}
	})

	t.Run("missing required capability fails", func(t *testing.T) {
		h := &fakeHost{callFn: func(_ context.Context, method string, _ any) (json.RawMessage, error) {
			return json.RawMessage(`{"protocol_version":1,"capabilities":["session","turn","run"],"code_mode_available":true}`), nil
		}}
		a := newAgent(h)
		_, err := a.Initialize(context.Background(), initReq())
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32603 {
			t.Fatalf("want -32603, got %v", err)
		}
	})
}

func TestDuplicateInitializePreservesSessions(t *testing.T) {
	h := fullCapsHost()
	a := newAgent(h)
	mustInitialize(t, a)

	req := acp.NewSessionRequest{Cwd: "/tmp"}
	if _, err := a.NewSession(context.Background(), req); err != nil {
		t.Fatalf("new session: %v", err)
	}
	if got := h.callCount("session/create"); got != 1 {
		t.Fatalf("session/create calls = %d", got)
	}

	// A second initialize is a safe invalid-request error and must not
	// reset negotiated capabilities or owned sessions.
	if _, err := a.Initialize(context.Background(), initReq()); err == nil {
		t.Fatal("duplicate initialize accepted")
	} else {
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32602 {
			t.Fatalf("duplicate initialize error = %v", err)
		}
	}
	if got := h.callCount("session/create"); got != 1 {
		t.Fatalf("session/create calls after dup init = %d", got)
	}
	if _, err := a.lookupSession("sess_test"); err != nil {
		t.Fatalf("session lost after duplicate initialize: %v", err)
	}
}

func TestSessionAdmission(t *testing.T) {
	newHost := func() *fakeHost {
		var n int
		var mu sync.Mutex
		return &fakeHost{callFn: func(_ context.Context, method string, params any) (json.RawMessage, error) {
			switch method {
			case "initialize":
				return json.RawMessage(`{"protocol_version":1,"capabilities":["session","turn","run","run.subscribe","approval","question","review"],"code_mode_available":true}`), nil
			case "session/create":
				mu.Lock()
				n++
				id := n
				mu.Unlock()
				out, _ := json.Marshal(map[string]any{"id": "sess_" + itoa(id)})
				return json.RawMessage(out), nil
			}
			return nil, errors.New("unexpected call: " + method)
		}}
	}

	t.Run("validates before any durable effect", func(t *testing.T) {
		h := newHost()
		a := newAgent(h)
		mustInitialize(t, a)
		bad := []acp.NewSessionRequest{
			{Cwd: ""},
			{Cwd: "relative/path"},
			{Cwd: "/tmp/a\x00b"},
			{Cwd: "/tmp/a\x1fb"},
			{Cwd: "/tmp", AdditionalDirectories: []string{"/other"}},
		}
		for i, req := range bad {
			if _, err := a.NewSession(context.Background(), req); err == nil {
				t.Fatalf("case %d accepted", i)
			} else {
				var re *acp.RPCError
				if !errors.As(err, &re) || re.Code != -32602 {
					t.Fatalf("case %d error = %v", i, err)
				}
			}
		}
		if got := h.callCount("session/create"); got != 0 {
			t.Fatalf("invalid requests created %d sessions", got)
		}
	})

	t.Run("nonempty mcpServers is CLIENT_MCP_UNSUPPORTED without echo", func(t *testing.T) {
		h := newHost()
		a := newAgent(h)
		mustInitialize(t, a)
		req := acp.NewSessionRequest{
			Cwd: "/tmp",
			MCPServers: []acp.MCPServer{
				{StdioVariant: &acp.MCPServerStdioVariant{MCPServerStdio: acp.MCPServerStdio{Name: "secret-mcp", Command: "/bin/secret", Args: []string{"--token=abc"}}}},
			},
		}
		_, err := a.NewSession(context.Background(), req)
		var re *acp.RPCError
		if !errors.As(err, &re) {
			t.Fatalf("want RPCError, got %v", err)
		}
		if re.Code != -32602 {
			t.Fatalf("code = %d", re.Code)
		}
		var data map[string]string
		if err := json.Unmarshal(re.Data, &data); err != nil || data["reason"] != "CLIENT_MCP_UNSUPPORTED" {
			t.Fatalf("data = %s", string(re.Data))
		}
		// The wire error must not echo the rejected config.
		wire, _ := json.Marshal(re)
		for _, leak := range []string{"secret-mcp", "/bin/secret", "--token=abc"} {
			if contains(wire, leak) {
				t.Fatalf("wire error echoes %q: %s", leak, string(wire))
			}
		}
		if got := h.callCount("session/create"); got != 0 {
			t.Fatalf("mcpServers request created %d sessions", got)
		}
	})

	t.Run("mcpServers absent, null and empty are admitted", func(t *testing.T) {
		h := newHost()
		a := newAgent(h)
		mustInitialize(t, a)
		for i, req := range []acp.NewSessionRequest{
			{Cwd: "/tmp"},
			{Cwd: "/tmp", MCPServers: nil},
			{Cwd: "/tmp", MCPServers: []acp.MCPServer{}},
		} {
			if _, err := a.NewSession(context.Background(), req); err != nil {
				t.Fatalf("case %d rejected: %v", i, err)
			}
		}
		if got := h.callCount("session/create"); got != 3 {
			t.Fatalf("created %d sessions, want 3", got)
		}
	})

	t.Run("32-session boundary including concurrent reservations", func(t *testing.T) {
		h := newHost()
		a := newAgent(h)
		mustInitialize(t, a)

		// 32 concurrent creates; all must land.
		var wg sync.WaitGroup
		errs := make(chan error, maxSessionsPerConnection)
		for i := 0; i < maxSessionsPerConnection; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := a.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp"})
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("create failed under cap: %v", err)
			}
		}
		if got := h.callCount("session/create"); got != maxSessionsPerConnection {
			t.Fatalf("created %d, want %d", got, maxSessionsPerConnection)
		}

		// The 33rd must fail with -32001 and create nothing.
		_, err := a.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp"})
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32001 {
			t.Fatalf("overflow error = %v", err)
		}
		var data map[string]string
		_ = json.Unmarshal(re.Data, &data)
		if data["reason"] != "CAPACITY_EXCEEDED" {
			t.Fatalf("reason = %q", data["reason"])
		}
		if got := h.callCount("session/create"); got != maxSessionsPerConnection {
			t.Fatalf("overflow created %d sessions", got)
		}
	})

	t.Run("uninitialized NewSession is rejected", func(t *testing.T) {
		a := newAgent(newHost())
		_, err := a.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp"})
		var re *acp.RPCError
		if !errors.As(err, &re) || re.Code != -32602 {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("successful create returns only sessionId and keeps canonical root", func(t *testing.T) {
		h := newHost()
		a := newAgent(h)
		mustInitialize(t, a)
		resp, err := a.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp/../tmp/x"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if resp.SessionID == "" {
			t.Fatal("empty session id")
		}
		wire, _ := json.Marshal(resp)
		var obj map[string]json.RawMessage
		_ = json.Unmarshal(wire, &obj)
		for k := range obj {
			if k != "sessionId" {
				t.Fatalf("wire field %q must not be returned", k)
			}
		}
		s, err := a.lookupSession(string(resp.SessionID))
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if want := filepath.Clean("/tmp/../tmp/x"); s.root != want {
			t.Fatalf("root = %q, want %q", s.root, want)
		}
	})

	t.Run("foreign-looking session id is -32002 before Control", func(t *testing.T) {
		h := newHost()
		a := newAgent(h)
		mustInitialize(t, a)
		if _, err := a.lookupSession("sess_999"); err == nil {
			t.Fatal("foreign id resolved")
		} else {
			var re *acp.RPCError
			if !errors.As(err, &re) || re.Code != -32002 {
				t.Fatalf("error = %v", err)
			}
		}
		if got := h.callCount("session/create"); got != 0 {
			t.Fatalf("lookup issued %d creates", got)
		}
	})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}

func contains(hay []byte, needle string) bool {
	return len(needle) > 0 && string(hay) != "" && (len(hay) >= len(needle)) &&
		(func() bool {
			for i := 0; i+len(needle) <= len(hay); i++ {
				if string(hay[i:i+len(needle)]) == needle {
					return true
				}
			}
			return false
		})()
}
