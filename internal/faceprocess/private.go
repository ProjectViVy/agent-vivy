// Package faceprocess hosts the seams for launching a generated Face as its
// own process: private runtime allocation shared with the code face, and the
// selected-Face dispatch used by the accepted packaging route.
package faceprocess

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
)

// Instance namespaces: each launch kind gets a sibling directory under the
// shared data root. Keeping the set closed prevents a caller from writing a
// private runtime under an arbitrary path.
const (
	// NamespaceCodeInstances is the code face's per-launch runtime directory.
	NamespaceCodeInstances = "code-instances"
	// NamespaceFaceInstances is the selected Face's per-launch runtime
	// directory (e.g. the restricted ACP pilot).
	NamespaceFaceInstances = "face-instances"
)

// Prepared is the split between shared configuration and private runtime
// state for one launched face process.
type Prepared struct {
	Config             config.Config
	SharedSettingsPath string
	InstanceRoot       string
}

// PreparePrivate allocates a private SQLite-backed runtime for one launched
// process. Provider/model settings remain at the original config data root;
// sessions, messages, approvals, runs, checkpoints, eval scratch, and logs
// move under a unique instance root beneath <shared>/<namespace>. World and
// workspace selection stay with the caller — this helper owns storage and
// logging only.
func PreparePrivate(cfg config.Config, namespace string) (Prepared, error) {
	if namespace != NamespaceCodeInstances && namespace != NamespaceFaceInstances {
		return Prepared{}, fmt.Errorf("faceprocess: unknown instance namespace %q", namespace)
	}
	sharedRoot := cfg.DataDirectory()
	sharedSettingsPath := settings.Path(sharedRoot)
	instancesRoot := filepath.Join(sharedRoot, namespace)
	if err := os.MkdirAll(instancesRoot, 0o700); err != nil {
		return Prepared{}, fmt.Errorf("faceprocess: create instances root: %w", err)
	}
	prefix := fmt.Sprintf("%s-%d-", time.Now().Format("20060102-150405"), os.Getpid())
	instanceRoot, err := os.MkdirTemp(instancesRoot, prefix)
	if err != nil {
		return Prepared{}, fmt.Errorf("faceprocess: allocate instance: %w", err)
	}

	cfg.Storage.Backend = "sqlite"
	cfg.Storage.DataDir = instanceRoot
	cfg.Storage.SQLite.Path = filepath.Join(instanceRoot, "vivy.db")
	cfg.Logging.Dir = filepath.Join(instanceRoot, "logs")
	if err := cfg.Validate(); err != nil {
		return Prepared{}, fmt.Errorf("faceprocess: validate instance config: %w", err)
	}
	return Prepared{Config: cfg, SharedSettingsPath: sharedSettingsPath, InstanceRoot: instanceRoot}, nil
}
