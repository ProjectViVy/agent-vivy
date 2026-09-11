package modulehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/toolworld"
)

var (
	ErrDenied        = errors.New("module capability denied")
	ErrInvalidConfig = errors.New("invalid module host configuration")
)

type SecretReader interface {
	ReadSecret(context.Context, string) (string, error)
}

type ProcessSpawner interface {
	Spawn(context.Context, toolworld.SpawnSpec) (toolworld.Proc, error)
}

type RPCClient interface {
	Call(context.Context, string, json.RawMessage) (json.RawMessage, error)
}

type Config struct {
	ModuleID      string
	InstanceID    string
	WorkspaceRoot string
	Grants        []module.GrantBinding
	Secrets       SecretReader
	Spawner       ProcessSpawner
	Transport     http.RoundTripper
	RPC           RPCClient
}

type Facade struct {
	moduleID      string
	instanceID    string
	workspaceRoot string
	grants        map[module.Grant]map[string]struct{}
	secrets       SecretReader
	spawner       ProcessSpawner
	transport     http.RoundTripper
	rpc           RPCClient
}

func New(cfg Config) (*Facade, error) {
	moduleID := strings.TrimSpace(cfg.ModuleID)
	instanceID := strings.TrimSpace(cfg.InstanceID)
	if moduleID == "" || instanceID == "" {
		return nil, fmt.Errorf("%w: module and instance identities are required", ErrInvalidConfig)
	}
	root := strings.TrimSpace(cfg.WorkspaceRoot)
	if root != "" {
		absolute, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("%w: resolve workspace: %w", ErrInvalidConfig, err)
		}
		root = filepath.Clean(absolute)
	}
	facade := &Facade{
		moduleID:      moduleID,
		instanceID:    instanceID,
		workspaceRoot: root,
		grants:        make(map[module.Grant]map[string]struct{}, len(cfg.Grants)),
		secrets:       cfg.Secrets,
		spawner:       cfg.Spawner,
		transport:     cfg.Transport,
		rpc:           cfg.RPC,
	}
	for _, binding := range cfg.Grants {
		if !binding.Name.Valid() {
			return nil, fmt.Errorf("%w: unknown grant %q", ErrInvalidConfig, binding.Name)
		}
		if _, duplicate := facade.grants[binding.Name]; duplicate {
			return nil, fmt.Errorf("%w: duplicate grant %q", ErrInvalidConfig, binding.Name)
		}
		flat := make(map[string]struct{})
		for key, values := range binding.Constraints {
			for _, value := range values {
				flat[key+"\x00"+strings.TrimSpace(value)] = struct{}{}
			}
		}
		facade.grants[binding.Name] = flat
	}
	return facade, nil
}

func (facade *Facade) ModuleID() string   { return facade.moduleID }
func (facade *Facade) InstanceID() string { return facade.instanceID }
func (facade *Facade) Workspace() string  { return facade.workspaceRoot }

func (facade *Facade) OpenRead(name string) (io.ReadCloser, error) {
	resolved, err := facade.resolveWorkspacePath(name, module.GrantFSRead, false)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (facade *Facade) OpenWrite(name string) (io.WriteCloser, error) {
	resolved, err := facade.resolveWorkspacePath(name, module.GrantFSWrite, true)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(resolved, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (facade *Facade) resolveWorkspacePath(name string, grant module.Grant, writing bool) (string, error) {
	if _, ok := facade.grants[grant]; !ok || facade.workspaceRoot == "" {
		return "", denied("workspace access")
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || filepath.IsAbs(trimmed) || filepath.VolumeName(trimmed) != "" {
		return "", denied("workspace path")
	}
	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", denied("workspace path")
	}
	if !facade.allowedRoot(grant, filepath.ToSlash(cleaned)) {
		return "", denied("workspace root")
	}
	resolved := filepath.Join(facade.workspaceRoot, cleaned)
	if !within(facade.workspaceRoot, resolved) {
		return "", denied("workspace path")
	}

	probe := resolved
	if writing {
		probe = filepath.Dir(resolved)
	}
	for {
		evaluated, err := filepath.EvalSymlinks(probe)
		if err == nil {
			if !within(facade.workspaceRoot, evaluated) {
				return "", denied("workspace symlink")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(probe)
		if parent == probe || !within(facade.workspaceRoot, parent) {
			break
		}
		probe = parent
	}
	return resolved, nil
}

func (facade *Facade) allowedRoot(grant module.Grant, cleaned string) bool {
	constraints := facade.grants[grant]
	found := false
	for entry := range constraints {
		key, value, ok := strings.Cut(entry, "\x00")
		if !ok || key != "roots" {
			continue
		}
		found = true
		root := strings.Trim(strings.TrimSpace(filepath.ToSlash(value)), "/")
		if root == "" || root == "." || cleaned == root || strings.HasPrefix(cleaned, root+"/") {
			return true
		}
	}
	return !found
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func (facade *Facade) Secret(ctx context.Context, name string) (string, error) {
	if !facade.constraintAllows(module.GrantSecretRead, "names", name, true) || facade.secrets == nil {
		return "", denied("secret read")
	}
	value, err := facade.secrets.ReadSecret(ctx, name)
	if err != nil {
		return "", redacted("secret read failed", err)
	}
	return value, nil
}

func (facade *Facade) Spawn(ctx context.Context, spec toolworld.SpawnSpec) (toolworld.Proc, error) {
	command := strings.TrimSpace(spec.Command)
	if command == "" || !facade.constraintAllows(module.GrantProcSpawn, "commands", command, true) || facade.spawner == nil {
		return nil, denied("process spawn")
	}
	proc, err := facade.spawner.Spawn(ctx, spec)
	if err != nil {
		return nil, redacted("process spawn failed", err)
	}
	return proc, nil
}

func (facade *Facade) Do(ctx context.Context, request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil || facade.transport == nil {
		return nil, denied("network request")
	}
	if _, ok := facade.grants[module.GrantNetClient]; !ok {
		return nil, denied("network request")
	}
	scheme := strings.ToLower(strings.TrimSpace(request.URL.Scheme))
	host := strings.ToLower(strings.TrimSuffix(request.URL.Hostname(), "."))
	if scheme == "" || host == "" {
		return nil, denied("network target")
	}
	if !facade.constraintAllows(module.GrantNetClient, "schemes", scheme, true) || !facade.constraintAllows(module.GrantNetClient, "hosts", host, true) {
		return nil, denied("network target")
	}
	port := request.URL.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	if facade.hasConstraint(module.GrantNetClient, "ports") && !facade.constraintAllows(module.GrantNetClient, "ports", port, false) {
		return nil, denied("network port")
	}
	if request.URL.User != nil {
		return nil, denied("network credentials in URL")
	}
	clone := request.Clone(ctx)
	response, err := facade.transport.RoundTrip(clone)
	if err != nil {
		return nil, redacted("network request failed", err)
	}
	return response, nil
}

func (facade *Facade) CallRPC(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if _, ok := facade.grants[module.GrantRPCClient]; !ok || facade.rpc == nil || strings.TrimSpace(method) == "" {
		return nil, denied("rpc.client")
	}
	result, err := facade.rpc.Call(ctx, method, payload)
	if err != nil {
		return nil, redacted("rpc.client call failed", err)
	}
	return result, nil
}

func (facade *Facade) hasConstraint(grant module.Grant, key string) bool {
	for entry := range facade.grants[grant] {
		constraintKey, _, ok := strings.Cut(entry, "\x00")
		if ok && constraintKey == key {
			return true
		}
	}
	return false
}

func (facade *Facade) constraintAllows(grant module.Grant, key, value string, requireConstraint bool) bool {
	constraints, ok := facade.grants[grant]
	if !ok {
		return false
	}
	needle := key + "\x00" + strings.TrimSpace(value)
	if _, ok := constraints[needle]; ok {
		return true
	}
	return !requireConstraint && !facade.hasConstraint(grant, key)
}

type redactedError struct {
	message string
	cause   error
}

func (err redactedError) Error() string { return err.message }
func (err redactedError) Unwrap() error { return err.cause }

func redacted(message string, cause error) error {
	if cause == nil {
		return errors.New(message)
	}
	return redactedError{message: message, cause: cause}
}

func denied(scope string) error {
	return fmt.Errorf("%w: %s", ErrDenied, scope)
}

var _ = net.IP{}
