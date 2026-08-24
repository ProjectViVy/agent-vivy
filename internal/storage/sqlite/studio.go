package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func (b *Backend) CreateGeneration(ctx context.Context, g domain.Generation) error {
	if g.ID == "" || !g.Phase.Valid() {
		return fmt.Errorf("storage: invalid generation")
	}
	recipe, err := json.Marshal(g.Recipe)
	if err != nil {
		return fmt.Errorf("storage: encode generation recipe: %w", err)
	}
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO generations (id, parent_id, artifact_sha256, source_ref, recipe_json, phase, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.ParentID, g.ArtifactSHA256, g.SourceRef, recipe, string(g.Phase), g.CreatedAt); err != nil {
		return fmt.Errorf("storage: create generation %s: %w", g.ID, err)
	}
	return nil
}

func (b *Backend) GetGeneration(ctx context.Context, id string) (domain.Generation, error) {
	row := b.db.QueryRowContext(ctx,
		`SELECT id, parent_id, artifact_sha256, source_ref, recipe_json, phase, created_at
		 FROM generations WHERE id = ?`, id)
	g, err := scanGeneration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Generation{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.Generation{}, fmt.Errorf("storage: get generation %s: %w", id, err)
	}
	return g, nil
}

func (b *Backend) ListGenerations(ctx context.Context) ([]domain.Generation, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT id, parent_id, artifact_sha256, source_ref, recipe_json, phase, created_at
		 FROM generations ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("storage: list generations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Generation{}
	for rows.Next() {
		g, err := scanGeneration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (b *Backend) UpdateGenerationPhase(ctx context.Context, id string, phase domain.GenerationPhase) error {
	if !phase.Valid() {
		return fmt.Errorf("storage: invalid generation phase")
	}
	res, err := b.db.ExecContext(ctx, `UPDATE generations SET phase = ? WHERE id = ?`, string(phase), id)
	if err != nil {
		return fmt.Errorf("storage: update generation phase %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: update generation phase %s: %w", id, err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (b *Backend) CreateEvalRun(ctx context.Context, e domain.EvalRun) error {
	if e.ID == "" || e.CandidateID == "" || !e.Verdict.Valid() {
		return fmt.Errorf("storage: invalid eval run")
	}
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO eval_runs (id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.CandidateID, e.BaselineID, e.Suite, string(e.Verdict), e.JournalRef, e.CreatedAt); err != nil {
		return fmt.Errorf("storage: create eval run %s: %w", e.ID, err)
	}
	return nil
}

func (b *Backend) GetEvalRun(ctx context.Context, id string) (domain.EvalRun, error) {
	var e domain.EvalRun
	var verdict string
	err := b.db.QueryRowContext(ctx,
		`SELECT id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at
		 FROM eval_runs WHERE id = ?`, id).
		Scan(&e.ID, &e.CandidateID, &e.BaselineID, &e.Suite, &verdict, &e.JournalRef, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EvalRun{}, storage.ErrNotFound
	}
	if err != nil {
		return domain.EvalRun{}, fmt.Errorf("storage: get eval run %s: %w", id, err)
	}
	e.Verdict = domain.EvalVerdict(verdict)
	return e, nil
}

func (b *Backend) ListEvalRuns(ctx context.Context) ([]domain.EvalRun, error) {
	return b.listEvalRuns(ctx, "")
}

func (b *Backend) ListEvalRunsFor(ctx context.Context, generationID string) ([]domain.EvalRun, error) {
	return b.listEvalRuns(ctx, generationID)
}

func (b *Backend) listEvalRuns(ctx context.Context, generationID string) ([]domain.EvalRun, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if generationID == "" {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at
			 FROM eval_runs ORDER BY created_at DESC, id DESC`)
	} else {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, candidate_id, baseline_id, suite, verdict, journal_ref, created_at
			 FROM eval_runs WHERE candidate_id = ? ORDER BY created_at DESC, id DESC`, generationID)
	}
	if err != nil {
		return nil, fmt.Errorf("storage: list eval runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.EvalRun{}
	for rows.Next() {
		var e domain.EvalRun
		var verdict string
		if err := rows.Scan(&e.ID, &e.CandidateID, &e.BaselineID, &e.Suite, &verdict, &e.JournalRef, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan eval run: %w", err)
		}
		e.Verdict = domain.EvalVerdict(verdict)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (b *Backend) CreatePromotion(ctx context.Context, p domain.Promotion) error {
	if p.ID == "" || p.FromID == "" || p.ToID == "" || !p.Phase.Valid() {
		return fmt.Errorf("storage: invalid promotion")
	}
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO promotions (id, from_id, to_id, eval_id, actor, phase, applies_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.FromID, p.ToID, p.EvalID, p.Actor, string(p.Phase), p.AppliesAt, p.CreatedAt); err != nil {
		if isUniqueConflict(err) {
			return storage.ErrConflict
		}
		return fmt.Errorf("storage: create promotion %s: %w", p.ID, err)
	}
	return nil
}

func (b *Backend) ListPromotions(ctx context.Context) ([]domain.Promotion, error) {
	return b.listPromotions(ctx, "")
}

func (b *Backend) ListPromotionsFrom(ctx context.Context, fromID string) ([]domain.Promotion, error) {
	return b.listPromotions(ctx, fromID)
}

func (b *Backend) listPromotions(ctx context.Context, fromID string) ([]domain.Promotion, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if fromID == "" {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, from_id, to_id, eval_id, actor, phase, applies_at, created_at
			 FROM promotions ORDER BY created_at DESC, id DESC`)
	} else {
		rows, err = b.db.QueryContext(ctx,
			`SELECT id, from_id, to_id, eval_id, actor, phase, applies_at, created_at
			 FROM promotions WHERE from_id = ? ORDER BY created_at DESC, id DESC`, fromID)
	}
	if err != nil {
		return nil, fmt.Errorf("storage: list promotions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Promotion{}
	for rows.Next() {
		var p domain.Promotion
		var phase string
		if err := rows.Scan(&p.ID, &p.FromID, &p.ToID, &p.EvalID, &p.Actor, &phase, &p.AppliesAt, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan promotion: %w", err)
		}
		p.Phase = domain.PromotionPhase(phase)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (b *Backend) AppendStudioEvent(ctx context.Context, ev domain.StudioEvent) (int64, error) {
	if !ev.Type.Valid() || ev.ObjectID == "" {
		return 0, fmt.Errorf("storage: invalid studio event")
	}
	res, err := b.db.ExecContext(ctx,
		`INSERT INTO studio_events (type, object_id, created_at, payload) VALUES (?, ?, ?, ?)`,
		string(ev.Type), ev.ObjectID, ev.CreatedAt, ev.Payload)
	if err != nil {
		return 0, fmt.Errorf("storage: append studio event: %w", err)
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("storage: studio event seq: %w", err)
	}
	return seq, nil
}

func (b *Backend) ListStudioEvents(ctx context.Context) ([]domain.StudioEvent, error) {
	rows, err := b.db.QueryContext(ctx,
		`SELECT seq, type, object_id, created_at, payload FROM studio_events ORDER BY seq ASC`)
	if err != nil {
		return nil, fmt.Errorf("storage: list studio events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.StudioEvent{}
	for rows.Next() {
		var ev domain.StudioEvent
		var typ string
		if err := rows.Scan(&ev.Seq, &typ, &ev.ObjectID, &ev.CreatedAt, &ev.Payload); err != nil {
			return nil, fmt.Errorf("storage: scan studio event: %w", err)
		}
		ev.Type = domain.StudioEventType(typ)
		out = append(out, ev)
	}
	return out, rows.Err()
}

type generationScanner interface {
	Scan(dest ...any) error
}

func scanGeneration(row generationScanner) (domain.Generation, error) {
	var g domain.Generation
	var phase string
	var recipe []byte
	if err := row.Scan(&g.ID, &g.ParentID, &g.ArtifactSHA256, &g.SourceRef, &recipe, &phase, &g.CreatedAt); err != nil {
		return domain.Generation{}, err
	}
	if len(recipe) > 0 {
		if err := json.Unmarshal(recipe, &g.Recipe); err != nil {
			return domain.Generation{}, fmt.Errorf("storage: decode generation recipe: %w", err)
		}
	}
	g.Phase = domain.GenerationPhase(phase)
	return g, nil
}

func isUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed")
}
