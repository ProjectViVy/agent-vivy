package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"

	"github.com/ProjectViVy/laputa/garden/agentapi"
)

func recapPort(t *testing.T, b cognitivecontract.Bundle) cognitivecontract.RecapPort {
	t.Helper()
	provider, ok := b.(cognitivecontract.RecapProvider)
	if !ok || provider.Recap() == nil {
		t.Fatal("cognitive bundle is missing optional Recap provider")
	}
	return provider.Recap()
}

func TestIssue58RecapProviderOptional(t *testing.T) {
	ctx, b := openBundle(t)
	port := recapPort(t, b)
	head, err := port.ReadRecap(ctx)
	var apiErr *agentapi.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "recap_unavailable" || head != (cognitivecontract.RecapHead{}) {
		t.Fatalf("unarmed=%+v %v", head, err)
	}
	_, err = b.Prepare(ctx, cognitivecontract.PrimaryContextInput{SessionID: domain.SessionID("unnamed"), RunID: domain.RunID("run")})
	if !errors.Is(err, cognitivecontract.ErrPersonaUninitialized) {
		t.Fatalf("ordinary unnamed chat changed: %v", err)
	}
}

func TestIssue58RecapTrustedFactoryAndLifecycle(t *testing.T) {
	ctx := context.Background()
	in := testFactoryInput(t)
	calls := 0
	binding := cognitivecontract.RecapBinding{SubjectID: "organism-subject-uuid", DomainID: "subject-personal-v1"}
	var root string
	in.ResolveRecapBinding = func(_ context.Context, actualRoot string) (cognitivecontract.RecapBinding, error) {
		calls++
		root = actualRoot
		if actualRoot != filepath.Join(in.Config.DataDirectory(), "garden", "persona", "actmem") {
			t.Fatalf("owner root=%q", actualRoot)
		}
		if _, err := os.Stat(filepath.Join(in.Config.DataDirectory(), "garden", "garden.db")); !os.IsNotExist(err) {
			t.Fatalf("resolver ran after ingestion: %v", err)
		}
		return binding, nil
	}
	b, err := Open(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	port := recapPort(t, b)
	summary := cognitivecontract.RecapCommit{OperationID: "summary", Cause: "summary", Text: "exact\r\n\n# Recap\n---\n", Sources: cognitivecontract.SourceRange{Through: 9, Digest: "host-verification-receipt"}}
	first, err := port.CommitRecap(ctx, summary)
	if err != nil || first.Binding != binding || first.Text != summary.Text || calls != 1 {
		t.Fatalf("first=%+v calls=%d err=%v", first, calls, err)
	}
	cleared, err := port.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: first.Revision, OperationID: "clear", Cause: "clear"})
	if err != nil || cleared.InvalidationEpoch != 1 || cleared.CoveredThrough != 9 {
		t.Fatalf("clear=%+v %v", cleared, err)
	}
	restored, err := port.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: cleared.Revision, OperationID: "restore", Cause: "restore", RestoredFrom: first.Revision})
	if err != nil || restored.Text != first.Text || restored.Revision != 3 || restored.CoveredThrough != 9 {
		t.Fatalf("restore=%+v %v", restored, err)
	}
	replay, err := port.CommitRecap(ctx, summary)
	if err != nil || replay != first {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	receipt, found, err := port.LookupRecapOperation(ctx, "clear")
	if err != nil || !found || receipt != cleared {
		t.Fatalf("receipt=%+v %v %v", receipt, found, err)
	}
	history, err := port.ListRecapHistory(ctx, 0, 2)
	if err != nil || len(history) != 2 || history[0] != restored {
		t.Fatalf("history=%+v %v", history, err)
	}
	revision, err := port.ReadRecapRevision(ctx, 1)
	if err != nil || revision != first {
		t.Fatalf("revision=%+v %v", revision, err)
	}
	wrong := summary
	wrong.Binding = cognitivecontract.RecapBinding{SubjectID: profileID, DomainID: binding.DomainID}
	got, err := port.CommitRecap(ctx, wrong)
	var apiErr *agentapi.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "recap_scope_denied" || got != (cognitivecontract.RecapHead{}) {
		t.Fatalf("scope=%+v %v", got, err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = port.ReadRecap(ctx)
	if !errors.As(err, &apiErr) || apiErr.Code != "recap_unavailable" || got != (cognitivecontract.RecapHead{}) {
		t.Fatalf("closed=%+v %v", got, err)
	}
	in.ResolveRecapBinding = func(_ context.Context, actualRoot string) (cognitivecontract.RecapBinding, error) {
		if actualRoot != root {
			t.Fatalf("reopened path drifted: %q", actualRoot)
		}
		return binding, nil
	}
	reopened, err := Open(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err = recapPort(t, reopened).ReadRecap(ctx)
	if err != nil || got != restored {
		t.Fatalf("reopened=%+v %v", got, err)
	}
}

func TestIssue58RecapResolverFailure(t *testing.T) {
	in := testFactoryInput(t)
	want := errors.New("registered owner path mismatch")
	in.ResolveRecapBinding = func(context.Context, string) (cognitivecontract.RecapBinding, error) {
		return cognitivecontract.RecapBinding{}, want
	}
	b, err := Open(context.Background(), in)
	if b != nil || !errors.Is(err, want) {
		t.Fatalf("bundle=%v err=%v", b, err)
	}
}

func TestIssue58RecapErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		code, status string
		retryable    bool
	}{
		{"recap_conflict", "failed", false}, {"recap_operation_conflict", "failed", false}, {"recap_scope_denied", "failed", false},
		{"recap_legacy_nonempty", "failed", false}, {"recap_invalid_range", "failed", false},
		{"recap_unavailable", "unavailable", true}, {"recap_owner_busy", "unavailable", true}, {"recap_commit_unknown", "unknown", true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			raw, err := marshalOutcome(nil, &agentapi.Error{Code: tc.code, Message: "Recap operation failed"})
			if err != nil {
				t.Fatal(err)
			}
			var out outcome
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatal(err)
			}
			if out.Status != tc.status || out.Error == nil || out.Error.Code != tc.code || out.Error.Retryable != tc.retryable || len(out.Value) != 0 {
				t.Fatalf("outcome=%s", raw)
			}
		})
	}
}
