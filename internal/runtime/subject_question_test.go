package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/conformance"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func issue58QuestionService(t *testing.T, b *sqlite.Backend, source *storage.SubjectSourceBinding, chat model.ToolCallingChatModel) *Service {
	t.Helper()
	checkpoints, err := NewVersionedCheckpointStore(b.Blobs(), "issue58-question")
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(context.Background(), chat, []tools.Tool{tools.NewAskUser()}, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10, MaxToolResultBytes: 128, Checkpoints: checkpoints,
	})
	if err != nil {
		t.Fatal(err)
	}
	deps := ServiceDeps{Journal: b, Runs: b, Messages: b, Sessions: b, Questions: b, ApprovalExpiration: 5 * time.Minute, Sink: newTestSink()}
	if source != nil {
		deps.SubjectSources = func(context.Context, domain.SessionID) (*storage.SubjectSourceBinding, error) { return source, nil }
	}
	return NewService(engine, "scripted", "scripted-v0", deps)
}

func TestIssue58RetainedQuestionRestartKeepsExactAnswer(t *testing.T) {
	for _, retained := range []bool{true, false} {
		name := "retained"
		if !retained {
			name = "ordinary-unshared"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			path := filepath.Join(t.TempDir(), "question.db")
			b, err := sqlite.Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = b.Close() }()
			conformance.Issue58Identity(t, b)
			source := conformance.Issue58Source("question-source", false)
			mustCreateSession(t, b, source.SessionID)
			var binding *storage.SubjectSourceBinding
			if retained {
				binding = &source
			}
			initial := issue58QuestionService(t, b, binding, NewQuestionFlowModel())
			rid, err := initial.Run(ctx, source.SessionID, "ask for an exact answer")
			if err != nil {
				t.Fatal(err)
			}
			question := waitForPendingQuestion(t, b, rid)
			if !initial.WaitIdle(ctx) {
				t.Fatal("question did not suspend")
			}
			if _, err := b.GetToolOperation(ctx, rid, QuestionFlowCallID); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("pending question invented an answer receipt: %v", err)
			}
			if head, err := b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding); err != nil || head != 0 {
				t.Fatalf("pending question sealed: head=%d err=%v", head, err)
			}
			if err := b.Close(); err != nil {
				t.Fatal(err)
			}
			b, err = sqlite.Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			recorder := &recordingChatModel{inner: NewScriptedModel(schema.AssistantMessage("Recovered answer.", nil), schema.AssistantMessage("Next interaction.", nil))}
			restarted := issue58QuestionService(t, b, binding, recorder)
			if err := restarted.Recover(ctx); err != nil {
				t.Fatal(err)
			}
			answer := untrustedToolResultHeader + strings.Repeat("exact human answer\n", 2000) + "-tail"
			if err := restarted.AnswerQuestion(ctx, question.ID, answer); err != nil {
				t.Fatal(err)
			}
			if !restarted.WaitIdle(ctx) {
				t.Fatal("answered question did not settle")
			}
			if run, err := b.GetRun(ctx, rid); err != nil || run.Status != domain.RunCompleted {
				t.Fatalf("native question failed: %+v %v", run, err)
			}
			stored, err := b.GetQuestion(ctx, question.ID)
			if err != nil || stored.Status != domain.QuestionAnswered || stored.Answer != answer {
				t.Fatalf("question authority lost answer: %+v %v", stored, err)
			}
			if err := restarted.AnswerQuestion(ctx, question.ID, "duplicate answer"); !errors.Is(err, ErrQuestionAlreadyAnswered) {
				t.Fatalf("question answered twice: %v", err)
			}
			op, opErr := b.GetToolOperation(ctx, rid, QuestionFlowCallID)
			if retained {
				if opErr != nil || op.State != domain.ToolOperationCompleted {
					t.Fatalf("answered native question has no completed exact receipt: state=%s err=%v", op.State, opErr)
				}
				ref, err := storage.DecodeToolEvidenceReference(op)
				if err != nil {
					t.Fatal(err)
				}
				raw, found, err := b.Blobs().Get(ctx, ref.ID)
				if err != nil || !found {
					t.Fatalf("missing exact answer artifact: %v", err)
				}
				evidence, err := storage.VerifyToolEvidence(ref, raw)
				if err != nil || evidence.Result != answer || evidence.Failure != "" || ref.ResultByteCount != len(answer) || ref.ResultSHA256 != sha256Hex([]byte(answer)) {
					t.Fatalf("answer bytes/hash changed: %v", err)
				}
				request, found, err := b.Blobs().Get(ctx, storage.ToolEvidenceArgumentsID(op))
				if err != nil || !found || string(request) != `{"question":"Which color should I use?"}` {
					t.Fatalf("original request lost: %q %v", request, err)
				}
				rows, interaction, err := b.ReadSubjectRunEvidence(ctx, source.SubjectHistoryBinding, rid)
				if err != nil {
					t.Fatal(err)
				}
				exact := false
				for _, row := range rows {
					if row.Role == domain.RoleTool && row.ToolCallID == QuestionFlowCallID && row.Content == answer {
						exact = true
					}
				}
				if !exact {
					t.Fatal("original reader lost exact human answer")
				}
				if err := restarted.RepairSubjectHistory(ctx, source.SessionID); err != nil {
					t.Fatal(err)
				}
				if head, err := b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding); err != nil || head != 1 {
					t.Fatalf("question not sealed exactly once: head=%d err=%v", head, err)
				}
				if _, err := b.ReadSubjectInteraction(ctx, source.SubjectHistoryBinding, interaction.ID); err != nil {
					t.Fatal(err)
				}
				t.Logf("manifest resumed question %s %s answer %s bytes %d", interaction.ID, interaction.SourceDigest, ref.ResultSHA256, ref.ResultByteCount)
			} else if !errors.Is(opErr, storage.ErrNotFound) {
				t.Fatalf("ordinary question gained retained receipt: %v", opErr)
			}
			visible := false
			for _, msg := range recorder.lastInput() {
				if msg.Role != schema.Tool {
					continue
				}
				encoded, _ := json.Marshal(msg)
				visible = true
				if len(encoded) >= len(answer) || strings.Contains(string(encoded), "exact tool result:") != retained {
					t.Fatalf("wrong answer projection/reference: %.600s", encoded)
				}
			}
			if !visible {
				t.Fatal("native resumed model did not receive an answer")
			}
			if _, err := restarted.Run(ctx, source.SessionID, "next same-domain interaction"); err != nil {
				t.Fatal(err)
			}
			if !restarted.WaitIdle(ctx) {
				t.Fatal("next interaction did not settle")
			}
			want := uint64(0)
			if retained {
				want = 2
			}
			if head, err := b.CaptureSubjectHistory(ctx, source.SubjectHistoryBinding); err != nil || head != want {
				t.Fatalf("answered question blocked next seal: head=%d want=%d err=%v", head, want, err)
			}
			if events := replayAll(t, b, rid); countTerminal(events) != 1 {
				t.Fatalf("native question emitted duplicate terminal events: %d", countTerminal(events))
			}
		})
	}
}
