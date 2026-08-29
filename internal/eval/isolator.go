// Package eval launches an air-gapped candidate species and records the
// resulting EvalRun. It does not pack a new EXE and does not share the
// production Journal writer.
package eval

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/config"
)

const SuiteAirgapProbe = "airgap.probe"

var (
	ErrInvalidSuite = errors.New("eval: suite not supported")
	ErrBlockedPath  = errors.New("eval: candidate would touch production storage")
)

// Isolation is the production surface the candidate must not inherit.
type Isolation struct {
	ProductionSQLite    string
	ProductionWorkspace string
	ProductionListen    string
	BundleDir           string
}

// Layout is one eval directory on disk.
type Layout struct {
	Root       string
	ConfigPath string
	SQLitePath string
	Workspace  string
	Skills     string
	Addr       string
}

// Prepare creates an isolated eval tree and writes a candidate config.
func Prepare(root string, iso Isolation) (Layout, error) {
	if root == "" {
		return Layout{}, fmt.Errorf("eval: empty root")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("eval: abs root: %w", err)
	}
	layout := Layout{
		Root:       absRoot,
		ConfigPath: filepath.Join(absRoot, "config.yaml"),
		SQLitePath: filepath.Join(absRoot, "data", "vivy.db"),
		Workspace:  filepath.Join(absRoot, "data", "workspaces"),
		Skills:     filepath.Join(absRoot, "data", "skills"),
	}
	if overlapsProduction(layout, iso) {
		return Layout{}, ErrBlockedPath
	}
	addr, err := freeLoopback(iso.ProductionListen)
	if err != nil {
		return Layout{}, err
	}
	layout.Addr = addr
	if err := os.MkdirAll(layout.Workspace, 0o700); err != nil {
		return Layout{}, fmt.Errorf("eval: mkdir workspace: %w", err)
	}
	if err := os.MkdirAll(layout.Skills, 0o700); err != nil {
		return Layout{}, fmt.Errorf("eval: mkdir skills: %w", err)
	}
	bundleDir := iso.BundleDir
	if bundleDir == "" {
		bundleDir = "fixtures/provider"
	}
	if abs, err := filepath.Abs(bundleDir); err == nil {
		bundleDir = abs
	}
	doc := map[string]any{
		"server": map[string]any{"addr": layout.Addr},
		"storage": map[string]any{
			"backend": "sqlite",
			"sqlite":  map[string]any{"path": layout.SQLitePath},
		},
		"providers": map[string]any{
			"active":     "openai",
			"bundle_dir": bundleDir,
			"openai":     map[string]any{"env_key": "OPENAI_API_KEY", "default_model": "gpt-4o-mini"},
			"anthropic":  map[string]any{"env_key": "ANTHROPIC_API_KEY", "default_model": "claude-sonnet-4-5"},
		},
		"runtime": map[string]any{
			"workspace_root": layout.Workspace,
			"skills_root":    layout.Skills,
		},
		"tools": map[string]any{
			"enabled":  []string{"echo_info"},
			"approval": map[string]any{"expiration": "5m"},
		},
	}
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return Layout{}, fmt.Errorf("eval: encode config: %w", err)
	}
	if configLeaksProduction(string(raw), iso) {
		return Layout{}, ErrBlockedPath
	}
	if strings.Contains(strings.ToLower(string(raw)), "api_key:") {
		return Layout{}, ErrBlockedPath
	}
	if err := os.WriteFile(layout.ConfigPath, raw, 0o600); err != nil {
		return Layout{}, fmt.Errorf("eval: write config: %w", err)
	}
	if _, err := config.Load(layout.ConfigPath); err != nil {
		return Layout{}, fmt.Errorf("eval: invalid candidate config: %w", err)
	}
	return layout, nil
}

// ChildEnv is a stripped environment that only carries VIVY_CONFIG and
// the host process bits a Windows child needs to start.
func ChildEnv(configPath, userHome string) []string {
	env := []string{"VIVY_CONFIG=" + configPath}
	if userHome != "" {
		env = append(env, "VIVY_USER_HOME="+userHome)
	}
	for _, key := range []string{
		"PATH", "PATHEXT", "SYSTEMROOT", "SYSTEMDRIVE", "WINDIR", "COMSPEC",
		"TEMP", "TMP", "USERPROFILE", "HOMEDRIVE", "HOMEPATH",
	} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func overlapsProduction(layout Layout, iso Isolation) bool {
	if samePath(layout.SQLitePath, iso.ProductionSQLite) {
		return true
	}
	if iso.ProductionSQLite != "" && inside(layout.Root, iso.ProductionSQLite) {
		return true
	}
	if iso.ProductionWorkspace != "" && (samePath(layout.Workspace, iso.ProductionWorkspace) || inside(layout.Workspace, iso.ProductionWorkspace)) {
		return true
	}
	return false
}

func configLeaksProduction(raw string, iso Isolation) bool {
	return containsPath(raw, iso.ProductionSQLite) || containsPath(raw, iso.ProductionWorkspace) || containsPath(raw, iso.ProductionListen)
}

func containsPath(raw, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	lower := strings.ToLower(raw)
	candidates := []string{path}
	if abs, err := filepath.Abs(path); err == nil {
		candidates = append(candidates, abs, filepath.ToSlash(abs))
	}
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(lower, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return strings.EqualFold(absA, absB)
}

func inside(parent, child string) bool {
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	absChild, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absParent, absChild)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

func freeLoopback(avoid string) (string, error) {
	for i := 0; i < 8; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", fmt.Errorf("eval: reserve listen: %w", err)
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		if avoid == "" || !strings.EqualFold(addr, avoid) {
			return addr, nil
		}
	}
	return "", fmt.Errorf("eval: could not reserve a listen address")
}
