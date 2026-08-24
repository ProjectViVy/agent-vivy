package eval

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/studio"
)

func TestStartUnknownSuite(t *testing.T) {
	runner, _ := newEvalRunner(t, filepath.Join(t.TempDir(), "missing.exe"))
	_, err := runner.Start(context.Background(), "missing", "", "not-a-suite")
	if err != ErrInvalidSuite {
		t.Fatalf("err = %v, want ErrInvalidSuite", err)
	}
}

func TestStartMissingBinaryRecordsFailedToRun(t *testing.T) {
	runner, backend := newEvalRunner(t, filepath.Join(t.TempDir(), "missing.exe"))
	ctx := context.Background()
	seedProductionSession(t, backend)
	gen, err := runner.Studio.CreateGeneration(ctx, domain.Generation{ArtifactSHA256: "cand"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.Start(ctx, gen.ID, "", SuiteAirgapProbe)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != domain.EvalFailedToRun || got.Suite != SuiteAirgapProbe || got.JournalRef == "" {
		t.Fatalf("eval = %+v", got)
	}
	assertProductionUntouched(t, backend)
}

func TestEvalAirGapDoesNotTouchProduction(t *testing.T) {
	exe := buildVivy(t)
	runner, backend := newEvalRunner(t, exe)
	ctx := context.Background()
	seedProductionSession(t, backend)
	gen, err := runner.Studio.CreateGeneration(ctx, domain.Generation{ArtifactSHA256: "cand"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.Start(ctx, gen.ID, "", SuiteAirgapProbe)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != domain.EvalMixed {
		t.Fatalf("eval = %+v", got)
	}
	assertProductionUntouched(t, backend)
	candidateDB := filepath.Join(runner.EvalRoot, got.ID, "data", "vivy.db")
	if _, err := os.Stat(candidateDB); err != nil {
		t.Fatalf("candidate sqlite missing: %v", err)
	}
}

func TestEvalUsesFileArtifactNotParentExe(t *testing.T) {
	exe := buildVivy(t)
	runner, backend := newEvalRunner(t, filepath.Join(t.TempDir(), "parent-not-used.exe"))
	ctx := context.Background()
	seedProductionSession(t, backend)
	gen, err := runner.Studio.CreateGeneration(ctx, domain.Generation{
		ArtifactSHA256: "packed",
		SourceRef:      "file:" + exe,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.Start(ctx, gen.ID, "", SuiteAirgapProbe)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != domain.EvalMixed {
		t.Fatalf("eval = %+v", got)
	}
	assertProductionUntouched(t, backend)
}

func TestEvalMissingFileArtifactDoesNotFallBack(t *testing.T) {
	exe := buildVivy(t)
	runner, backend := newEvalRunner(t, exe)
	ctx := context.Background()
	seedProductionSession(t, backend)
	gen, err := runner.Studio.CreateGeneration(ctx, domain.Generation{
		ArtifactSHA256: "missing",
		SourceRef:      "file:" + filepath.Join(t.TempDir(), "no-such-vivy.exe"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.Start(ctx, gen.ID, "", SuiteAirgapProbe)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != domain.EvalFailedToRun {
		t.Fatalf("eval = %+v, want failed_to_run", got)
	}
	assertProductionUntouched(t, backend)
}

func newEvalRunner(t *testing.T, executable string) (*Runner, *sqlite.Backend) {
	t.Helper()
	ctx := context.Background()
	productionDir := t.TempDir()
	backend, err := sqlite.Open(ctx, filepath.Join(productionDir, "vivy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return NewRunner(Runner{
		Studio:     studio.NewService(backend),
		Executable: executable,
		EvalRoot:   filepath.Join(productionDir, "evals"),
		Isolation: Isolation{
			ProductionSQLite:    filepath.Join(productionDir, "vivy.db"),
			ProductionWorkspace: filepath.Join(productionDir, "workspaces"),
			ProductionListen:    "127.0.0.1:8787",
			BundleDir:           fixtureBundle(t),
		},
		Timeout: 60 * time.Second,
	}), backend
}

func seedProductionSession(t *testing.T, backend *sqlite.Backend) {
	t.Helper()
	if err := backend.CreateSession(context.Background(), domain.Session{
		ID: "ses_canary", Title: "canary", CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
}

func assertProductionUntouched(t *testing.T, backend *sqlite.Backend) {
	t.Helper()
	sessions, err := backend.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "ses_canary" {
		t.Fatalf("production sessions = %+v", sessions)
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
