package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// The model's scripted invocation of present_files commits one set whose id
// travels back in the tool result; after a restart the new service instance
// lists the same committed set — no tool.finished record is required to
// recover it.
func TestPresentFilesToolCommitsAndRecovers(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-int")
	f.writeFile(t, "run-deliv-int", "out.bin", []byte{0xDE, 0xAD, 0xBE, 0xEF})

	tool := tools.NewPresentFiles(f.svc)
	out, err := tool.InvokableRun(f.modelCtx("run-deliv-int", "call-int-1"), json.RawMessage(`{"files":[{"path":"out.bin","description":"bin"}]}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	var result struct {
		SetID string `json:"set_id"`
		Items []struct {
			ID     string `json:"id"`
			SHA256 string `json:"sha256"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("tool result: %v", err)
	}
	committed, err := f.svc.Get(f.operatorCtx("B"), "B", result.SetID)
	if err != nil {
		t.Fatalf("get committed set: %v", err)
	}

	// Simulate restart after the tool.finished record was lost: a fresh
	// service instance over the same stores reconstructs every set from the
	// committed events alone.
	restarted, err := NewDeliverableService(f.manager, f.backend, f.backend, f.backend, t.TempDir())
	if err != nil {
		t.Fatalf("restarted service: %v", err)
	}
	page, err := restarted.List(f.operatorCtx("B"), "B", "", 10)
	if err != nil {
		t.Fatalf("restart list: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("restart list = %d sets, want 1", len(page.Items))
	}
	afterRestart := page.Items[0]
	if committed.ID != afterRestart.ID {
		t.Fatal("lost durable set")
	}
	if len(afterRestart.Items) != 1 || afterRestart.Items[0].SHA256 != committed.Items[0].SHA256 {
		t.Fatal("downloaded wrong bytes")
	}
}

// present_files is effectful: it reaches the service only through the
// governed adapter, so a deny profile refuses it before any commit lands.
func TestPresentFilesStaysInsideToolHost(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-pol")
	f.writeFile(t, "run-deliv-pol", "a.txt", []byte("alpha"))

	tool := tools.NewPresentFiles(f.svc)
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	ctx := withPolicyProfile(withSelectedTools(context.Background(), []string{tools.PresentFilesName}), domain.PolicyProfilePlan)
	ctx = withRunID(withSessionID(ctx, "B"), "run-deliv-pol")
	result, err := adapter.InvokableRun(ctx, `{"files":[{"path":"a.txt","description":"d"}]}`)
	if err == nil && !strings.Contains(result, "did not run") {
		t.Fatalf("bypassedPolicy: denied profile served result %q", result)
	}
	page, listErr := f.svc.List(f.operatorCtx("B"), "B", "", 10)
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	if len(page.Items) != 0 {
		t.Fatal("bypassedPolicy: denied call committed a set")
	}
}

// Sets committed by a turn a rewind marker hides leave every listing; the
// durable event stays on disk, only the view narrows.
func TestDeliverableListExcludesRewoundTurn(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-rewind")
	f.writeFile(t, "run-deliv-rewind", "a.txt", []byte("alpha"))
	if err := f.backend.AppendMessage(context.Background(), domain.Message{
		ID: "m-own", SessionID: "B", RunID: "run-deliv-rewind", Role: domain.RoleUser, CreatedAt: 30, Content: "present it",
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}
	f.svc.SetViewStores(f.backend, f.backend)
	set := presentOne(t, f, "run-deliv-rewind", "call-rw", domain.PresentFile{Path: "a.txt", Description: "d"})

	page, err := f.svc.List(f.operatorCtx("B"), "B", "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("list before rewind = %d, want 1", len(page.Items))
	}

	if err := f.backend.RecordSessionTruncation(context.Background(), storage.SessionTruncation{
		SessionID: "B", CutoffMessageID: "m-own", TailMessageID: "m-own",
		Reason: storage.TruncationRewind, CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("record truncation: %v", err)
	}
	page, err = f.svc.List(f.operatorCtx("B"), "B", "", 10)
	if err != nil {
		t.Fatalf("list after rewind: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatal("hidden rewound turn still listed")
	}
	if _, err := f.svc.Get(f.operatorCtx("B"), "B", set.ID); err == nil {
		t.Fatal("hidden rewound set still readable")
	}
}

// artifact_id search follows committed set/item ids inside the
// deliverables.presented event; unrelated ids disclose no metadata.
func TestDeliverableArtifactSearch(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-art")
	f.writeFile(t, "run-deliv-art", "a.txt", []byte("alpha"))
	set := presentOne(t, f, "run-deliv-art", "call-art", domain.PresentFile{Path: "a.txt", Description: "d"})
	item := set.Items[0]

	history := NewHistoryService(f.backend, f.backend)
	ctx := WithHistoryOperator(tools.WithSessionID(context.Background(), "B"))

	search := func(artifactID, taskID string) domain.HistoryPage {
		page, err := history.Search(ctx, domain.HistorySearchRequest{ArtifactID: artifactID, TaskID: taskID})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		return page
	}
	bySet := search(set.ID, "")
	if bySet.Status != string(domain.HistoryStatusOK) || len(bySet.Items) != 1 {
		t.Fatalf("artifact search by set id = %#v", bySet)
	}
	if !strings.Contains(bySet.Items[0].Text, set.ID) {
		t.Fatalf("artifact item = %q", bySet.Items[0].Text)
	}
	byItem := search(item.ID, "")
	if len(byItem.Items) != 1 {
		t.Fatalf("artifact search by item id = %d items", len(byItem.Items))
	}
	if none := search("dvs_unrelated", ""); len(none.Items) != 0 {
		t.Fatal("unrelated delivery id disclosed metadata")
	}
	foreign := WithHistoryOperator(tools.WithSessionID(context.Background(), "A"))
	page, err := history.Search(foreign, domain.HistorySearchRequest{ArtifactID: set.ID})
	if err != nil {
		t.Fatalf("foreign search: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatal("out-of-scope delivery id disclosed metadata")
	}
	if task := search("", "task-1"); task.Status != string(domain.HistoryStatusInvalidArgument) {
		t.Fatalf("task filter = %s, want unsupported_filter", task.Status)
	}
}
