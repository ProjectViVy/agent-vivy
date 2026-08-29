package runtime

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"agent-vivy/internal/domain"
)

// ErrSandboxDenied is returned when a sandbox check blocks an operation.
var ErrSandboxDenied = errors.New("runtime: sandbox denied operation")

// FileOp identifies the type of filesystem operation being validated.
type FileOp string

const (
	FileOpRead  FileOp = "read"
	FileOpWrite FileOp = "write"
	FileOpExec  FileOp = "execute"
)

// SandboxManager enforces the file-effect policy boundary (D-021). The
// workspace root and command allowlist are fixed at construction. Default
// mode and network policy can be updated from the settings overlay; each
// tool call may still pass an explicit session mode that outranks the default.
type SandboxManager struct {
	mu            sync.RWMutex
	mode          domain.SandboxMode
	workspaceRoot string
	cmdWhitelist  map[string]struct{}
	netPolicy     domain.NetworkPolicy
}

// NewSandboxManager validates and constructs a sandbox manager. The root
// path is resolved to an absolute path but not created until needed.
func NewSandboxManager(mode domain.SandboxMode, workspaceRoot string, cmdWhitelist []string, netPolicy *domain.NetworkPolicy) (*SandboxManager, error) {
	if !mode.Valid() {
		return nil, fmt.Errorf("runtime: invalid sandbox mode %q", mode)
	}

	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, errors.New("runtime: workspace root must not be empty")
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("runtime: resolve workspace root: %w", err)
	}

	whitelist := make(map[string]struct{}, len(cmdWhitelist))
	for _, cmd := range cmdWhitelist {
		name := normalizeCommandName(cmd)
		if name != "" {
			whitelist[name] = struct{}{}
		}
	}

	policy := domain.NetworkPolicy{DenyPrivateIPs: true}
	if netPolicy != nil {
		policy = *netPolicy
		policy.AllowedDomains = append([]string(nil), netPolicy.AllowedDomains...)
	}

	return &SandboxManager{
		mode:          mode,
		workspaceRoot: filepath.Clean(abs),
		cmdWhitelist:  whitelist,
		netPolicy:     policy,
	}, nil
}

// Mode returns the current default sandbox mode.
func (m *SandboxManager) Mode() domain.SandboxMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

// SetDefaultMode updates the fallback mode used when a call does not carry
// an explicit session mode. Invalid values are ignored.
func (m *SandboxManager) SetDefaultMode(mode domain.SandboxMode) {
	if m == nil || !mode.Valid() {
		return
	}
	m.mu.Lock()
	m.mode = mode
	m.mu.Unlock()
}

// SetNetworkPolicy replaces the live network policy used by CheckNetwork.
func (m *SandboxManager) SetNetworkPolicy(policy domain.NetworkPolicy) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.netPolicy = domain.NetworkPolicy{
		AllowedDomains:   append([]string(nil), policy.AllowedDomains...),
		DenyPrivateIPs:   policy.DenyPrivateIPs,
		MaxResponseBytes: policy.MaxResponseBytes,
	}
	m.mu.Unlock()
}

func (m *SandboxManager) resolveMode(mode domain.SandboxMode) domain.SandboxMode {
	if mode.Valid() {
		return mode
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.mode.Valid() {
		return m.mode
	}
	return domain.SandboxModeWorkspaceWrite
}

// WorkspaceRoot returns the absolute workspace root path.
func (m *SandboxManager) WorkspaceRoot() string {
	return m.workspaceRoot
}

// ValidatePath checks whether a path and operation are permitted under the
// current sandbox mode. In read-only mode, all writes are denied. In
// workspace-write mode, paths must stay within the workspace root. In
// danger-full-access mode, all paths are allowed (but still audited).
func (m *SandboxManager) ValidatePath(path string, op FileOp) error {
	return m.ValidatePathWithMode(path, op, "")
}

// ValidatePathWithMode checks a path under an explicit sandbox mode.
func (m *SandboxManager) ValidatePathWithMode(path string, op FileOp, mode domain.SandboxMode) error {
	if m == nil {
		return errors.New("runtime: sandbox manager not initialized")
	}

	mode = m.resolveMode(mode)
	// danger-full-access bypasses all checks (but caller should still audit).
	if mode == domain.SandboxModeDangerFullAccess {
		return nil
	}

	// Normalize the path first.
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("runtime: resolve path: %w", err)
	}
	abs = filepath.Clean(abs)
	root := m.workspaceRoot
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}

	// Check for symlink traversal before resolving.
	if strings.Contains(path, "..") {
		// Allow .. only if it resolves within workspace.
		cleaned := filepath.Clean(abs)
		rel, err := filepath.Rel(root, cleaned)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: path escapes workspace via traversal", ErrSandboxDenied)
		}
	}

	// Resolve symlinks to prevent escape via symlink chains.
	realPath, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("runtime: resolve symlinks: %w", err)
		}
		// If the target doesn't exist yet (e.g., new file), use the parent.
		parent := filepath.Dir(abs)
		realParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return fmt.Errorf("runtime: resolve parent symlinks: %w", err)
		}
		realPath = filepath.Join(realParent, filepath.Base(abs))
	}

	// Deny writes in read-only mode.
	if mode == domain.SandboxModeReadOnly && op == FileOpWrite {
		return fmt.Errorf("%w: write operations not allowed in read-only mode", ErrSandboxDenied)
	}

	// In workspace-write and read-only modes, ensure path stays within workspace.
	if mode.IsConfined() {
		rel, err := filepath.Rel(root, realPath)
		if err != nil {
			return fmt.Errorf("%w: cannot determine path relationship", ErrSandboxDenied)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return fmt.Errorf("%w: path %q escapes workspace root", ErrSandboxDenied, path)
		}
	}

	return nil
}

// ConfineCommand checks whether a command is allowed under the current
// sandbox mode and whitelist. In read-only mode, all command execution is
// denied.
func (m *SandboxManager) ConfineCommand(command string, args []string) error {
	return m.ConfineCommandWithMode(command, args, "")
}

// ConfineCommandWithMode checks a command under an explicit sandbox mode.
func (m *SandboxManager) ConfineCommandWithMode(command string, args []string, mode domain.SandboxMode) error {
	if m == nil {
		return errors.New("runtime: sandbox manager not initialized")
	}

	mode = m.resolveMode(mode)
	// Deny all commands in read-only mode.
	if mode == domain.SandboxModeReadOnly {
		return fmt.Errorf("%w: command execution not allowed in read-only mode", ErrSandboxDenied)
	}

	// Normalize the command name.
	name := normalizeCommandName(command)
	if name == "" {
		return fmt.Errorf("%w: invalid command name", ErrSandboxDenied)
	}

	// Check whitelist (always enforced except in danger-full-access).
	if mode != domain.SandboxModeDangerFullAccess {
		if _, ok := m.cmdWhitelist[name]; !ok {
			return fmt.Errorf("%w: command %q is not in the allowlist", ErrSandboxDenied, command)
		}
	}

	// In danger-full-access mode, still block obviously dangerous patterns.
	if mode == domain.SandboxModeDangerFullAccess {
		if isDangerousCommand(command, args) {
			return fmt.Errorf("%w: command pattern is too dangerous even in full-access mode", ErrSandboxDenied)
		}
	}

	return nil
}

// CheckNetwork validates whether a URL is accessible under the network policy.
func (m *SandboxManager) CheckNetwork(rawURL string) error {
	if m == nil {
		return errors.New("runtime: sandbox manager not initialized")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("runtime: parse URL: %w", err)
	}

	// Only check HTTP(S) URLs.
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: only HTTP/HTTPS URLs are allowed", ErrSandboxDenied)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w: URL has no hostname", ErrSandboxDenied)
	}

	m.mu.RLock()
	denyPrivate := m.netPolicy.DenyPrivateIPs
	allowedDomains := append([]string(nil), m.netPolicy.AllowedDomains...)
	m.mu.RUnlock()

	// Check if private IPs are denied.
	if denyPrivate {
		ips, err := net.LookupIP(host)
		if err == nil {
			for _, ip := range ips {
				if isPrivateIP(ip) {
					return fmt.Errorf("%w: access to private IP %s is denied", ErrSandboxDenied, ip.String())
				}
			}
		}
		// If DNS lookup fails, continue; we'll check the domain whitelist.
	}

	// Check domain whitelist if configured.
	if len(allowedDomains) > 0 {
		allowed := false
		for _, domain := range allowedDomains {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: domain %q is not in the allowed list", ErrSandboxDenied, host)
		}
	}

	return nil
}

// isPrivateIP checks whether an IP address is in RFC1918 private ranges
// or localhost.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check RFC1918 private ranges.
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7", // ULA
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}

	return false
}

// isDangerousCommand detects obviously destructive command patterns that
// should be blocked even in danger-full-access mode.
func isDangerousCommand(command string, args []string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))

	// Block format commands on Windows.
	if cmd == "format" || cmd == "diskpart" {
		return true
	}

	// Block recursive force-delete patterns.
	if cmd == "rm" || cmd == "del" {
		for _, arg := range args {
			arg = strings.ToLower(arg)
			if (arg == "-rf" || arg == "-fr" || arg == "/f" || arg == "/s") &&
				(arg == "/" || arg == "*" || arg == ".") {
				return true
			}
		}
	}

	return false
}
