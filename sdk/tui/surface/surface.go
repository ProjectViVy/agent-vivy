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
	CreatedAt int64
	// UpdatedAt is the durable last-activity timestamp in unix milliseconds.
	// A zero value means the server did not expose activity truth (for example,
	// an older/offline driver), and must not be replaced with process time.
	UpdatedAt int64
}

// Context is the server-owned context pressure snapshot for one session.
// It is deliberately separate from Meta: context belongs to the active
// session, while Meta describes the transport/run chrome.
type Context struct {
	FeedTokens           int  `json:"feed_tokens"`
	ModelLimitTokens     int  `json:"model_limit_tokens"`
	TokenCountsEstimated bool `json:"token_counts_estimated"`
	ModelLimitKnown      bool `json:"model_limit_known"`
	TriggerTokens        int  `json:"trigger_tokens"`
	TotalMessages        int  `json:"total_messages"`
	FeedMessages         int  `json:"feed_messages"`
	ThinkingSupported    bool `json:"thinking_supported"`
	ImageSupportKnown    bool `json:"image_support_known"`
	ImageSupported       bool `json:"image_supported"`
	CompactionEnabled    bool `json:"compaction_enabled"`
	WouldCompact         bool `json:"would_compact"`
	HasCompactionSummary bool `json:"has_compaction_summary"`
}

// SidebarUsage is the authoritative session-wide token/cost aggregate. Cost
// is meaningful only when CostKnown is true; false is distinct from a free
// session.
type SidebarUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	ReasoningTokens  int
	CachedTokens     int
	RequestCount     int
	CostUSD          float64
	CostKnown        bool
}

// SidebarDiff is a bounded line-diff summary for one file version transition.
type SidebarDiff struct {
	Additions int
	Deletions int
}

// ModifiedFile is one project-relative file changed in the active session.
// Diff is the net line change between the oldest and newest retained snapshot;
// ordering is newest UpdatedAt first and is bounded by the control plane.
type ModifiedFile struct {
	Path      string
	Diff      SidebarDiff
	UpdatedAt int64
}

// MCPServer is one configured server and its current in-process handshake
// state. State is "configured" or "initialized"; it is never inferred by the
// terminal from a settings document.
type MCPServer struct {
	Name  string
	State string
}

// SidebarSkill is one enabled skill from the backend that supplies the
// runtime skill middleware. It describes availability, not per-turn use.
type SidebarSkill struct {
	Name string
}

// LanguageServer is one live process owned by a workspace that belongs to
// the active session. State is "starting" or "initialized".
type LanguageServer struct {
	Language string
	State    string
}

// Sidebar is the optional server-backed snapshot used by the Crush-style
// right rail. Missing fields remain missing; the view never infers them from
// process state or aggregate statistics.
type Sidebar struct {
	Session            Session
	CWD                string
	Model              string
	Provider           string
	ReasoningKnown     bool
	ReasoningSupported bool
	Context            Context
	HasContext         bool
	Usage              SidebarUsage
	HasUsage           bool
	ModifiedFiles      []ModifiedFile
	ModifiedFilesKnown bool
	MCP                []MCPServer
	MCPKnown           bool
	Skills             []SidebarSkill
	SkillsKnown        bool
	LSP                []LanguageServer
	LSPKnown           bool
}

// ToolCard is an inline tool result / pending approval inside the chat.
type ToolCard struct {
	ToolName   string
	ToolCallID string
	Status     string // pending | done | denied | failed
	Preview    string
	Result     string
	ApprovalID string
}

// Attachment is safe, terminal-facing metadata for one pending or persisted
// image. Raw image bytes deliberately never cross the TUI surface: the
// control plane resolves project-relative paths and the server owns the
// durable binary attachment.
type Attachment struct {
	Path     string `json:"path,omitempty"`
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// FileContext is metadata for one project-relative @file reference. The
// context body is intentionally absent from the terminal surface: the
// control plane resolves and bounds it again at turn/start time, while
// history only needs a safe path/name/size snapshot.
type FileContext struct {
	Path string `json:"path,omitempty"`
	Name string `json:"name,omitempty"`
	Size int64  `json:"size,omitempty"`
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
	// Attachments retains image metadata for history and compact rendering;
	// data is never included in a surface message.
	Attachments []Attachment
	// FileContexts retains @file metadata for history and compact rendering;
	// file contents never cross the TUI surface.
	FileContexts []FileContext
}

// Gate is the modal approval / question overlay.
type Gate struct {
	Kind             string // approval | question
	ID               string
	Title            string
	Body             string
	Action           string
	Target           string
	PreconditionHash string
	Preview          string
	Risks            []string
	Submitting       bool
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

// DynamicCommand is a server-authorized command that can change as installed
// skills or remote catalogs change. ID is opaque execution identity; Name is
// the slash spelling rendered by the client.
type DynamicCommand struct {
	ID          string
	Kind        string
	Name        string
	Usage       string
	Description string
	Arguments   []DynamicCommandArgument
}

type DynamicCommandArgument struct {
	Name        string
	Description string
	Required    bool
}

// DynamicCommandProvider exposes the latest authoritative catalog snapshot.
// Static commands remain available when this optional surface is absent.
type DynamicCommandProvider interface {
	DynamicCommands() []DynamicCommand
}

type DynamicCommandRefresher interface {
	RefreshDynamicCommands(request uint64) tea.Cmd
}

type DynamicCommandsMsg struct {
	Request  uint64
	Commands []DynamicCommand
	Err      error
}

// DynamicCommandExecutor expands an opaque dynamic command through the
// control plane. The resulting model input still travels through Driver.Send.
type DynamicCommandExecutor interface {
	ExecuteDynamicCommand(request uint64, sessionID, id string, args []string) tea.Cmd
}

// DynamicCommandExpandedMsg carries server-expanded model input back through
// Bubble Tea before it is sent, keeping async commands free of model mutation.
type DynamicCommandExpandedMsg struct {
	Request   uint64
	SessionID string
	ID        string
	Text      string
	Err       error
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

// ThinkingController owns the draft-time extended-thinking preference. The
// preference is local to this TUI instance and is snapshotted when a turn is
// sent or queued; the control plane remains authoritative about whether the
// active model supports thinking.
type ThinkingController interface {
	ThinkingMode() string
	SetThinkingMode(mode string) error
}

// ModelOption is one server-authorized model selection. BaseURL is carried
// back to the control plane as opaque selection identity and is deliberately
// never rendered by the terminal face.
type ModelOption struct {
	Provider    string
	Model       string
	BaseURL     string
	DisplayName string
	Current     bool
}

// ModelCatalog is the complete redacted candidate set returned by
// settings/providers. ReadOnly and Frozen keep the catalog browsable while
// making selection fail closed.
type ModelCatalog struct {
	Options  []ModelOption
	ReadOnly bool
	Frozen   bool
}

// ModelController owns the global active-model picker. Requests are echoed
// in result messages so a closed/reopened dialog cannot consume stale RPC
// results.
type ModelController interface {
	SupportsModelSelection() bool
	ModelCatalog() ModelCatalog
	RefreshModels(request uint64) tea.Cmd
	SelectModel(request uint64, option ModelOption) tea.Cmd
}

type ModelsMsg struct {
	Request uint64
	Catalog ModelCatalog
	Err     error
}

type ModelSelectedMsg struct {
	Request uint64
	Option  ModelOption
	Catalog ModelCatalog
	Err     error
}

// AttachmentProvider supplies the pending draft images owned by the active
// session. It is optional so offline/demo drivers can keep the base surface
// small; the shared renderer only shows chips when the driver has an
// authoritative provider.
type AttachmentProvider interface {
	PendingAttachments() []Attachment
}

// ContextSender is implemented by live drivers that can send project
// context paths. Paths are parsed by the shared command layer but remain
// untrusted hints; the driver sends them to the control plane, which resolves
// them again immediately before RunWithOptions.
type ContextSender interface {
	SendWithContext(text string, paths []string) tea.Cmd
}

// FileContextSender is the descriptive alias used by callers that prefer the
// file-oriented name. It intentionally has the same method set as
// ContextSender.
type FileContextSender = ContextSender

// ProjectFileCompleter asks the control plane for safe metadata-only project
// file candidates. Query is a user-entered project-relative path prefix; the
// view never scans the local filesystem. Request is echoed in ProjectFilesMsg
// so stale asynchronous responses can be rejected.
type ProjectFileCompleter interface {
	CompleteProjectFiles(request uint64, query string) tea.Cmd
}

// ProjectFilesMsg is the asynchronous result of one project file completion
// request. File bodies are deliberately absent.
type ProjectFilesMsg struct {
	Request   uint64
	Query     string
	Files     []FileContext
	Truncated bool
	Err       error
}

// ShellExecutor is the governed direct-shell seam for the !script input.
// Implementations must call the server-owned shell/start route; no terminal
// face may execute a process locally.
type ShellExecutor interface {
	ExecuteShell(script string) tea.Cmd
}

// CapabilityReporter exposes only capabilities returned by initialize.
// Optional effect surfaces use it to hide and reject unavailable actions.
type CapabilityReporter interface {
	SupportsCapability(name string) bool
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

// RestoreInputMsg returns a failed asynchronous submission to the editor.
// Text is reconstructed from already-parsed input and contains no file body.
type RestoreInputMsg struct{ Text string }

// GateResolvedMsg lets the view clear local input only after the remote
// approval/question response was durably accepted.
type GateResolvedMsg struct{ Kind string }
