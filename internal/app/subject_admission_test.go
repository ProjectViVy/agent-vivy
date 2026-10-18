package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	divacognitive "agent-vivy/internal/modules/diva-cognitive"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

type issue58Main struct {
	binding cognitivecontract.MainConversationBinding
}

func (r *issue58Main) ResolveMainConversation(context.Context) (cognitivecontract.MainConversationBinding, error) {
	return r.binding, nil
}
func issue58Service(t *testing.T, b *sqlite.Backend, main cognitivecontract.MainConversationResolver, sources cognitivecontract.SubjectSourceResolver) *runtime.Service {
	t.Helper()
	ctx := context.Background()
	ts, err := tools.Builtin(b).Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := runtime.NewEngine(ctx, runtime.WrapModel(testsupport.NewEchoModel()), ts, runtime.EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	svc := runtime.NewService(eng, "test", "test", runtime.ServiceDeps{ToolEvidence: b.Blobs(), Journal: b, Runs: b, Messages: b, Sessions: b, Sink: newHeadlessSink(&out, &errw), MainConversation: main, SubjectSources: sources, Recap: issue58FixtureRecap(t, main)})
	svc.SetCatalog(issue58FixtureCatalog())
	return svc
}

// These headless producer fixtures use the same bound bundle port consumed by
// app.New. They do not relax production refusal when Recap/capacity is absent.
func issue58FixtureRecap(t *testing.T, main cognitivecontract.MainConversationResolver) cognitivecontract.RecapPort {
	t.Helper()
	if main == nil {
		return nil
	}
	binding, err := main.ResolveMainConversation(context.Background())
	if err != nil {
		return nil
	}
	bundle, err := divacognitive.Open(context.Background(), cognitivecontract.FactoryInput{Config: config.Config{Storage: config.Storage{DataDir: t.TempDir()}}, GenerationID: "issue58-fixture", ResolveRecapBinding: func(context.Context, string) (cognitivecontract.RecapBinding, error) {
		return cognitivecontract.RecapBinding{SubjectID: binding.SubjectID, DomainID: binding.DomainID}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bundle.Close() })
	port, ok := bundle.(cognitivecontract.RecapProvider)
	if !ok {
		t.Fatal("bundle has no bound Recap port")
	}
	return port.Recap()
}
func issue58FixtureCatalog() *provider.Catalog {
	return provider.NewCatalog(provider.Vendor{Name: "test", Endpoints: []provider.Endpoint{{Adapter: provider.AdapterOpenAICompletions, DefaultModel: "test", Models: []provider.Model{{ID: "test", ContextWindow: 128000}}}}})
}

func TestIssue58SingletonAdmission(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, t.TempDir()+"/core.db")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	conformance.Issue58Identity(t, b)
	main := conformance.Issue58Source("main", true)
	resolver := &issue58Main{binding: cognitivecontract.MainConversationBinding{SubjectID: main.SubjectID, DomainID: main.DomainID, ConversationID: main.ConversationID, ActiveSessionID: main.SessionID, PolicyID: main.PolicyID, ToolPolicyRef: main.ToolPolicyRef, SchemaVersion: main.SchemaVersion, Grant: main.Grant}}
	for _, sid := range []domain.SessionID{"main", "ordinary", "source", "convert"} {
		if err := b.CreateSession(ctx, domain.Session{ID: sid}); err != nil {
			t.Fatal(err)
		}
	}
	source := conformance.Issue58Source("source", false)
	svc := issue58Service(t, b, resolver, func(_ context.Context, sid domain.SessionID) (*storage.SubjectSourceBinding, error) {
		if sid == source.SessionID {
			return &source, nil
		}
		return nil, nil
	})
	for _, sid := range []domain.SessionID{"main", "ordinary", "source", "convert"} {
		rid, err := svc.Run(ctx, sid, "hello")
		if err != nil {
			t.Fatal(err)
		}
		if !svc.WaitIdle(ctx) {
			t.Fatal("run did not become idle")
		}
		pin, err := b.GetRunSubjectSource(ctx, rid)
		if sid == "ordinary" || sid == "convert" {
			if !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("ordinary acquired resident policy: %+v %v", pin, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if sid == "main" && (pin.PolicyID != "life-recap-v1" || !pin.Grant.WholeDomainRead) {
			t.Fatalf("main policy missing: %+v", pin)
		}
		if sid == "source" && (pin.PolicyID == "life-recap-v1" || pin.Grant.WholeDomainRead) {
			t.Fatalf("contribution escalated read rights: %+v", pin)
		}
	}
	if _, err := svc.AuthorizeSubjectHistory(ctx, "source"); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("contribution acquired whole-domain read: %v", err)
	}
	if _, err := svc.AuthorizeSubjectHistory(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	// Restart without trusted producer must not downgrade a durable main.
	restarted := issue58Service(t, b, nil, nil)
	if _, err := restarted.Run(ctx, "main", "again"); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("missing resolver downgraded main: %v", err)
	}
	// Presenting an already ordinary conversation as main is not conversion.
	resolver.binding.ActiveSessionID = "convert"
	if _, err := svc.Run(ctx, "convert", "convert"); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("ordinary converted: %v", err)
	}
	resolver.binding.ActiveSessionID = "main"
	resolver.binding.Grant.Version++
	if _, err := svc.Run(ctx, "main", "changed grant"); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("changed grant accepted: %v", err)
	}
	sessions, err := b.ListSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.ID == "main" {
			t.Fatal("internal main appears as ordinary session")
		}
	}
}

func TestIssue58ShellFirstEligibleUnsupportedToolFailsBeforeSideEffects(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, t.TempDir()+"/core.db")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	conformance.Issue58Identity(t, b)
	s := conformance.Issue58Source("shell-first", true)
	if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	resolver := &issue58Main{binding: cognitivecontract.MainConversationBinding{SubjectID: s.SubjectID, DomainID: s.DomainID, ConversationID: s.ConversationID, ActiveSessionID: s.SessionID, PolicyID: s.PolicyID, ToolPolicyRef: s.ToolPolicyRef, SchemaVersion: s.SchemaVersion, Grant: s.Grant}}
	svc := issue58Service(t, b, resolver, nil)
	if _, err := svc.RunShell(ctx, s.SessionID, "echo never"); !errors.Is(err, runtime.ErrShellUnavailable) {
		t.Fatalf("unsupported eligible shell did not fail before admission: %v", err)
	}
	runs, err := b.ListRunsBySession(ctx, s.SessionID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("eligible shell admitted a run: %v %v", runs, err)
	}
	if _, err := b.GetSubjectSource(ctx, s.SessionID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("failed shell left misleading registration: %v", err)
	}
}

func TestIssue58ExactTextInteractionSealsThroughServiceRun(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, t.TempDir()+"/core.db")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	conformance.Issue58Identity(t, b)
	s := conformance.Issue58Source("exact-main", true)
	if err := b.CreateSession(ctx, domain.Session{ID: s.SessionID}); err != nil {
		t.Fatal(err)
	}
	resolver := &issue58Main{binding: cognitivecontract.MainConversationBinding{SubjectID: s.SubjectID, DomainID: s.DomainID, ConversationID: s.ConversationID, ActiveSessionID: s.SessionID, PolicyID: s.PolicyID, ToolPolicyRef: s.ToolPolicyRef, SchemaVersion: s.SchemaVersion, Grant: s.Grant}}
	svc := issue58Service(t, b, resolver, nil)
	rid, err := svc.Run(ctx, s.SessionID, "exact user evidence")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("run did not finish")
	}
	messages, err := b.ListMessages(ctx, s.SessionID)
	if err != nil || len(messages) != 2 {
		t.Fatalf("exact projection missing: %v %v", messages, err)
	}
	var terminal domain.EventSeq
	for _, event := range replayEvents(t, b, rid) {
		if event.Type == domain.EventRunCompleted {
			terminal = event.Seq
		}
	}
	digest, err := storage.SubjectEvidenceDigest(messages)
	if err != nil {
		t.Fatal(err)
	}
	in := storage.SubjectInteraction{ID: storage.SubjectInteractionID(rid, terminal), SessionID: s.SessionID, RunID: rid, TerminalSeq: terminal, SourceDigest: digest}
	sealed, err := b.SealSubjectInteraction(ctx, s.SubjectHistoryBinding, in)
	if err != nil || sealed.Sequence != 1 {
		t.Fatalf("complete exact evidence did not seal: %+v %v", sealed, err)
	}
	got, err := b.ReadSubjectInteraction(ctx, s.SubjectHistoryBinding, sealed.ID)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := storage.SubjectEvidenceDigest(got)
	if err != nil || actual != digest {
		t.Fatalf("read did not preserve original: %q %v", actual, err)
	}
}

func TestIssue58RetainedDeleteDoesNotTombstoneSingleton(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		t.Run(fmt.Sprintf("admitted-%t", admitted), func(t *testing.T) {
			ctx := context.Background()
			b, err := sqlite.Open(ctx, t.TempDir()+"/core.db")
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			conformance.Issue58Identity(t, b)
			source := conformance.Issue58Source("delete-main", true)
			if err := b.CreateSession(ctx, domain.Session{ID: source.SessionID}); err != nil {
				t.Fatal(err)
			}
			resolver := &issue58Main{binding: cognitivecontract.MainConversationBinding{SubjectID: source.SubjectID, DomainID: source.DomainID, ConversationID: source.ConversationID, ActiveSessionID: source.SessionID, PolicyID: source.PolicyID, ToolPolicyRef: source.ToolPolicyRef, SchemaVersion: source.SchemaVersion, Grant: source.Grant}}
			svc := issue58Service(t, b, resolver, nil)
			if admitted {
				if _, err := svc.Run(ctx, source.SessionID, "first"); err != nil {
					t.Fatal(err)
				}
				if !svc.WaitIdle(ctx) {
					t.Fatal("run did not finish")
				}
			}
			if err := svc.DeleteSession(ctx, source.SessionID); !errors.Is(err, storage.ErrRetainedSession) {
				t.Fatalf("main delete did not reject before destructive lifecycle: %v", err)
			}
			if admitted {
				missing := issue58Service(t, b, nil, nil)
				if err := missing.DeleteSession(ctx, source.SessionID); !errors.Is(err, storage.ErrRetainedSession) {
					t.Fatalf("restart lost deletion protection: %v", err)
				}
			}
			if _, err := svc.Run(ctx, source.SessionID, "still addressable"); err != nil {
				t.Fatalf("rejected deletion tombstoned main: %v", err)
			}
			if !svc.WaitIdle(ctx) {
				t.Fatal("run did not finish")
			}
		})
	}
}

func TestIssue58FormattedGrantAuthorizedRead(t *testing.T) {
	ctx := context.Background()
	b, err := sqlite.Open(ctx, t.TempDir()+"/core.db")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	conformance.Issue58Identity(t, b)
	source := conformance.Issue58Source("formatted-main", true)
	source.Grant.Snapshot = json.RawMessage("{\n  \"decision\": \"allow\", \"reason\": \"<personal>&trusted\"\n}")
	original := string(source.Grant.Snapshot)
	if err := b.CreateSession(ctx, domain.Session{ID: source.SessionID}); err != nil {
		t.Fatal(err)
	}
	resolver := &issue58Main{binding: cognitivecontract.MainConversationBinding{SubjectID: source.SubjectID, DomainID: source.DomainID, ConversationID: source.ConversationID, ActiveSessionID: source.SessionID, PolicyID: source.PolicyID, ToolPolicyRef: source.ToolPolicyRef, SchemaVersion: source.SchemaVersion, Grant: source.Grant}}
	svc := issue58Service(t, b, resolver, nil)
	rid, err := svc.Run(ctx, source.SessionID, "exact original <text>&bytes")
	if err != nil {
		t.Fatal(err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("first run did not finish")
	}
	messages, err := b.ListMessages(ctx, source.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	var terminal domain.EventSeq
	for _, event := range replayEvents(t, b, rid) {
		if event.Type == domain.EventRunCompleted {
			terminal = event.Seq
		}
	}
	digest, err := storage.SubjectEvidenceDigest(messages)
	if err != nil {
		t.Fatal(err)
	}
	interaction := storage.SubjectInteraction{ID: storage.SubjectInteractionID(rid, terminal), SessionID: source.SessionID, RunID: rid, TerminalSeq: terminal, SourceDigest: digest}
	if _, err := b.SealSubjectInteraction(ctx, source.SubjectHistoryBinding, interaction); err != nil {
		t.Fatal(err)
	}
	binding, err := svc.AuthorizeSubjectHistory(ctx, source.SessionID)
	if err != nil {
		t.Fatalf("unchanged formatted/HTML grant rejected after persistence: %v", err)
	}
	read, err := b.ReadSubjectInteraction(ctx, binding, interaction.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := storage.SubjectEvidenceDigest(read)
	if err != nil || got != digest {
		t.Fatalf("authorized original read changed evidence: %s %v", got, err)
	}
	if _, err := svc.Run(ctx, source.SessionID, "second admission"); err != nil {
		t.Fatalf("unchanged producer grant rejected at next admission: %v", err)
	}
	if !svc.WaitIdle(ctx) {
		t.Fatal("second run did not finish")
	}
	resolver.binding.Grant.Snapshot = json.RawMessage("{ \"decision\": \"deny\", \"reason\": \"<personal>&trusted\" }")
	if _, err := svc.AuthorizeSubjectHistory(ctx, source.SessionID); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("genuinely changed grant authorized: %v", err)
	}
	if _, err := svc.Run(ctx, source.SessionID, "changed decision"); !errors.Is(err, storage.ErrSubjectBinding) {
		t.Fatalf("genuinely changed grant admitted: %v", err)
	}
	if string(source.Grant.Snapshot) != original {
		t.Fatal("producer snapshot was mutated")
	}
}
