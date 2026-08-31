package domain

// Role is the author role of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Valid reports whether the role is part of the vocabulary.
func (r Role) Valid() bool {
	switch r {
	case RoleUser, RoleAssistant, RoleTool:
		return true
	}
	return false
}

// Session is one conversation. Sandbox fields control the permission
// boundary for all runs in this session (D-021).
type Session struct {
	ID             SessionID
	Title          string
	CreatedAt      int64  // unix milli
	SandboxMode    string // read_only | workspace_write | danger_full_access
	ApprovalPolicy string // ask | never | auto
}

// EffectiveSandbox returns the session's sandbox knobs, substituting the
// product defaults when a stored value is empty or unrecognized.
func (s Session) EffectiveSandbox() (SandboxMode, ApprovalPolicy) {
	mode := SandboxMode(s.SandboxMode)
	if !mode.Valid() {
		mode = SandboxModeWorkspaceWrite
	}
	policy := ApprovalPolicy(s.ApprovalPolicy)
	if !policy.Valid() {
		policy = ApprovalPolicyAsk
	}
	return mode, policy
}

// PermissionPreset is the named bundle matching EffectiveSandbox, or custom.
func (s Session) PermissionPreset() PermissionPreset {
	mode, policy := s.EffectiveSandbox()
	return PermissionPresetOf(mode, policy)
}

// Provenance marks the world entry of one user turn. The runtime stamps
// it onto the user message row; a nil Provenance (or an empty Source,
// see Message.EffectiveSource) means the built-in UI.
type Provenance struct {
	Source           string // "ui" | "channel"
	Channel          string // platform name, e.g. "telegram"; channel turns only
	ChatID           string // platform chat the turn arrived in; channel turns only
	ChannelMessageID string // platform-side message id; channel turns only
}

// Attachment is one binary image carried on a user message (VC-1g-2).
// Vivy accepts images only; the mime whitelist and size cap are enforced
// at the RPC boundary, storage persists the raw bytes as given.
type Attachment struct {
	Name     string
	MimeType string
	Data     []byte
}

// Message is one turn in a session. Content is append-only; there is no
// silent mutation path (FR-2). ToolCallID/ToolName/ToolArgs project a
// model-visible tool turn (ADR-010); they are empty on ordinary text rows.
// Provenance: Source is "ui" | "channel" (empty reads as ui, see
// EffectiveSource); Channel/ChatID/ChannelMessageID carry channel
// provenance and stay empty on ui rows.
type Message struct {
	ID               string
	SessionID        SessionID
	RunID            RunID // empty for user-authored messages
	Role             Role
	CreatedAt        int64 // unix milli
	Content          string
	Attachments      []Attachment // user rows only; images delivered as multimodal input
	ToolCallID       string
	ToolName         string
	ToolArgs         []byte
	Source           string
	Channel          string
	ChatID           string
	ChannelMessageID string
}

// EffectiveSource returns the provenance of this message; an empty Source
// (legacy rows, in-process appends) reads as the built-in UI.
func (m Message) EffectiveSource() string {
	if m.Source == "" {
		return "ui"
	}
	return m.Source
}
