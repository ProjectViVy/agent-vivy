package studio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var (
	ErrInvalid        = errors.New("studio: invalid object")
	ErrNotReady       = errors.New("studio: generation has no eval run")
	ErrNotHuman       = errors.New("studio: promote actor must be human")
	ErrAlreadyDecided = errors.New("studio: generation is already decided")
)

// Service owns Generation / EvalRun / Promotion writes. It does not spawn
// processes or change the running binary.
type Service struct {
	store storage.StudioStore
}

func NewService(store storage.StudioStore) *Service {
	return &Service{store: store}
}

func (s *Service) ListGenerations(ctx context.Context) ([]domain.Generation, error) {
	if s == nil || s.store == nil {
		return []domain.Generation{}, nil
	}
	return s.store.ListGenerations(ctx)
}

func (s *Service) GetGeneration(ctx context.Context, id string) (domain.Generation, error) {
	if s == nil || s.store == nil {
		return domain.Generation{}, storage.ErrNotFound
	}
	return s.store.GetGeneration(ctx, id)
}

func (s *Service) ListEvalRuns(ctx context.Context) ([]domain.EvalRun, error) {
	if s == nil || s.store == nil {
		return []domain.EvalRun{}, nil
	}
	return s.store.ListEvalRuns(ctx)
}

func (s *Service) ListPromotions(ctx context.Context) ([]domain.Promotion, error) {
	if s == nil || s.store == nil {
		return []domain.Promotion{}, nil
	}
	return s.store.ListPromotions(ctx)
}

func (s *Service) ListStudioEvents(ctx context.Context) ([]domain.StudioEvent, error) {
	if s == nil || s.store == nil {
		return []domain.StudioEvent{}, nil
	}
	return s.store.ListStudioEvents(ctx)
}

func (s *Service) CreateGeneration(ctx context.Context, g domain.Generation) (domain.Generation, error) {
	if g.ID == "" {
		g.ID = newStudioID("gen_")
	}
	if g.CreatedAt == 0 {
		g.CreatedAt = time.Now().UnixMilli()
	}
	if g.Phase == "" {
		g.Phase = domain.GenerationBuilt
	}
	if !g.Phase.Valid() || g.ArtifactSHA256 == "" {
		return domain.Generation{}, ErrInvalid
	}
	if err := s.store.CreateGeneration(ctx, g); err != nil {
		return domain.Generation{}, err
	}
	if _, err := s.store.AppendStudioEvent(ctx, domain.StudioEvent{
		Type: domain.StudioGenerationCreated, ObjectID: g.ID, CreatedAt: g.CreatedAt,
		Payload: mustJSON(map[string]string{"id": g.ID, "phase": string(g.Phase)}),
	}); err != nil {
		return domain.Generation{}, err
	}
	return g, nil
}

func (s *Service) RecordEval(ctx context.Context, e domain.EvalRun) (domain.EvalRun, error) {
	if e.ID == "" {
		e.ID = newStudioID("evl_")
	}
	if e.CreatedAt == 0 {
		e.CreatedAt = time.Now().UnixMilli()
	}
	if e.CandidateID == "" || e.Suite == "" || !e.Verdict.Valid() {
		return domain.EvalRun{}, ErrInvalid
	}
	if _, err := s.store.GetGeneration(ctx, e.CandidateID); err != nil {
		return domain.EvalRun{}, err
	}
	if err := s.store.CreateEvalRun(ctx, e); err != nil {
		return domain.EvalRun{}, err
	}
	if err := s.store.UpdateGenerationPhase(ctx, e.CandidateID, domain.GenerationEvaluated); err != nil {
		return domain.EvalRun{}, err
	}
	if _, err := s.store.AppendStudioEvent(ctx, domain.StudioEvent{
		Type: domain.StudioEvalRunRecorded, ObjectID: e.ID, CreatedAt: e.CreatedAt,
		Payload: mustJSON(map[string]string{"id": e.ID, "candidate_id": e.CandidateID, "verdict": string(e.Verdict)}),
	}); err != nil {
		return domain.EvalRun{}, err
	}
	return e, nil
}

func (s *Service) Promote(ctx context.Context, fromID, toID, evalID, actor string) (domain.Promotion, error) {
	if actor == "" {
		actor = domain.PromotionActorHuman
	}
	if actor != domain.PromotionActorHuman {
		return domain.Promotion{}, ErrNotHuman
	}
	if fromID == "" || toID == "" {
		return domain.Promotion{}, ErrInvalid
	}
	if _, err := s.store.GetGeneration(ctx, fromID); err != nil {
		return domain.Promotion{}, err
	}
	if _, err := s.store.GetGeneration(ctx, toID); err != nil {
		return domain.Promotion{}, err
	}
	evals, err := s.store.ListEvalRunsFor(ctx, toID)
	if err != nil {
		return domain.Promotion{}, err
	}
	if len(evals) == 0 {
		return domain.Promotion{}, ErrNotReady
	}
	if evalID == "" {
		evalID = evals[0].ID
	} else if _, err := s.store.GetEvalRun(ctx, evalID); err != nil {
		return domain.Promotion{}, err
	}
	now := time.Now().UnixMilli()
	p := domain.Promotion{
		ID:        newStudioID("pro_"),
		FromID:    fromID,
		ToID:      toID,
		EvalID:    evalID,
		Actor:     actor,
		Phase:     domain.PromotionAccepted,
		AppliesAt: domain.PromotionAppliesNextLaunch,
		CreatedAt: now,
	}
	if err := s.store.CreatePromotion(ctx, p); err != nil {
		return domain.Promotion{}, err
	}
	if err := s.store.UpdateGenerationPhase(ctx, toID, domain.GenerationPromoted); err != nil {
		return domain.Promotion{}, err
	}
	if _, err := s.store.AppendStudioEvent(ctx, domain.StudioEvent{
		Type: domain.StudioPromotionAccepted, ObjectID: p.ID, CreatedAt: now,
		Payload: mustJSON(map[string]string{"id": p.ID, "from_id": fromID, "to_id": toID, "eval_id": evalID}),
	}); err != nil {
		return domain.Promotion{}, err
	}
	return p, nil
}

func (s *Service) Reject(ctx context.Context, id string) (domain.Generation, error) {
	if id == "" {
		return domain.Generation{}, ErrInvalid
	}
	g, err := s.store.GetGeneration(ctx, id)
	if err != nil {
		return domain.Generation{}, err
	}
	if g.Phase == domain.GenerationPromoted || g.Phase == domain.GenerationRejected {
		return domain.Generation{}, ErrAlreadyDecided
	}
	if err := s.store.UpdateGenerationPhase(ctx, id, domain.GenerationRejected); err != nil {
		return domain.Generation{}, err
	}
	now := time.Now().UnixMilli()
	if _, err := s.store.AppendStudioEvent(ctx, domain.StudioEvent{
		Type: domain.StudioGenerationRejected, ObjectID: id, CreatedAt: now,
		Payload: mustJSON(map[string]string{"id": id}),
	}); err != nil {
		return domain.Generation{}, err
	}
	g.Phase = domain.GenerationRejected
	return g, nil
}

func newStudioID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s%x", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b[:])
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return raw
}
