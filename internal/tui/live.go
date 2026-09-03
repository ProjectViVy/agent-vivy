package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/surface"
)

// Live is a surface.Driver backed by a control-plane Client.
type Live struct {
	client         *Client
	host           string
	title          string
	face           string
	initialPrompt  string
	continueNewest bool

	mu sync.Mutex

	sessions []surface.Session
	messages map[string][]surface.Message
	activeID string

	busy    bool
	runID   string
	gate    *surface.Gate
	lastErr string
	queue   []string

	seq int
	// lastEventSeq is the latest contiguous durable event applied for the
	// active run. recovering prevents duplicate replay subscriptions.
	lastEventSeq     int
	recovering       bool
	recoveryInFlight bool
	nextRecoveryAt   time.Time

	eventMu   sync.Mutex
	events    []eventNotice
	eventWake chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

// LiveOptions configure one live fullscreen session.
type LiveOptions struct {
	Host           string
	Title          string
	Face           string
	InitialPrompt  string
	ContinueNewest bool
}

// NewLive wraps a connected client. Call Init from the Bubble Tea model.
func NewLive(client *Client, opts LiveOptions) *Live {
	if opts.Title == "" {
		opts.Title = "VIVY CODE"
	}
	if strings.TrimSpace(opts.Face) == "" {
		opts.Face = string(domain.FaceCode)
	}
	ctx, cancel := context.WithCancel(context.Background())
	l := &Live{
		client:         client,
		host:           strings.TrimSpace(opts.Host),
		title:          opts.Title,
		face:           opts.Face,
		initialPrompt:  strings.TrimSpace(opts.InitialPrompt),
		continueNewest: opts.ContinueNewest,
		messages:       map[string][]surface.Message{},
		eventWake:      make(chan struct{}, 1),
		ctx:            ctx,
		cancel:         cancel,
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
		if notice.Seq == 0 && notice.Kind == "" && notice.Delta == "" && notice.Gate == nil && !notice.Done {
			return
		}
		l.enqueueNotice(notice)
	})
	return l
}

// Close stops the live event pump context.
func (l *Live) Close() {
	if l.cancel != nil {
		l.cancel()
	}
	l.eventMu.Lock()
	l.events = nil
	l.eventMu.Unlock()
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
	footer := l.title
	if footer == "" {
		footer = "VIVY CODE"
	}
	if l.host != "" {
		footer += " · " + l.host
	}
	if l.busy {
		footer += " · run…"
	}
	if len(l.queue) > 0 {
		footer += fmt.Sprintf(" · queued %d", len(l.queue))
	}
	return surface.Meta{
		Mode:   "live",
		Host:   l.host,
		Busy:   l.busy,
		Queued: len(l.queue),
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

// liveTurnStartedMsg is returned after turn/start. Subscription starts after
// Handle installs run ownership, so replay cannot race the new run.
type liveTurnStartedMsg struct {
	UserText string
	RunID    string
	Err      error
}

type liveSubscribedMsg struct {
	RunID    string
	AfterSeq int
	Recovery bool
	Err      error
}

// liveRPCMsg is a generic RPC completion (approval, cancel, question).
type liveRPCMsg struct {
	Kind   string
	Preset string
	Err    error
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
		startFresh := len(out) == 0 || (l.initialPrompt != "" && !l.continueNewest)
		if startFresh {
			title := l.title
			if l.initialPrompt != "" {
				title = trimTitle(l.initialPrompt)
			}
			created, err := l.client.createSession(ctx, title)
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
		finished, gap := l.drainEvents()
		if gap {
			return tea.Batch(l.recoverSubscriptionCmd(), l.tickCmd())
		}
		if finished {
			return tea.Batch(l.dequeueCmd(), l.tickCmd())
		}
		return l.tickCmd()
	case liveBootMsg:
		return l.applyBoot(msg)
	case liveLoadedMsg:
		return l.applyLoaded(msg)
	case liveTurnStartedMsg:
		return l.applyTurnStarted(msg)
	case liveSubscribedMsg:
		return l.applySubscribed(msg)
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
	autoSend := ""
	l.mu.Lock()
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		if len(msg.Sessions) > 0 {
			l.sessions = msg.Sessions
			l.activeID = msg.ActiveID
		}
		l.mu.Unlock()
		return nil
	}
	l.sessions = msg.Sessions
	l.activeID = msg.ActiveID
	l.messages[msg.ActiveID] = msg.Messages
	l.lastErr = ""
	if l.initialPrompt != "" {
		autoSend = l.initialPrompt
		l.initialPrompt = ""
	}
	l.mu.Unlock()
	if autoSend != "" {
		return l.Send(autoSend)
	}
	return nil
}

func trimTitle(prompt string) string {
	runes := []rune(strings.TrimSpace(prompt))
	if len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return string(runes)
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
		l.mu.Unlock()
		return nil
	}
	l.busy = true
	l.runID = msg.RunID
	l.lastEventSeq = 0
	l.recovering = false
	l.recoveryInFlight = false
	l.nextRecoveryAt = time.Time{}
	l.lastErr = ""
	l.ensureAssistantDraftLocked()
	l.mu.Unlock()
	return l.subscribeCmd(msg.RunID, 0, false)
}

func (l *Live) subscribeCmd(runID string, afterSeq int, recovery bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
		defer cancel()
		return liveSubscribedMsg{RunID: runID, AfterSeq: afterSeq, Recovery: recovery, Err: l.client.subscribe(ctx, runID, afterSeq)}
	}
}

func (l *Live) applySubscribed(msg liveSubscribedMsg) tea.Cmd {
	l.mu.Lock()
	if l.runID != msg.RunID {
		l.mu.Unlock()
		return nil
	}
	if msg.Err == nil {
		if msg.Recovery {
			l.recoveryInFlight = false
		}
		l.mu.Unlock()
		return nil
	}
	// A newer subscription (or the original live stream) may already have
	// advanced the durable cursor while this RPC result was in flight.
	// Do not let that stale failure cancel a healthy run.
	if l.lastEventSeq > msg.AfterSeq {
		if msg.Recovery {
			l.recoveryInFlight = false
		}
		l.mu.Unlock()
		return nil
	}
	if msg.Recovery {
		l.recoveryInFlight = false
		l.nextRecoveryAt = time.Now().Add(time.Second)
		l.lastErr = "stream replay: " + shortErr(msg.Err)
		l.mu.Unlock()
		return nil
	}
	l.lastErr = shortErr(msg.Err)
	l.busy = false
	l.runID = ""
	l.recovering = false
	l.recoveryInFlight = false
	l.finishStreamingLocked()
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
		defer cancel()
		_ = l.client.cancelRun(ctx, msg.RunID)
		return surface.RefreshMsg{}
	}
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
	if msg.Kind == "permission" && msg.Preset != "" {
		for i := range l.sessions {
			if l.sessions[i].ID == l.activeID {
				l.sessions[i].PermissionPreset = msg.Preset
				break
			}
		}
	}
	l.lastErr = ""
	return nil
}

func (l *Live) drainEvents() (finished bool, gap bool) {
	l.eventMu.Lock()
	pending := l.events
	l.events = nil
	l.eventMu.Unlock()
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].Seq <= 0 {
			return false
		}
		if pending[j].Seq <= 0 {
			return true
		}
		return pending[i].Seq < pending[j].Seq
	})
	for i, notice := range pending {
		accept, missing := l.acceptSequence(notice)
		if missing {
			gap = true
			l.eventMu.Lock()
			l.events = append(append([]eventNotice(nil), pending[i:]...), l.events...)
			l.eventMu.Unlock()
			break
		}
		if !accept {
			continue
		}
		if notice.Kind != "" {
			l.applyNotice(notice)
		}
		finished = finished || notice.Kind == "done"
		if notice.Kind == "done" {
			break
		}
	}
	select {
	case <-l.eventWake:
	default:
	}
	return finished, gap
}

func (l *Live) acceptSequence(notice eventNotice) (accept bool, gap bool) {
	if notice.Seq <= 0 {
		return true, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if notice.RunID != "" && l.runID != "" && notice.RunID != l.runID {
		return false, false
	}
	if notice.Seq <= l.lastEventSeq {
		return false, false
	}
	if notice.Seq != l.lastEventSeq+1 {
		if !l.recovering {
			l.recovering = true
		}
		return false, true
	}
	l.lastEventSeq = notice.Seq
	l.recovering = false
	return true, false
}

func (l *Live) recoverSubscriptionCmd() tea.Cmd {
	l.mu.Lock()
	if l.recoveryInFlight || time.Now().Before(l.nextRecoveryAt) {
		l.mu.Unlock()
		return nil
	}
	runID, afterSeq := l.runID, l.lastEventSeq
	if runID != "" {
		l.recoveryInFlight = true
	}
	l.mu.Unlock()
	if runID == "" {
		return nil
	}
	return l.subscribeCmd(runID, afterSeq, true)
}

// enqueueNotice keeps the event stream ordered and lossless between UI
// ticks. eventWake only coalesces redraw notifications; notices are never
// discarded when the renderer is temporarily behind.
func (l *Live) enqueueNotice(notice eventNotice) {
	select {
	case <-l.ctx.Done():
		return
	default:
	}
	l.eventMu.Lock()
	l.events = append(l.events, notice)
	l.eventMu.Unlock()
	select {
	case l.eventWake <- struct{}{}:
	default:
	}
}

func (l *Live) dequeueCmd() tea.Cmd {
	l.mu.Lock()
	if l.busy || l.gate != nil || len(l.queue) == 0 {
		l.mu.Unlock()
		return nil
	}
	text := l.queue[0]
	l.queue = l.queue[1:]
	l.mu.Unlock()
	return l.Send(text)
}

func (l *Live) applyNotice(notice eventNotice) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if notice.RunID != "" && l.runID != "" && notice.RunID != l.runID {
		return
	}
	switch notice.Kind {
	case "delta":
		// Old journals and remote peers may still contain empty model.delta
		// events. They are not a reasoning/answer boundary.
		if notice.Delta == "" {
			return
		}
		msgs := l.messages[l.activeID]
		for i := range msgs {
			if msgs[i].Reasoning {
				msgs[i].Streaming = false
			}
		}
		l.messages[l.activeID] = msgs
		l.ensureAssistantDraftLocked()
		msgs = l.messages[l.activeID]
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == string(domain.RoleAssistant) && msgs[i].Streaming && !msgs[i].Reasoning {
				msgs[i].Content += notice.Delta
				l.messages[l.activeID] = msgs
				return
			}
		}
	case "reasoning":
		msgs := l.messages[l.activeID]
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Reasoning && msgs[i].Streaming {
				msgs[i].Content += notice.Delta
				l.messages[l.activeID] = msgs
				return
			}
			break
		}
		l.appendLocked(surface.Message{ID: l.nextID("thinking"), Role: string(domain.RoleAssistant), Content: notice.Delta, Streaming: true, Reasoning: true})
	case "tool_requested":
		l.finishStreamingLocked()
		l.appendLocked(surface.Message{
			ID:   l.nextID("tool"),
			Role: "tool",
			Tool: &surface.ToolCard{
				ToolName: notice.Message,
				Status:   "pending",
				Preview:  notice.Line,
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
					msgs[i].Tool.Result = notice.Line
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
	case "done":
		l.finishStreamingLocked()
		l.busy = false
		l.runID = ""
		l.recovering = false
		l.recoveryInFlight = false
		l.nextRecoveryAt = time.Time{}
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
		if msgs[i].Role == string(domain.RoleAssistant) && msgs[i].Streaming && !msgs[i].Reasoning {
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
		l.queue = append(l.queue, text)
		l.mu.Unlock()
		return func() tea.Msg { return surface.RefreshMsg{} }
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
		accepted, err := l.client.startTurn(ctx, sessionID, text, l.face)
		if err != nil {
			return liveTurnStartedMsg{UserText: text, Err: err}
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

// SetPermission persists the active session's cautious/smart/trusted bundle.
func (l *Live) SetPermission(preset string) tea.Cmd {
	preset = strings.TrimSpace(preset)
	if !domain.PermissionPreset(preset).ValidSwitch() {
		return nil
	}
	l.mu.Lock()
	sessionID := l.activeID
	busy := l.busy || l.gate != nil
	l.mu.Unlock()
	if sessionID == "" || busy {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		session, err := l.client.setSessionPermission(ctx, sessionID, preset)
		if session.PermissionPreset != "" {
			preset = session.PermissionPreset
		}
		return liveRPCMsg{Kind: "permission", Preset: preset, Err: err}
	}
}

func (l *Live) ClearQueue() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.queue) == 0 {
		return false
	}
	l.queue = nil
	return true
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
