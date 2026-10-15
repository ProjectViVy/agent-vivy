package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

func TestIssue58RecapCheckpointResume(t *testing.T) {
	for _, kind := range []string{"same-process", "restart", "clear", "revoked"} {
		t.Run(kind, func(t *testing.T) {
			f := newIssue58Wire(t, tools.NewAskUser())
			row := f.completed(t, "source-A", "resume-A")
			issue58SeedSummary(t, f, row, "")
			f.reply = func(i int) *schema.Message {
				if i == 0 {
					return issue58ToolReply(tools.AskUserName, `{"question":"fixture question"}`)
				}
				return schema.AssistantMessage("resumed complete", nil)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			rid, err := f.svc.Run(ctx, "main-reader", "suspend once")
			if err != nil {
				t.Fatal(err)
			}
			question := waitForPendingQuestion(t, f.b, rid)
			if !f.svc.WaitIdle(ctx) {
				t.Fatal("suspend did not settle")
			}
			if len(f.capture()) != 1 {
				t.Fatal("expected exactly one initial HTTP")
			}
			if kind == "restart" {
				f.restart(t)
			}
			if kind == "clear" {
				head, err := f.owner.ReadRecap(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.owner.CommitRecap(ctx, cognitivecontract.RecapCommit{BaseRevision: head.Revision, OperationID: "clear-suspended", Cause: "clear"}); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "revoked" {
				delete(f.sources, row.SessionID)
			}
			if err = f.svc.AnswerQuestion(ctx, question.ID, "exact answer"); err != nil {
				t.Fatal(err)
			}
			want := domain.RunCompleted
			if kind == "clear" || kind == "revoked" {
				want = domain.RunFailed
			}
			waitForRunStatus(t, f.b, rid, want)
			if !f.svc.WaitIdle(ctx) {
				t.Fatal("resume did not settle")
			}
			requests := f.capture()
			for _, raw := range requests {
				t.Logf("HTTP_CAPTURE %s", raw)
			}
			if want == domain.RunFailed {
				if len(requests) != 1 {
					t.Fatal("invalid resumed cut reached provider")
				}
				return
			}
			if len(requests) != 2 || !strings.Contains(requests[1], "exact answer") || strings.Count(requests[1], "suspend once") != 1 {
				t.Fatalf("resume lost/duplicated original %v", requests)
			}
			through, err := f.b.CaptureSubjectHistory(ctx, conformanceBinding())
			if err != nil || through != 2 {
				t.Fatalf("completed resumed interaction was not sealed exactly once through=%d err=%v", through, err)
			}
			original, _, err := f.b.ReadSubjectRunEvidence(ctx, conformanceBinding(), rid)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, m := range original {
				if m.Role == domain.RoleTool && m.Content == "exact answer" {
					found = true
				}
			}
			if !found {
				t.Fatal("native resume answer not retained")
			}
		})
	}
}
