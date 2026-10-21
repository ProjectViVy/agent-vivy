package runtime

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
	"context"
	"errors"
	"os"
	"testing"
)

func TestD09ReferenceReceiptReplaysBeforeBudgetAndSourceReads(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "receipt-run")
	limits := domain.DefaultContinuityLimits()
	limits.ReferencesPerTask = 1
	f.refs.SetLimits(limits)
	ctx := f.modelCtx(t, "receipt-run", "receipt-call", "A")
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	selection := domain.ReferenceSelection{Selection: preview.Selection, ExpectedItems: preview.Items}
	first, err := f.refs.Attach(ctx, selection)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.refs.Attach(ctx, selection)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("budget prevented receipt replay: %s %v", replay.ID, err)
	}
	changed := selection
	changed.Selection = safeSelectionA()
	changed.Selection.Refs[0].MessageID = "a-two"
	if _, err := f.refs.Attach(ctx, changed); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed request ID input must conflict: %v", err)
	}
	if err := f.backend.DeleteSession(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	replay, err = f.refs.Attach(ctx, selection)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("committed destination copy did not replay: %s %v", replay.ID, err)
	}
}

func TestD09PresentationReceiptReplaysWithoutWorkspace(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "receipt-run")
	f.writeFile(t, "receipt-run", "out.txt", []byte("first presentation"))
	ctx := f.modelCtx("receipt-run", "receipt-call")
	request := domain.PresentRequest{Files: []domain.PresentFile{{Path: "out.txt"}}}
	first, err := f.svc.Present(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.workspaceOf("receipt-run")); err != nil {
		t.Fatal(err)
	}
	replay, err := f.svc.Present(ctx, request)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("receipt depended on removed workspace: %s %v", replay.ID, err)
	}
	request.Title = "changed intention"
	if _, err := f.svc.Present(ctx, request); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed presentation input must conflict: %v", err)
	}
}

func TestD09AdmissionReceiptReplaysWhileOriginalRunActive(t *testing.T) {
	svc, backend, _ := newContinuityService(t, blockingModel{})
	mustCreateSession(t, backend, "active-receipt")
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
	options := RunOptions{Continuity: &domain.ContinuityInput{RequestID: "active-retry"}}
	first, err := svc.RunWithOptions(context.Background(), "active-receipt", "same task", options)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := svc.RunWithOptions(context.Background(), "active-receipt", "same task", options)
	if err != nil || replay != first {
		t.Fatalf("active-run gate hid receipt: %s %v", replay, err)
	}
	explicit := options
	explicit.Mode = domain.RunModeNormal
	explicit.Face = domain.FaceWeb
	explicit.Thinking = domain.ThinkingModeAuto
	replay, err = svc.RunWithOptions(context.Background(), "active-receipt", "same task", explicit)
	if err != nil || replay != first {
		t.Fatalf("equivalent normalized defaults conflict: %s %v", replay, err)
	}
}
func TestD09AdmissionReceiptReplaysBeforeChangedBudget(t *testing.T) {
	svc, backend, _ := newContinuityService(t, testsupport.NewEchoModel())
	mustCreateSession(t, backend, "budget-receipt")
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
	options := RunOptions{Continuity: &domain.ContinuityInput{RequestID: "budget-retry"}}
	first, err := svc.RunWithOptions(context.Background(), "budget-receipt", "same task", options)
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, first, domain.RunCompleted)
	svc.WaitIdle(context.Background())
	svc.deps.Budget.MaxEvents = -1
	replay, err := svc.RunWithOptions(context.Background(), "budget-receipt", "same task", options)
	if err != nil || replay != first {
		t.Fatalf("new-admission budget hid receipt: %s %v", replay, err)
	}
}
func TestD09AdmissionReceiptBindsCallerSelectedOptions(t *testing.T) {
	svc, backend, _ := newContinuityService(t, testsupport.NewEchoModel())
	mustCreateSession(t, backend, "options-receipt")
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
	options := RunOptions{Continuity: &domain.ContinuityInput{RequestID: "options-retry"}}
	first, err := svc.RunWithOptions(context.Background(), "options-receipt", "same task", options)
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, first, domain.RunCompleted)
	svc.WaitIdle(context.Background())
	for _, change := range []struct {
		name string
		edit func(*RunOptions)
	}{
		{"mode", func(o *RunOptions) { o.Mode = domain.RunModePlan }},
		{"file_context", func(o *RunOptions) {
			o.FileContexts = []domain.FileContext{{Path: "notes.txt", Content: []byte("new text"), Size: 8}}
		}},
		{"attachment", func(o *RunOptions) {
			o.Attachments = []domain.Attachment{{Name: "image.png", MimeType: "image/png", Data: []byte("different image")}}
		}},
		{"provenance", func(o *RunOptions) { o.Provenance = &domain.Provenance{Source: domain.SourceHeadless} }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := options
			change.edit(&changed)
			if _, err := svc.RunWithOptions(context.Background(), "options-receipt", "same task", changed); !errors.Is(err, storage.ErrConflict) {
				t.Fatalf("changed admitted option did not conflict: %v", err)
			}
		})
	}
}

func TestD09AdmissionReceiptDefaultProfileIsCallerDeclaration(t *testing.T) {
	svc, backend, _ := newContinuityService(t, testsupport.NewEchoModel())
	mustCreateSession(t, backend, "default-receipt")
	t.Cleanup(func() { svc.CancelAll(); svc.WaitIdle(context.Background()) })
	options := RunOptions{Continuity: &domain.ContinuityInput{RequestID: "default-retry"}}
	first, err := svc.RunWithOptions(context.Background(), "default-receipt", "same task", options)
	if err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, backend, first, domain.RunCompleted)
	svc.WaitIdle(context.Background())
	svc.defaultProfile = domain.PolicyProfileReadOnly
	replay, err := svc.RunWithOptions(context.Background(), "default-receipt", "same task", options)
	if err != nil || replay != first {
		t.Fatalf("changed server default rewrote caller intent: %s %v", replay, err)
	}
	explicit := options
	explicit.Profile = domain.PolicyProfileReadOnly
	if _, err := svc.RunWithOptions(context.Background(), "default-receipt", "same task", explicit); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed explicit caller profile did not conflict: %v", err)
	}
}
