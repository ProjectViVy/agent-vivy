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

// Message is one turn in a session. Content is append-only; there is no
// silent mutation path (FR-2). ToolCallID/ToolName/ToolArgs project a
// model-visible tool turn (ADR-010); they are empty on ordinary text rows.
type Message struct {
	ID         string
	SessionID  SessionID
	RunID      RunID // empty for user-authored messages
	Role       Role
	CreatedAt  int64 // unix milli
	Content    string
	ToolCallID string
	ToolName   string
	ToolArgs   []byte
}
