package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		wantRoot, err := filepath.EvalSymlinks(filepath.Join(in.Config.DataDirectory(), "garden", "persona", "actmem"))
		if err != nil {
			return cognitivecontract.RecapBinding{}, fmt.Errorf("resolve expected owner root: %w", err)
		}
		if actualRoot != wantRoot {
			return cognitivecontract.RecapBinding{}, fmt.Errorf("owner root=%q want=%q", actualRoot, wantRoot)
		}
		if _, err := os.Stat(filepath.Join(in.Config.DataDirectory(), "garden", "garden.db")); !os.IsNotExist(err) {
			return cognitivecontract.RecapBinding{}, fmt.Errorf("resolver ran after ingestion: %v", err)
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
			return cognitivecontract.RecapBinding{}, fmt.Errorf("reopened path drifted: %q want=%q", actualRoot, root)
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

// Exercise both the cognitive port and Garden's bound facade against the same
// real owner. A pre-reconcile owner rejects the first correction below.
func TestIssue58RecapBoundReconcile(t *testing.T) {
	for _, coverage := range []uint64{0, 9} {
		t.Run(fmt.Sprint(coverage), func(t *testing.T) {
			ctx := context.Background()
			in := testFactoryInput(t)
			binding := cognitivecontract.RecapBinding{SubjectID: "organism-subject-uuid", DomainID: "subject-personal-v1"}
			in.ResolveRecapBinding = func(context.Context, string) (cognitivecontract.RecapBinding, error) {
				return binding, nil
			}
			b, err := Open(ctx, in)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			port := recapPort(t, b)
			commit := func(request cognitivecontract.RecapCommit) cognitivecontract.RecapHead {
				t.Helper()
				head, err := port.CommitRecap(ctx, request)
				if err != nil {
					var apiErr *agentapi.Error
					if errors.As(err, &apiErr) {
						t.Fatalf("%s: code=%s head=%+v err=%v", request.OperationID, apiErr.Code, head, err)
					}
					t.Fatalf("%s: %+v %v", request.OperationID, head, err)
				}
				return head
			}
			assertError := func(request cognitivecontract.RecapCommit, code string) {
				t.Helper()
				head, err := port.CommitRecap(ctx, request)
				var apiErr *agentapi.Error
				if !errors.As(err, &apiErr) || apiErr.Code != code || head != (cognitivecontract.RecapHead{}) {
					t.Fatalf("want %s without head: %+v %v", code, head, err)
				}
			}
			base := cognitivecontract.RecapHead{Binding: binding}
			if coverage != 0 {
				base = commit(cognitivecontract.RecapCommit{OperationID: "summary", Cause: "summary", Text: "历史原文", Sources: cognitivecontract.SourceRange{Through: coverage, Digest: "historical-plain-receipt"}})
				base = commit(cognitivecontract.RecapCommit{BaseRevision: base.Revision, OperationID: "initial-clear", Cause: "clear"})
			}
			text := "  用户纠正：日期是十八日。\r\n\r\n## 自由标题\r\n---\r\n```\r\n承诺原文\r\n```\r\n\r\n"
			receipt := " {\"v\":99,\"dispositions\":\"已更正\",\"correction_evidence_digest\":\"opaque\"}\r\n"
			request := cognitivecontract.RecapCommit{BaseRevision: base.Revision, OperationID: "reconcile", Cause: "reconcile", Text: text, Sources: cognitivecontract.SourceRange{After: coverage, Through: coverage, Digest: receipt}}
			first := commit(request) // Omitted binding is stamped by the bound owner.
			if first.Binding != binding || first.Text != text || first.Sources != request.Sources || first.Cause != "reconcile" || first.CoveredThrough != coverage || first.InvalidationEpoch != base.InvalidationEpoch || first.Revision != base.Revision+1 || first.RestoredFrom != 0 {
				t.Fatalf("reconcile changed exact receipt or progress: %+v", first)
			}
			equal := request
			equal.Binding = binding
			if got := commit(equal); got != first {
				t.Fatalf("equal binding did not replay omitted binding: %+v", got)
			}
			for _, foreign := range []cognitivecontract.RecapBinding{
				{SubjectID: "foreign-subject", DomainID: binding.DomainID},
				{SubjectID: binding.SubjectID, DomainID: "foreign-domain"},
			} {
				wrong := request
				wrong.Binding = foreign
				assertError(wrong, "recap_scope_denied")
			}
			changed := request
			changed.Sources.Digest += " "
			assertError(changed, "recap_operation_conflict")
			invalid := request
			invalid.OperationID = "invalid-range"
			invalid.BaseRevision = first.Revision
			invalid.Sources.Through++
			assertError(invalid, "recap_invalid_range")
			current := first
			for _, cause := range []string{"edit", "clear", "restore"} {
				if cause != "edit" {
					next := request
					next.Binding = binding
					next.BaseRevision = current.Revision
					next.OperationID = "certificate-before-" + cause
					previous := current
					current = commit(next)
					if current.Sources != request.Sources || current.Text != text || current.CoveredThrough != coverage || current.InvalidationEpoch != previous.InvalidationEpoch {
						t.Fatalf("reconcile before %s changed receipt or progress: %+v", cause, current)
					}
				}
				mutation := cognitivecontract.RecapCommit{BaseRevision: current.Revision, OperationID: cause, Cause: cause}
				if cause == "edit" {
					mutation.Text = "手工修改\r\n"
				}
				if cause == "restore" {
					mutation.RestoredFrom = first.Revision
				}
				previous := current
				current = commit(mutation)
				wantEpoch := previous.InvalidationEpoch
				if cause == "clear" {
					wantEpoch++
				}
				if current.Sources != (cognitivecontract.SourceRange{}) || current.CoveredThrough != coverage || current.InvalidationEpoch != wantEpoch || current.Revision != previous.Revision+1 {
					t.Fatalf("%s retained certificate or changed progress: %+v", cause, current)
				}
				if cause == "edit" && current.Text != mutation.Text || cause == "clear" && current.Text != "" || cause == "restore" && (current.Text != text || current.RestoredFrom != first.Revision) {
					t.Fatalf("%s changed text semantics: %+v", cause, current)
				}
				if got := commit(request); got != first {
					t.Fatalf("%s lost original receipt: %+v", cause, got)
				}
			}
			got, found, err := port.LookupRecapOperation(ctx, request.OperationID)
			if err != nil || !found || got != first {
				t.Fatalf("lookup=%+v %v %v", got, found, err)
			}
			got, err = port.ReadRecapRevision(ctx, first.Revision)
			if err != nil || got != first {
				t.Fatalf("revision=%+v %v", got, err)
			}
			history, err := port.ListRecapHistory(ctx, 0, 20)
			if err != nil || len(history) != int(current.Revision) || history[0] != current {
				t.Fatalf("history=%+v %v", history, err)
			}
			if err := b.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, in)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			got, err = recapPort(t, reopened).ReadRecap(ctx)
			if err != nil || got != current {
				t.Fatalf("reopen=%+v %v", got, err)
			}
		})
	}
}
