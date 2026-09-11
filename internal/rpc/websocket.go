package rpc

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"agent-vivy/internal/actionhost"
)

type WebSocketServer struct {
	Handler Handler
	Token   string
	Origins OriginPolicy
	Options Options
}

func NewSessionToken() string {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "fallback-local-token"
	}
	return hex.EncodeToString(value)
}

func (s WebSocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.Origins.Allows(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	if s.Token == "" || !validToken(r, s.Token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  32 << 10,
		WriteBufferSize: 32 << 10,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	// The handshake token is the authenticated connection credential. Bind it
	// to Peer-owned request context after validation; it never comes from
	// browser JSON. The identity is a transport-attested Face identity and is
	// intentionally independent of session/action fields supplied by callers.
	options := s.Options
	options.Caller = actionhost.NewCaller(s.Token)
	if options.Identity.ID == "" {
		options.Identity = actionhost.Identity{ID: "face/connection", Face: "web"}
	}
	peer := NewPeer(NewWebSocketTransport(conn), s.Handler, options)
	_ = peer.Serve(r.Context())
}

func validToken(r *http.Request, expected string) bool {
	if token := r.URL.Query().Get("token"); token != "" {
		return token == expected
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) == expected
}
