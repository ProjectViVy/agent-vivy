package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/testsupport"
)

func TestCloneSessionCopiesVisibleView(t *testing.T) {
	svc, _, sink := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, svc.deps.Sessions, "sess-cl")
	appendRewindFixture(t, svc, "sess-cl")

	// Fold msg-3/msg-4 out of the view, then clone: the child must carry
	// the EFFECTIVE view only — folded rows never resurrect.
	if _, err := svc.RewindSession(ctx, "sess-cl", "msg-3"); err != nil {
		t.Fatalf("RewindSession: %v", err)
	}
	result, err := svc.CloneSession(ctx, "sess-cl", "")
	if err != nil {
		t.Fatalf("CloneSession: %v", err)
	}
	child := domain.SessionID(result.SessionID)
	messages := mustListMessages(t, svc, child)
	if len(messages) != 2 || messages[0].Content != "one" || messages[1].Content != "two" {
		t.Fatalf("clone messages = %+v, want the 2-row effective view", messages)
	}
	if messages[0].ID == "msg-1" || messages[0].SessionID != child {
		t.Fatalf("clone message = %+v, want fresh id bound to the child", messages[0])
	}
	cloned := 0
	for _, ev := range sink.snapshot() {
		if ev.Type == domain.EventSessionClonedFrom {
			cloned++
			if !strings.Contains(string(ev.Payload), `"parent_session_id":"sess-cl"`) {
				t.Fatalf("session.cloned_from payload = %s, want parent sess-cl", ev.Payload)
			}
		}
	}
	if cloned != 1 {
		t.Fatalf("session.cloned_from count = %d, want 1", cloned)
	}
}

func TestCloneEmptySession(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, svc.deps.Sessions, "sess-empty")
	result, err := svc.CloneSession(ctx, "sess-empty", "copy")
	if err != nil {
		t.Fatalf("CloneSession empty: %v", err)
	}
	if result.CopiedCount != 0 {
		t.Fatalf("CopiedCount = %d, want 0", result.CopiedCount)
	}
	if got := mustListMessages(t, svc, domain.SessionID(result.SessionID)); len(got) != 0 {
		t.Fatalf("empty clone messages = %+v, want none", got)
	}
}

func TestSessionTreeForkRewindForkEdges(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	mustCreateSession(t, svc.deps.Sessions, "sess-a")
	appendRewindFixture(t, svc, "sess-a")

	mid, err := svc.ForkSession(ctx, "sess-a", "msg-2", "")
	if err != nil {
		t.Fatalf("ForkSession mid: %v", err)
	}
	if _, err := svc.RewindSession(ctx, "sess-a", "msg-3"); err != nil {
		t.Fatalf("RewindSession: %v", err)
	}
	tail, err := svc.CloneSession(ctx, "sess-a", "")
	if err != nil {
		t.Fatalf("CloneSession: %v", err)
	}
	tree, err := svc.SessionTree(ctx)
	if err != nil {
		t.Fatalf("SessionTree: %v", err)
	}
	wantEdges := map[string]string{ // to -> from
		mid.SessionID:  "sess-a",
		tail.SessionID: "sess-a",
	}
	seen := map[string]string{}
	for _, e := range tree.Edges {
		if e.Kind != "fork" {
			t.Fatalf("edge kind = %q, want fork", e.Kind)
		}
		seen[e.To] = e.From
	}
	for to, from := range wantEdges {
		if seen[to] != from {
			t.Fatalf("edge %s→%s missing (got %+v)", from, to, tree.Edges)
		}
	}
	byID := map[string]SessionTreeNode{}
	for _, n := range tree.Nodes {
		byID[n.SessionID] = n
	}
	if byID[mid.SessionID].ForkPointMessageID != "msg-2" {
		t.Fatalf("mid node fork point = %q, want msg-2", byID[mid.SessionID].ForkPointMessageID)
	}
	if byID[tail.SessionID].ForkPointMessageID != "msg-2" {
		t.Fatalf("clone node fork point = %q, want the effective tail msg-2", byID[tail.SessionID].ForkPointMessageID)
	}
	if byID["sess-a"].ParentSessionID != "" {
		t.Fatalf("root node parent = %q, want empty", byID["sess-a"].ParentSessionID)
	}
}

func TestImportSessionPiJSONL(t *testing.T) {
	svc, _, sink := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	data, err := os.ReadFile(filepath.Join("testdata", "pi-session.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := svc.ImportSession(ctx, string(data), "")
	if err != nil {
		t.Fatalf("ImportSession: %v", err)
	}
	// 4 message entries → 5 rows (assistant text + tool-call split);
	// skipped: model_change + usage + label + the unparseable line +
	// the dropped thinking block = 5.
	if result.Imported != 5 || result.Skipped != 5 {
		t.Fatalf("ImportSession = %+v, want imported=5 skipped=5", result)
	}
	messages := mustListMessages(t, svc, domain.SessionID(result.SessionID))
	roles := []struct {
		role    domain.Role
		content string
		tool    string
	}{
		{domain.RoleUser, "hello <world>", ""},
		{domain.RoleAssistant, "I'll look.", ""},
		{domain.RoleAssistant, "", "read"},
		{domain.RoleTool, "package a", "read"},
		{domain.RoleAssistant, "done & dusted", ""},
	}
	if len(messages) != len(roles) {
		t.Fatalf("imported rows = %+v", messages)
	}
	for i, want := range roles {
		got := messages[i]
		if got.Role != want.role || got.Content != want.content || got.ToolName != want.tool {
			t.Fatalf("row %d = %+v, want role=%s content=%q tool=%q", i, got, want.role, want.content, want.tool)
		}
	}
	if messages[2].ToolCallID != "call_1" || !strings.Contains(string(messages[2].ToolArgs), `"a.go"`) {
		t.Fatalf("tool-call row = %+v, want call_1/read args", messages[2])
	}
	if messages[3].ToolCallID != "call_1" {
		t.Fatalf("tool result call id = %q, want call_1", messages[3].ToolCallID)
	}
	session, err := svc.deps.Sessions.GetSession(ctx, domain.SessionID(result.SessionID))
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if session.Title != "Imported session c7f3a2b1" {
		t.Fatalf("title = %q, want source-id labeled default", session.Title)
	}
	imported := 0
	for _, ev := range sink.snapshot() {
		if ev.Type == domain.EventSessionImported {
			imported++
			if !strings.Contains(string(ev.Payload), `"source":"pi-jsonl"`) {
				t.Fatalf("session.imported payload = %s", ev.Payload)
			}
		}
	}
	if imported != 1 {
		t.Fatalf("session.imported count = %d, want 1", imported)
	}
}

func TestImportSessionMalformedHeader(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	ctx := context.Background()
	if _, err := svc.ImportSession(ctx, `{"type":"message","message":{"role":"user","content":"x"}}`, ""); !errors.Is(err, ErrImportMalformed) {
		t.Fatalf("import w/o header = %v, want ErrImportMalformed", err)
	}
	if _, err := svc.ImportSession(ctx, "garbage", ""); !errors.Is(err, ErrImportMalformed) {
		t.Fatalf("import garbage = %v, want ErrImportMalformed", err)
	}
}

func TestExportSessionHTML(t *testing.T) {
	svc, _, _ := newTestService(t, testsupport.NewEchoModel())
	svc.deps.ExportDir = t.TempDir()
	ctx := context.Background()
	mustCreateSession(t, svc.deps.Sessions, "sess-ex")
	appendRewindFixture(t, svc, "sess-ex")
	if _, err := svc.RewindSession(ctx, "sess-ex", "msg-4"); err != nil {
		t.Fatalf("RewindSession: %v", err)
	}
	// Rows appended after the tail anchor stay visible (edit-flow retry).
	if err := svc.deps.Messages.AppendMessage(ctx, domain.Message{
		ID: "msg-tc", SessionID: "sess-ex", Role: domain.RoleAssistant,
		ToolCallID: "call_9", ToolName: "bash", ToolArgs: []byte(`{"cmd":"<img src=x>"}`), CreatedAt: 9,
	}); err != nil {
		t.Fatalf("AppendMessage tool call: %v", err)
	}
	result, err := svc.ExportSession(ctx, "sess-ex", "html")
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}
	// The export renders the VISIBLE view: msg-4 folded out.
	if result.MessageCount != 4 {
		t.Fatalf("MessageCount = %d, want 4", result.MessageCount)
	}
	body, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	html := string(body)
	for _, want := range []string{
		"default-src 'none'",
		"one", "two", "three",
		"tool call: bash",
		"&lt;img src=x&gt;", // escaped tool args
		"4 messages (2 user, 2 assistant, 0 tool)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("export missing %q", want)
		}
	}
	for _, banned := range []string{"<script", "http://", "https://", "four"} {
		if strings.Contains(html, banned) {
			t.Fatalf("export contains %q (folded rows and external refs must not appear)", banned)
		}
	}
	if _, err := svc.ExportSession(ctx, "sess-ex", "pdf"); err == nil {
		t.Fatalf("export format=pdf = nil, want unsupported-format error")
	}
}
