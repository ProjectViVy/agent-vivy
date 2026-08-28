package domain

// PermissionPreset is the product-facing bundle of sandbox mode plus
// approval policy. Chat and Settings switch these three names; the
// runtime still enforces the two knobs independently.
type PermissionPreset string

const (
	// PermissionPresetCautious is read-only sandbox; effectful tools still ask.
	PermissionPresetCautious PermissionPreset = "cautious"
	// PermissionPresetSmart is workspace-write sandbox; effectful tools still ask.
	PermissionPresetSmart PermissionPreset = "smart"
	// PermissionPresetTrusted is unconfined sandbox with auto-approve
	// for readonly and allowlisted tools.
	PermissionPresetTrusted PermissionPreset = "trusted"
	// PermissionPresetCustom is derived when the two knobs do not match
	// a named preset. It is display-only and never a switch target.
	PermissionPresetCustom PermissionPreset = "custom"
)

// ValidSwitch reports whether the preset may be written by the user.
func (p PermissionPreset) ValidSwitch() bool {
	switch p {
	case PermissionPresetCautious, PermissionPresetSmart, PermissionPresetTrusted:
		return true
	default:
		return false
	}
}

// Bundle returns the sandbox mode and approval policy this preset writes.
func (p PermissionPreset) Bundle() (SandboxMode, ApprovalPolicy, bool) {
	switch p {
	case PermissionPresetCautious:
		return SandboxModeReadOnly, ApprovalPolicyAsk, true
	case PermissionPresetSmart:
		return SandboxModeWorkspaceWrite, ApprovalPolicyAsk, true
	case PermissionPresetTrusted:
		return SandboxModeDangerFullAccess, ApprovalPolicyAuto, true
	default:
		return "", "", false
	}
}

// PermissionPresetOf derives the named preset from the two knobs.
// Unmatched combinations are custom.
func PermissionPresetOf(mode SandboxMode, policy ApprovalPolicy) PermissionPreset {
	switch {
	case mode == SandboxModeReadOnly && policy == ApprovalPolicyAsk:
		return PermissionPresetCautious
	case mode == SandboxModeWorkspaceWrite && policy == ApprovalPolicyAsk:
		return PermissionPresetSmart
	case mode == SandboxModeDangerFullAccess && policy == ApprovalPolicyAuto:
		return PermissionPresetTrusted
	default:
		return PermissionPresetCustom
	}
}

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
