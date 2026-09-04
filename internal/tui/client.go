package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/tui/surface"
	"agent-vivy/sdk/plugin"
)

// Client is one in-process JSON-RPC peer attached to the control plane.
type Client struct {
	peer *controlrpc.Peer
	call func(context.Context, string, any) (json.RawMessage, error)

	mu     sync.Mutex
	notify func(method string, params json.RawMessage)
}

// Attach wraps an already-serving client peer. Tests use a JSONL pair;
// the product path uses Dial.
func Attach(peer *controlrpc.Peer) *Client {
	client := &Client{peer: peer}
	return client
}

// AttachFaceEnv adapts the gateway-less FaceHost control plane to the same
// client used by the resident-gateway TUI.
func AttachFaceEnv(env plugin.FaceEnv) *Client {
	c := &Client{call: env.Call}
	env.OnEvent(c.dispatch)
	return c
}

func (c *Client) dispatch(method string, params json.RawMessage) {
	c.mu.Lock()
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify(method, params)
	}
}

// Handler implements controlrpc.Handler so the peer can deliver
// notifications (run/event) while Call waits for responses.
func (c *Client) Handle(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
	if request.Method == "" {
		return nil, nil
	}
	c.dispatch(request.Method, request.Params)
	return nil, nil
}

// OnNotify registers the callback for server notifications.
func (c *Client) OnNotify(fn func(method string, params json.RawMessage)) {
	c.mu.Lock()
	c.notify = fn
	c.mu.Unlock()
}

// Call issues one JSON-RPC method and waits for its result.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("tui: client is not connected")
	}
	if c.call != nil {
		return c.call(ctx, method, params)
	}
	if c.peer == nil {
		return nil, fmt.Errorf("tui: client is not connected")
	}
	return c.peer.Call(ctx, method, params)
}

// resolveAttachments asks the control plane to validate and describe
// project-relative image paths. The response is metadata only; raw bytes stay
// server-side until turn/start resolves the same paths into domain.Attachments.
func (c *Client) resolveAttachments(ctx context.Context, paths []string) ([]surface.Attachment, error) {
	raw, err := c.Call(ctx, "attachments/resolve", map[string]any{"attachment_paths": append([]string(nil), paths...)})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Attachments []surface.Attachment `json:"attachments"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("tui: attachments/resolve: %w", err)
	}
	if len(envelope.Attachments) != len(paths) {
		return nil, fmt.Errorf("tui: attachments/resolve returned %d attachments, want %d", len(envelope.Attachments), len(paths))
	}
	return envelope.Attachments, nil
}

// Close tears down the peer transport.
func (c *Client) Close() error {
	if c == nil || c.peer == nil {
		return nil
	}
	return c.peer.Close()
}

// Dial connects to a resident vivy control plane. addr is host:port or
// an http(s) URL. Token is fetched from /rpc/bootstrap when empty.
func Dial(ctx context.Context, addr, token string) (*Client, error) {
	httpBase, wsBase, err := splitAddr(addr)
	if err != nil {
		return nil, err
	}
	if token == "" {
		token, err = fetchBootstrapToken(ctx, httpBase)
		if err != nil {
			return nil, err
		}
	}
	wsURL, err := url.Parse(wsBase + "/rpc")
	if err != nil {
		return nil, fmt.Errorf("tui: websocket url: %w", err)
	}
	query := wsURL.Query()
	query.Set("token", token)
	wsURL.RawQuery = query.Encode()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("tui: dial %s: %w", wsURL.Host, err)
	}
	client := &Client{}
	peer := controlrpc.NewPeer(controlrpc.NewWebSocketTransport(conn), client, controlrpc.Options{})
	client.peer = peer
	go func() { _ = peer.Serve(ctx) }()
	if _, err := client.Call(ctx, "initialize", nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("tui: initialize: %w", err)
	}
	return client, nil
}

func fetchBootstrapToken(ctx context.Context, httpBase string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpBase+"/rpc/bootstrap", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tui: bootstrap %s: %w (is vivy running?)", httpBase, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("tui: read bootstrap: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tui: bootstrap %s: HTTP %d", httpBase, resp.StatusCode)
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Token == "" {
		return "", fmt.Errorf("tui: bootstrap token missing")
	}
	return payload.Token, nil
}

func splitAddr(addr string) (httpBase, wsBase string, err error) {
	value := strings.TrimSpace(addr)
	if value == "" {
		value = "127.0.0.1:8787"
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", "", fmt.Errorf("tui: invalid addr %q", addr)
	}
	scheme := strings.ToLower(parsed.Scheme)
	httpScheme := "http"
	wsScheme := "ws"
	switch scheme {
	case "https", "wss":
		httpScheme = "https"
		wsScheme = "wss"
	case "http", "ws":
	default:
		return "", "", fmt.Errorf("tui: unsupported addr scheme %q", parsed.Scheme)
	}
	host := parsed.Host
	return httpScheme + "://" + host, wsScheme + "://" + host, nil
}
