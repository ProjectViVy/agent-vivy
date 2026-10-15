package runtime

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/testsupport"
	"github.com/ProjectViVy/laputa/laputa/actmem"
)

func TestIssue58SharedRecentActualRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"fixture\",\"object\":\"chat.completion.chunk\",\"model\":\"fixture\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"complete\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	ref, err := provider.NewCatalog().RefForSelection("fixture", provider.Endpoint{Adapter: provider.AdapterOpenAICompletions, BaseURL: server.URL, DefaultModel: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	model, err := ref.Model(ctx, provider.ModelSpec{ID: "fixture", APIKey: "fixture", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	svc, b, _ := newTestService(t, testsupport.NewEchoModel())
	eng, err := NewEngine(ctx, model, nil, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	svc.engine = eng
	svc.deps.Recap = issue58Owner(t)
	svc.SetCatalog(provider.NewCatalog(provider.Vendor{Name: "test", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, DefaultModel: "test-model", Models: []provider.Model{{ID: "test-model", ContextWindow: 128000}}}}}))
	conformance.Issue58Identity(t, b)
	mustCreateSession(t, b, "main-reader")
	sources := map[domain.SessionID]storage.SubjectSourceBinding{}
	for _, id := range []string{"source-A", "source-B"} {
		source := conformance.Issue58Source(domain.SessionID(id), false)
		sources[source.SessionID] = source
		mustCreateSession(t, b, source.SessionID)
		in := conformance.Issue58Completed(t, b, source, id+"-original")
		if _, err := b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, in); err != nil {
			t.Fatal(err)
		}
	}
	issue58Sources(svc, sources)
	rid, err := svc.Run(ctx, "main-reader", "current once")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not settle")
	}
	run, err := b.GetRun(ctx, rid)
	if err != nil || run.Status != domain.RunCompleted {
		t.Fatalf("run %+v %v events=%+v", run, err, replayAll(t, b, rid))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	wire := requests[0]
	t.Logf("HTTP_CAPTURE %s", wire)
	// Both originals have the same text, but distinct canonical identities.
	if strings.Count(wire, "exact source source-A-original") != 1 || strings.Count(wire, "exact source source-B-original") != 1 || strings.Count(wire, "current once") != 1 {
		t.Fatalf("shared originals/current lost or duplicated: %s", wire)
	}
}

// This fixture binds the real C1 owner to the same subject identity. It has no
// inference or alternate evidence store; production receives its bundle port.
type issue58BoundOwner struct {
	store   *actmem.Store
	binding cognitivecontract.RecapBinding
}

func issue58Owner(t *testing.T) *issue58BoundOwner {
	t.Helper()
	store, err := actmem.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	source := conformance.Issue58Source("main-reader", true)
	return &issue58BoundOwner{store: store, binding: cognitivecontract.RecapBinding{SubjectID: source.SubjectID, DomainID: source.DomainID}}
}
func (o *issue58BoundOwner) ReadRecap(ctx context.Context) (cognitivecontract.RecapHead, error) {
	return o.store.ReadRecap(ctx, o.binding)
}
func (o *issue58BoundOwner) CommitRecap(ctx context.Context, c cognitivecontract.RecapCommit) (cognitivecontract.RecapHead, error) {
	c.Binding = o.binding
	return o.store.CommitRecap(ctx, c)
}
func (o *issue58BoundOwner) LookupRecapOperation(ctx context.Context, id string) (cognitivecontract.RecapHead, bool, error) {
	return o.store.LookupRecapOperation(ctx, o.binding, id)
}
func (o *issue58BoundOwner) ListRecapHistory(ctx context.Context, before uint64, limit int) ([]cognitivecontract.RecapHead, error) {
	return o.store.ListRecapHistory(ctx, o.binding, before, limit)
}
func (o *issue58BoundOwner) ReadRecapRevision(ctx context.Context, rev uint64) (cognitivecontract.RecapHead, error) {
	return o.store.ReadRecapRevision(ctx, o.binding, rev)
}
