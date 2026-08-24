package domain

// SandboxMode governs filesystem and command effects for a session.
// It mirrors the DeepSeek Harness three-tier permission model:
// read-only denies writes, workspace-write permits writes under the
// workspace root, danger-full-access bypasses confinement (D-021).
type SandboxMode string

const (
	// SandboxModeReadOnly denies all file writes and command execution.
	// Only read operations within the workspace are permitted.
	SandboxModeReadOnly SandboxMode = "read_only"

	// SandboxModeWorkspaceWrite permits reads and writes under the
	// configured workspace root, but blocks access outside it.
	SandboxModeWorkspaceWrite SandboxMode = "workspace_write"

	// SandboxModeDangerFullAccess bypasses sandbox confinement entirely.
	// This mode is intended only for explicitly trusted sessions and must
	// not be the default for new sessions.
	SandboxModeDangerFullAccess SandboxMode = "danger_full_access"
)

// Valid reports whether the sandbox mode is one of the recognized values.
func (m SandboxMode) Valid() bool {
	switch m {
	case SandboxModeReadOnly, SandboxModeWorkspaceWrite, SandboxModeDangerFullAccess:
		return true
	default:
		return false
	}
}

// IsConfined returns true if the mode enforces any sandbox boundary.
// danger-full-access is unconfined by definition.
func (m SandboxMode) IsConfined() bool {
	return m != SandboxModeDangerFullAccess
}

// ApprovalPolicy determines whether effectful tool calls require user
// approval before execution. The policy is evaluated per-session and can
// be changed at runtime (D-021).
type ApprovalPolicy string

const (
	// ApprovalPolicyAsk requires user approval for every effectful tool
	// call that is not auto-approved by the allowlist.
	ApprovalPolicyAsk ApprovalPolicy = "ask"

	// ApprovalPolicyNever denies all effectful tool calls without asking.
	// This is the strict headless stance for CI or unattended runs.
	ApprovalPolicyNever ApprovalPolicy = "never"

	// ApprovalPolicyAuto automatically approves readonly tools and
	// whitelisted commands; other effectful tools still require approval.
	ApprovalPolicyAuto ApprovalPolicy = "auto"
)

// Valid reports whether the approval policy is recognized.
func (p ApprovalPolicy) Valid() bool {
	switch p {
	case ApprovalPolicyAsk, ApprovalPolicyNever, ApprovalPolicyAuto:
		return true
	default:
		return false
	}
}

// SandboxPolicy carries the complete sandbox configuration for one session.
// The workspace root is carried even in read-only mode so callers can
// resolve policy once before choosing the enforcement path.
type SandboxPolicy struct {
	Mode          SandboxMode
	WorkspaceRoot string // Absolute root directory for workspace_write mode
	SessionID     string // Opaque session identity for backend state isolation
}

// NetworkPolicy defines allowed network destinations for HTTP requests.
// This prevents agents from accessing internal services or private IPs.
type NetworkPolicy struct {
	AllowedDomains   []string // Whitelist of permitted domains
	DenyPrivateIPs   bool     // Block RFC1918 private IP ranges
	MaxResponseBytes int      // Cap on response body size (0 = unlimited)
}
