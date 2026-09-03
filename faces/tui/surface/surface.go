// Package surface is the shared read/command face for the fullscreen TUI.
// Demo and live drivers both implement Driver so view never imports a
// concrete store.
package surface

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Session is one sidebar row.
type Session struct {
	ID               string
	Title            string
	PermissionPreset string
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

// ErrMsg is shown in the status line when an async RPC fails.
type ErrMsg struct {
	Err error
}

// RefreshMsg asks the view to re-render after driver state changed.
// Drivers may return nil instead; the view always redraws after Update.
type RefreshMsg struct{}

// GateResolvedMsg lets the view clear local input only after the remote
// approval/question response was durably accepted.
type GateResolvedMsg struct{ Kind string }
