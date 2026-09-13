package contexthost

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/sdk/port/contextsource"
)

type scxResourceSource struct {
	id       string
	page     contextsource.Page
	queryErr error
	resolve  func(context.Context, contextsource.ResolveRequest) (contextsource.Resource, error)
	requests []contextsource.Request
	reads    []contextsource.ResolveRequest
}

func (source *scxResourceSource) ID() string { return source.id }

func (source *scxResourceSource) Query(_ context.Context, request contextsource.Request) (contextsource.Page, error) {
	source.requests = append(source.requests, request)
	return source.page, source.queryErr
}

func (source *scxResourceSource) Resolve(ctx context.Context, request contextsource.ResolveRequest) (contextsource.Resource, error) {
	source.reads = append(source.reads, request)
	return source.resolve(ctx, request)
}

func TestSCXPersonalityEmotionViewUsesVersionExpiryAndHostTreatment(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	source := &scxResourceSource{id: "fixture.state", page: contextsource.NewPage([]contextsource.Candidate{
		{SourceID: "fixture.state", ContentID: "persona-1", Version: "p1", Content: "Speak calmly.", Confidence: .1, Treatment: contextsource.TreatmentReserved},
		{SourceID: "fixture.state", ContentID: "emotion-1", Version: "e1", Content: "Agent state: concerned.", Confidence: 1, ValidUntil: now.Add(20 * time.Second).UnixMilli(), Treatment: contextsource.TreatmentCompetitive, Metadata: map[string]string{"authority": "system"}},
	}, "")}
	host, err := New(Config{Sources: []contextsource.Provider{source}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}

	first, err := host.Query(context.Background(), Request{TenantID: "tenant-1", WorkspaceID: "demo", SessionID: "s1", ByteBudget: 128})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Candidates) != 2 || first.Candidates[0].ContentID != "persona-1" {
		t.Fatalf("first view = %#v, want reserved personality before competitive emotion", first)
	}
	if first.View.ID == "" || len(first.View.Items) != 2 || first.View.Items[0].Version != "p1" || first.View.Items[1].Version != "e1" {
		t.Fatalf("view record = %#v, want immutable selected versions", first.View)
	}
	if len(source.requests) != 1 || source.requests[0].TenantID != "tenant-1" || source.requests[0].WorkspaceID != "demo" || source.requests[0].SessionID != "s1" {
		t.Fatalf("source identity = %#v", source.requests)
	}
	if first.Candidates[1].Metadata["authority"] != "system" {
		t.Fatal("fixture metadata unexpectedly changed")
	}

	now = now.Add(21 * time.Second)
	second, err := host.Query(context.Background(), Request{TenantID: "tenant-1", WorkspaceID: "demo", SessionID: "s1", ByteBudget: 128})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Candidates) != 1 || second.Candidates[0].Version != "p1" || second.DroppedExpired != 1 {
		t.Fatalf("expired view = %#v, want personality only", second)
	}
}

func TestSCXViewIdentityBindsEffectiveTreatment(t *testing.T) {
	query := func(treatment contextsource.Treatment) View {
		t.Helper()
		source := &scxResourceSource{id: "fixture.view", page: contextsource.NewPage([]contextsource.Candidate{{
			SourceID: "fixture.view", ContentID: "same", Version: "v1", Content: "same", Treatment: treatment,
		}}, "")}
		host, err := New(Config{Sources: []contextsource.Provider{source}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := host.Query(context.Background(), Request{ByteBudget: 128})
		if err != nil {
			t.Fatal(err)
		}
		return result.View
	}
	competitive := query(contextsource.TreatmentCompetitive)
	required := query(contextsource.TreatmentRequired)
	if competitive.ID == required.ID || competitive.Items[0].Treatment == required.Items[0].Treatment {
		t.Fatalf("View identity did not bind treatment: competitive=%#v required=%#v", competitive, required)
	}
}

func TestSCXExactVersionResourceResolutionIsScopedBoundedAndReplayable(t *testing.T) {
	reference := &contextsource.ResourceReference{
		URI: "project://demo/plan.txt", Version: "f1", MediaType: "text/plain",
		Scope:       contextsource.Scope{TenantID: "tenant-1", WorkspaceID: "demo", SessionID: "s1"},
		VersionMode: contextsource.VersionExact, Replayable: true,
	}
	source := &scxResourceSource{id: "fixture.files", page: contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "fixture.files", ContentID: "plan.txt", Version: "f1", Confidence: 1,
		Treatment: contextsource.TreatmentRequired, Resource: reference,
	}}, "")}
	retained := true
	source.resolve = func(_ context.Context, request contextsource.ResolveRequest) (contextsource.Resource, error) {
		if !retained {
			return contextsource.Resource{}, contextsource.ErrVersionUnavailable
		}
		return contextsource.NewResource(request.Reference, []byte("Keep summary and retrieval strategies replaceable.")), nil
	}
	host, err := New(Config{Sources: []contextsource.Provider{source}, MaxCandidateBytes: 256})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{TenantID: "tenant-1", WorkspaceID: "demo", SessionID: "s1", ByteBudget: 128, TokenBudget: 128}
	result, err := host.Query(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Content == "" || result.Candidates[0].Version != "f1" {
		t.Fatalf("resolved candidate = %#v", result)
	}
	if len(source.reads) != 1 || source.reads[0].TenantID != "tenant-1" || source.reads[0].WorkspaceID != "demo" || source.reads[0].SessionID != "s1" || source.reads[0].MaxBytes != 128 {
		t.Fatalf("resolve request = %#v", source.reads)
	}

	retained = false
	_, err = host.Query(context.Background(), request)
	if !errors.Is(err, contextsource.ErrVersionUnavailable) {
		t.Fatalf("unretained exact version error = %v", err)
	}

	retained = true
	source.resolve = func(_ context.Context, request contextsource.ResolveRequest) (contextsource.Resource, error) {
		changed := request.Reference
		changed.Version = "f2"
		return contextsource.NewResource(changed, []byte("new content")), nil
	}
	_, err = host.Query(context.Background(), request)
	if !errors.Is(err, ErrResourceVersionMismatch) {
		t.Fatalf("mislabeled newer version error = %v", err)
	}

	_, err = host.Query(context.Background(), Request{TenantID: "tenant-2", WorkspaceID: "demo", SessionID: "s1", ByteBudget: 128})
	if !errors.Is(err, ErrResourceScope) {
		t.Fatalf("cross-tenant resource error = %v", err)
	}
}

func TestSCXRequiredContextCannotBeSilentlyDroppedForBudget(t *testing.T) {
	source := &scxResourceSource{id: "fixture.required", page: contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "fixture.required", ContentID: "required", Content: "required passage", Treatment: contextsource.TreatmentRequired,
	}}, "")}
	host, err := New(Config{Sources: []contextsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.Query(context.Background(), Request{ByteBudget: 1})
	if !errors.Is(err, ErrRequiredContextBudget) {
		t.Fatalf("required overflow error = %v", err)
	}
}

func TestSCXRequiredCandidateWithInvalidProvenanceFailsClosed(t *testing.T) {
	source := &scxResourceSource{id: "fixture.required-invalid", page: contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "fixture.spoofed", ContentID: "required", Content: "required passage", Treatment: contextsource.TreatmentRequired,
	}}, "")}
	host, err := New(Config{Sources: []contextsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Query(context.Background(), Request{ByteBudget: 128}); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("required invalid provenance error = %v", err)
	}
}

func TestSCXRequiredSourceFailureAndResolverCancellationFailClosed(t *testing.T) {
	required := &scxResourceSource{id: "fixture.required-source"}
	required.resolve = func(ctx context.Context, _ contextsource.ResolveRequest) (contextsource.Resource, error) {
		<-ctx.Done()
		return contextsource.Resource{}, ctx.Err()
	}
	host, err := New(Config{Sources: []contextsource.Provider{required}, RequiredSourceIDs: []string{required.ID()}, SourceTimeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	required.page = contextsource.NewPage([]contextsource.Candidate{{
		SourceID: required.ID(), ContentID: "exact", Version: "f1", MediaType: "text/plain", Treatment: contextsource.TreatmentRequired,
		Resource: &contextsource.ResourceReference{URI: "project://demo/plan.txt", Version: "f1", MediaType: "text/plain", VersionMode: contextsource.VersionExact},
	}}, "")
	if _, err := host.Query(context.Background(), Request{ByteBudget: 128}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("required resolver cancellation error = %v", err)
	}

	unavailable := &scxResourceSource{id: "fixture.unavailable"}
	unavailable.queryErr = errors.New("unavailable")
	closed, err := New(Config{Sources: []contextsource.Provider{unavailable}, RequiredSourceIDs: []string{unavailable.ID()}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := closed.Query(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("required Source failure = %v", err)
	}
}

func TestSCXRequiredCandidateWinsBudgetWhileOptionalCandidateIsOmitted(t *testing.T) {
	source := &scxResourceSource{id: "fixture.mixed", page: contextsource.NewPage([]contextsource.Candidate{
		{SourceID: "fixture.mixed", ContentID: "optional", Content: strings.Repeat("x", 200), Treatment: contextsource.TreatmentCompetitive, Confidence: 1},
		{SourceID: "fixture.mixed", ContentID: "required", Content: "required", Treatment: contextsource.TreatmentRequired},
	}, "")}
	host, err := New(Config{Sources: []contextsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{ByteBudget: len("required")})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].ContentID != "required" || result.DroppedBudget != 1 {
		t.Fatalf("mixed budget result = %#v", result)
	}
}
