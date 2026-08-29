package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/surface"
)

// Live is a surface.Driver backed by a control-plane Client.
type Live struct {
	client *Client
	host   string
	title  string

	mu sync.Mutex

	sessions []surface.Session
	messages map[string][]surface.Message
	activeID string

	busy    bool
	runID   string
	gate    *surface.Gate
	lastErr string

	seq int

	events chan eventNotice
	ctx    context.Context
	cancel context.CancelFunc
}

// LiveOptions configure one live fullscreen session.
type LiveOptions struct {
	Host  string
	Title string
}

// NewLive wraps a connected client. Call Init from the Bubble Tea model.
func NewLive(client *Client, opts LiveOptions) *Live {
	if opts.Title == "" {
		opts.Title = "TUI"
	}
	ctx, cancel := context.WithCancel(context.Background())
	l := &Live{
		client:   client,
		host:     strings.TrimSpace(opts.Host),
		title:    opts.Title,
		messages: map[string][]surface.Message{},
		events:   make(chan eventNotice, 128),
		ctx:      ctx,
		cancel:   cancel,
	}
	client.OnNotify(func(method string, params json.RawMessage) {
		if method != "run/event" {
			return
		}
		event, ok := decodeStreamEvent(params)
		if !ok {
			return
		}
		notice := interpret(event)
		if notice.Kind == "" && notice.Delta == "" && notice.Gate == nil && !notice.Done {
			return
		}
		select {
		case l.events <- notice:
		case <-l.ctx.Done():
		default:
			// Drop if UI is stalled; next poll still drains what it can.
			select {
			case l.events <- notice:
			default:
			}
		}
	})
	return l
}

// Close stops the live event pump context.
func (l *Live) Close() {
	if l.cancel != nil {
		l.cancel()
	}
}

// Sessions implements surface.Driver.
func (l *Live) Sessions() []surface.Session {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]surface.Session(nil), l.sessions...)
}

// Active implements surface.Driver.
func (l *Live) Active() surface.Session {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, session := range l.sessions {
		if session.ID == l.activeID {
			return session
		}
	}
	return surface.Session{}
}

// ActiveMessages implements surface.Driver.
func (l *Live) ActiveMessages() []surface.Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]surface.Message(nil), l.messages[l.activeID]...)
}

// PendingGate implements surface.Driver.
func (l *Live) PendingGate() *surface.Gate {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.gate == nil {
		return nil
	}
	cp := *l.gate
	return &cp
}

// Meta implements surface.Driver.
func (l *Live) Meta() surface.Meta {
	l.mu.Lock()
	defer l.mu.Unlock()
	footer := "live"
	if l.host != "" {
		footer = "live · " + l.host
	}
	if l.busy {
		footer += " · run…"
	}
	return surface.Meta{
		Mode:   "live",
		Host:   l.host,
		Busy:   l.busy,
		RunID:  l.runID,
		Error:  l.lastErr,
		Footer: footer,
	}
}

// liveTickMsg is the periodic drain of the notify channel into Update.
type liveTickMsg struct{}

// liveBootMsg is the result of the initial session list / create.
type liveBootMsg struct {
	Sessions []surface.Session
	ActiveID string
	Messages []surface.Message
	Err      error
}

// liveLoadedMsg is history after a session switch / new session.
type liveLoadedMsg struct {
	Session  surface.Session
	Messages []surface.Message
	Replace  bool // true = set sessions list from Session only append path
	Sessions []surface.Session
	Err      error
}

// liveTurnStartedMsg is returned after turn/start + subscribe.
type liveTurnStartedMsg struct {
	UserText string
	RunID    string
	Err      error
}

// liveRPCMsg is a generic RPC completion (approval, cancel, question).
type liveRPCMsg struct {
	Kind string
	Err  error
}

// Init implements surface.Driver.
func (l *Live) Init() tea.Cmd {
	return tea.Batch(l.bootCmd(), l.tickCmd())
}

func (l *Live) tickCmd() tea.Cmd {
	return tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
		return liveTickMsg{}
	})
}

func (l *Live) bootCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		sessions, err := l.client.listSessions(ctx)
		if err != nil {
			return liveBootMsg{Err: err}
		}
		out := make([]surface.Session, 0, len(sessions))
		for _, s := range sessions {
			out = append(out, surface.Session{
				ID: s.ID, Title: s.Title, PermissionPreset: s.PermissionPreset,
			})
		}
		activeID := ""
		var messages []surface.Message
		if len(out) == 0 {
			created, err := l.client.createSession(ctx, l.title)
			if err != nil {
				return liveBootMsg{Err: err}
			}
			out = []surface.Session{{
				ID: created.ID, Title: created.Title, PermissionPreset: created.PermissionPreset,
			}}
			activeID = created.ID
		} else {
			activeID = out[0].ID
			msgs, err := l.client.sessionMessages(ctx, activeID)
			if err != nil {
				return liveBootMsg{Sessions: out, ActiveID: activeID, Err: err}
			}
			messages = mapHistory(msgs)
		}
		return liveBootMsg{Sessions: out, ActiveID: activeID, Messages: messages}
	}
}

// Handle implements surface.Driver.
func (l *Live) Handle(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case liveTickMsg:
		l.drainEvents()
		return l.tickCmd()
	case liveBootMsg:
		return l.applyBoot(msg)
	case liveLoadedMsg:
		return l.applyLoaded(msg)
	case liveTurnStartedMsg:
		return l.applyTurnStarted(msg)
	case liveRPCMsg:
		return l.applyRPC(msg)
	case surface.ErrMsg:
		l.mu.Lock()
		if msg.Err != nil {
			l.lastErr = shortErr(msg.Err)
		}
		l.mu.Unlock()
	}
	return nil
}

func (l *Live) applyBoot(msg liveBootMsg) tea.Cmd {
	l.mu.Lock()
	defer l.mu.Unlock()
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		if len(msg.Sessions) > 0 {
			l.sessions = msg.Sessions
			l.activeID = msg.ActiveID
		}
		return nil
	}
	l.sessions = msg.Sessions
	l.activeID = msg.ActiveID
	l.messages[msg.ActiveID] = msg.Messages
	l.lastErr = ""
	return nil
}

func (l *Live) applyLoaded(msg liveLoadedMsg) tea.Cmd {
	l.mu.Lock()
	defer l.mu.Unlock()
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		return nil
	}
	if len(msg.Sessions) > 0 {
		l.sessions = msg.Sessions
	}
	if msg.Session.ID != "" {
		found := false
		for i, s := range l.sessions {
			if s.ID == msg.Session.ID {
				l.sessions[i] = msg.Session
				found = true
				break
			}
		}
		if !found {
			l.sessions = append(l.sessions, msg.Session)
		}
		l.activeID = msg.Session.ID
	}
	if msg.Session.ID != "" {
		l.messages[msg.Session.ID] = msg.Messages
	}
	l.gate = nil
	l.busy = false
	l.runID = ""
	l.lastErr = ""
	return nil
}

func (l *Live) applyTurnStarted(msg liveTurnStartedMsg) tea.Cmd {
	l.mu.Lock()
	defer l.mu.Unlock()
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		l.busy = false
		l.runID = ""
		// Keep the optimistic user bubble; append an error assistant line.
		l.appendLocked(surface.Message{
			ID:      l.nextID("err"),
			Role:    string(domain.RoleAssistant),
			Content: "turn failed: " + shortErr(msg.Err),
		})
		return nil
	}
	l.busy = true
	l.runID = msg.RunID
	l.lastErr = ""
	l.ensureAssistantDraftLocked()
	return nil
}

func (l *Live) applyRPC(msg liveRPCMsg) tea.Cmd {
	l.mu.Lock()
	defer l.mu.Unlock()
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		return nil
	}
	if msg.Kind == "approval" || msg.Kind == "question" {
		l.gate = nil
	}
	l.lastErr = ""
	return nil
}

func (l *Live) drainEvents() {
	for {
		select {
		case notice := <-l.events:
			l.applyNotice(notice)
		default:
			return
		}
	}
}

func (l *Live) applyNotice(notice eventNotice) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if notice.RunID != "" && l.runID != "" && notice.RunID != l.runID {
		return
	}
	switch notice.Kind {
	case "delta":
		l.ensureAssistantDraftLocked()
		msgs := l.messages[l.activeID]
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == string(domain.RoleAssistant) && msgs[i].Streaming {
				msgs[i].Content += notice.Delta
				l.messages[l.activeID] = msgs
				return
			}
		}
	case "tool_requested":
		l.finishStreamingLocked()
		l.appendLocked(surface.Message{
			ID:   l.nextID("tool"),
			Role: "tool",
			Tool: &surface.ToolCard{
				ToolName: notice.Message,
				Status:   "pending",
				Preview:  "",
			},
		})
	case "tool_finished":
		l.finishStreamingLocked()
		msgs := l.messages[l.activeID]
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Tool != nil && msgs[i].Tool.ToolName == notice.Message {
				if notice.Failed {
					msgs[i].Tool.Status = "failed"
					msgs[i].Tool.Result = notice.Line
				} else {
					msgs[i].Tool.Status = "done"
					if msgs[i].Tool.Result == "" {
						msgs[i].Tool.Result = "done"
					}
				}
				l.messages[l.activeID] = msgs
				return
			}
		}
	case "gate":
		l.finishStreamingLocked()
		if notice.Gate == nil {
			return
		}
		l.gate = &surface.Gate{
			Kind:  notice.Gate.Kind,
			ID:    notice.Gate.ID,
			Title: notice.Gate.Title,
			Body:  notice.Gate.Body,
		}
		if notice.Gate.Kind == "approval" {
			// Ensure a pending tool card exists for the overlay pair.
			found := false
			msgs := l.messages[l.activeID]
			for i := range msgs {
				if msgs[i].Tool != nil && msgs[i].Tool.ApprovalID == notice.Gate.ID {
					msgs[i].Tool.Status = "pending"
					found = true
					break
				}
			}
			if !found {
				l.appendLocked(surface.Message{
					ID:   l.nextID("tool"),
					Role: "tool",
					Tool: &surface.ToolCard{
						ToolName:   notice.Gate.Title,
						Status:     "pending",
						Preview:    notice.Gate.Body,
						ApprovalID: notice.Gate.ID,
					},
				})
			}
		}
	case "line":
		// reasoning / misc — ignore in chat projection for the thin face
	case "done":
		l.finishStreamingLocked()
		l.busy = false
		l.runID = ""
		l.gate = nil
		if notice.Failed && notice.Message != "" {
			l.appendLocked(surface.Message{
				ID:      l.nextID("end"),
				Role:    string(domain.RoleAssistant),
				Content: "[" + notice.Message + "]",
			})
		}
	}
}

func (l *Live) ensureAssistantDraftLocked() {
	msgs := l.messages[l.activeID]
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == string(domain.RoleAssistant) && msgs[i].Streaming {
			return
		}
		// If last assistant is complete and we stream again, fall through.
		break
	}
	l.appendLocked(surface.Message{
		ID:        l.nextID("asst"),
		Role:      string(domain.RoleAssistant),
		Content:   "",
		Streaming: true,
	})
}

func (l *Live) finishStreamingLocked() {
	msgs := l.messages[l.activeID]
	for i := range msgs {
		if msgs[i].Streaming {
			msgs[i].Streaming = false
		}
	}
	l.messages[l.activeID] = msgs
}

func (l *Live) appendLocked(msg surface.Message) {
	if l.messages == nil {
		l.messages = map[string][]surface.Message{}
	}
	l.messages[l.activeID] = append(l.messages[l.activeID], msg)
}

func (l *Live) nextID(prefix string) string {
	l.seq++
	return fmt.Sprintf("%s_%d", prefix, l.seq)
}

// MoveSession implements surface.Driver.
func (l *Live) MoveSession(delta int) tea.Cmd {
	l.mu.Lock()
	if l.busy || len(l.sessions) == 0 {
		l.mu.Unlock()
		return nil
	}
	idx := 0
	for i, session := range l.sessions {
		if session.ID == l.activeID {
			idx = i
			break
		}
	}
	n := len(l.sessions)
	idx = (idx + delta) % n
	if idx < 0 {
		idx += n
	}
	target := l.sessions[idx]
	l.mu.Unlock()
	return l.loadSessionCmd(target.ID)
}

// NewSession implements surface.Driver.
func (l *Live) NewSession(title string) tea.Cmd {
	l.mu.Lock()
	if l.busy {
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()
	title = strings.TrimSpace(title)
	if title == "" {
		title = l.title
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		created, err := l.client.createSession(ctx, title)
		if err != nil {
			return liveLoadedMsg{Err: err}
		}
		return liveLoadedMsg{
			Session: surface.Session{
				ID: created.ID, Title: created.Title, PermissionPreset: created.PermissionPreset,
			},
			Messages: nil,
		}
	}
}

func (l *Live) loadSessionCmd(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		msgs, err := l.client.sessionMessages(ctx, id)
		if err != nil {
			return liveLoadedMsg{Err: err}
		}
		// Title may be unknown if only id known — keep existing row title via ID match.
		l.mu.Lock()
		session := surface.Session{ID: id}
		for _, s := range l.sessions {
			if s.ID == id {
				session = s
				break
			}
		}
		l.mu.Unlock()
		return liveLoadedMsg{
			Session:  session,
			Messages: mapHistory(msgs),
		}
	}
}

// Send implements surface.Driver.
func (l *Live) Send(text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	l.mu.Lock()
	if l.busy || l.gate != nil {
		l.mu.Unlock()
		return nil
	}
	sessionID := l.activeID
	l.appendLocked(surface.Message{
		ID:      l.nextID("user"),
		Role:    string(domain.RoleUser),
		Content: text,
	})
	l.busy = true
	l.mu.Unlock()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
		defer cancel()
		accepted, err := l.client.startTurn(ctx, sessionID, text)
		if err != nil {
			return liveTurnStartedMsg{UserText: text, Err: err}
		}
		if err := l.client.subscribe(ctx, accepted.RunID); err != nil {
			return liveTurnStartedMsg{UserText: text, RunID: accepted.RunID, Err: err}
		}
		return liveTurnStartedMsg{UserText: text, RunID: accepted.RunID}
	}
}

// DecideApproval implements surface.Driver.
func (l *Live) DecideApproval(decision string) tea.Cmd {
	l.mu.Lock()
	gate := l.gate
	if gate == nil || gate.Kind != "approval" || gate.ID == "" {
		l.mu.Unlock()
		return nil
	}
	id := gate.ID
	// Optimistic tool card update.
	msgs := l.messages[l.activeID]
	for i := range msgs {
		if msgs[i].Tool != nil && (msgs[i].Tool.ApprovalID == id || msgs[i].Tool.Status == "pending") {
			if decision == domain.ApprovalApproved {
				msgs[i].Tool.Status = "done"
				msgs[i].Tool.Result = "approved"
			} else {
				msgs[i].Tool.Status = "denied"
				msgs[i].Tool.Result = "denied"
			}
			break
		}
	}
	l.messages[l.activeID] = msgs
	l.gate = nil
	l.mu.Unlock()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		err := l.client.respondApproval(ctx, id, decision)
		return liveRPCMsg{Kind: "approval", Err: err}
	}
}

// AnswerQuestion implements surface.Driver.
func (l *Live) AnswerQuestion(answer string) tea.Cmd {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil
	}
	l.mu.Lock()
	gate := l.gate
	if gate == nil || gate.Kind != "question" || gate.ID == "" {
		l.mu.Unlock()
		return nil
	}
	id := gate.ID
	l.gate = nil
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		err := l.client.respondQuestion(ctx, id, answer)
		return liveRPCMsg{Kind: "question", Err: err}
	}
}

// Cancel implements surface.Driver.
func (l *Live) Cancel() tea.Cmd {
	l.mu.Lock()
	runID := l.runID
	if !l.busy || runID == "" {
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
		defer cancel()
		err := l.client.cancelRun(ctx, runID)
		return liveRPCMsg{Kind: "cancel", Err: err}
	}
}

func mapHistory(msgs []messageView) []surface.Message {
	out := make([]surface.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, surface.Message{
			ID:      m.ID,
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return out
}

func shortErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 80 {
		return s[:77] + "…"
	}
	return s
}

var _ surface.Driver = (*Live)(nil)
