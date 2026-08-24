package eval

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-vivy/internal/domain"
)

// LaunchRequest is everything needed to spawn one air-gapped candidate.
// It never names a store: recording the EvalRun belongs to the caller.
type LaunchRequest struct {
	// Executable is the candidate EXE (from a generation's file: source
	// ref, or the fallback default). Empty means the candidate cannot run.
	Executable string
	// EvalRoot is where the isolated eval directory is created. It must
	// not overlap any production path.
	EvalRoot  string
	Isolation Isolation
	Timeout   time.Duration
}

// LaunchResult is one spawned candidate.
type LaunchResult struct {
	// ID is the eval id, also used as the journal ref (candidate dir name).
	ID     string
	Layout Layout
	// Verdict is mixed when the candidate answered /healthz, otherwise
	// failed_to_run.
	Verdict domain.EvalVerdict
}

const fileRefPrefix = "file:"

// Launch creates an isolated eval dir and probes the candidate EXE. It
// records nothing; the caller owns the ledger write. This is the seam the
// Studio core uses to parent candidates without any live species.
func Launch(ctx context.Context, req LaunchRequest) (LaunchResult, error) {
	if req.EvalRoot == "" {
		return LaunchResult{}, fmt.Errorf("eval: launch requires eval root")
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}
	evalID := newEvalID()
	root := filepath.Join(req.EvalRoot, evalID)
	layout, err := Prepare(root, req.Isolation)
	if err != nil {
		return LaunchResult{}, err
	}
	verdict := domain.EvalFailedToRun
	if req.Executable != "" {
		if err := probe(ctx, layout, req.Executable, timeout); err == nil {
			verdict = domain.EvalMixed
		}
	}
	return LaunchResult{ID: evalID, Layout: layout, Verdict: verdict}, nil
}

// CandidateExecutable resolves the EXE a generation's source_ref points at.
// A file: ref wins; otherwise the fallback is used only when it exists.
func CandidateExecutable(sourceRef, fallback string) (string, bool) {
	if strings.HasPrefix(sourceRef, fileRefPrefix) {
		path := filepath.Clean(strings.TrimPrefix(sourceRef, fileRefPrefix))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return "", false
		}
		return path, true
	}
	if fallback == "" {
		return "", false
	}
	if _, err := os.Stat(fallback); err != nil {
		return "", false
	}
	return fallback, true
}

// probe spawns the candidate with the isolated config and waits for
// /healthz on the layout's loopback address.
func probe(ctx context.Context, layout Layout, executable string, timeout time.Duration) error {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stderrPath := filepath.Join(layout.Root, "stderr.log")
	stderr, err := os.Create(stderrPath)
	if err != nil {
		return fmt.Errorf("eval: create stderr log: %w", err)
	}
	defer func() { _ = stderr.Close() }()

	cmd := exec.CommandContext(probeCtx, executable)
	cmd.Dir = layout.Root
	cmd.Env = ChildEnv(layout.ConfigPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("eval: start candidate: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	health := "http://" + layout.Addr + "/healthz"
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-probeCtx.Done():
			return fmt.Errorf("eval: candidate probe timed out")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, health, nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				continue
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func newEvalID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("evl_%x", time.Now().UnixNano())
	}
	return "evl_" + hex.EncodeToString(b[:])
}
