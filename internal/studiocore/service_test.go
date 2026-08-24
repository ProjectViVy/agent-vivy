package studiocore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

// newTestService opens a Studio service against a temp worktree. The
// worktree is a fake source tree; tenant data paths are pointed at temp
// files so the air gap assertions are real.
func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	worktree := t.TempDir()
	ctx := context.Background()
	svc, err := NewService(ctx, Options{
		Worktree:   worktree,
		LedgerPath: filepath.Join(worktree, "data", "studio-home", "studio.db"),
		EvalRoot:   filepath.Join(worktree, "data", "studio-home", "evals"),
		Timeout:    60 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc, worktree
}

// TestRecordGenerationRecordsPackOutput proves Studio records a
// generation.json written by vivy-sdk into the Studio ledger with the
// built phase. The pack exec itself is covered by the sdk pack tests and
// the end-to-end demo; here the record seam is exercised directly.
func TestRecordGenerationRecordsPackOutput(t *testing.T) {
	svc, worktree := newTestService(t)
	ctx := context.Background()
	outDir := filepath.Join(worktree, "data", "studio-home", "generations", "gen_demo")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(outDir, "vivy.exe")
	if err := os.WriteFile(exe, []byte("candidate body"), 0o700); err != nil {
		t.Fatal(err)
	}
	sum, err := hashFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
		"id": "gen_demo",
		"artifact_sha256": "` + sum + `",
		"source_ref": "file:` + filepath.ToSlash(exe) + `",
		"recipe": {"loop":"eino","world":"sandbox","plugins":["hello-fs"]},
		"phase": "built"
	}`)
	if err := os.WriteFile(filepath.Join(outDir, "generation.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	g, err := svc.recordGeneration(ctx, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != "gen_demo" || g.Phase != domain.GenerationBuilt {
		t.Fatalf("generation = %+v", g)
	}
	if g.ArtifactSHA256 != sum {
		t.Fatalf("sha = %s, want %s", g.ArtifactSHA256, sum)
	}
	if len(g.Recipe.Plugins) != 1 || g.Recipe.Plugins[0] != "hello-fs" {
		t.Fatalf("recipe = %+v", g.Recipe)
	}
	got, err := svc.Ledger().GetGeneration(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != domain.GenerationBuilt {
		t.Fatalf("recorded phase = %q", got.Phase)
	}
}

func TestPackRequiresPlugin(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Pack(ctx, nil, ""); err == nil || !strings.Contains(err.Error(), "--with") {
		t.Fatalf("pack with no plugins = %v", err)
	}
}

func TestReleaseRequiresHuman(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	gen := domain.Generation{
		ID: "gen_1", ArtifactSHA256: "sha", SourceRef: "file:" + filepath.Join(t.TempDir(), "vivy.exe"),
		Phase: domain.GenerationBuilt, CreatedAt: 1,
	}
	if err := svc.Ledger().CreateGeneration(ctx, gen); err != nil {
		t.Fatal(err)
	}
	evl := domain.EvalRun{
		ID: "evl_1", CandidateID: gen.ID, Suite: SuiteAirgapProbe,
		Verdict: domain.EvalMixed, CreatedAt: 2,
	}
	if err := svc.Ledger().CreateEvalRun(ctx, evl); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Release(ctx, gen.ID, evl.ID, "machine"); err != ErrNotHuman {
		t.Fatalf("release by machine = %v, want ErrNotHuman", err)
	}
	rel, err := svc.Release(ctx, gen.ID, evl.ID, PromotionActorHuman)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Phase != domain.ReleaseAccepted || rel.Actor != PromotionActorHuman {
		t.Fatalf("release = %+v", rel)
	}
	got, err := svc.Ledger().GetGeneration(ctx, gen.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != domain.GenerationReleased {
		t.Fatalf("generation phase = %q, want released", got.Phase)
	}
	// Second release of the same generation conflicts.
	if _, err := svc.Release(ctx, gen.ID, evl.ID, PromotionActorHuman); err != ErrConflict {
		t.Fatalf("second release = %v, want ErrConflict", err)
	}
}

func TestReleaseRequiresEval(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	gen := domain.Generation{
		ID: "gen_1", ArtifactSHA256: "sha", Phase: domain.GenerationBuilt, CreatedAt: 1,
	}
	if err := svc.Ledger().CreateGeneration(ctx, gen); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Release(ctx, gen.ID, "", PromotionActorHuman); err != ErrNotEvaluated {
		t.Fatalf("release without eval = %v, want ErrNotEvaluated", err)
	}
}

// TestEvalSpawnsCandidateItself proves ST-5: the Studio spawns the
// candidate EXE directly (zero live-species participation) and records the
// EvalRun in the Studio ledger with an isolated data dir.
func TestEvalSpawnsCandidateItself(t *testing.T) {
	exe := buildVivy(t)
	svc, worktree := newTestService(t)
	ctx := context.Background()
	// The candidate config needs a real provider fixture bundle; the fake
	// worktree has none, so point at the repo's fixtures.
	svc.opt.Isolation.BundleDir = fixtureBundle(t)

	gen := domain.Generation{
		ID: "gen_cand", ArtifactSHA256: "cand", SourceRef: "file:" + exe,
		Phase: domain.GenerationBuilt, CreatedAt: 1,
	}
	if err := svc.Ledger().CreateGeneration(ctx, gen); err != nil {
		t.Fatal(err)
	}

	e, err := svc.Eval(ctx, gen.ID, "", SuiteAirgapProbe)
	if err != nil {
		t.Fatal(err)
	}
	if e.Verdict != domain.EvalMixed {
		t.Fatalf("eval verdict = %q, want mixed (candidate must boot)", e.Verdict)
	}
	if e.Suite != SuiteAirgapProbe || e.CandidateID != gen.ID {
		t.Fatalf("eval = %+v", e)
	}
	// The candidate journal lives under the Studio eval root, never in
	// tenant data.
	if !strings.HasPrefix(filepath.Clean(e.JournalRef), filepath.Clean(svc.opt.EvalRoot)) {
		t.Fatalf("journal ref %q outside eval root %q", e.JournalRef, svc.opt.EvalRoot)
	}
	if _, err := os.Stat(filepath.Join(e.JournalRef, "data", "vivy.db")); err != nil {
		t.Fatalf("candidate sqlite missing: %v", err)
	}
	got, err := svc.Ledger().GetGeneration(ctx, gen.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != domain.GenerationEvaluated {
		t.Fatalf("generation phase = %q, want evaluated", got.Phase)
	}
	assertTenantUntouched(t, worktree)
}

// TestInstallAndRollbackRoundTrip proves ST-7/ST-8: install writes the new
// EXE into the daily location (next launch is the new body), rollback
// restores the previous release, and the tenant Journal is never touched.
func TestInstallAndRollbackRoundTrip(t *testing.T) {
	svc, worktree := newTestService(t)
	ctx := context.Background()
	daily := filepath.Join(t.TempDir(), "daily")
	tenantJournal := filepath.Join(worktree, "data", "vivy.db")
	if err := os.MkdirAll(filepath.Dir(tenantJournal), 0o700); err != nil {
		t.Fatal(err)
	}
	before := []byte("tenant journal content")
	if err := os.WriteFile(tenantJournal, before, 0o600); err != nil {
		t.Fatal(err)
	}

	mkGen := func(id, content string) domain.Generation {
		exe := filepath.Join(t.TempDir(), "vivy.exe")
		if err := os.WriteFile(exe, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
		sum, err := hashFile(exe)
		if err != nil {
			t.Fatal(err)
		}
		return domain.Generation{
			ID: id, ArtifactSHA256: sum, SourceRef: "file:" + exe,
			Phase: domain.GenerationBuilt, CreatedAt: time.Now().UnixMilli(),
		}
	}
	genA := mkGen("gen_a", "first body")
	genB := mkGen("gen_b", "second body")
	for _, g := range []domain.Generation{genA, genB} {
		if err := svc.Ledger().CreateGeneration(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	evl := domain.EvalRun{ID: "evl_a", CandidateID: genA.ID, Suite: SuiteAirgapProbe, Verdict: domain.EvalMixed, CreatedAt: 1}
	if err := svc.Ledger().CreateEvalRun(ctx, evl); err != nil {
		t.Fatal(err)
	}
	evlB := domain.EvalRun{ID: "evl_b", CandidateID: genB.ID, Suite: SuiteAirgapProbe, Verdict: domain.EvalMixed, CreatedAt: 2}
	if err := svc.Ledger().CreateEvalRun(ctx, evlB); err != nil {
		t.Fatal(err)
	}
	relA, err := svc.Release(ctx, genA.ID, evl.ID, PromotionActorHuman)
	if err != nil {
		t.Fatal(err)
	}
	relB, err := svc.Release(ctx, genB.ID, evlB.ID, PromotionActorHuman)
	if err != nil {
		t.Fatal(err)
	}

	// Install A, then B over it.
	insA, err := svc.Install(ctx, relA.ID, daily)
	if err != nil {
		t.Fatal(err)
	}
	if insA.Phase != domain.InstallCurrent {
		t.Fatalf("install A phase = %q", insA.Phase)
	}
	if !sameContent(t, filepath.Join(daily, "vivy.exe"), genA) {
		t.Fatal("daily location does not hold body A after install A")
	}
	insB, err := svc.Install(ctx, relB.ID, daily)
	if err != nil {
		t.Fatal(err)
	}
	if !sameContent(t, filepath.Join(daily, "vivy.exe"), genB) {
		t.Fatal("daily location does not hold body B after install B")
	}
	cur, err := svc.Ledger().CurrentInstall(ctx, daily)
	if err != nil {
		t.Fatal(err)
	}
	if cur.ID != insB.ID {
		t.Fatalf("current install = %s, want %s", cur.ID, insB.ID)
	}
	if _, err := os.Stat(filepath.Join(daily, "install.json")); err != nil {
		t.Fatalf("install manifest missing: %v", err)
	}

	// Rollback restores the previous release (A) and records it as a fresh
	// current install (rollback is still an Install, §8).
	rb, err := svc.Rollback(ctx, daily)
	if err != nil {
		t.Fatal(err)
	}
	if rb.Phase != domain.InstallCurrent {
		t.Fatalf("rollback phase = %q, want current", rb.Phase)
	}
	if rb.ReleaseID != relA.ID {
		t.Fatalf("rollback release = %s, want %s", rb.ReleaseID, relA.ID)
	}
	if !sameContent(t, filepath.Join(daily, "vivy.exe"), genA) {
		t.Fatal("daily location was not restored to body A after rollback")
	}
	curAfter, err := svc.Ledger().CurrentInstall(ctx, daily)
	if err != nil {
		t.Fatal(err)
	}
	if curAfter.ID != rb.ID {
		t.Fatalf("current install after rollback = %s, want %s", curAfter.ID, rb.ID)
	}
	// The install that was rolled back is now marked rolled_back.
	prev, err := svc.Ledger().GetInstall(ctx, insB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prev.Phase != domain.InstallRolledBack {
		t.Fatalf("superseded install phase = %q, want rolled_back", prev.Phase)
	}
	after, err := os.ReadFile(tenantJournal)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("tenant Journal was modified by install/rollback")
	}
	events, err := svc.Ledger().ListStudioEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hasRollbackEvent := false
	for _, ev := range events {
		if ev.Type == domain.StudioInstallRolledBack {
			hasRollbackEvent = true
		}
	}
	if !hasRollbackEvent {
		t.Fatal("missing install.rolled_back event")
	}
}

func TestRollbackWithNothingToRestore(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	daily := filepath.Join(t.TempDir(), "daily")
	exe := filepath.Join(t.TempDir(), "vivy.exe")
	if err := os.WriteFile(exe, []byte("first body"), 0o700); err != nil {
		t.Fatal(err)
	}
	sum, err := hashFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	gen := domain.Generation{
		ID: "gen_a", ArtifactSHA256: sum, SourceRef: "file:" + exe,
		Phase: domain.GenerationBuilt, CreatedAt: 1,
	}
	if err := svc.Ledger().CreateGeneration(ctx, gen); err != nil {
		t.Fatal(err)
	}
	evl := domain.EvalRun{ID: "evl_a", CandidateID: gen.ID, Suite: SuiteAirgapProbe, Verdict: domain.EvalMixed, CreatedAt: 1}
	if err := svc.Ledger().CreateEvalRun(ctx, evl); err != nil {
		t.Fatal(err)
	}
	rel, err := svc.Release(ctx, gen.ID, evl.ID, PromotionActorHuman)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Install(ctx, rel.ID, daily); err != nil {
		t.Fatal(err)
	}
	// No previous release ever installed → nothing to roll back to.
	if _, err := svc.Rollback(ctx, daily); err != ErrNothingToRollBack {
		t.Fatalf("rollback = %v, want ErrNothingToRollBack", err)
	}
}

func TestInstallRejectsTargetInsideWorktree(t *testing.T) {
	svc, worktree := newTestService(t)
	ctx := context.Background()
	gen := domain.Generation{ID: "gen_a", ArtifactSHA256: "sha", Phase: domain.GenerationBuilt, CreatedAt: 1}
	if err := svc.Ledger().CreateGeneration(ctx, gen); err != nil {
		t.Fatal(err)
	}
	evl := domain.EvalRun{ID: "evl_a", CandidateID: gen.ID, Suite: SuiteAirgapProbe, Verdict: domain.EvalMixed, CreatedAt: 1}
	if err := svc.Ledger().CreateEvalRun(ctx, evl); err != nil {
		t.Fatal(err)
	}
	rel, err := svc.Release(ctx, gen.ID, evl.ID, PromotionActorHuman)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(worktree, "data", "demo")
	if _, err := svc.Install(ctx, rel.ID, bad); err != ErrBlockedTarget {
		t.Fatalf("install into worktree/data = %v, want ErrBlockedTarget", err)
	}
}

func sameContent(t *testing.T, path string, gen domain.Generation) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum != gen.ArtifactSHA256 {
		t.Fatalf("%s hash = %s, want %s", path, sum, gen.ArtifactSHA256)
	}
	return len(raw) > 0
}

// assertTenantUntouched verifies the fake tenant Journal and workspace were
// not created or written by the Studio service.
func assertTenantUntouched(t *testing.T, worktree string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(worktree, "data", "vivy.db"),
		filepath.Join(worktree, "data", "workspaces"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("Studio touched tenant path %s", path)
		}
	}
}

func buildVivy(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "vivy-eval.exe")
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}
	cmd := exec.Command(goBin, "build", "-o", exe, "./cmd/vivy")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build vivy: %v\n%s", err, out)
	}
	return exe
}

func fixtureBundle(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "provider"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}
