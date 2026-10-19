package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ProjectViVy/laputa/laputa/evolution"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

const cognitiveSkillProposalTool = "diva.skill_proposal"

type cognitiveSkillApprovalData struct {
	RevisionID    string `json:"revision_id"`
	SkillName     string `json:"skill_name"`
	OperationID   string `json:"operation_id"`
	PayloadDigest string `json:"payload_digest"`
}

// submitCognitiveSkillProposal stores the exact reviewed bytes and a durable
// Review Center item. It never mutates a Skill while the DIVA run is active.
func (s *Service) submitCognitiveSkillProposal(ctx context.Context, runID domain.RunID, skills tools.SkillOperations, effect evolution.Effect) (evolution.EffectReceipt, error) {
	payload := effect.SkillProposal
	if payload == nil || skills == nil || s.deps.Approvals == nil {
		return evolution.EffectReceipt{}, ErrCognitiveUnavailable
	}
	lifecycle, ok := skills.(tools.SkillRevisionLifecycle)
	if !ok {
		return evolution.EffectReceipt{}, errors.New("runtime: Skill revision lifecycle unavailable")
	}
	approvalID := cognitiveSkillApprovalID(payload.SkillName, payload.Sources)
	s.skillProposalMu.Lock()
	defer s.skillProposalMu.Unlock()

	if prior, err := s.deps.Approvals.GetApproval(ctx, approvalID); err == nil {
		return s.cognitiveSkillReplayReceipt(ctx, skills, lifecycle, prior, effect)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return evolution.EffectReceipt{}, err
	}
	if existing, err := s.pendingCognitiveSkillProposal(ctx, payload.SkillName); err != nil {
		return evolution.EffectReceipt{}, err
	} else if existing != nil {
		return evolution.EffectReceipt{OperationID: effect.OperationID, PayloadDigest: effect.PayloadDigest,
			Status: evolution.StatusRejected, ErrorCode: "skill_review_pending"}, nil
	}

	if err := s.validateCognitiveSkillTarget(ctx, skills, runID, *payload); err != nil {
		return evolution.EffectReceipt{OperationID: effect.OperationID, PayloadDigest: effect.PayloadDigest,
			Status: evolution.StatusRejected, ErrorCode: "skill_target_conflict"}, nil
	}
	evidence, err := json.Marshal(payload.Sources)
	if err != nil {
		return evolution.EffectReceipt{}, err
	}
	action := string(payload.Operation)
	if payload.Operation == evolution.SkillProposalUpdate {
		action = "edit"
	}
	req := tools.SkillManageRequest{
		Action: action, SkillName: payload.SkillName, Content: payload.ProposedContent,
		Evolution: &tools.SkillEvolutionMetadata{Origin: "diva", OperationID: effect.OperationID,
			ApprovalID: approvalID, Rationale: payload.Rationale, SourceEvidence: evidence},
	}
	proposal, err := skills.PrepareSkillProposal(ctx, runID, req)
	if err != nil {
		return evolution.EffectReceipt{}, fmt.Errorf("runtime: stage DIVA Skill revision: %w", err)
	}
	var ref struct {
		RevisionID string `json:"revision_id"`
	}
	if err := json.Unmarshal(proposal.Data, &ref); err != nil || ref.RevisionID == "" {
		return evolution.EffectReceipt{}, errors.New("runtime: staged DIVA Skill revision reference is invalid")
	}
	wantBase := payload.BaseHash
	if proposal.PreconditionHash != wantBase {
		_ = lifecycle.SetSkillRevisionStatus(ctx, ref.RevisionID, domain.SkillRevisionStale)
		return evolution.EffectReceipt{OperationID: effect.OperationID, PayloadDigest: effect.PayloadDigest,
			Status: evolution.StatusRejected, TargetRef: ref.RevisionID, ErrorCode: "stale_skill"}, nil
	}

	data, err := json.Marshal(cognitiveSkillApprovalData{
		RevisionID: ref.RevisionID, SkillName: payload.SkillName, OperationID: effect.OperationID,
		PayloadDigest: effect.PayloadDigest,
	})
	if err != nil {
		_ = lifecycle.SetSkillRevisionStatus(ctx, ref.RevisionID, domain.SkillRevisionFailed)
		return evolution.EffectReceipt{}, err
	}
	now := time.Now()
	approval := domain.Approval{
		ID: approvalID, RunID: runID, ToolCallID: effect.OperationID, Decision: domain.ApprovalPending,
		ExpiresAt: s.approvalDeadline(now, s.deps.ApprovalExpiration).UnixMilli(), CreatedAt: now.UnixMilli(),
		Kind: domain.ApprovalKindSkillRevision, ToolName: cognitiveSkillProposalTool,
		Action: proposal.Action, Target: proposal.Target, PreconditionHash: proposal.PreconditionHash,
		Preview: cognitiveSkillReviewPreview(*payload, proposal.Preview), RiskFindings: proposal.RiskFindings,
		ProposalData: data, SandboxMode: string(domain.SandboxModeReadOnly), ApprovalPolicy: string(domain.ApprovalPolicyAsk),
	}
	if err := s.deps.Approvals.CreateApproval(ctx, approval); err != nil {
		_ = lifecycle.SetSkillRevisionStatus(context.WithoutCancel(ctx), ref.RevisionID, domain.SkillRevisionFailed)
		if prior, getErr := s.deps.Approvals.GetApproval(ctx, approvalID); getErr == nil {
			return s.cognitiveSkillReplayReceipt(ctx, skills, lifecycle, prior, effect)
		}
		return evolution.EffectReceipt{}, fmt.Errorf("runtime: persist DIVA Skill review: %w", err)
	}
	return evolution.EffectReceipt{OperationID: effect.OperationID, PayloadDigest: effect.PayloadDigest,
		Status: evolution.StatusSubmitted, TargetRef: ref.RevisionID}, nil
}

func (s *Service) validateCognitiveSkillTarget(ctx context.Context, skills tools.SkillOperations, runID domain.RunID, proposal evolution.SkillProposalPayload) error {
	if proposal.Operation == evolution.SkillProposalUpdate {
		view, err := skills.ViewSkill(ctx, runID, proposal.SkillName, "")
		if err != nil {
			return err
		}
		if view.Origin != tools.SkillOriginUser || view.RelativePath != "SKILL.md" || view.Hash != proposal.BaseHash {
			return errors.New("runtime: DIVA Skill update target is not the reviewed writable revision")
		}
		return nil
	}
	items, err := skills.ListSkills(ctx, runID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if strings.EqualFold(item.Name, proposal.SkillName) {
			return errors.New("runtime: DIVA Skill creation duplicates an existing Skill")
		}
	}
	return nil
}

func (s *Service) pendingCognitiveSkillProposal(ctx context.Context, skillName string) (*domain.Approval, error) {
	items, err := s.deps.Approvals.ListPendingApprovals(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Kind != domain.ApprovalKindSkillRevision || items[i].ToolName != cognitiveSkillProposalTool {
			continue
		}
		var data cognitiveSkillApprovalData
		if json.Unmarshal(items[i].ProposalData, &data) == nil && strings.EqualFold(data.SkillName, skillName) {
			return &items[i], nil
		}
	}
	return nil, nil
}

func cognitiveSkillApprovalID(skillName string, sources []evolution.SourceRef) string {
	encoded, _ := json.Marshal(struct {
		SkillName string                `json:"skill_name"`
		Sources   []evolution.SourceRef `json:"sources"`
	}{SkillName: strings.ToLower(skillName), Sources: sources})
	sum := sha256.Sum256(encoded)
	return "apr_evo_" + hex.EncodeToString(sum[:16])
}

func cognitiveSkillReviewPreview(proposal evolution.SkillProposalPayload, diff string) string {
	var b strings.Builder
	b.WriteString("DIVA local Skill proposal\nRationale: ")
	b.WriteString(strings.TrimSpace(proposal.Rationale))
	b.WriteString("\nEvidence:")
	for _, source := range proposal.Sources {
		b.WriteString("\n- ")
		b.WriteString(source.SourceID)
		b.WriteByte('/')
		b.WriteString(source.RecordID)
		b.WriteString("@")
		b.WriteString(fmt.Sprint(source.Revision))
	}
	b.WriteString("\n\n")
	b.WriteString(diff)
	preview := b.String()
	if len(preview) > 32<<10 {
		preview = preview[:safeUTF8Prefix(preview, 32<<10)] + "\n[preview truncated]\n"
	}
	return preview
}

func (s *Service) cognitiveSkillReplayReceipt(ctx context.Context, skills tools.SkillOperations, lifecycle tools.SkillRevisionLifecycle, approval domain.Approval, effect evolution.Effect) (evolution.EffectReceipt, error) {
	var data cognitiveSkillApprovalData
	if err := json.Unmarshal(approval.ProposalData, &data); err != nil || data.RevisionID == "" {
		return evolution.EffectReceipt{}, errors.New("runtime: DIVA Skill approval has invalid revision data")
	}
	if data.PayloadDigest != effect.PayloadDigest {
		return evolution.EffectReceipt{OperationID: effect.OperationID, PayloadDigest: effect.PayloadDigest,
			Status: evolution.StatusRejected, ErrorCode: "idempotency_conflict"}, nil
	}
	revision, err := lifecycle.GetSkillRevision(ctx, data.RevisionID)
	if err != nil {
		return evolution.EffectReceipt{}, err
	}
	status := evolution.StatusRejected
	switch revision.Status {
	case domain.SkillRevisionApplied:
		status = evolution.StatusApplied
	case domain.SkillRevisionPending:
		if approval.Decision == domain.ApprovalApproved {
			result, applyErr := skills.ApplySkillRevision(ctx, approval.RunID, revision.ID)
			if applyErr != nil {
				terminal := domain.SkillRevisionFailed
				if errors.Is(applyErr, ErrSkillRevisionStale) {
					terminal = domain.SkillRevisionStale
				}
				_ = lifecycle.SetSkillRevisionStatus(context.WithoutCancel(ctx), revision.ID, terminal)
				return evolution.EffectReceipt{OperationID: effect.OperationID, PayloadDigest: effect.PayloadDigest,
					Status: evolution.StatusRejected, TargetRef: revision.ID, ErrorCode: string(terminal)}, nil
			}
			if result.Status == string(domain.SkillRevisionApplied) {
				status = evolution.StatusApplied
			} else {
				status = evolution.StatusSubmitted
			}
		} else {
			status = evolution.StatusSubmitted
		}
	case domain.SkillRevisionRejected, domain.SkillRevisionExpired, domain.SkillRevisionCancelled,
		domain.SkillRevisionStale, domain.SkillRevisionFailed:
		status = evolution.StatusRejected
	}
	return evolution.EffectReceipt{OperationID: effect.OperationID, PayloadDigest: effect.PayloadDigest,
		Status: status, TargetRef: revision.ID}, nil
}

func (s *Service) checkCognitiveSkillApproval(ctx context.Context, approval domain.Approval) error {
	data, lifecycle, err := s.cognitiveSkillApproval(ctx, approval)
	if err != nil {
		return err
	}
	if err := lifecycle.CheckSkillRevision(ctx, data.RevisionID); err != nil {
		_ = lifecycle.SetSkillRevisionStatus(context.WithoutCancel(ctx), data.RevisionID, domain.SkillRevisionStale)
		if approvals, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore); ok {
			_, _ = approvals.MarkApprovalStale(context.WithoutCancel(ctx), approval.ID, "Skill revision precondition no longer matches")
		}
		return fmt.Errorf("runtime: DIVA Skill revision is stale: %w", err)
	}
	return nil
}

func (s *Service) settleCognitiveSkillDecision(ctx context.Context, approval domain.Approval, decision string) error {
	if decision == domain.ApprovalDenied {
		return s.setCognitiveSkillApprovalStatus(ctx, approval, domain.SkillRevisionRejected)
	}
	if s.deps.Skills == nil {
		return errors.New("runtime: Skill authority unavailable after approval")
	}
	lifecycle, ok := s.deps.Skills.(tools.SkillRevisionLifecycle)
	if !ok {
		return errors.New("runtime: Skill revision lifecycle unavailable after approval")
	}
	data, _, err := s.cognitiveSkillApproval(ctx, approval)
	if err != nil {
		return err
	}
	result, err := s.deps.Skills.ApplySkillRevision(ctx, approval.RunID, data.RevisionID)
	if err != nil {
		status := domain.SkillRevisionFailed
		if errors.Is(err, ErrSkillRevisionStale) {
			status = domain.SkillRevisionStale
			if approvals, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore); ok {
				_, _ = approvals.MarkApprovalStale(context.WithoutCancel(ctx), approval.ID, "Skill revision precondition no longer matches")
			}
		}
		_ = lifecycle.SetSkillRevisionStatus(context.WithoutCancel(ctx), data.RevisionID, status)
		return fmt.Errorf("runtime: approved Skill revision could not be applied (%s): %w", status, err)
	}
	if result.Status != string(domain.SkillRevisionApplied) {
		return fmt.Errorf("runtime: approved Skill revision returned unexpected status %q", result.Status)
	}
	return nil
}

func (s *Service) setCognitiveSkillApprovalStatus(ctx context.Context, approval domain.Approval, status domain.SkillRevisionStatus) error {
	data, lifecycle, err := s.cognitiveSkillApproval(ctx, approval)
	if err != nil {
		return err
	}
	return lifecycle.SetSkillRevisionStatus(ctx, data.RevisionID, status)
}

func (s *Service) cognitiveSkillApproval(ctx context.Context, approval domain.Approval) (cognitiveSkillApprovalData, tools.SkillRevisionLifecycle, error) {
	var data cognitiveSkillApprovalData
	if err := json.Unmarshal(approval.ProposalData, &data); err != nil || data.RevisionID == "" {
		return data, nil, errors.New("runtime: DIVA Skill approval has invalid revision data")
	}
	if s.deps.Skills == nil {
		return data, nil, errors.New("runtime: Skill authority unavailable")
	}
	lifecycle, ok := s.deps.Skills.(tools.SkillRevisionLifecycle)
	if !ok {
		return data, nil, errors.New("runtime: Skill revision lifecycle unavailable")
	}
	return data, lifecycle, nil
}

// recoverCognitiveSkillRevisions reconciles revision status with durable Review
// decisions before ordinary run recovery. Pending reviews remain visible in
// Review Center; an approved revision interrupted before application resumes.
func (s *Service) recoverCognitiveSkillRevisions(ctx context.Context) error {
	if s.deps.Skills == nil || s.deps.Approvals == nil {
		return nil
	}
	lifecycle, ok := s.deps.Skills.(tools.SkillRevisionLifecycle)
	if !ok {
		return nil
	}
	revisions, err := lifecycle.ListPendingSkillRevisions(ctx)
	if errors.Is(err, tools.ErrSkillRevisionHistoryUnavailable) {
		return nil // DIVA local Skill proposals are unavailable in this composition.
	}
	if err != nil {
		return fmt.Errorf("runtime: list pending Skill revisions: %w", err)
	}
	for _, revision := range revisions {
		var req tools.SkillManageRequest
		if err := json.Unmarshal(revision.Payload, &req); err != nil {
			return fmt.Errorf("runtime: decode pending Skill revision %s: %w", revision.ID, err)
		}
		if req.Evolution == nil || req.Evolution.Origin != "diva" {
			continue // ordinary user proposals retain their existing recovery path
		}
		var approval *domain.Approval
		items, err := s.deps.Approvals.ListPendingApprovals(ctx)
		if err != nil {
			return fmt.Errorf("runtime: list Review Center items for Skill revision %s: %w", revision.ID, err)
		}
		for i := range items {
			if items[i].Kind != domain.ApprovalKindSkillRevision || items[i].ToolName != cognitiveSkillProposalTool {
				continue
			}
			var data cognitiveSkillApprovalData
			if json.Unmarshal(items[i].ProposalData, &data) == nil && data.RevisionID == revision.ID {
				approval = &items[i]
				break
			}
		}
		if approval != nil {
			if approval.ExpiresAt <= time.Now().UnixMilli() {
				if err := s.expireApproval(ctx, *approval, "human review timed out during restart"); err != nil {
					return fmt.Errorf("runtime: expire DIVA Skill review %s during recovery: %w", approval.ID, err)
				}
				continue
			}
			continue
		}
		// Revision metadata carries the approval ID and is the durable link when
		// the approval has already reached a terminal decision.
		if req.Evolution.ApprovalID == "" {
			if err := lifecycle.SetSkillRevisionStatus(ctx, revision.ID, domain.SkillRevisionFailed); err != nil {
				return fmt.Errorf("runtime: mark unlinked DIVA Skill revision %s failed: %w", revision.ID, err)
			}
			continue
		}
		decided, err := s.deps.Approvals.GetApproval(ctx, req.Evolution.ApprovalID)
		if errors.Is(err, storage.ErrNotFound) {
			if statusErr := lifecycle.SetSkillRevisionStatus(ctx, revision.ID, domain.SkillRevisionFailed); statusErr != nil {
				return fmt.Errorf("runtime: mark DIVA Skill revision %s without Review Center item failed: %w", revision.ID, statusErr)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("runtime: load Review Center item %s for DIVA Skill revision %s: %w", req.Evolution.ApprovalID, revision.ID, err)
		}
		if decided.RunID == "" {
			return fmt.Errorf("runtime: Review Center item %s has no originating Run for DIVA Skill revision %s", decided.ID, revision.ID)
		}
		switch decided.Decision {
		case domain.ApprovalApproved:
			if _, err := s.deps.Skills.ApplySkillRevision(ctx, decided.RunID, revision.ID); err != nil {
				status := domain.SkillRevisionFailed
				if errors.Is(err, ErrSkillRevisionStale) {
					status = domain.SkillRevisionStale
				}
				if statusErr := lifecycle.SetSkillRevisionStatus(context.WithoutCancel(ctx), revision.ID, status); statusErr != nil {
					return errors.Join(
						fmt.Errorf("runtime: apply approved DIVA Skill revision %s: %w", revision.ID, err),
						fmt.Errorf("runtime: mark DIVA Skill revision %s %s: %w", revision.ID, status, statusErr),
					)
				}
			}
		case domain.ApprovalDenied:
			if err := lifecycle.SetSkillRevisionStatus(ctx, revision.ID, domain.SkillRevisionRejected); err != nil {
				return fmt.Errorf("runtime: mark denied DIVA Skill revision %s rejected: %w", revision.ID, err)
			}
		case domain.ApprovalExpired:
			if err := lifecycle.SetSkillRevisionStatus(ctx, revision.ID, domain.SkillRevisionExpired); err != nil {
				return fmt.Errorf("runtime: mark expired DIVA Skill revision %s: %w", revision.ID, err)
			}
		case domain.ApprovalCancelled:
			if err := lifecycle.SetSkillRevisionStatus(ctx, revision.ID, domain.SkillRevisionCancelled); err != nil {
				return fmt.Errorf("runtime: mark cancelled DIVA Skill revision %s: %w", revision.ID, err)
			}
		case domain.ApprovalStale:
			if err := lifecycle.SetSkillRevisionStatus(ctx, revision.ID, domain.SkillRevisionStale); err != nil {
				return fmt.Errorf("runtime: mark stale DIVA Skill revision %s: %w", revision.ID, err)
			}
		default:
			return fmt.Errorf("runtime: Review Center item %s has non-terminal decision %q for pending DIVA Skill revision %s", decided.ID, decided.Decision, revision.ID)
		}
	}
	return nil
}
