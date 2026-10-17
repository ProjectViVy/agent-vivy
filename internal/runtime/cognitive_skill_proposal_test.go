package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProjectViVy/laputa/laputa/evolution"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func newCognitiveSkillProposalFixture(t *testing.T) (*Service, *sqlite.Backend, *EinoSkillBackend, evolution.Effect, domain.RunID, string) {
	t.Helper()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "cognitive-skills.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.CreateSession(ctx, domain.Session{ID: "sess-diva-skill", Title: "DIVA skill review", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	runID := domain.RunID("run-diva-skill")
	if err := store.CreateRun(ctx, domain.Run{ID: runID, SessionID: "sess-diva-skill", Status: domain.RunCompleted, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "skills")
	writeSkillFixture(t, root, "release-notes", "Keep the release summary concise.")
	skills, err := NewEinoSkillBackend(root, store)
	if err != nil {
		t.Fatal(err)
	}
	view, err := skills.ViewSkill(ctx, runID, "release-notes", "")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(nil, "", "", ServiceDeps{Approvals: store, Skills: skills, ApprovalExpiration: time.Hour})
	payload := &evolution.SkillProposalPayload{
		Operation: evolution.SkillProposalUpdate, SkillName: "release-notes", BaseHash: view.Hash,
		ProposedContent: "---\nname: release-notes\ndescription: A test skill\n---\n\nKeep release summaries concise.",
		Rationale:       "The same release workflow recurred.", Sources: []evolution.SourceRef{{SourceID: "activity", RecordID: "event-1", Revision: 1,
			Scope: evolution.Scope{SubjectID: "profile-1", Kind: evolution.ScopePersonal}}},
	}
	effect, err := evolution.NewEffect("diva-op-1", evolution.KindSkillProposal, payload, evolution.Scope{SubjectID: "profile-1", Kind: evolution.ScopePersonal}, "local")
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, skills, effect, runID, root
}

func TestCognitiveSkillProposalReplayRejectExpiryAndRestart(t *testing.T) {
	t.Run("replay produces one pending revision and denial leaves file untouched", func(t *testing.T) {
		ctx := context.Background()
		svc, store, skills, effect, runID, root := newCognitiveSkillProposalFixture(t)
		first, err := svc.submitCognitiveSkillProposal(ctx, runID, skills, effect)
		if err != nil || first.Status != evolution.StatusSubmitted {
			t.Fatalf("submit = %+v, %v", first, err)
		}
		second, err := svc.submitCognitiveSkillProposal(ctx, runID, skills, effect)
		if err != nil || second.TargetRef != first.TargetRef {
			t.Fatalf("replay = %+v, %v", second, err)
		}
		pending, err := skills.ListPendingSkillRevisions(ctx)
		if err != nil || len(pending) != 1 {
			t.Fatalf("pending revisions = %d, %v", len(pending), err)
		}
		approvalID := cognitiveSkillApprovalID(effect.SkillProposal.SkillName, effect.SkillProposal.Sources)
		if _, err := store.GetReview(ctx, approvalID); err != nil {
			t.Fatalf("Review Center item missing: %v", err)
		}
		if err := svc.DecideApproval(ctx, approvalID, domain.ApprovalDenied); err != nil {
			t.Fatal(err)
		}
		revision, err := skills.GetSkillRevision(ctx, first.TargetRef)
		if err != nil || revision.Status != domain.SkillRevisionRejected {
			t.Fatalf("revision after denial = %+v, %v", revision, err)
		}
		body, err := os.ReadFile(filepath.Join(root, "release-notes", "SKILL.md"))
		if err != nil || string(body) == effect.SkillProposal.ProposedContent {
			t.Fatalf("denial changed file: %v", err)
		}
	})

	t.Run("expired review closes the revision", func(t *testing.T) {
		ctx := context.Background()
		svc, store, skills, effect, runID, _ := newCognitiveSkillProposalFixture(t)
		receipt, err := svc.submitCognitiveSkillProposal(ctx, runID, skills, effect)
		if err != nil {
			t.Fatal(err)
		}
		approvalID := cognitiveSkillApprovalID(effect.SkillProposal.SkillName, effect.SkillProposal.Sources)
		approval, err := store.GetApproval(ctx, approvalID)
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.expireApproval(ctx, approval, "test timeout"); err != nil {
			t.Fatal(err)
		}
		revision, err := skills.GetSkillRevision(ctx, receipt.TargetRef)
		if err != nil || revision.Status != domain.SkillRevisionExpired {
			t.Fatalf("revision after expiry = %+v, %v", revision, err)
		}
	})

	t.Run("stale base hash is terminal", func(t *testing.T) {
		ctx := context.Background()
		svc, store, skills, effect, runID, root := newCognitiveSkillProposalFixture(t)
		receipt, err := svc.submitCognitiveSkillProposal(ctx, runID, skills, effect)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "release-notes", "SKILL.md"), []byte("---\nname: release-notes\ndescription: changed\n---\n\nExternal edit."), 0o600); err != nil {
			t.Fatal(err)
		}
		approvalID := cognitiveSkillApprovalID(effect.SkillProposal.SkillName, effect.SkillProposal.Sources)
		if err := svc.DecideApproval(ctx, approvalID, domain.ApprovalApproved); err == nil {
			t.Fatal("stale revision was approved")
		}
		approval, err := store.GetApproval(ctx, approvalID)
		if err != nil || approval.Decision != domain.ApprovalStale {
			t.Fatalf("approval after stale check = %+v, %v", approval, err)
		}
		revision, err := skills.GetSkillRevision(ctx, receipt.TargetRef)
		if err != nil || revision.Status != domain.SkillRevisionStale {
			t.Fatalf("revision after stale check = %+v, %v", revision, err)
		}
	})

	t.Run("restart preserves pending review and applies an approved one", func(t *testing.T) {
		ctx := context.Background()
		svc, store, skills, effect, runID, root := newCognitiveSkillProposalFixture(t)
		receipt, err := svc.submitCognitiveSkillProposal(ctx, runID, skills, effect)
		if err != nil {
			t.Fatal(err)
		}
		approvalID := cognitiveSkillApprovalID(effect.SkillProposal.SkillName, effect.SkillProposal.Sources)
		restarted := NewService(nil, "", "", ServiceDeps{Approvals: store, Skills: skills})
		if err := restarted.recoverCognitiveSkillRevisions(ctx); err != nil {
			t.Fatal(err)
		}
		pending, err := store.GetApproval(ctx, approvalID)
		if err != nil || pending.Decision != domain.ApprovalPending {
			t.Fatalf("pending review lost on restart: %+v, %v", pending, err)
		}
		if _, err := store.DecideApproval(ctx, approvalID, domain.ApprovalApproved); err != nil {
			t.Fatal(err)
		}
		if err := restarted.recoverCognitiveSkillRevisions(ctx); err != nil {
			t.Fatal(err)
		}
		revision, err := skills.GetSkillRevision(ctx, receipt.TargetRef)
		if err != nil || revision.Status != domain.SkillRevisionApplied {
			t.Fatalf("revision after restart = %+v, %v", revision, err)
		}
		var req tools.SkillManageRequest
		if err := json.Unmarshal(revision.Payload, &req); err != nil || req.Evolution == nil {
			t.Fatalf("revision provenance lost: %+v, %v", req, err)
		}
		body, err := os.ReadFile(filepath.Join(root, "release-notes", "SKILL.md"))
		if err != nil || string(body) != effect.SkillProposal.ProposedContent {
			t.Fatalf("recovered file = %q, %v", body, err)
		}
	})
}
