package runtime

import (
	"context"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"github.com/cloudwego/eino/schema"
)

func TestIssue58LoadedRecapTracksAdmittedModelNotNewHead(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	initial, err := f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{OperationID: "loaded-seed", Cause: "edit", Text: "first loaded text"})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	f.reply = func(int) *schema.Message {
		called = true
		loaded, err := f.svc.ReadRecapLoaded(ctx, "main-reader")
		if err != nil || loaded.Status != "loaded" || loaded.Revision != initial.Revision {
			t.Errorf("actual request missing pin: %+v %v", loaded, err)
		}
		if _, err = f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: initial.Revision, OperationID: "loaded-edit", Cause: "edit", Text: "next call text"}); err != nil {
			t.Error(err)
		}
		unchanged, err := f.svc.ReadRecapLoaded(ctx, "main-reader")
		if err != nil || unchanged.Revision != initial.Revision {
			t.Errorf("new head misreported loaded: %+v %v", unchanged, err)
		}
		if _, err = f.svc.ReadRecapLoaded(ctx, "ordinary"); err == nil {
			t.Error("foreign session learned loaded revision")
		}
		return schema.AssistantMessage("done", nil)
	}
	if before, err := f.svc.ReadRecapLoaded(ctx, "main-reader"); err != nil || before.Status != "unavailable" {
		t.Fatalf("invented initial state: %+v %v", before, err)
	}
	if _, err = f.svc.Run(ctx, "main-reader", "inspect loaded revision"); err != nil {
		t.Fatal(err)
	}
	if !f.svc.WaitIdle(ctx) {
		t.Fatal("not idle")
	}
	if !called {
		t.Fatal("actual model request absent")
	}
	if after, err := f.svc.ReadRecapLoaded(ctx, "main-reader"); err != nil || after.Status != "unavailable" {
		t.Fatalf("completed run retained loaded state: %+v %v", after, err)
	}
}
