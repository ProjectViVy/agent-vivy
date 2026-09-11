package contexthost

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/contextsource"
)

type fixtureSource struct {
	id    string
	page  contextsource.Page
	err   error
	block bool
}

type limitRecordingSource struct {
	id    string
	page  contextsource.Page
	limit int
}

func (source *limitRecordingSource) ID() string { return source.id }
func (source *limitRecordingSource) Query(_ context.Context, request contextsource.Request) (contextsource.Page, error) {
	source.limit = request.Limit
	return source.page, nil
}

type cancellingSource struct {
	id     string
	cancel context.CancelFunc
}

type nilResultAfterTimeoutSource struct {
	resultReady    chan struct{}
	deadlinePassed chan struct{}
}

func (source *nilResultAfterTimeoutSource) ID() string { return "late" }
func (source *nilResultAfterTimeoutSource) Query(ctx context.Context, _ contextsource.Request) (contextsource.Page, error) {
	if _, ok := ctx.Deadline(); !ok {
		return contextsource.Page{}, errors.New("source context has no deadline")
	}
	context.AfterFunc(ctx, func() {
		close(source.deadlinePassed)
	})
	close(source.resultReady)
	return contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "late", ContentID: "result", Content: "must not pass", Confidence: 1,
	}}, ""), nil
}

type postResultTimeoutContext struct {
	context.Context
	resultReady    <-chan struct{}
	deadlinePassed <-chan struct{}
}

func (ctx *postResultTimeoutContext) Err() error {
	select {
	case <-ctx.resultReady:
		<-ctx.deadlinePassed
	default:
	}
	return nil
}

func (source cancellingSource) ID() string { return source.id }
func (source cancellingSource) Query(_ context.Context, _ contextsource.Request) (contextsource.Page, error) {
	source.cancel()
	return contextsource.NewPage([]contextsource.Candidate{{
		SourceID: source.id, ContentID: "late", Content: "must not pass", Confidence: 1,
	}}, ""), nil
}

func (source fixtureSource) ID() string { return source.id }
func (source fixtureSource) Query(ctx context.Context, _ contextsource.Request) (contextsource.Page, error) {
	if source.block {
		<-ctx.Done()
		return contextsource.Page{}, ctx.Err()
	}
	return source.page, source.err
}

func TestHostDeduplicatesAndRanksCandidates(t *testing.T) {
	host, err := New(Config{Sources: []contextsource.Provider{
		fixtureSource{id: "source-b", page: contextsource.NewPage([]contextsource.Candidate{
			{SourceID: "source-b", ContentID: "b", Content: "same body", Confidence: 0.9},
			{SourceID: "source-b", ContentID: "c", Content: "unique", Confidence: 0.7},
		}, "")},
		fixtureSource{id: "source-a", page: contextsource.NewPage([]contextsource.Candidate{
			{SourceID: "source-a", ContentID: "a", Content: "same body", Confidence: 0.8},
		}, "")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{Query: "docs"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(result.Candidates))
	}
	if result.Candidates[0].SourceID != "source-b" || result.Candidates[0].ContentID != "b" {
		t.Fatalf("top candidate = %#v", result.Candidates[0])
	}
}

func TestHostSourceTimeoutIsBounded(t *testing.T) {
	host, err := New(Config{
		SourceTimeout: time.Millisecond,
		Sources:       []contextsource.Provider{fixtureSource{id: "slow", block: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{Query: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failures) != 1 || !errors.Is(result.Failures[0].Cause, context.DeadlineExceeded) {
		t.Fatalf("failures = %#v, want deadline failure", result.Failures)
	}
}

func TestHostRejectsNilErrorResultAfterSourceTimeout(t *testing.T) {
	source := &nilResultAfterTimeoutSource{
		resultReady:    make(chan struct{}),
		deadlinePassed: make(chan struct{}),
	}
	ctx := &postResultTimeoutContext{
		Context:        context.Background(),
		resultReady:    source.resultReady,
		deadlinePassed: source.deadlinePassed,
	}
	host, err := New(Config{
		SourceTimeout: 20 * time.Millisecond,
		Sources:       []contextsource.Provider{source},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(ctx, Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || len(result.Failures) != 1 || !errors.Is(result.Failures[0].Cause, context.DeadlineExceeded) {
		t.Fatalf("late nil-error result survived source timeout: %#v", result)
	}
}

func TestHostRejectsOversizedCandidate(t *testing.T) {
	host, err := New(Config{
		MaxCandidateBytes: 4,
		Sources: []contextsource.Provider{fixtureSource{id: "large", page: contextsource.NewPage([]contextsource.Candidate{
			{SourceID: "large", ContentID: "one", Content: "12345"},
		}, "")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || result.DroppedOversize != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestHostAppliesTokenBudgetAfterRanking(t *testing.T) {
	host, err := New(Config{
		TokenEstimator: func(text string) int { return len(text) },
		Sources: []contextsource.Provider{fixtureSource{id: "docs", page: contextsource.NewPage([]contextsource.Candidate{
			{SourceID: "docs", ContentID: "high", Content: "1234", Confidence: 0.9},
			{SourceID: "docs", ContentID: "low", Content: "5678", Confidence: 0.4},
		}, "")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{TokenBudget: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].ContentID != "high" || result.Tokens != 4 {
		t.Fatalf("budgeted result = %#v", result)
	}
}

func TestHostRedactsSecretsAndBuildsStableProvenance(t *testing.T) {
	candidate := contextsource.Candidate{
		SourceID: "docs", ContentID: "secret", Version: "v2", Content: "token sk-test-12345678901234567890", Confidence: 0.5,
	}
	host, err := New(Config{Sources: []contextsource.Provider{fixtureSource{id: "docs", page: contextsource.NewPage([]contextsource.Candidate{candidate}, "")}}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Candidates) != 1 || first.Candidates[0].Content == candidate.Content {
		t.Fatalf("secret content was not redacted: %#v", first.Candidates)
	}
	if first.Candidates[0].ProvenanceID == "" || first.Candidates[0].ProvenanceID != second.Candidates[0].ProvenanceID {
		t.Fatalf("unstable provenance: %#v vs %#v", first.Candidates[0], second.Candidates[0])
	}
}

func TestHostRejectsProviderIdentitySpoof(t *testing.T) {
	host, err := New(Config{Sources: []contextsource.Provider{fixtureSource{id: "trusted", page: contextsource.NewPage([]contextsource.Candidate{
		{SourceID: "other", ContentID: "one", Content: "body"},
	}, "")}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || result.DroppedInvalid != 1 {
		t.Fatalf("identity spoof result = %#v", result)
	}
}

func TestHostRejectsNonFiniteAndUnboundedCandidateFields(t *testing.T) {
	cases := []struct {
		name       string
		candidate  contextsource.Candidate
		wantDrop   bool
		wantSource string
	}{
		{name: "nan confidence", candidate: contextsource.Candidate{SourceID: "docs", ContentID: "nan", Content: "body", Confidence: math.NaN()}, wantDrop: true},
		{name: "infinite confidence", candidate: contextsource.Candidate{SourceID: "docs", ContentID: "inf", Content: "body", Confidence: math.Inf(1)}, wantDrop: true},
		{name: "newline content id", candidate: contextsource.Candidate{SourceID: "docs", ContentID: "bad\nname", Content: "body", Confidence: 0.5}, wantDrop: true},
		{name: "oversized content id", candidate: contextsource.Candidate{SourceID: "docs", ContentID: strings.Repeat("c", maxContentIDBytes+1), Content: "body", Confidence: 0.5}, wantDrop: true},
		{name: "oversized metadata", candidate: contextsource.Candidate{SourceID: "docs", ContentID: "metadata", Content: "body", Confidence: 0.5, Metadata: map[string]string{"description": strings.Repeat("x", 4096)}}, wantDrop: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, err := New(Config{Sources: []contextsource.Provider{fixtureSource{id: "docs", page: contextsource.NewPage([]contextsource.Candidate{tc.candidate}, "")}}})
			if err != nil {
				t.Fatal(err)
			}
			result, err := host.Query(context.Background(), Request{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantDrop && (len(result.Candidates) != 0 || result.DroppedInvalid != 1) {
				t.Fatalf("invalid candidate was accepted: %#v", result)
			}
		})
	}

	longID := strings.Repeat("s", 1024)
	if _, err := New(Config{Sources: []contextsource.Provider{fixtureSource{id: longID}}}); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("oversized source id error = %v, want ErrInvalidSource", err)
	}
}

func TestHostRejectsLiteralWhitespaceIdentifiersAndMetadataKeys(t *testing.T) {
	host, err := New(Config{Sources: []contextsource.Provider{fixtureSource{id: "docs", page: contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "docs", ContentID: " guide ", Content: "body", Metadata: map[string]string{" key ": "value"},
	}}, "")}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || result.DroppedInvalid != 1 {
		t.Fatalf("literal invalid fields were normalized or accepted: %#v", result)
	}
}

func TestHostRedactionCannotExpandPastFinalBudget(t *testing.T) {
	host, err := New(Config{Sources: []contextsource.Provider{fixtureSource{id: "docs", page: contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "docs", ContentID: "secret", Content: "token sk-test-12345678901234567890", Confidence: 0.5,
	}}, "")}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{
		ByteBudget:     8,
		CandidateBytes: func(candidate Candidate) int { return len(candidate.Content) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || result.DroppedBudget != 1 || result.Bytes > 8 {
		t.Fatalf("redacted candidate escaped final budget: %#v", result)
	}
}

func TestHostBoundsEachProviderPageBeforeCollection(t *testing.T) {
	items := make([]contextsource.Candidate, 128)
	for i := range items {
		items[i] = contextsource.Candidate{SourceID: "docs", ContentID: fmt.Sprintf("item-%d", i), Content: fmt.Sprintf("body-%d", i), Confidence: 0.5}
	}
	source := &limitRecordingSource{id: "docs", page: contextsource.NewPage(items, "next")}
	host, err := New(Config{MaxCandidates: 3, Sources: []contextsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if source.limit != 3 {
		t.Fatalf("source limit = %d, want host candidate bound 3", source.limit)
	}
	if len(result.Candidates) != 3 || result.DroppedBudget != 125 {
		t.Fatalf("bounded page result = %#v, want 3 candidates and 125 dropped", result)
	}
}

func TestHostRejectsCandidateReturnedAfterRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	source := cancellingSource{id: "docs", cancel: cancel}
	host, err := New(Config{Sources: []contextsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(ctx, Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || len(result.Failures) != 1 || !errors.Is(result.Failures[0].Cause, context.Canceled) {
		t.Fatalf("late candidate survived cancellation: %#v", result)
	}
}

func TestFileSnapshotSourcePreservesEmptySnapshot(t *testing.T) {
	source := NewFileSnapshotSource("vivy.project-files", []domain.FileContext{{
		Path: "empty.txt", Name: "empty.txt", Size: 0, Content: []byte{},
	}})
	host, err := New(Config{Sources: []contextsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].ContentID != "empty.txt" || result.Candidates[0].Content != "" {
		t.Fatalf("empty snapshot was not preserved: %#v", result)
	}
}

func TestFileSnapshotSourceUsesLiveContext(t *testing.T) {
	source := NewFileSnapshotSource("vivy.project-files", []domain.FileContext{{
		Path: "main.go", Name: "main.go", Size: 1, Content: []byte("x"),
	}})
	host, err := New(Config{Sources: []contextsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := host.Query(ctx, Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || len(result.Failures) != 1 || !errors.Is(result.Failures[0].Cause, context.Canceled) {
		t.Fatalf("file source ignored live cancellation: %#v", result)
	}
}
