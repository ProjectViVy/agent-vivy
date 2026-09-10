// Package codeface composes an independent VIVY CODE process. It shares the
// operator configuration surface with Vivy while keeping every launch's
// Journal and runtime records in a distinct instance directory.
package codeface

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"agent-vivy/internal/app"
	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"agent-vivy/internal/logging"
	plugin "agent-vivy/sdk/port/face"
	tuiface "agent-vivy/sdk/tui/face"
)

// Prepared is the split between shared configuration and private runtime
// state for one code process.
type Prepared struct {
	Config             config.Config
	SharedSettingsPath string
	InstanceRoot       string
}

// Prepare allocates a private SQLite-backed runtime for one code process.
// Provider/model settings remain at the original config data root; sessions,
// messages, approvals, runs, checkpoints, eval scratch, and logs move under a
// unique instance root. The current project is mounted as the local world.
func Prepare(cfg config.Config, projectDir string) (Prepared, error) {
	projectRoot, err := filepath.Abs(projectDir)
	if err != nil {
		return Prepared{}, fmt.Errorf("vivy-code: resolve project: %w", err)
	}
	info, err := os.Lstat(projectRoot)
	if err != nil {
		return Prepared{}, fmt.Errorf("vivy-code: inspect project: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Prepared{}, fmt.Errorf("vivy-code: project is not a directory")
	}
	// Keep the code face's local-world root canonical even when the caller
	// reached it through a symlink/junction in a parent component. The root
	// itself remains rejected as a symlink, preserving the explicit project
	// boundary while making downstream containment comparisons physical.
	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return Prepared{}, fmt.Errorf("vivy-code: canonicalize project: %w", err)
	}
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return Prepared{}, fmt.Errorf("vivy-code: resolve canonical project: %w", err)
	}

	sharedRoot := cfg.DataDirectory()
	sharedSettingsPath := settings.Path(sharedRoot)
	instancesRoot := filepath.Join(sharedRoot, "code-instances")
	if err := os.MkdirAll(instancesRoot, 0o700); err != nil {
		return Prepared{}, fmt.Errorf("vivy-code: create instances root: %w", err)
	}
	prefix := fmt.Sprintf("%s-%d-", time.Now().Format("20060102-150405"), os.Getpid())
	instanceRoot, err := os.MkdirTemp(instancesRoot, prefix)
	if err != nil {
		return Prepared{}, fmt.Errorf("vivy-code: allocate instance: %w", err)
	}

	cfg.Storage.Backend = "sqlite"
	cfg.Storage.DataDir = instanceRoot
	cfg.Storage.SQLite.Path = filepath.Join(instanceRoot, "vivy.db")
	cfg.Logging.Dir = filepath.Join(instanceRoot, "logs")
	cfg.Runtime.World = "local"
	cfg.Runtime.WorkspaceRoot = projectRoot
	cfg.Runtime.Sandbox.WorkspaceRoot = projectRoot
	if err := cfg.Validate(); err != nil {
		return Prepared{}, fmt.Errorf("vivy-code: validate instance config: %w", err)
	}
	return Prepared{Config: cfg, SharedSettingsPath: sharedSettingsPath, InstanceRoot: instanceRoot}, nil
}

// Run starts the real terminal face on the prepared private runtime.
func Run(ctx context.Context, cfg config.Config, projectDir string, out, errOut io.Writer) (plugin.FaceResult, error) {
	prepared, err := Prepare(cfg, projectDir)
	if err != nil {
		return plugin.FaceResult{Status: "failed"}, err
	}
	vivyLog, _, closeLog, err := logging.Setup(logging.Options{
		Level: cfg.Logging.Level, Format: cfg.Logging.Format,
		Dir: prepared.Config.LogDirectory(), RetentionDays: cfg.Logging.RetentionDays,
		Stdout: false,
	})
	if err != nil {
		return plugin.FaceResult{Status: "failed"}, fmt.Errorf("vivy-code: logging: %w", err)
	}
	defer closeLog.Close()
	slog.SetDefault(vivyLog)
	vivyLog.Info("vivy-code instance initialized", "instance_root", prepared.InstanceRoot, "settings_path", prepared.SharedSettingsPath)
	return app.RunFaceWithAppOptions(
		ctx,
		prepared.Config,
		tuiface.New,
		plugin.FaceOptions{DebugToolOutput: prepared.Config.TUI.Debug, Out: out, Err: errOut},
		codeAppOptions(prepared)...,
	)
}

// The canonical TUI face hydrates settings/get and forwards controller.Locale()
// to the view. Keep its settings authority at the shared root, not the private
// instance's Journal directory.
func codeAppOptions(prepared Prepared) []app.AppOption {
	return []app.AppOption{
		app.WithSettingsPath(prepared.SharedSettingsPath),
		app.WithCodeProjectRoot(prepared.Config.Runtime.WorkspaceRoot),
		app.WithInstructionRoot(prepared.Config.Runtime.WorkspaceRoot),
	}
}
