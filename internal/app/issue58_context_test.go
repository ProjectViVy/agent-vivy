package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	divacognitive "agent-vivy/internal/modules/diva-cognitive"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"github.com/ProjectViVy/laputa/garden/agentapi"
)

type issue58AppMain struct{ source storage.SubjectSourceBinding }

func (m issue58AppMain) ResolveMainConversation(context.Context) (cognitivecontract.MainConversationBinding, error) {
	s := m.source
	return cognitivecontract.MainConversationBinding{SubjectID: s.SubjectID, DomainID: s.DomainID, ConversationID: s.ConversationID, ActiveSessionID: s.SessionID, PolicyID: s.PolicyID, ToolPolicyRef: s.ToolPolicyRef, SchemaVersion: s.SchemaVersion, Grant: s.Grant}, nil
}

// Inject a lost response and unavailable live bound port. The real Garden
// bundle still owns its actual native Store and is closed by App's lifecycle.
// Owner-level replacement fault injection is separately covered in Laputa.
type issue58AppUnknownPort struct {
	cognitivecontract.RecapPort
	unknown   bool
	operation string
}

func (p *issue58AppUnknownPort) CommitRecap(ctx context.Context, c cognitivecontract.RecapCommit) (cognitivecontract.RecapHead, error) {
	if p.unknown {
		return cognitivecontract.RecapHead{}, &agentapi.Error{Code: "recap_commit_unknown"}
	}
	_, err := p.RecapPort.CommitRecap(ctx, c)
	if err != nil {
		return cognitivecontract.RecapHead{}, err
	}
	p.unknown = true
	p.operation = c.OperationID
	return cognitivecontract.RecapHead{}, &agentapi.Error{Code: "recap_commit_unknown"}
}
func (p *issue58AppUnknownPort) ReadRecap(ctx context.Context) (cognitivecontract.RecapHead, error) {
	if p.unknown {
		return cognitivecontract.RecapHead{}, &agentapi.Error{Code: "recap_commit_unknown"}
	}
	return p.RecapPort.ReadRecap(ctx)
}
func (p *issue58AppUnknownPort) LookupRecapOperation(ctx context.Context, id string) (cognitivecontract.RecapHead, bool, error) {
	if p.unknown {
		return cognitivecontract.RecapHead{}, false, &agentapi.Error{Code: "recap_commit_unknown"}
	}
	return p.RecapPort.LookupRecapOperation(ctx, id)
}

func issue58AppOpener(t *testing.T, handler http.Handler) (func(bool) (*App, *issue58AppUnknownPort, cognitivecontract.RecapPort, *appEventSink), storage.SubjectSourceBinding, storage.SubjectSourceBinding) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	database := filepath.Join(root, "runtime.db")
	main := conformance.Issue58Source("main-reader", true)
	source := conformance.Issue58Source("source-A", false)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	ref, err := provider.NewCatalog().RefForSelection("fixture", provider.Endpoint{Adapter: provider.AdapterOpenAICompletions, BaseURL: server.URL, DefaultModel: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	model, err := ref.Model(ctx, provider.ModelSpec{ID: "fixture", APIKey: "fixture", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	open := func(fault bool) (*App, *issue58AppUnknownPort, cognitivecontract.RecapPort, *appEventSink) {
		backend, err := sqlite.Open(ctx, database)
		if err != nil {
			t.Fatal(err)
		}
		in := cognitivecontract.FactoryInput{Config: config.Config{Storage: config.Storage{DataDir: filepath.Join(root, "owner")}}, GenerationID: "issue58-test", ResolveRecapBinding: func(context.Context, string) (cognitivecontract.RecapBinding, error) {
			return cognitivecontract.RecapBinding{SubjectID: main.SubjectID, DomainID: main.DomainID}, nil
		}}
		bundle, err := divacognitive.Open(ctx, in)
		if err != nil {
			backend.Close()
			t.Fatal(err)
		}
		port := bundle.(cognitivecontract.RecapProvider).Recap()
		wrapper := &issue58AppUnknownPort{RecapPort: port}
		var bound cognitivecontract.RecapPort = port
		if fault {
			bound = wrapper
		}
		eng, err := runtime.NewEngine(ctx, model, nil, runtime.EngineConfig{MaxEventPayloadBytes: 64 << 10})
		if err != nil {
			t.Fatal(err)
		}
		sink := &appEventSink{}
		svc := runtime.NewService(eng, "fixture", "fixture", runtime.ServiceDeps{Sink: sink, Journal: backend, Runs: backend, Messages: backend, Sessions: backend, Truncations: backend, ToolEvidence: backend.Blobs(), Recap: bound, MainConversation: issue58AppMain{main}, SubjectSources: func(_ context.Context, sid domain.SessionID) (*storage.SubjectSourceBinding, error) {
			if sid == source.SessionID {
				return &source, nil
			}
			return nil, nil
		}})
		svc.SetCatalog(provider.NewCatalog(provider.Vendor{Name: "fixture", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, Models: []provider.Model{{ID: "fixture", ContextWindow: 128000}}}}}))
		return &App{service: svc, backend: backend, cognitive: bundle, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, wrapper, port, sink
	}
	return open, main, source
}

func issue58SeedAppSources(t *testing.T, b *sqlite.Backend, main, source storage.SubjectSourceBinding) {
	t.Helper()
	ctx := context.Background()
	var err error
	conformance.Issue58Identity(t, b)
	for _, sid := range []domain.SessionID{main.SessionID, source.SessionID} {
		if err = b.CreateSession(ctx, domain.Session{ID: sid}); err != nil {
			t.Fatal(err)
		}
	}
	for i, text := range []string{strings.Repeat("A", 10000), "retained suffix"} {
		admission := conformance.Issue58Admission(source, fmt.Sprintf("host-source-%d", i))
		admission.Message.Content = text
		if _, err = b.CommitRunAdmission(ctx, admission); err != nil {
			t.Fatal(err)
		}
		seq, e := b.Append(ctx, storage.Commit{RunID: admission.Run.ID, Events: []domain.RunEvent{{RunID: admission.Run.ID, Type: domain.EventRunCompleted, PayloadVersion: 1, Payload: []byte(`{}`)}}})
		if e != nil {
			t.Fatal(e)
		}
		if err = b.SetRunStatus(ctx, admission.Run.ID, domain.RunCompleted); err != nil {
			t.Fatal(err)
		}
		digest, e := storage.SubjectEvidenceDigest([]domain.Message{admission.Message})
		if e != nil {
			t.Fatal(e)
		}
		if _, err = b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, storage.SubjectInteraction{ID: storage.SubjectInteractionID(admission.Run.ID, seq), SessionID: source.SessionID, RunID: admission.Run.ID, TerminalSeq: seq, SourceDigest: digest}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIssue58CompactionCrashCutsHostReopen(t *testing.T) {
	ctx := context.Background()
	var calls atomic.Int32
	open, main, source := issue58AppOpener(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"summary","object":"chat.completion","model":"fixture","choices":[{"index":0,"message":{"role":"assistant","content":"durable native summary"},"finish_reason":"stop"}]}`)
	}))
	app, unknown, _, _ := open(true)
	b := app.backend.(*sqlite.Backend)
	issue58SeedAppSources(t, b, main, source)
	if _, err := app.service.CompactSession(ctx, main.SessionID, runtime.CompactOptions{}); !errors.Is(err, runtime.ErrRecapCommitPending) {
		t.Fatalf("live unknown not paused: %v", err)
	}
	pending, err := b.Blobs().(storage.BlobPrefixLister).ListPrefix(ctx, "recap/pending/v1/")
	if err != nil || len(pending) != 1 || unknown.operation == "" {
		t.Fatalf("durable pending unavailable %v %v", pending, err)
	}
	raw, found, err := b.Blobs().Get(ctx, pending[0])
	if err != nil || !found {
		t.Fatal("intent not durable")
	}
	var intent struct{ Request cognitivecontract.RecapCommit }
	if json.Unmarshal(raw, &intent) != nil || intent.Request.OperationID != unknown.operation {
		t.Fatal("exact pending request lost")
	}
	if err = app.CloseContext(ctx); err != nil {
		t.Fatal(err)
	}
	// A normal new host constructs the sole renewed bound owner on the same root.
	reopened, _, renewed, _ := open(false)
	defer reopened.CloseContext(ctx)
	if _, err = reopened.service.CompactSession(ctx, main.SessionID, runtime.CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	receipt, ok, err := renewed.LookupRecapOperation(ctx, intent.Request.OperationID)
	if err != nil || !ok || receipt.CoveredThrough != 1 || receipt.Text != intent.Request.Text || receipt.Sources != intent.Request.Sources || calls.Load() != 1 {
		t.Fatalf("renewed host lost exact receipt %+v found=%v err=%v calls=%d", receipt, ok, err, calls.Load())
	}
}

type issue58AppCloseProbe struct {
	cognitivecontract.Bundle
	before func() error
}

func (b *issue58AppCloseProbe) Close() error { return errors.Join(b.before(), b.Bundle.Close()) }

func TestIssue58ExplicitCompactionShutdownSettlesBeforeOwnerClose(t *testing.T) {
	ctx := context.Background()
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	open, main, source := issue58AppOpener(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
		http.Error(w, "cancelled fixture", http.StatusServiceUnavailable)
	}))
	app, _, _, sink := open(false)
	b := app.backend.(*sqlite.Backend)
	issue58SeedAppSources(t, b, main, source)
	done := make(chan error, 1)
	app.cognitive = &issue58AppCloseProbe{Bundle: app.cognitive, before: func() error {
		var control domain.RunID
		for _, event := range sink.capturedEvents() {
			if strings.HasPrefix(string(event.RunID), "sessctl_recap_") {
				control = event.RunID
				break
			}
		}
		if control == "" {
			return errors.New("owner close preceded native summary request")
		}
		iterator, err := b.Replay(ctx, control, 0)
		if err != nil {
			return err
		}
		defer iterator.Close()
		finished := false
		for iterator.Next() {
			event := iterator.Value().Event
			if _, terminal := event.Type.RunStatus(); terminal {
				return errors.New("synthetic control fabricated conversation terminal")
			}
			if event.Type == domain.EventModelCallFinished {
				var p struct {
					Status           string
					ResponseComplete bool `json:"response_complete"`
				}
				if json.Unmarshal(event.Payload, &p) != nil || p.Status != "cancelled" || p.ResponseComplete {
					return fmt.Errorf("cancellation closure lost actual outcome: %s", event.Payload)
				}
				finished = true
			}
		}
		if err = iterator.Err(); err != nil {
			return err
		}
		if !finished {
			return errors.New("owner close preceded native summary cancellation settlement")
		}
		if _, err = b.GetRun(ctx, control); !errors.Is(err, storage.ErrNotFound) {
			return errors.New("control acquired a real RunStore row")
		}
		return nil
	}}
	go func() {
		_, err := app.service.CompactSession(ctx, main.SessionID, runtime.CompactOptions{})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("summary did not enter actual HTTP")
	}
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := app.CloseContext(closeCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("compaction cancellation result %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("host closed before compaction returned")
	}
}
