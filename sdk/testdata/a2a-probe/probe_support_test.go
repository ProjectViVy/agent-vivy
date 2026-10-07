package probe

import (
	"context"
	"iter"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

const (
	probeGoodToken          = "probe-good-token"
	probeRequiredExtension  = "urn:probe:required-extension"
	probeBoundTenantDefault = ""
)

type markerKey struct{}

// streamItem is one scripted stream element: either an event or an error.
type streamItem struct {
	event a2a.Event
	err   error
}

// probeObservation records what the handler actually saw for one call.
type probeObservation struct {
	method            string
	tenant            string // req.Tenant as sent on the wire
	callCtxTenant     string // CallContext.Tenant(): populated only by InterceptedHandler
	versionParams     []string
	extensionParams   []string
	authHeaders       []string
	marker            string
	returnImmediately bool
}

// probeHandler is the minimal custom a2asrv.RequestHandler used by the probe:
// an in-memory scripted handler that enforces host-side validation and records
// every call. It is testdata code; it never ships.
type probeHandler struct {
	mu           sync.Mutex
	observed     []probeObservation
	sendResult   a2a.SendMessageResult
	sendErr      error
	storedTask   *a2a.Task
	getErr       error
	subscribeErr error
	streamScript []streamItem
	boundTenant  string
}

var _ a2asrv.RequestHandler = (*probeHandler)(nil)

func (h *probeHandler) observe(ctx context.Context, method, tenant string) {
	obs := probeObservation{method: method, tenant: tenant}
	if v := ctx.Value(markerKey{}); v != nil {
		obs.marker, _ = v.(string)
	}
	if cc, ok := a2asrv.CallContextFrom(ctx); ok {
		obs.callCtxTenant = cc.Tenant()
		if sp := cc.ServiceParams(); sp != nil {
			obs.versionParams, _ = sp.Get(a2a.SvcParamVersion)
			obs.extensionParams, _ = sp.Get(a2a.SvcParamExtensions)
			obs.authHeaders, _ = sp.Get("authorization")
		}
	}
	h.mu.Lock()
	h.observed = append(h.observed, obs)
	h.mu.Unlock()
}

func (h *probeHandler) markReturnImmediately(req *a2a.SendMessageRequest) {
	if req != nil && req.Config != nil && req.Config.ReturnImmediately {
		h.mu.Lock()
		h.observed[len(h.observed)-1].returnImmediately = true
		h.mu.Unlock()
	}
}

func (h *probeHandler) last() probeObservation {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.observed) == 0 {
		return probeObservation{}
	}
	return h.observed[len(h.observed)-1]
}

func (h *probeHandler) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.observed)
}

func (h *probeHandler) checkTenant(ctx context.Context) error {
	if h.boundTenant == "" {
		return nil
	}
	cc, ok := a2asrv.CallContextFrom(ctx)
	if !ok || cc.Tenant() != h.boundTenant {
		return a2a.NewError(a2a.ErrUnauthorized, "caller tenant does not match this server")
	}
	return nil
}

func checkMessage(req *a2a.SendMessageRequest) error {
	if req == nil || req.Message == nil {
		return a2a.NewError(a2a.ErrInvalidParams, "message is required")
	}
	for _, p := range req.Message.Parts {
		if p == nil {
			return a2a.NewError(a2a.ErrInvalidParams, "nil part")
		}
		if _, ok := p.Content.(a2a.Text); !ok {
			return a2a.NewError(a2a.ErrUnsupportedContentType, "only text parts accepted")
		}
	}
	return nil
}

func (h *probeHandler) GetTask(ctx context.Context, req *a2a.GetTaskRequest) (*a2a.Task, error) {
	h.observe(ctx, "GetTask", req.Tenant)
	if h.getErr != nil {
		return nil, h.getErr
	}
	if h.storedTask == nil {
		return nil, a2a.NewError(a2a.ErrTaskNotFound, "no such task")
	}
	return h.storedTask, nil
}

func (h *probeHandler) ListTasks(ctx context.Context, req *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	h.observe(ctx, "ListTasks", req.Tenant)
	resp := &a2a.ListTasksResponse{}
	if h.storedTask != nil {
		resp.Tasks = []*a2a.Task{h.storedTask}
		resp.TotalSize = 1
	}
	return resp, nil
}

func (h *probeHandler) CancelTask(ctx context.Context, req *a2a.CancelTaskRequest) (*a2a.Task, error) {
	h.observe(ctx, "CancelTask", req.Tenant)
	if h.storedTask == nil {
		return nil, a2a.NewError(a2a.ErrTaskNotFound, "no such task")
	}
	if h.storedTask.Status.State.Terminal() {
		return nil, a2a.NewError(a2a.ErrTaskNotCancelable, "task already terminal")
	}
	cancelled := *h.storedTask
	cancelled.Status.State = a2a.TaskStateCanceled
	return &cancelled, nil
}

func (h *probeHandler) SendMessage(ctx context.Context, req *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	h.observe(ctx, "SendMessage", req.Tenant)
	h.markReturnImmediately(req)
	if err := checkMessage(req); err != nil {
		return nil, err
	}
	if err := h.checkTenant(ctx); err != nil {
		return nil, err
	}
	if h.sendErr != nil {
		return nil, h.sendErr
	}
	return h.sendResult, nil
}

func (h *probeHandler) SubscribeToTask(ctx context.Context, req *a2a.SubscribeToTaskRequest) iter.Seq2[a2a.Event, error] {
	h.observe(ctx, "SubscribeToTask", req.Tenant)
	return func(yield func(a2a.Event, error) bool) {
		if h.subscribeErr != nil {
			yield(nil, h.subscribeErr)
			return
		}
		if h.storedTask != nil {
			yield(h.storedTask, nil)
		}
	}
}

func (h *probeHandler) SendStreamingMessage(ctx context.Context, req *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error] {
	h.observe(ctx, "SendStreamingMessage", req.Tenant)
	return func(yield func(a2a.Event, error) bool) {
		if err := checkMessage(req); err != nil {
			yield(nil, err)
			return
		}
		if err := h.checkTenant(ctx); err != nil {
			yield(nil, err)
			return
		}
		for _, item := range h.streamScript {
			if !yield(item.event, item.err) {
				return
			}
		}
	}
}

func (h *probeHandler) GetTaskPushConfig(ctx context.Context, req *a2a.GetTaskPushConfigRequest) (*a2a.PushConfig, error) {
	h.observe(ctx, "GetTaskPushConfig", req.Tenant)
	return nil, a2a.NewError(a2a.ErrPushNotificationNotSupported, "push not supported by probe")
}

func (h *probeHandler) ListTaskPushConfigs(ctx context.Context, req *a2a.ListTaskPushConfigRequest) (*a2a.ListTaskPushConfigResponse, error) {
	h.observe(ctx, "ListTaskPushConfigs", req.Tenant)
	return nil, a2a.NewError(a2a.ErrPushNotificationNotSupported, "push not supported by probe")
}

func (h *probeHandler) CreateTaskPushConfig(ctx context.Context, req *a2a.PushConfig) (*a2a.PushConfig, error) {
	h.observe(ctx, "CreateTaskPushConfig", req.Tenant)
	return nil, a2a.NewError(a2a.ErrPushNotificationNotSupported, "push not supported by probe")
}

func (h *probeHandler) DeleteTaskPushConfig(ctx context.Context, req *a2a.DeleteTaskPushConfigRequest) error {
	h.observe(ctx, "DeleteTaskPushConfig", req.Tenant)
	return a2a.NewError(a2a.ErrPushNotificationNotSupported, "push not supported by probe")
}

func (h *probeHandler) GetExtendedAgentCard(ctx context.Context, req *a2a.GetExtendedAgentCardRequest) (*a2a.AgentCard, error) {
	h.observe(ctx, "GetExtendedAgentCard", req.Tenant)
	return nil, a2a.NewError(a2a.ErrExtendedCardNotConfigured, "no extended card")
}

// guardInterceptor enforces protocol-level admission in front of the handler:
// exactly one A2A-Version=1.0 service parameter and every required extension
// must be declared. This is the adapter-side seam VIVY's real server will own.
type guardInterceptor struct {
	requiredExts []string
}

func (g *guardInterceptor) Before(ctx context.Context, callCtx *a2asrv.CallContext, req *a2asrv.Request) (context.Context, any, error) {
	if sp := callCtx.ServiceParams(); sp != nil {
		if versions, ok := sp.Get(a2a.SvcParamVersion); ok {
			if len(versions) != 1 || versions[0] != "1.0" {
				return ctx, nil, a2a.NewError(a2a.ErrVersionNotSupported, "server requires A2A-Version: 1.0")
			}
		}
	}
	if len(g.requiredExts) > 0 {
		declared := callCtx.Extensions().RequestedURIs()
		for _, want := range g.requiredExts {
			if !slices.Contains(declared, want) {
				return ctx, nil, a2a.NewError(a2a.ErrExtensionSupportRequired, "extension "+want+" must be declared")
			}
		}
	}
	return ctx, nil, nil
}

func (g *guardInterceptor) After(ctx context.Context, callCtx *a2asrv.CallContext, resp *a2asrv.Response) error {
	return nil
}

// authMiddleware models host-owned authentication: the bearer is validated
// then STRIPPED so credentials never reach the handler, while a private
// principal marker is attached to the request context.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+probeGoodToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-31401,"message":"unauthenticated"}}`))
			return
		}
		r2 := r.Clone(context.WithValue(r.Context(), markerKey{}, "probe-principal-1"))
		r2.Header.Del("Authorization")
		next.ServeHTTP(w, r2)
	})
}

// newProbeServer mounts a bare custom handler behind the official JSON-RPC
// transport. The transport itself attaches CallContext with ServiceParams, so
// the handler observes request metadata without any interception layer.
func newProbeServer(t *testing.T, h *probeHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(a2asrv.NewJSONRPCHandler(h))
	t.Cleanup(srv.Close)
	return srv
}

// newGuardedServer mounts the handler behind host auth middleware plus an
// InterceptedHandler carrying the guard interceptor. Note: WithCallInterceptors
// is a RequestHandlerOption bound to the stock NewHandler pipeline, so a
// custom-only handler composes interceptors by constructing
// a2asrv.InterceptedHandler directly.
func newGuardedServer(t *testing.T, h *probeHandler, requiredExts ...string) *httptest.Server {
	t.Helper()
	wrapped := &a2asrv.InterceptedHandler{
		Handler:      h,
		Interceptors: []a2asrv.CallInterceptor{&guardInterceptor{requiredExts: requiredExts}},
	}
	srv := httptest.NewServer(authMiddleware(a2asrv.NewJSONRPCHandler(wrapped)))
	t.Cleanup(srv.Close)
	return srv
}

// svcParamInjector is a client-side interceptor that sets request service
// parameters (A2A-* headers on the wire).
type svcParamInjector map[string][]string

func (p svcParamInjector) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	for k, v := range p {
		req.ServiceParams[k] = v
	}
	return ctx, nil, nil
}

func (svcParamInjector) After(ctx context.Context, resp *a2aclient.Response) error { return nil }

type bearerRT struct{ token string }

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r2)
}

func mustClient(t *testing.T, srv *httptest.Server, hc *http.Client, inject map[string][]string) *a2aclient.Client {
	t.Helper()
	card := &a2a.AgentCard{
		Name:         "probe-agent",
		Capabilities: a2a.AgentCapabilities{Streaming: true},
		SupportedInterfaces: []*a2a.AgentInterface{{
			URL:             srv.URL,
			ProtocolBinding: a2a.TransportProtocolJSONRPC,
			ProtocolVersion: a2a.Version,
		}},
	}
	opts := []a2aclient.FactoryOption{a2aclient.WithJSONRPCTransport(hc)}
	if len(inject) > 0 {
		opts = append(opts, a2aclient.WithCallInterceptors(svcParamInjector(inject)))
	}
	client, err := a2aclient.NewFromCard(context.Background(), card, opts...)
	if err != nil {
		t.Fatalf("new official client: %v", err)
	}
	return client
}

func newProbeClient(t *testing.T, srv *httptest.Server, inject map[string][]string) *a2aclient.Client {
	t.Helper()
	return mustClient(t, srv, nil, inject)
}

func newAuthedProbeClient(t *testing.T, srv *httptest.Server, token string, inject map[string][]string) *a2aclient.Client {
	t.Helper()
	return mustClient(t, srv, &http.Client{Transport: bearerRT{token: token}}, inject)
}
