package studiocore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
)

const (
	// SuiteAirgapProbe is the boot/health probe suite Studio runs against a
	// candidate EXE before release.
	SuiteAirgapProbe = eval.SuiteAirgapProbe

	// PromotionActorHuman is the only actor allowed to release (NG-25).
	PromotionActorHuman = domain.PromotionActorHuman
)

var (
	// ErrNotHuman is returned when a release is attempted by a non-human actor.
	ErrNotHuman = errors.New("studiocore: release actor must be human")
	// ErrNotEvaluated is returned when a release references a generation with no eval run.
	ErrNotEvaluated = errors.New("studiocore: generation has no eval run")
	// ErrNothingToRollBack is returned when there is no previous release to restore.
	ErrNothingToRollBack = errors.New("studiocore: nothing to roll back to")
	// ErrBlockedTarget is returned when an install target overlaps tenant data.
	ErrBlockedTarget = errors.New("studiocore: install target overlaps tenant data")
)

// Options is the Studio service configuration.
type Options struct {
	// Worktree is the canonical source tree (repo root).
	Worktree string
	// SDKPath is the vivy-sdk executable. Empty means resolve on PATH.
	SDKPath string
	// LedgerPath is the Studio-owned ledger DB.
	LedgerPath string
	// EvalRoot is where candidate eval dirs are created (Studio-owned).
	EvalRoot string
	// Isolation is the production surface the candidate must not inherit.
	Isolation eval.Isolation
	// InstallTarget is the daily install location.
	InstallTarget string
	// Timeout bounds one candidate probe.
	Timeout time.Duration
}

// Service drives the Studio lifecycle over the ledger.
type Service struct {
	ledger *Ledger
	opt    Options
}

// NewService opens the Studio ledger and returns a lifecycle service.
func NewService(ctx context.Context, opt Options) (*Service, error) {
	if opt.Worktree == "" {
		return nil, errors.New("studiocore: worktree is required")
	}
	if opt.LedgerPath == "" {
		opt.LedgerPath = filepath.Join(opt.Worktree, "data", "studio-home", "studio.db")
	}
	if opt.EvalRoot == "" {
		opt.EvalRoot = filepath.Join(opt.Worktree, "data", "studio-home", "evals")
	}
	if opt.Isolation.ProductionSQLite == "" {
		opt.Isolation.ProductionSQLite = filepath.Join(opt.Worktree, "data", "vivy.db")
	}
	if opt.Isolation.ProductionWorkspace == "" {
		opt.Isolation.ProductionWorkspace = filepath.Join(opt.Worktree, "data", "workspaces")
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 45 * time.Second
	}
	ledger, err := OpenLedger(ctx, opt.LedgerPath)
	if err != nil {
		return nil, err
	}
	return &Service{ledger: ledger, opt: opt}, nil
}

// Close releases the ledger.
func (s *Service) Close() error {
	if s == nil || s.ledger == nil {
		return nil
	}
	return s.ledger.Close()
}

// Ledger exposes the underlying Studio ledger.
func (s *Service) Ledger() *Ledger { return s.ledger }

// ---- Worktree ----

// PinWorktree records the canonical source tree. Pinning does not touch
// tenant data.
func (s *Service) PinWorktree(ctx context.Context, path, kind string) (domain.Worktree, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return domain.Worktree{}, err
	}
	w := domain.Worktree{
		ID:        NewID("wt_"),
		Path:      abs,
		Kind:      kind,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := s.ledger.CreateWorktree(ctx, w); err != nil {
		return domain.Worktree{}, err
	}
	if _, err := s.appendEvent(ctx, domain.StudioWorktreePinned, w.ID, map[string]string{
		"id": w.ID, "path": w.Path, "kind": w.Kind,
	}); err != nil {
		return domain.Worktree{}, err
	}
	return w, nil
}

// ListWorktrees returns the pinned worktrees, newest first.
func (s *Service) ListWorktrees(ctx context.Context) ([]domain.Worktree, error) {
	return s.ledger.ListWorktrees(ctx)
}

// ---- Pack ----

// sdkPackArtifact mirrors the JSON object vivy-sdk prints on stdout for a
// successful pack (sdk/internal.Artifact). Studio does not import
// sdk/internal, so the wire shape is restated here field by field: the v1
// GenerationManifest carries generationId/recipeDigest and no longer the
// v0 id/artifact_sha256/source_ref/recipe/phase keys.
type sdkPackArtifact struct {
	Directory string `json:"directory"`
	Binary    string `json:"binary"`
	Manifest  struct {
		GenerationID string `json:"generationId"`
		RecipeDigest string `json:"recipeDigest"`
	} `json:"manifest"`
}

// Pack runs vivy-sdk pack (Studio invokes the sdk; the live species does
// not participate) and records the resulting Generation in the Studio
// ledger. recipe is a Generation recipe YAML (apiVersion
// vivy.generation/v1); sources names Module directories outside the
// build-owned repo table, each forwarded as --source. outDir must not
// exist yet (the sdk publishes by rename) and defaults to
// data/studio-home/generations/<id>.
func (s *Service) Pack(ctx context.Context, recipe, outDir string, sources []string) (domain.Generation, error) {
	if recipe == "" {
		return domain.Generation{}, errors.New("studiocore: pack requires a recipe (--recipe)")
	}
	sdkPath, err := s.resolveSDK()
	if err != nil {
		return domain.Generation{}, err
	}
	if outDir == "" {
		outDir = filepath.Join(s.opt.Worktree, "data", "studio-home", "generations", NewID("gen_"))
	}
	args := []string{"pack", "--recipe", recipe, "--output", outDir}
	for _, src := range sources {
		args = append(args, "--source", src)
	}
	cmd := exec.CommandContext(ctx, sdkPath, args...)
	cmd.Dir = s.opt.Worktree
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return domain.Generation{}, fmt.Errorf("studiocore: vivy-sdk pack failed: %s (%w)", strings.TrimSpace(stderr.String()), err)
	}
	art, err := decodePackArtifact(stdout.String())
	if err != nil {
		return domain.Generation{}, err
	}
	return s.recordGeneration(ctx, art)
}

// decodePackArtifact parses the single Artifact JSON object vivy-sdk pack
// prints on stdout.
func decodePackArtifact(out string) (sdkPackArtifact, error) {
	var art sdkPackArtifact
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &art); err != nil {
		return art, fmt.Errorf("studiocore: decode vivy-sdk pack output: %w", err)
	}
	if art.Manifest.GenerationID == "" || art.Binary == "" {
		return art, errors.New("studiocore: incomplete pack artifact from vivy-sdk")
	}
	return art, nil
}

// recordGeneration seals the packed artifact into the Studio ledger. The
// v1 manifest reports generationId and recipeDigest; the ledger's v0
// AssemblyRecipe bill (loop/world/providers/tools/plugins/settings) is
// not restorable from it, so Recipe stays empty rather than being
// fabricated. Eval consumes the candidate through the file: source ref.
func (s *Service) recordGeneration(ctx context.Context, art sdkPackArtifact) (domain.Generation, error) {
	sum, err := hashFile(art.Binary)
	if err != nil {
		return domain.Generation{}, fmt.Errorf("studiocore: hash packed binary: %w", err)
	}
	parent := ""
	if gens, err := s.ledger.ListGenerations(ctx); err == nil && len(gens) > 0 {
		parent = gens[0].ID
	}
	g := domain.Generation{
		ID:             art.Manifest.GenerationID,
		ParentID:       parent,
		ArtifactSHA256: sum,
		SourceRef:      "file:" + filepath.ToSlash(art.Binary),
		Phase:          domain.GenerationBuilt,
		CreatedAt:      time.Now().UnixMilli(),
	}
	if err := s.ledger.CreateGeneration(ctx, g); err != nil {
		return domain.Generation{}, err
	}
	if _, err := s.appendEvent(ctx, domain.StudioGenerationCreated, g.ID, map[string]string{
		"id": g.ID, "phase": string(g.Phase), "parent_id": parent,
	}); err != nil {
		return domain.Generation{}, err
	}
	return g, nil
}

func (s *Service) resolveSDK() (string, error) {
	if s.opt.SDKPath != "" {
		if _, err := os.Stat(s.opt.SDKPath); err != nil {
			return "", fmt.Errorf("studiocore: sdk path %s: %w", s.opt.SDKPath, err)
		}
		return s.opt.SDKPath, nil
	}
	exe := "vivy-sdk"
	if isWindows() {
		exe += ".exe"
	}
	if found, err := exec.LookPath(exe); err == nil {
		return found, nil
	}
	local := filepath.Join(s.opt.Worktree, exe)
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}
	return "", errors.New("studiocore: vivy-sdk not found (set VIVY_SDK or put it on PATH)")
}

// ---- Eval ----

// Eval spawns the candidate EXE itself (the Studio is the parent; the
// live species does not participate) and records the EvalRun in the Studio
// ledger.
func (s *Service) Eval(ctx context.Context, candidateID, baselineID, suite string) (domain.EvalRun, error) {
	if suite == "" {
		suite = SuiteAirgapProbe
	}
	if suite != SuiteAirgapProbe {
		return domain.EvalRun{}, eval.ErrInvalidSuite
	}
	gen, err := s.ledger.GetGeneration(ctx, candidateID)
	if err != nil {
		return domain.EvalRun{}, err
	}
	if err := s.ledger.UpdateGenerationPhase(ctx, candidateID, domain.GenerationEvalPending); err != nil {
		return domain.EvalRun{}, err
	}
	exe, exeOK := eval.CandidateExecutable(gen.SourceRef, "")
	executable := ""
	if exeOK {
		executable = exe
	}
	res, err := eval.Launch(ctx, eval.LaunchRequest{
		Executable: executable,
		EvalRoot:   s.opt.EvalRoot,
		Isolation:  s.opt.Isolation,
		Timeout:    s.opt.Timeout,
	})
	if err != nil {
		return domain.EvalRun{}, err
	}
	e := domain.EvalRun{
		ID:          res.ID,
		CandidateID: candidateID,
		BaselineID:  baselineID,
		Suite:       suite,
		Verdict:     res.Verdict,
		JournalRef:  filepath.Join(s.opt.EvalRoot, res.ID),
		CreatedAt:   time.Now().UnixMilli(),
	}
	if err := s.ledger.CreateEvalRun(ctx, e); err != nil {
		return domain.EvalRun{}, err
	}
	if err := s.ledger.UpdateGenerationPhase(ctx, candidateID, domain.GenerationEvaluated); err != nil {
		return domain.EvalRun{}, err
	}
	if _, err := s.appendEvent(ctx, domain.StudioEvalRunRecorded, e.ID, map[string]string{
		"id": e.ID, "candidate_id": candidateID, "verdict": string(e.Verdict),
	}); err != nil {
		return domain.EvalRun{}, err
	}
	return e, nil
}

// ---- Release ----

// Release records a human-accepted Release. It refuses any non-human
// actor and any generation without an eval run.
func (s *Service) Release(ctx context.Context, generationID, evalID, actor string) (domain.Release, error) {
	if actor != PromotionActorHuman {
		return domain.Release{}, ErrNotHuman
	}
	gen, err := s.ledger.GetGeneration(ctx, generationID)
	if err != nil {
		return domain.Release{}, err
	}
	if gen.Phase == domain.GenerationReleased || gen.Phase == domain.GenerationRejected {
		return domain.Release{}, ErrConflict
	}
	if evalID == "" {
		evals, err := s.ledger.ListEvalRunsFor(ctx, generationID)
		if err != nil {
			return domain.Release{}, err
		}
		if len(evals) == 0 {
			return domain.Release{}, ErrNotEvaluated
		}
		evalID = evals[0].ID
	} else if _, err := s.ledger.GetEvalRun(ctx, evalID); err != nil {
		return domain.Release{}, err
	}
	rel := domain.Release{
		ID:           NewID("rel_"),
		GenerationID: generationID,
		EvalID:       evalID,
		Actor:        actor,
		Phase:        domain.ReleaseAccepted,
		CreatedAt:    time.Now().UnixMilli(),
	}
	if err := s.ledger.CreateRelease(ctx, rel); err != nil {
		return domain.Release{}, err
	}
	if err := s.ledger.UpdateGenerationPhase(ctx, generationID, domain.GenerationReleased); err != nil {
		return domain.Release{}, err
	}
	if _, err := s.appendEvent(ctx, domain.StudioReleaseAccepted, rel.ID, map[string]string{
		"id": rel.ID, "generation_id": generationID, "eval_id": evalID, "actor": actor,
	}); err != nil {
		return domain.Release{}, err
	}
	return rel, nil
}

// Reject rejects a generation that is not yet decided.
func (s *Service) Reject(ctx context.Context, generationID string) (domain.Generation, error) {
	gen, err := s.ledger.GetGeneration(ctx, generationID)
	if err != nil {
		return domain.Generation{}, err
	}
	if gen.Phase == domain.GenerationReleased || gen.Phase == domain.GenerationRejected {
		return domain.Generation{}, ErrConflict
	}
	if err := s.ledger.UpdateGenerationPhase(ctx, generationID, domain.GenerationRejected); err != nil {
		return domain.Generation{}, err
	}
	if _, err := s.appendEvent(ctx, domain.StudioGenerationRejected, generationID, map[string]string{
		"id": generationID,
	}); err != nil {
		return domain.Generation{}, err
	}
	gen.Phase = domain.GenerationRejected
	return gen, nil
}

// ---- Install / rollback ----

// installManifest is written beside the installed EXE in the daily
// location so the location is self-describing.
type installManifest struct {
	ReleaseID      string `json:"release_id"`
	GenerationID   string `json:"generation_id"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	InstalledAt    int64  `json:"installed_at"`
}

// Install writes a released generation's EXE into the daily install
// location and records an Install row (phase current). The previous
// release's files are preserved under the Studio-owned rollback dir; the
// running process is never touched (no hot-swap).
func (s *Service) Install(ctx context.Context, releaseID, target string) (domain.Install, error) {
	if target == "" {
		target = s.opt.InstallTarget
	}
	if target == "" {
		return domain.Install{}, errors.New("studiocore: install target is required")
	}
	if err := s.guardInstallTarget(target); err != nil {
		return domain.Install{}, err
	}
	rel, err := s.ledger.GetRelease(ctx, releaseID)
	if err != nil {
		return domain.Install{}, err
	}
	gen, err := s.ledger.GetGeneration(ctx, rel.GenerationID)
	if err != nil {
		return domain.Install{}, err
	}
	exe, ok := eval.CandidateExecutable(gen.SourceRef, "")
	if !ok {
		return domain.Install{}, fmt.Errorf("studiocore: generation %s has no runnable artifact", gen.ID)
	}
	sum, err := hashFile(exe)
	if err != nil {
		return domain.Install{}, err
	}
	if sum != gen.ArtifactSHA256 {
		return domain.Install{}, fmt.Errorf("studiocore: artifact hash mismatch: %s != %s", sum, gen.ArtifactSHA256)
	}
	installID := NewID("ins_")

	// Preserve the previous release's files for rollback (Studio-owned).
	rollbackDir := filepath.Join(filepath.Dir(s.opt.LedgerPath), "rollback", installID)
	if err := os.MkdirAll(rollbackDir, 0o700); err != nil {
		return domain.Install{}, fmt.Errorf("studiocore: rollback dir: %w", err)
	}
	if err := copyInstallState(target, rollbackDir); err != nil {
		return domain.Install{}, fmt.Errorf("studiocore: preserve previous install: %w", err)
	}

	if err := os.MkdirAll(target, 0o700); err != nil {
		return domain.Install{}, fmt.Errorf("studiocore: install target: %w", err)
	}
	dst := filepath.Join(target, "vivy.exe")
	if err := copyFile(exe, dst); err != nil {
		return domain.Install{}, fmt.Errorf("studiocore: copy exe: %w", err)
	}
	man := installManifest{
		ReleaseID:      releaseID,
		GenerationID:   gen.ID,
		ArtifactSHA256: gen.ArtifactSHA256,
		InstalledAt:    time.Now().UnixMilli(),
	}
	if err := writeManifest(filepath.Join(target, "install.json"), man); err != nil {
		return domain.Install{}, err
	}

	// Any prior current install for this target is no longer current.
	if prev, err := s.ledger.CurrentInstall(ctx, target); err == nil {
		if err := s.ledger.UpdateInstallPhase(ctx, prev.ID, domain.InstallRolledBack); err != nil {
			return domain.Install{}, err
		}
	}
	in := domain.Install{
		ID:        installID,
		ReleaseID: releaseID,
		Target:    target,
		Phase:     domain.InstallCurrent,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := s.ledger.CreateInstall(ctx, in); err != nil {
		return domain.Install{}, err
	}
	if _, err := s.appendEvent(ctx, domain.StudioInstallRecorded, in.ID, map[string]string{
		"id": in.ID, "release_id": releaseID, "target": target,
	}); err != nil {
		return domain.Install{}, err
	}
	return in, nil
}

// Rollback restores the previous release's files into the daily install
// location. Per §8 rollback is still an Install: it records a new Install
// row (phase current) for the restored release and marks the superseded
// row rolled_back. The tenant Journal is never opened.
func (s *Service) Rollback(ctx context.Context, target string) (domain.Install, error) {
	if target == "" {
		target = s.opt.InstallTarget
	}
	if target == "" {
		return domain.Install{}, errors.New("studiocore: rollback target is required")
	}
	if err := s.guardInstallTarget(target); err != nil {
		return domain.Install{}, err
	}
	cur, err := s.ledger.CurrentInstall(ctx, target)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Install{}, ErrNothingToRollBack
		}
		return domain.Install{}, err
	}
	rollbackDir := filepath.Join(filepath.Dir(s.opt.LedgerPath), "rollback", cur.ID)
	// The rollback dir holds the state that was in the location before the
	// current install — i.e. the previous release's files.
	if _, err := os.Stat(filepath.Join(rollbackDir, "vivy.exe")); err != nil {
		return domain.Install{}, ErrNothingToRollBack
	}
	// Identify the release being restored from the preserved manifest.
	restored := cur.ReleaseID
	if raw, err := os.ReadFile(filepath.Join(rollbackDir, "install.json")); err == nil {
		var man installManifest
		if err := json.Unmarshal(raw, &man); err == nil && man.ReleaseID != "" {
			restored = man.ReleaseID
		}
	}
	if err := restoreInstallState(rollbackDir, target); err != nil {
		return domain.Install{}, fmt.Errorf("studiocore: restore previous release: %w", err)
	}
	if err := s.ledger.UpdateInstallPhase(ctx, cur.ID, domain.InstallRolledBack); err != nil {
		return domain.Install{}, err
	}
	in := domain.Install{
		ID:        NewID("ins_"),
		ReleaseID: restored,
		Target:    target,
		Phase:     domain.InstallCurrent,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := s.ledger.CreateInstall(ctx, in); err != nil {
		return domain.Install{}, err
	}
	if _, err := s.appendEvent(ctx, domain.StudioInstallRolledBack, in.ID, map[string]string{
		"id": in.ID, "release_id": restored, "target": target, "from_release_id": cur.ReleaseID,
	}); err != nil {
		return domain.Install{}, err
	}
	return in, nil
}

// InspectInstall reads the daily location's manifest and EXE hash without
// starting any process.
func (s *Service) InspectInstall(ctx context.Context, target string) (map[string]any, error) {
	if target == "" {
		target = s.opt.InstallTarget
	}
	raw, err := os.ReadFile(filepath.Join(target, "install.json"))
	if err != nil {
		return nil, fmt.Errorf("studiocore: no install at %s: %w", target, err)
	}
	var man installManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return nil, fmt.Errorf("studiocore: decode install manifest: %w", err)
	}
	sum, err := hashFile(filepath.Join(target, "vivy.exe"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"target":           target,
		"release_id":       man.ReleaseID,
		"generation_id":    man.GenerationID,
		"artifact_sha256":  man.ArtifactSHA256,
		"installed_sha256": sum,
		"installed_at":     man.InstalledAt,
	}, nil
}

// guardInstallTarget forbids installs into the worktree or tenant data.
func (s *Service) guardInstallTarget(target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	worktree, err := filepath.Abs(s.opt.Worktree)
	if err != nil {
		return err
	}
	// The daily location must be outside the source tree (including data/).
	if sameOrInside(abs, worktree) {
		return ErrBlockedTarget
	}
	return nil
}

// ---- helpers ----

func (s *Service) appendEvent(ctx context.Context, typ domain.StudioEventType, objectID string, payload map[string]string) (int64, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte("{}")
	}
	err = s.ledger.AppendStudioEvent(ctx, domain.StudioEvent{
		Type: typ, ObjectID: objectID, CreatedAt: time.Now().UnixMilli(), Payload: raw,
	})
	return time.Now().UnixMilli(), err
}

func copyInstallState(target, rollbackDir string) error {
	// Preserve whatever is currently in the daily location (previous
	// release) before the new EXE lands.
	for _, name := range []string{"vivy.exe", "install.json"} {
		src := filepath.Join(target, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(rollbackDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func restoreInstallState(rollbackDir, target string) error {
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"vivy.exe", "install.json"} {
		src := filepath.Join(rollbackDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(target, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func writeManifest(path string, man installManifest) error {
	raw, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sameOrInside(abs, parent string) bool {
	if strings.EqualFold(abs, parent) {
		return true
	}
	rel, err := filepath.Rel(parent, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
