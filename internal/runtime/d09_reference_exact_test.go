package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestD09ReferenceAttachBindsExactPreviewTextAndPosition(t *testing.T) {
	for _, mutation := range []string{"text", "position", "missing"} {
		t.Run(mutation, func(t *testing.T) {
			f := newReferenceFixture(t)
			preview := f.PreviewAsOperator(t, "B", safeSelectionA())
			items := append([]domain.HistoryItem(nil), preview.Items...)
			switch mutation {
			case "text":
				items[0].Text = "forged preview text"
			case "position":
				items[0].Ref.MessageID = "a-two"
			case "missing":
				items = nil
			}
			raw, err := json.Marshal(map[string]any{"selection": preview.Selection, "expected_items": items})
			if err != nil {
				t.Fatal(err)
			}
			var request domain.ReferenceSelection
			if err := json.Unmarshal(raw, &request); err != nil {
				t.Fatal(err)
			}
			_, err = f.refs.AttachForAdmission(context.Background(), request, "B", "candidate")
			if err == nil {
				t.Fatal("changed or missing preview values were accepted")
			}
			if mutation != "missing" && !errors.Is(err, storage.ErrSourceChanged) {
				t.Fatalf("changed preview must conflict: %v", err)
			}
		})
	}
}

type d09ReferenceCommitRace struct {
	storage.ContinuityStore
	before func() error
}

func (r d09ReferenceCommitRace) CommitContinuityOperation(ctx context.Context, mutation storage.ContinuityMutation) (storage.ContinuityResult, error) {
	if err := r.before(); err != nil {
		return storage.ContinuityResult{}, err
	}
	return r.ContinuityStore.CommitContinuityOperation(ctx, mutation)
}
func TestD09ReferenceAttachRechecksSourceAtCommit(t *testing.T) {
	for _, change := range []string{"delete", "rewind"} {
		t.Run(change, func(t *testing.T) {
			f := newReferenceFixture(t)
			f.createLiveRun(t, "race-run")
			preview := f.PreviewAsOperator(t, "B", safeSelectionA())
			f.refs.continuity = d09ReferenceCommitRace{ContinuityStore: f.backend, before: func() error {
				if change == "delete" {
					return f.backend.DeleteSession(context.Background(), "A")
				}
				return f.backend.RecordSessionTruncation(context.Background(), storage.SessionTruncation{SessionID: "A", CutoffMessageID: "a-safe", TailMessageID: "a-safe", Reason: storage.TruncationRewind, CreatedAt: 30})
			}}
			_, err := f.refs.Attach(f.modelCtx(t, "race-run", "race-call", "A"), domain.ReferenceSelection{Selection: preview.Selection, ExpectedItems: preview.Items})
			if !errors.Is(err, storage.ErrSourceChanged) {
				t.Fatalf("source changed before commit without conflict: %v", err)
			}
			if events := committedReferenceEvents(t, f.backend, "race-run"); len(events) != 0 {
				t.Fatal("stale source committed destination copy")
			}
		})
	}
}

type d09ReferenceVisibilitySpy struct {
	storage.TruncationStore
	calls *int
}

func (s d09ReferenceVisibilitySpy) ListViewTruncations(ctx context.Context, id domain.SessionID) ([]storage.SessionTruncation, error) {
	*s.calls++
	return s.TruncationStore.ListViewTruncations(ctx, id)
}
func TestD09ReferenceAttachAuthorizesBeforeSourceVisibilityRead(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "revoked-run")
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	calls := 0
	f.refs.truncations = d09ReferenceVisibilitySpy{TruncationStore: f.backend, calls: &calls}
	_, err := f.refs.Attach(f.modelCtx(t, "revoked-run", "revoked-call"), domain.ReferenceSelection{Selection: preview.Selection, ExpectedItems: preview.Items})
	var denied ReferenceError
	if !errors.As(err, &denied) || denied.Status != string(domain.HistoryStatusForbidden) {
		t.Fatalf("revoked source scope not denied: %v", err)
	}
	if calls != 0 {
		t.Fatalf("unauthorized source visibility was read %d times", calls)
	}
}
