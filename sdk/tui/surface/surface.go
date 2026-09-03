// Package surface defines the protocol-independent data and command surface
// shared by the built-in and packed fullscreen TUI faces.
package surface

import (
	tea "github.com/charmbracelet/bubbletea"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Session is one sidebar row.
type Session struct {
	ID               string
	Title            string
	PermissionPreset string
	// CreatedAt is the durable session creation time in unix milliseconds.
	// session/list currently exposes creation rather than an independent
	// update timestamp; consumers must not relabel it as UpdatedAt.
	CreatedAt int64
}

// Context is the server-owned context pressure snapshot for one session.
// It is deliberately separate from Meta: context belongs to the active
// session, while Meta describes the transport/run chrome.
type Context struct {
	FeedTokens           int  `json:"feed_tokens"`
	ModelLimitTokens     int  `json:"model_limit_tokens"`
	TriggerTokens        int  `json:"trigger_tokens"`
	TotalMessages        int  `json:"total_messages"`
	FeedMessages         int  `json:"feed_messages"`
	ThinkingSupported    bool `json:"thinking_supported"`
	CompactionEnabled    bool `json:"compaction_enabled"`
	WouldCompact         bool `json:"would_compact"`
	HasCompactionSummary bool `json:"has_compaction_summary"`
}

// Sidebar is the optional server-backed snapshot used by the Crush-style
// right rail. Missing fields remain missing; the view never infers them from
// process state or aggregate statistics.
type Sidebar struct {
	Session    Session
	Context    Context
	HasContext bool
}

// ToolCard is an inline tool result / pending approval inside the chat.
type ToolCard struct {
	ToolName   string
	Status     string // pending | done | denied | failed
	Preview    string
	Result     string
	ApprovalID string
}

// Message is one chat bubble or tool card.
type Message struct {
	ID      string
	Role    string // user | assistant | tool
	Content string
	Tool    *ToolCard
	// Streaming marks an in-progress assistant bubble (live tail cursor).
	Streaming bool
	Reasoning bool
}

// Gate is the modal approval / question overlay.
type Gate struct {
	Kind       string // approval | question
	ID         string
	Title      string
	Body       string
	Submitting bool
}

// Meta is footer / chrome status for the active driver.
type Meta struct {
	Mode   string // demo | live
	Host   string
	Busy   bool
	Queued int
	RunID  string
	Error  string
	Footer string // short status fragment after help keys
}

// Driver is the fullscreen shell's data plane + async commands.
// Read methods must be safe to call from View; mutations happen in
// Handle/Send/… and only apply state when their tea.Msg is handled.
type Driver interface {
	Sessions() []Session
	Active() Session
	ActiveMessages() []Message
	PendingGate() *Gate
	Meta() Meta

	Init() tea.Cmd
	Handle(msg tea.Msg) tea.Cmd
	MoveSession(delta int) tea.Cmd
	NewSession(title string) tea.Cmd
	Send(text string) tea.Cmd
	DecideApproval(decision string) tea.Cmd
	AnswerQuestion(answer string) tea.Cmd
	SetPermission(preset string) tea.Cmd
	ClearQueue() bool
	Cancel() tea.Cmd
}

// CommandExecutor is the optional command adapter implemented by live
// drivers. The shared view validates/parses command syntax and policy before
// calling this seam; the driver only translates an already-canonical command
// into its authoritative async operation. Drivers that do not implement it
// are handled by the view's safe compatibility path.
type CommandExecutor interface {
	ExecuteCommand(name string, args []string) tea.Cmd
}

// CommandResultMsg carries a local command result back into the shared Tea
// state. It is intentionally not printed directly: the view renders it in a
// transient overlay and keeps the packed and built-in faces identical.
type CommandResultMsg struct {
	Name      string
	Output    string
	Err       error
	Mutation  bool
	SessionID string
}

// SidebarProvider supplies authoritative active-session details to the view.
// It is optional so small offline drivers can render only the data they own.
type SidebarProvider interface {
	Sidebar() Sidebar
}

// SessionController supplies the independent Sessions dialog actions. The
// fullscreen view checks this interface rather than baking RPC knowledge into
// the shared renderer.
type SessionController interface {
	RefreshSessions() tea.Cmd
	SelectSession(id string) tea.Cmd
	RenameSession(id, title string) tea.Cmd
	DeleteSession(id string) tea.Cmd
}

// SessionsMsg is emitted by a SessionController after list or mutation RPCs.
// The driver handles it first to update its authoritative state; the shared
// view then consumes the same message to retain focus and error context.
type SessionsMsg struct {
	Action   string // list | rename | delete
	Request  uint64 // monotonic per driver; zero keeps compatibility with simple drivers
	ID       string
	Session  Session
	Sessions []Session
	Err      error
}

// ErrMsg is shown in the status line when an async RPC fails.
type ErrMsg struct {
	Err error
}

// RefreshMsg asks the view to re-render after driver state changed.
type RefreshMsg struct{}

// GateResolvedMsg lets the view clear local input only after the remote
// approval/question response was durably accepted.
type GateResolvedMsg struct{ Kind string }
