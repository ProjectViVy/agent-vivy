// Dedicated task HTTP listener (A2A design §10–10.1): a Host-owned,
// authenticated, loopback-only endpoint serving the channel's
// ListenHandler. TLS terminates at a reverse proxy; this process binds
// loopback and enforces auth, bounds and lifecycle truthfully.
package channelhost

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/port/channel"
)

// taskHTTPState is the listener lifecycle vocabulary (§10.1).
type taskHTTPState string

const (
	taskHTTPAbsent    taskHTTPState = "absent"
	taskHTTPInactive  taskHTTPState = "inactive"
	taskHTTPStarting  taskHTTPState = "starting"
	taskHTTPServing   taskHTTPState = "serving"
	taskHTTPDraining  taskHTTPState = "draining"
	taskHTTPFailed    taskHTTPState = "failed"
	taskHTTPStopped   taskHTTPState = "stopped"
)

// §10 protective bounds (initial settings, not throughput targets).
const (
	taskHTTPBodyLimit     = 256 << 10 // request body cap before admission
	taskHTTPReadHeaders   = 5 * time.Second
	taskHTTPPerWrite      = 15 * time.Second
	taskHTTPBlockingSend  = 60 * time.Second
	taskHTTPIdleKeepalive = 15 * time.Second
	taskHTTPDrain         = 10 * time.Second
	taskHTTPRatePerSec    = 10.0 // authenticated RPC per principal
	taskHTTPRateBurst     = 20
	taskHTTPStreamPerTask = 2
	taskHTTPStreamPerPrin = 16
)

// taskHTTPListener is one bound listener's live state.
type taskHTTPListener struct {
	srv  *http.Server
	ln   net.Listener
	name string
	principal string
	state taskHTTPState
	err   string // safe diagnostic, never credentials or paths

	mu       sync.Mutex
	rpc      *tokenBucket
	card     *tokenBucket
	streams  map[string]int // task -> open stream count
	streamN  int            // total open streams for the principal
}

// tokenBucket is a fixed-cost token bucket: capacity burst, refill rate
// per second, no queue.
type tokenBucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newTokenBucket(rate, burst float64) *tokenBucket {
	return &tokenBucket{rate: rate, burst: burst, tokens: burst, last: time.Now()}
}

// take consumes one token; false means reject now (no queueing).
func (b *tokenBucket) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// startTaskHTTP binds and serves the channel's dedicated task listener.
// The handler is the live ListenHandler consumed after provider Start —
// never a typed-nil discovery target.
func (h *Host) startTaskHTTP(ctx context.Context, channelName, moduleID string, handler http.Handler) (func(context.Context) error, error) {
	if handler == nil {
		return nil, errors.New("channelhost: task handler is nil")
	}
	envelope, ok := h.deps.Config[channelName]
	if !ok || envelope.HTTP == nil {
		return nil, fmt.Errorf("channelhost: channel %q has no http listener config", channelName)
	}
	cfg := envelope.HTTP

	h.mu.Lock()
	if h.taskListeners == nil {
		h.taskListeners = make(map[string]*taskHTTPListener)
	}
	if _, exists := h.taskListeners[channelName]; exists {
		h.mu.Unlock()
		return nil, fmt.Errorf("channelhost: task listener for %q already exists", channelName)
	}
	lis := &taskHTTPListener{
		name: channelName, principal: cfg.Principal.ID,
		state: taskHTTPStarting,
		rpc: newTokenBucket(taskHTTPRatePerSec, taskHTTPRateBurst),
		card: newTokenBucket(taskHTTPRatePerSec, taskHTTPRateBurst),
		streams: make(map[string]int),
	}
	h.taskListeners[channelName] = lis
	h.mu.Unlock()

	fail := func(err error) {
		h.mu.Lock()
		lis.state = taskHTTPFailed
		lis.err = err.Error()
		h.mu.Unlock()
	}

	// Resolve the bearer credential as a Host-private value — the plugin
	// never sees it, and no Secret grant is involved.
	if h.deps.Credentials == nil {
		fail(errors.New("credential resolver is unavailable"))
		return nil, errors.New("channelhost: credential resolver is unavailable")
	}
	token, err := h.deps.Credentials.Resolve(moduleID, cfg.Principal.TokenEnv)
	if err != nil || token == "" {
		fail(fmt.Errorf("credential resolution failed"))
		return nil, fmt.Errorf("channelhost: resolve %s principal token: %w", channelName, err)
	}

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		fail(fmt.Errorf("bind failed"))
		return nil, fmt.Errorf("channelhost: bind %s task listener: %w", channelName, err)
	}
	lis.ln = ln

	mux := http.NewServeMux()
	// The card is the sole unauthenticated route, behind one aggregate
	// public bucket — no per-source-address allocation.
	mux.HandleFunc("GET /.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		if !lis.card.take() {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		h.writeTaskCard(w, channelName)
	})
	// The RPC path is fixed at /a2a; every other path is absent.
	mux.Handle("POST /a2a", h.taskHTTPMiddleware(lis, moduleID, handler))

	lis.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: taskHTTPReadHeaders,
		IdleTimeout:       taskHTTPIdleKeepalive,
		// No WriteTimeout: a server-wide deadline would kill healthy SSE;
		// writes carry per-write deadlines via the deadlineWriter instead.
	}

	h.mu.Lock()
	lis.state = taskHTTPServing
	h.mu.Unlock()
	go func() {
		err := lis.srv.Serve(ln)
		h.mu.Lock()
		defer h.mu.Unlock()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			lis.state = taskHTTPFailed
			lis.err = "listener failed"
		} else if lis.state != taskHTTPFailed {
			lis.state = taskHTTPStopped
		}
	}()

	stop := func(ctx context.Context) error {
		h.mu.Lock()
		lis.state = taskHTTPDraining
		h.mu.Unlock()
		sctx, cancel := context.WithTimeout(ctx, taskHTTPDrain)
		defer cancel()
		err := lis.srv.Shutdown(sctx)
		h.mu.Lock()
		defer h.mu.Unlock()
		if lis.state != taskHTTPFailed {
			lis.state = taskHTTPStopped
		}
		delete(h.taskListeners, channelName)
		return err
	}
	return stop, nil
}

// writeTaskCard serves the safe discovery projection (public fields only).
func (h *Host) writeTaskCard(w http.ResponseWriter, channelName string) {
	envelope, ok := h.deps.Config[channelName]
	if !ok || envelope.HTTP == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	info := channel.TaskServiceInfo{}
	if h.deps.Tasks != nil {
		info = h.deps.Tasks.ServiceInfo
	}
	url := strings.TrimRight(envelope.HTTP.PublicBaseURL, "/")
	card := map[string]any{
		"name":        info.Name,
		"description": info.Description,
		"version":     info.Version,
		"url":         url + "/a2a",
		"capabilities": map[string]any{
			"streaming":         info.Streaming,
			"inputContinuation": info.InputContinuation,
		},
		"skills": info.Skills,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(card)
}

// taskHTTPMiddleware is the single ingress wrapper: authenticate, bound
// the body, strip trust headers, enforce rates and stream slots, then
// inject the private principal binding before SDK dispatch.
func (h *Host) taskHTTPMiddleware(lis *taskHTTPListener, moduleID string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lis.mu.Lock()
		draining := lis.state == taskHTTPDraining
		lis.mu.Unlock()
		if draining {
			// Drain rejects new RPC work; in-flight streams finish inside
			// the shutdown bound.
			http.Error(w, "listener is draining", http.StatusServiceUnavailable)
			return
		}
		// Authenticate: bearer token resolved by the Host; comparison is
		// constant-time. A missing or wrong credential never reaches the
		// adapter.
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || got == r.Header.Get("Authorization") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		token, err := h.deps.Credentials.Resolve(moduleID, envelopePrincipalTokenEnv(h.deps.Config[lis.name]))
		if err != nil || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			// A valid credential that is not the configured principal's
			// still fails: exactly one principal exists per endpoint.
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if !lis.rpc.take() {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}

		// Bound the body before admission; a frame beyond the cap errors
		// before any partial emission.
		r.Body = http.MaxBytesReader(w, r.Body, taskHTTPBodyLimit)

		// Stream slots: task id rides the JSON body; count in-flight
		// requests per task (2) and per principal (16).
		body, err := readBoundedBody(r)
		if err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = io.NopCloser(bytesReader(body))
		taskID := extractTaskID(body)
		release, ok := lis.acquireStream(taskID)
		if !ok {
			http.Error(w, "too many streams", http.StatusTooManyRequests)
			return
		}
		defer release()

		// Strip client authority and forwarding headers before dispatch;
		// the private binding is the only identity (§10.1). A2A-Version
		// survives.
		r.Header.Del("Authorization")
		for k := range r.Header {
			lk := strings.ToLower(k)
			if strings.HasPrefix(lk, "x-forwarded-") || lk == "x-real-ip" || lk == "forwarded" {
				r.Header.Del(k)
			}
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := WithTaskPrincipal(r.Context(), TaskPrincipal{
			ModuleID:    moduleID,
			ProviderID:  lis.name,
			InstanceID:  lis.name,
			PrincipalID: lis.principal,
			AuthorizationRevision: 1,
		})
		r = r.WithContext(ctx)

		// Per-write deadlines (including flush) keep SSE alive past a
		// server-wide write cap; each write gets a fresh bound.
		next.ServeHTTP(&deadlineWriter{ResponseWriter: w, now: func() time.Time { return time.Now().Add(taskHTTPPerWrite) }}, r)
	})
}

// deadlineWriter sets a fresh write deadline before every Write so slow
// individual writes time out without capping a stream's total lifetime.
type deadlineWriter struct {
	http.ResponseWriter
	now func() time.Time
}

func (w *deadlineWriter) Write(b []byte) (int, error) {
	if rc := http.NewResponseController(w.ResponseWriter); rc != nil {
		_ = rc.SetWriteDeadline(w.now())
	}
	return w.ResponseWriter.Write(b)
}

func (w *deadlineWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		if rc := http.NewResponseController(w.ResponseWriter); rc != nil {
			_ = rc.SetWriteDeadline(w.now())
		}
		f.Flush()
	}
}

// acquireStream registers one in-flight request against the task/principal
// stream caps; the returned func releases the slot.
func (l *taskHTTPListener) acquireStream(taskID string) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.streamN >= taskHTTPStreamPerPrin {
		return nil, false
	}
	if taskID != "" && l.streams[taskID] >= taskHTTPStreamPerTask {
		return nil, false
	}
	l.streamN++
	if taskID != "" {
		l.streams[taskID]++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.streamN--
			if taskID != "" {
				l.streams[taskID]--
				if l.streams[taskID] == 0 {
					delete(l.streams, taskID)
				}
			}
		})
	}, true
}

// envelopePrincipalTokenEnv is the credential reference of the configured
// principal — a name, never a value.
func envelopePrincipalTokenEnv(e config.ChannelEnvelope) string {
	if e.HTTP == nil {
		return ""
	}
	return e.HTTP.Principal.TokenEnv
}

// readBoundedBody drains the (already MaxBytesReader-capped) body.
func readBoundedBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// extractTaskID pulls message.taskId / params.taskId out of a JSON-RPC
// task payload for stream-slot accounting. Malformed bodies extract no
// key — admission validation still owns rejection downstream.
func extractTaskID(body []byte) string {
	var doc struct {
		Params struct {
			TaskID  string `json:"taskId"`
			Message struct {
				TaskID string `json:"taskId"`
			} `json:"message"`
		} `json:"params"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return ""
	}
	if doc.Params.Message.TaskID != "" {
		return doc.Params.Message.TaskID
	}
	return doc.Params.TaskID
}

// TaskHTTPStates reports listener lifecycle truth for Inspect/tests:
// channel name -> absent|inactive|starting|serving|draining|failed|stopped.
func (h *Host) TaskHTTPStates() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]string, len(h.taskListeners))
	for name, l := range h.taskListeners {
		l.mu.Lock()
		out[name] = string(l.state)
		l.mu.Unlock()
	}
	return out
}

