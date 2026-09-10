package contexthost

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-vivy/sdk/port/contextsource"
)

type fixtureSource struct {
	id    string
	page  contextsource.Page
	err   error
	block bool
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
