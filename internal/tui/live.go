package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/surface"
	"agent-vivy/sdk/tui/command"
	"agent-vivy/sdk/tui/stream"
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

	sessions       []surface.Session
	messages       map[string][]surface.Message
	activeID       string
	sidebar        surface.Sidebar
	loadRequest    uint64
	sessionRequest uint64
	loadPending    bool

	busy    bool
	runID   string
	gate    *surface.Gate
	lastErr string
	queue   []queuedTurn
	// drafts is keyed by session id so switching/new sessions cannot carry
	// unsent image chips into another conversation.
	drafts          map[string][]surface.Attachment
	thinkingMode    string
	commandInFlight bool

	seq int
	// cursor is the shared durable stream reducer state. The local fields
	// below are transport subscription bookkeeping only.
	cursor               stream.Cursor
	subscriptionID       string
	subscriptionRequest  uint64
	closed               bool
	recoveryInFlight     bool
	recoveryNeeded       bool
	replayPending        bool
	nextRecoveryAt       time.Time
	streamFailures       map[string]string
	retiredSubscriptions map[string]struct{}

	inbox     stream.Inbox
	eventWake chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

type queuedTurn struct {
	SessionID    string
	Text         string
	Thinking     string
	Attachments  []surface.Attachment
	ContextPaths []string
	ShellScript  string
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
		drafts:         map[string][]surface.Attachment{},
		thinkingMode:   "auto",
		eventWake:      make(chan struct{}, 1),
		ctx:            ctx,
		cancel:         cancel,
	}
	client.OnNotify(func(method string, params json.RawMessage) {
		switch method {
		case "run/event":
			event, ok := decodeStreamEvent(params)
			if !ok {
				return
			}
			notice := interpret(event)
			if notice.Seq == 0 && notice.Kind == "" && notice.Delta == "" && notice.Gate == nil && !notice.Done {
				return
			}
			l.enqueueNotice(notice)
		case "run/stream_error":
			if failure, ok := stream.DecodeStreamError(params); ok {
				l.recordStreamError(failure)
			}
		}
	})
	return l
}

// Close stops the live event pump context.
func (l *Live) Close() {
	l.mu.Lock()
	l.closed = true
	l.subscriptionRequest++
	subscriptionID := l.subscriptionID
	l.subscriptionID = ""
	l.mu.Unlock()
	if subscriptionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		l.unsubscribeWithRetry(ctx, subscriptionID)
		cancel()
	}
	if l.cancel != nil {
		l.cancel()
	}
	l.inbox.Close()
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

// Sidebar implements surface.SidebarProvider. The snapshot contains only
// facts returned by the control plane for the active session; unavailable
// sections stay absent instead of being inferred from process state.
func (l *Live) Sidebar() surface.Sidebar {
	l.mu.Lock()
	defer l.mu.Unlock()
	snapshot := l.sidebar
	if snapshot.Session.ID == "" {
		for _, session := range l.sessions {
			if session.ID == l.activeID {
				snapshot.Session = session
				break
			}
		}
	}
	return snapshot
}

func (l *Live) activeSessionLocked() surface.Session {
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

// PendingAttachments implements surface.AttachmentProvider. Only metadata
// returned by attachments/resolve is exposed to the renderer.
func (l *Live) PendingAttachments() []surface.Attachment {
	l.mu.Lock()
	defer l.mu.Unlock()
	return cloneAttachments(l.drafts[l.activeID])
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
	Sidebar  surface.Sidebar
	Err      error
}

// liveLoadedMsg is history after a session switch / new session.
type liveLoadedMsg struct {
	Request  uint64
	Session  surface.Session
	Messages []surface.Message
	Sidebar  surface.Sidebar
	Replace  bool // true = set sessions list from Session only append path
	Sessions []surface.Session
	Err      error
}

// liveTurnStartedMsg is returned after turn/start. Subscription starts after
// Handle installs run ownership, so replay cannot race the new run.
type liveTurnStartedMsg struct {
	SessionID    string
	UserText     string
	RunID        string
	Attachments  []surface.Attachment
	ContextPaths []string
	FileContexts []surface.FileContext
	Shell        bool
	Err          error
}

type liveAttachmentResolvedMsg struct {
	SessionID   string
	Attachments []surface.Attachment
	Err         error
}

type liveSubscribedMsg struct {
	RunID          string
	AfterSeq       int
	Recovery       bool
	Request        uint64
	SubscriptionID string
	Err            error
}

// liveRPCMsg is a generic RPC completion (approval, cancel, question).
type liveRPCMsg struct {
	Kind    string
	Preset  string
	Outcome string
	Err     error
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
				ID: s.ID, Title: s.Title, PermissionPreset: s.PermissionPreset, CreatedAt: s.CreatedAt,
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
				ID: created.ID, Title: created.Title, PermissionPreset: created.PermissionPreset, CreatedAt: created.CreatedAt,
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
		snapshot := surface.Sidebar{}
		if activeID != "" {
			for _, session := range out {
				if session.ID == activeID {
					snapshot.Session = session
					break
				}
			}
			if contextStatus, contextErr := l.client.sessionContext(ctx, activeID); contextErr == nil {
				snapshot.Context = mapContextView(contextStatus)
				snapshot.HasContext = true
			}
		}
		return liveBootMsg{Sessions: out, ActiveID: activeID, Messages: messages, Sidebar: snapshot}
	}
}

// Handle implements surface.Driver.
func (l *Live) Handle(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case liveTickMsg:
		finished, gap := l.drainEvents()
		if gap {
			l.markRecoveryNeeded()
		}
		if finished {
			return tea.Batch(l.dequeueCmd(), l.tickCmd())
		}
		return tea.Batch(l.recoverSubscriptionCmd(), l.tickCmd())
	case liveBootMsg:
		return l.applyBoot(msg)
	case liveLoadedMsg:
		return l.applyLoaded(msg)
	case liveTurnStartedMsg:
		return l.applyTurnStarted(msg)
	case liveAttachmentResolvedMsg:
		return l.applyAttachmentResolved(msg)
	case liveSubscribedMsg:
		return l.applySubscribed(msg)
	case liveRPCMsg:
		return l.applyRPC(msg)
	case surface.CommandResultMsg:
		return l.applyCommandResult(msg)
	case surface.SessionsMsg:
		return l.applySessionsMsg(msg)
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
	l.sidebar = msg.Sidebar
	if l.sidebar.Session.ID == "" {
		l.sidebar.Session = l.activeSessionLocked()
	}
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

func restoreFileInputCmd(text string, paths []string) tea.Cmd {
	retry := strings.TrimSpace(text)
	for _, path := range paths {
		retry += " @" + path
	}
	return func() tea.Msg { return surface.RestoreInputMsg{Text: retry} }
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
	if msg.Request > 0 && msg.Request != l.loadRequest {
		l.mu.Unlock()
		return nil
	}
	l.loadPending = false
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		l.mu.Unlock()
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
		l.sidebar = msg.Sidebar
		if l.sidebar.Session.ID == "" {
			l.sidebar.Session = msg.Session
		}
		if l.thinkingMode == "on" && (!l.sidebar.HasContext || !l.sidebar.Context.ThinkingSupported) {
			l.thinkingMode = "auto"
		}
	}
	l.gate = nil
	l.busy = false
	l.runID = ""
	subscriptionID := l.subscriptionID
	l.retireSubscriptionLocked(subscriptionID)
	l.subscriptionID = ""
	l.cursor.Reset()
	l.recoveryInFlight = false
	l.recoveryNeeded = false
	l.replayPending = false
	l.lastErr = ""
	l.mu.Unlock()
	return l.unsubscribeCmd(subscriptionID)
}

func (l *Live) applyTurnStarted(msg liveTurnStartedMsg) tea.Cmd {
	l.mu.Lock()
	if msg.SessionID != "" && l.activeID != "" && msg.SessionID != l.activeID {
		// The session may have changed while turn/start was in flight. Keep a
		// failed draft attached to its origin, but never mutate the new
		// session's busy state or transcript.
		if msg.Err != nil {
			if l.drafts == nil {
				l.drafts = make(map[string][]surface.Attachment)
			}
			l.drafts[msg.SessionID] = append(l.drafts[msg.SessionID], cloneAttachments(msg.Attachments)...)
		}
		l.mu.Unlock()
		return nil
	}
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		l.busy = false
		l.runID = ""
		if msg.SessionID != "" {
			if l.drafts == nil {
				l.drafts = make(map[string][]surface.Attachment)
			}
			l.drafts[msg.SessionID] = append(l.drafts[msg.SessionID], cloneAttachments(msg.Attachments)...)
		}
		// Keep the optimistic user bubble; append an error assistant line.
		l.appendLocked(surface.Message{
			ID:      l.nextID("err"),
			Role:    string(domain.RoleAssistant),
			Content: "turn failed: " + shortErr(msg.Err),
		})
		l.mu.Unlock()
		if len(msg.ContextPaths) > 0 {
			return restoreFileInputCmd(msg.UserText, msg.ContextPaths)
		}
		return nil
	}
	if len(msg.FileContexts) > 0 {
		// The resolver's metadata is authoritative for the optimistic user
		// bubble. The body itself never enters the TUI state.
		messages := l.messages[l.activeID]
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == string(domain.RoleUser) && messages[i].Content == msg.UserText {
				messages[i].FileContexts = cloneFileContexts(msg.FileContexts)
				break
			}
		}
		l.messages[l.activeID] = messages
	}
	l.busy = true
	l.runID = msg.RunID
	l.cursor.Reset()
	l.subscriptionID = ""
	l.recoveryInFlight = false
	l.recoveryNeeded = false
	l.replayPending = false
	l.nextRecoveryAt = time.Time{}
	l.streamFailures = nil
	l.lastErr = ""
	l.ensureAssistantDraftLocked()
	l.mu.Unlock()
	return l.subscribeCmd(msg.RunID, 0, false)
}

func (l *Live) subscribeCmd(runID string, afterSeq int, recovery bool) tea.Cmd {
	l.mu.Lock()
	l.subscriptionRequest++
	request := l.subscriptionRequest
	closed := l.closed
	l.mu.Unlock()
	return func() tea.Msg {
		if closed {
			return liveSubscribedMsg{RunID: runID, AfterSeq: afterSeq, Recovery: recovery, Request: request, Err: context.Canceled}
		}
		ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
		defer cancel()
		subscriptionID, err := l.client.subscribe(ctx, runID, afterSeq)
		return liveSubscribedMsg{RunID: runID, AfterSeq: afterSeq, Recovery: recovery, Request: request, SubscriptionID: subscriptionID, Err: err}
	}
}

func (l *Live) applySubscribed(msg liveSubscribedMsg) tea.Cmd {
	l.mu.Lock()
	if l.closed || l.runID != msg.RunID || msg.Request < l.subscriptionRequest {
		l.retireSubscriptionLocked(msg.SubscriptionID)
		l.mu.Unlock()
		return l.unsubscribeCmd(msg.SubscriptionID)
	}
	if msg.Err == nil {
		previous := l.subscriptionID
		l.retireSubscriptionLocked(previous)
		l.subscriptionID = msg.SubscriptionID
		streamFailed := false
		if message, failed := l.streamFailures[msg.SubscriptionID]; failed {
			l.markStreamFailureLocked(msg.SubscriptionID, message)
			streamFailed = true
		}
		l.streamFailures = nil
		if msg.Recovery && !streamFailed {
			l.recoveryInFlight = false
			l.recoveryNeeded = false
			l.replayPending = true
			l.nextRecoveryAt = time.Now().Add(2 * time.Second)
		}
		l.mu.Unlock()
		if previous != "" && previous != msg.SubscriptionID {
			return l.unsubscribeCmd(previous)
		}
		return nil
	}
	// A newer subscription (or the original live stream) may already have
	// advanced the durable cursor while this RPC result was in flight.
	// Do not let that stale failure cancel a healthy run.
	if l.cursor.LastSeq > msg.AfterSeq {
		if msg.Recovery {
			l.recoveryInFlight = false
		}
		l.mu.Unlock()
		return nil
	}
	if msg.Recovery {
		l.recoveryInFlight = false
		l.recoveryNeeded = true
		l.replayPending = false
		l.nextRecoveryAt = time.Now().Add(time.Second)
		l.lastErr = "stream replay: " + shortErr(msg.Err)
		l.mu.Unlock()
		return nil
	}
	l.lastErr = shortErr(msg.Err)
	l.busy = false
	l.runID = ""
	l.cursor.Reset()
	l.recoveryInFlight = false
	l.recoveryNeeded = false
	l.replayPending = false
	l.finishStreamingLocked()
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = l.client.cancelRun(ctx, msg.RunID)
		return surface.RefreshMsg{}
	}
}

func (l *Live) unsubscribeCmd(subscriptionID string) tea.Cmd {
	if subscriptionID == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		l.unsubscribeWithRetry(ctx, subscriptionID)
		return nil
	}
}

func (l *Live) unsubscribeWithRetry(ctx context.Context, subscriptionID string) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := l.client.unsubscribe(ctx, subscriptionID); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (l *Live) applyRPC(msg liveRPCMsg) tea.Cmd {
	l.mu.Lock()
	defer l.mu.Unlock()
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		if l.gate != nil && l.gate.Kind == msg.Kind {
			l.gate.Submitting = false
		}
		return nil
	}
	if msg.Kind == "approval" || msg.Kind == "question" {
		if msg.Kind == "approval" && l.gate != nil {
			for i := range l.messages[l.activeID] {
				tool := l.messages[l.activeID][i].Tool
				if tool != nil && tool.ApprovalID == l.gate.ID {
					tool.Status = "done"
					tool.Result = msg.Outcome
				}
			}
		}
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
	if msg.Kind == "approval" || msg.Kind == "question" {
		return func() tea.Msg { return surface.GateResolvedMsg{Kind: msg.Kind} }
	}
	return nil
}

func (l *Live) applyCommandResult(msg surface.CommandResultMsg) tea.Cmd {
	if !msg.Mutation {
		return nil
	}
	l.mu.Lock()
	l.commandInFlight = false
	sameSession := msg.SessionID == "" || msg.SessionID == l.activeID
	l.mu.Unlock()
	if msg.Err != nil || !sameSession {
		return nil
	}
	switch msg.Name {
	case "compact", "rewind":
		return l.loadSessionCmd(msg.SessionID)
	case "fork":
		var result struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(msg.Output), &result); err != nil || strings.TrimSpace(result.SessionID) == "" {
			return nil
		}
		l.mu.Lock()
		found := false
		for _, session := range l.sessions {
			if session.ID == result.SessionID {
				found = true
				break
			}
		}
		if !found {
			l.sessions = append(l.sessions, surface.Session{ID: result.SessionID})
		}
		l.mu.Unlock()
		return l.loadSessionCmd(result.SessionID)
	}
	return nil
}

func (l *Live) drainEvents() (finished bool, gap bool) {
	pending, overflow := l.inbox.TakeBatch(stream.DefaultDrainItems, stream.DefaultDrainBytes)
	stream.Order(pending)
	for i, notice := range pending {
		accept, missing := l.acceptSequence(notice)
		if missing {
			gap = true
			l.inbox.Prepend(pending[i:])
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
	if gap || overflow {
		l.markRecoveryNeeded()
	}
	return finished, gap || overflow
}

func (l *Live) acceptSequence(notice eventNotice) (accept bool, gap bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if notice.SubscriptionID != "" {
		if _, retired := l.retiredSubscriptions[notice.SubscriptionID]; retired {
			return false, false
		}
	}
	accept, gap = l.cursor.Accept(l.runID, stream.Notice(notice))
	if accept && notice.Seq > 0 {
		l.recoveryNeeded = false
		l.replayPending = false
		l.nextRecoveryAt = time.Time{}
	}
	return accept, gap
}

func (l *Live) recoverSubscriptionCmd() tea.Cmd {
	l.mu.Lock()
	now := time.Now()
	if !l.recoveryNeeded || l.recoveryInFlight || l.replayPending || now.Before(l.nextRecoveryAt) {
		l.mu.Unlock()
		return nil
	}
	l.replayPending = false
	runID, afterSeq := l.runID, l.cursor.LastSeq
	if runID != "" {
		l.recoveryInFlight = true
	}
	l.mu.Unlock()
	if runID == "" {
		return nil
	}
	return l.subscribeCmd(runID, afterSeq, true)
}

func (l *Live) markRecoveryNeeded() {
	l.mu.Lock()
	l.markRecoveryNeededLocked()
	l.mu.Unlock()
}

func (l *Live) markRecoveryNeededLocked() {
	if l.runID != "" {
		l.recoveryNeeded = true
		l.replayPending = false
		l.nextRecoveryAt = time.Time{}
	}
}

func (l *Live) recordStreamError(failure stream.StreamError) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.runID == "" {
		return
	}
	if _, retired := l.retiredSubscriptions[failure.SubscriptionID]; retired {
		return
	}
	if failure.SubscriptionID == l.subscriptionID {
		l.markStreamFailureLocked(failure.SubscriptionID, failure.Message)
		return
	}
	if l.streamFailures == nil {
		l.streamFailures = make(map[string]string)
	}
	if len(l.streamFailures) >= 4 {
		for id := range l.streamFailures {
			delete(l.streamFailures, id)
			break
		}
	}
	l.streamFailures[failure.SubscriptionID] = failure.Message
}

func (l *Live) markStreamFailureLocked(subscriptionID, message string) {
	l.retireSubscriptionLocked(subscriptionID)
	if l.subscriptionID == subscriptionID {
		l.subscriptionID = ""
	}
	l.recoveryInFlight = false
	l.recoveryNeeded = true
	l.replayPending = false
	l.nextRecoveryAt = time.Now().Add(250 * time.Millisecond)
	if strings.TrimSpace(message) == "" {
		message = "event replay failed"
	}
	l.lastErr = "stream replay: " + message
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
	if l.inbox.Push(stream.Notice(notice)) == stream.PushReplayRequired {
		l.mu.Lock()
		if l.runID != "" {
			l.markRecoveryNeededLocked()
			if notice.Seq <= 0 {
				l.lastErr = "stream backlog overflow: an unsequenced event could not be replayed"
			}
		}
		l.mu.Unlock()
	}
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
	turn := l.queue[0]
	if turn.SessionID != "" && turn.SessionID != l.activeID {
		// Keep the complete queued turn (text and image snapshot) in place
		// until its originating session is active again. Dropping only the
		// image draft here would silently lose the user's queued text.
		l.mu.Unlock()
		return nil
	}
	l.queue = l.queue[1:]
	l.mu.Unlock()
	if turn.ShellScript != "" {
		return l.sendShell(turn.ShellScript)
	}
	return l.sendWithAttachmentsAndContext(turn.Text, turn.Thinking, turn.Attachments, turn.ContextPaths, false)
}

func (l *Live) applyNotice(notice eventNotice) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if notice.RunID != "" && l.runID != "" && notice.RunID != l.runID {
		return
	}
	projection := stream.Projection{Messages: l.messages[l.activeID], Gate: l.gate}
	done := projection.Apply(stream.Notice(notice), l.nextID)
	l.messages[l.activeID] = projection.Messages
	l.gate = projection.Gate
	if done {
		l.busy = false
		l.runID = ""
		l.retireSubscriptionLocked(l.subscriptionID)
		l.subscriptionID = ""
		l.cursor.Reset()
		l.recoveryInFlight = false
		l.recoveryNeeded = false
		l.replayPending = false
		l.nextRecoveryAt = time.Time{}
	}
}

func (l *Live) retireSubscriptionLocked(subscriptionID string) {
	if subscriptionID == "" {
		return
	}
	if l.retiredSubscriptions == nil {
		l.retiredSubscriptions = make(map[string]struct{})
	}
	l.retiredSubscriptions[subscriptionID] = struct{}{}
	if len(l.retiredSubscriptions) > 8 {
		for id := range l.retiredSubscriptions {
			if id != subscriptionID {
				delete(l.retiredSubscriptions, id)
				break
			}
		}
	}
}

func (l *Live) ensureAssistantDraftLocked() {
	projection := stream.Projection{Messages: l.messages[l.activeID]}
	projection.EnsureAssistantDraft(l.nextID)
	l.messages[l.activeID] = projection.Messages
}

func (l *Live) finishStreamingLocked() {
	projection := stream.Projection{Messages: l.messages[l.activeID]}
	projection.FinishStreaming()
	l.messages[l.activeID] = projection.Messages
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
	l.loadRequest++
	request := l.loadRequest
	l.loadPending = true
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
			return liveLoadedMsg{Request: request, Err: err}
		}
		snapshot := surface.Sidebar{Session: surface.Session{
			ID: created.ID, Title: created.Title, PermissionPreset: created.PermissionPreset, CreatedAt: created.CreatedAt,
		}}
		if contextStatus, contextErr := l.client.sessionContext(ctx, created.ID); contextErr == nil {
			snapshot.Context = mapContextView(contextStatus)
			snapshot.HasContext = true
		}
		return liveLoadedMsg{
			Request:  request,
			Session:  snapshot.Session,
			Messages: nil,
			Sidebar:  snapshot,
		}
	}
}

// RefreshSessions implements surface.SessionController. It backs the
// independent Ctrl+S dialog with a fresh session/list snapshot.
func (l *Live) RefreshSessions() tea.Cmd {
	l.mu.Lock()
	l.sessionRequest++
	request := l.sessionRequest
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		sessions, err := l.client.listSessions(ctx)
		if err != nil {
			return surface.SessionsMsg{Action: "list", Request: request, Err: err}
		}
		out := make([]surface.Session, 0, len(sessions))
		for _, session := range sessions {
			out = append(out, surface.Session{
				ID: session.ID, Title: session.Title, PermissionPreset: session.PermissionPreset,
				CreatedAt: session.CreatedAt,
			})
		}
		return surface.SessionsMsg{Action: "list", Request: request, Sessions: out}
	}
}

// SelectSession implements surface.SessionController.
func (l *Live) SelectSession(id string) tea.Cmd {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	l.mu.Lock()
	busy := l.busy
	l.mu.Unlock()
	if busy {
		return nil
	}
	return l.loadSessionCmd(id)
}

// RenameSession implements surface.SessionController.
func (l *Live) RenameSession(id, title string) tea.Cmd {
	id = strings.TrimSpace(id)
	title = strings.TrimSpace(title)
	if id == "" || title == "" {
		return nil
	}
	l.mu.Lock()
	l.sessionRequest++
	request := l.sessionRequest
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		session, err := l.client.renameSession(ctx, id, title)
		if err != nil {
			return surface.SessionsMsg{Action: "rename", Request: request, ID: id, Err: err}
		}
		return surface.SessionsMsg{Action: "rename", Request: request, ID: id, Session: surface.Session{
			ID: session.ID, Title: session.Title, PermissionPreset: session.PermissionPreset, CreatedAt: session.CreatedAt,
		}}
	}
}

// DeleteSession implements surface.SessionController. The active session is
// protected while a run is in flight; the view also performs this check before
// opening confirmation so the refusal is visible without a round trip.
func (l *Live) DeleteSession(id string) tea.Cmd {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	l.mu.Lock()
	busy := l.busy && l.activeID == id
	l.mu.Unlock()
	if busy {
		return func() tea.Msg {
			return surface.SessionsMsg{Action: "delete", ID: id, Err: errors.New("cannot delete the active session while a run is in progress")}
		}
	}
	l.mu.Lock()
	l.sessionRequest++
	request := l.sessionRequest
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		err := l.client.deleteSession(ctx, id)
		return surface.SessionsMsg{Action: "delete", Request: request, ID: id, Err: err}
	}
}

func (l *Live) applySessionsMsg(msg surface.SessionsMsg) tea.Cmd {
	l.mu.Lock()
	if msg.Request > 0 && msg.Request != l.sessionRequest {
		l.mu.Unlock()
		return nil
	}
	l.mu.Unlock()
	if msg.Err != nil {
		l.mu.Lock()
		l.lastErr = shortErr(msg.Err)
		l.mu.Unlock()
		return nil
	}
	switch msg.Action {
	case "list":
		l.mu.Lock()
		l.sessions = append([]surface.Session(nil), msg.Sessions...)
		for _, session := range l.sessions {
			if session.ID == l.activeID && l.sidebar.Session.ID == session.ID {
				l.sidebar.Session = session
				break
			}
		}
		l.lastErr = ""
		l.mu.Unlock()
	case "rename":
		l.mu.Lock()
		for i := range l.sessions {
			if l.sessions[i].ID == msg.ID {
				l.sessions[i] = msg.Session
				break
			}
		}
		if l.sidebar.Session.ID == msg.ID {
			l.sidebar.Session = msg.Session
		}
		l.lastErr = ""
		l.mu.Unlock()
	case "delete":
		l.mu.Lock()
		remaining := l.sessions[:0]
		for _, session := range l.sessions {
			if session.ID != msg.ID {
				remaining = append(remaining, session)
			}
		}
		l.sessions = remaining
		delete(l.messages, msg.ID)
		loadID := ""
		if l.activeID == msg.ID {
			if len(l.sessions) > 0 {
				loadID = l.sessions[0].ID
				l.activeID = loadID
				l.sidebar = surface.Sidebar{Session: l.sessions[0]}
			} else {
				l.activeID = ""
				l.sidebar = surface.Sidebar{}
			}
		}
		l.lastErr = ""
		l.mu.Unlock()
		if loadID != "" {
			return l.loadSessionCmd(loadID)
		}
	}
	return nil
}

func (l *Live) loadSessionCmd(id string) tea.Cmd {
	l.mu.Lock()
	l.loadRequest++
	request := l.loadRequest
	l.loadPending = true
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		msgs, err := l.client.sessionMessages(ctx, id)
		if err != nil {
			return liveLoadedMsg{Request: request, Err: err}
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
		if session.Title == "" {
			if fetched, fetchErr := l.client.getSession(ctx, id); fetchErr == nil {
				session = surface.Session{
					ID: fetched.ID, Title: fetched.Title, PermissionPreset: fetched.PermissionPreset, CreatedAt: fetched.CreatedAt,
				}
			}
		}
		snapshot := surface.Sidebar{Session: session}
		if contextStatus, contextErr := l.client.sessionContext(ctx, id); contextErr == nil {
			snapshot.Context = mapContextView(contextStatus)
			snapshot.HasContext = true
		}
		return liveLoadedMsg{
			Request:  request,
			Session:  session,
			Messages: mapHistory(msgs),
			Sidebar:  snapshot,
		}
	}
}

// Send implements surface.Driver.
func (l *Live) Send(text string) tea.Cmd {
	l.mu.Lock()
	thinking := l.thinkingMode
	attachments := cloneAttachments(l.drafts[l.activeID])
	l.mu.Unlock()
	return l.sendWithAttachments(text, thinking, attachments, true)
}

// SendWithContext implements surface.ContextSender. Paths are untrusted
// parser hints; the command resolves metadata through the control plane and
// sends the same paths again in turn/start so the server can revalidate at
// the RunWithOptions boundary.
func (l *Live) SendWithContext(text string, paths []string) tea.Cmd {
	l.mu.Lock()
	thinking := l.thinkingMode
	attachments := cloneAttachments(l.drafts[l.activeID])
	l.mu.Unlock()
	return l.sendWithAttachmentsAndContext(text, thinking, attachments, paths, true)
}

func (l *Live) send(text, thinking string) tea.Cmd {
	l.mu.Lock()
	attachments := cloneAttachments(l.drafts[l.activeID])
	l.mu.Unlock()
	return l.sendWithAttachments(text, thinking, attachments, true)
}

func (l *Live) sendWithAttachments(text, thinking string, attachments []surface.Attachment, consumeDraft bool) tea.Cmd {
	return l.sendWithAttachmentsAndContext(text, thinking, attachments, nil, consumeDraft)
}

func (l *Live) sendWithAttachmentsAndContext(text, thinking string, attachments []surface.Attachment, contextPaths []string, consumeDraft bool) tea.Cmd {
	if strings.TrimSpace(text) == "" {
		if len(attachments) > 0 {
			return commandResultCmd("image", "", errors.New("text is required; image-only turns are not supported"))
		}
		if len(contextPaths) > 0 {
			return commandResultCmd("file", "", errors.New("@file references require a prompt"))
		}
		return nil
	}
	contextPaths = cloneStrings(contextPaths)
	l.mu.Lock()
	if l.loadPending {
		l.mu.Unlock()
		return nil
	}
	sessionID := l.activeID
	if l.busy || l.gate != nil {
		l.queue = append(l.queue, queuedTurn{SessionID: sessionID, Text: text, Thinking: thinking, Attachments: cloneAttachments(attachments), ContextPaths: contextPaths})
		if consumeDraft {
			delete(l.drafts, sessionID)
		}
		l.mu.Unlock()
		return func() tea.Msg { return surface.RefreshMsg{} }
	}
	if consumeDraft {
		delete(l.drafts, sessionID)
	}
	l.appendLocked(surface.Message{
		ID:          l.nextID("user"),
		Role:        string(domain.RoleUser),
		Content:     text,
		Attachments: cloneAttachments(attachments),
	})
	l.busy = true
	l.mu.Unlock()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
		defer cancel()
		var fileContexts []surface.FileContext
		var err error
		if len(contextPaths) > 0 {
			fileContexts, err = l.client.resolveProjectContext(ctx, contextPaths)
			if err != nil {
				return liveTurnStartedMsg{SessionID: sessionID, UserText: text, Attachments: cloneAttachments(attachments), ContextPaths: contextPaths, Err: err}
			}
		}
		accepted, err := l.client.startTurnWithAttachmentsAndContext(ctx, sessionID, text, l.face, thinking, attachments, contextPaths)
		if err != nil {
			return liveTurnStartedMsg{SessionID: sessionID, UserText: text, Attachments: cloneAttachments(attachments), ContextPaths: contextPaths, FileContexts: cloneFileContexts(fileContexts), Err: err}
		}
		return liveTurnStartedMsg{SessionID: sessionID, UserText: text, RunID: accepted.RunID, Attachments: cloneAttachments(attachments), ContextPaths: contextPaths, FileContexts: cloneFileContexts(fileContexts)}
	}
}

// ExecuteShell implements surface.ShellExecutor. It only requests the
// server-owned shell/start operation; no local process or command backend is
// reachable from this face.
func (l *Live) ExecuteShell(script string) tea.Cmd {
	if !l.SupportsCapability("shell.start") {
		return func() tea.Msg { return surface.ErrMsg{Err: errors.New("governed shell is unavailable")} }
	}
	return l.sendShell(script)
}

func (l *Live) SupportsCapability(name string) bool {
	return l != nil && l.client != nil && l.client.SupportsCapability(name)
}

func (l *Live) sendShell(script string) tea.Cmd {
	if strings.TrimSpace(script) == "" {
		return commandResultCmd("shell", "", errors.New("shell script is required"))
	}
	l.mu.Lock()
	if l.loadPending {
		l.mu.Unlock()
		return commandResultCmd("shell", "", errors.New("session is still loading"))
	}
	sessionID := l.activeID
	if sessionID == "" {
		l.mu.Unlock()
		return commandResultCmd("shell", "", errors.New("no active session"))
	}
	if l.busy || l.gate != nil {
		l.queue = append(l.queue, queuedTurn{SessionID: sessionID, ShellScript: script})
		l.mu.Unlock()
		return func() tea.Msg { return surface.RefreshMsg{} }
	}
	l.appendLocked(surface.Message{
		ID:      l.nextID("user"),
		Role:    string(domain.RoleUser),
		Content: "!" + script,
	})
	l.busy = true
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
		defer cancel()
		accepted, err := l.client.startShell(ctx, sessionID, script)
		if err != nil {
			return liveTurnStartedMsg{SessionID: sessionID, UserText: "!" + script, Shell: true, Err: err}
		}
		return liveTurnStartedMsg{SessionID: sessionID, UserText: "!" + script, Shell: true, RunID: accepted.RunID}
	}
}

// ThinkingMode returns the draft preference used for the next turn.
func (l *Live) ThinkingMode() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.thinkingMode == "" {
		return "auto"
	}
	return l.thinkingMode
}

// SetThinkingMode updates only future turns. Send snapshots the value so a
// later toggle cannot retroactively change already queued work.
func (l *Live) SetThinkingMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "auto" && mode != "on" && mode != "off" {
		return errors.New("thinking must be auto, on, or off")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if mode == "on" && (!l.sidebar.HasContext || !l.sidebar.Context.ThinkingSupported) {
		return errors.New("extended thinking is unavailable for the active model")
	}
	l.thinkingMode = mode
	return nil
}

// DecideApproval implements surface.Driver.
func (l *Live) DecideApproval(decision string) tea.Cmd {
	l.mu.Lock()
	gate := l.gate
	if gate == nil || gate.Kind != "approval" || gate.ID == "" || gate.Submitting {
		l.mu.Unlock()
		return nil
	}
	id := gate.ID
	gate.Submitting = true
	l.mu.Unlock()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		err := l.client.respondApproval(ctx, id, decision)
		return liveRPCMsg{Kind: "approval", Outcome: decision, Err: err}
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
	if gate == nil || gate.Kind != "question" || gate.ID == "" || gate.Submitting {
		l.mu.Unlock()
		return nil
	}
	id := gate.ID
	gate.Submitting = true
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		err := l.client.respondQuestion(ctx, id, answer)
		return liveRPCMsg{Kind: "question", Outcome: answer, Err: err}
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

// ExecuteCommand implements surface.CommandExecutor. The shared fullscreen
// view owns parsing and presentation; this adapter only translates validated
// command names into the live driver's existing authoritative operations.
func (l *Live) ExecuteCommand(name string, args []string) tea.Cmd {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return commandResultCmd(name, "", errors.New("command name is required"))
	}
	registry := command.DefaultRegistry()
	spec, ok := registry.Lookup(name)
	if !ok {
		return commandResultCmd(name, "", fmt.Errorf("unknown command /%s", name))
	}
	name = spec.Name
	if err := registry.Validate(&command.Invocation{Name: name, Args: args}); err != nil {
		return commandResultCmd(name, "", err)
	}
	if commandMutates(name) && name != "cancel" && name != "queue" {
		l.mu.Lock()
		blocked := l.busy || l.gate != nil || l.loadPending || l.commandInFlight
		l.mu.Unlock()
		if blocked {
			return commandResultCmd(name, "", errors.New("a run or gate is active; finish it before changing session state"))
		}
	}
	switch name {
	case "new":
		return l.NewSession(strings.TrimSpace(strings.Join(args, " ")))
	case "session":
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
			return commandResultCmd(name, "", errors.New("usage: /session <id>"))
		}
		if cmd := l.SelectSession(args[0]); cmd != nil {
			return cmd
		}
		return commandResultCmd(name, "", errors.New("session selection is unavailable"))
	case "rename":
		if len(args) == 0 {
			return commandResultCmd(name, "", errors.New("usage: /rename <title>"))
		}
		l.mu.Lock()
		activeID := l.activeID
		l.mu.Unlock()
		if activeID == "" {
			return commandResultCmd(name, "", errors.New("no active session"))
		}
		if cmd := l.RenameSession(activeID, strings.TrimSpace(strings.Join(args, " "))); cmd != nil {
			return cmd
		}
		return commandResultCmd(name, "", errors.New("session rename is unavailable"))
	case "cancel":
		if !l.Meta().Busy {
			return commandResultCmd(name, "nothing to cancel", nil)
		}
		if cmd := l.Cancel(); cmd != nil {
			return cmd
		}
		return commandResultCmd(name, "", errors.New("cancel is unavailable"))
	case "queue":
		if len(args) != 1 || !strings.EqualFold(args[0], "clear") {
			return commandResultCmd(name, "", errors.New("usage: /queue clear"))
		}
		if l.ClearQueue() {
			return commandResultCmd(name, "queued turns cleared", nil)
		}
		return commandResultCmd(name, "queue is already empty", nil)
	case "permission":
		preset := ""
		if len(args) == 1 {
			preset = strings.ToLower(strings.TrimSpace(args[0]))
		} else if len(args) == 0 {
			l.mu.Lock()
			current := l.activeSessionLocked().PermissionPreset
			l.mu.Unlock()
			preset = nextCommandPermission(current)
		} else {
			return commandResultCmd(name, "", errors.New("usage: /permission [cautious|smart|trusted]"))
		}
		if preset != "cautious" && preset != "smart" && preset != "trusted" {
			return commandResultCmd(name, "", errors.New("permission must be cautious, smart, or trusted"))
		}
		if cmd := l.SetPermission(preset); cmd != nil {
			return cmd
		}
		return commandResultCmd(name, "", errors.New("permission change is unavailable"))
	case "image":
		return l.executeImageCommand(args)
	case "compact":
		sessionID, ok := l.commandSessionID()
		if !ok {
			return commandResultCmd(name, "", errors.New("no active session"))
		}
		return l.commandRPCCmd(name, "context/compact", map[string]string{"session_id": sessionID})
	case "fork":
		sessionID, ok := l.commandSessionID()
		if !ok {
			return commandResultCmd(name, "", errors.New("no active session"))
		}
		params := map[string]string{"session_id": sessionID, "message_id": args[0]}
		if len(args) == 2 {
			params["title"] = strings.TrimSpace(args[1])
		}
		return l.commandRPCCmd(name, "session/fork", params)
	case "rewind":
		sessionID, ok := l.commandSessionID()
		if !ok {
			return commandResultCmd(name, "", errors.New("no active session"))
		}
		return l.commandRPCCmd(name, "session/rewind", map[string]string{"session_id": sessionID, "message_id": args[0]})
	case "todos":
		sessionID, ok := l.commandSessionID()
		if !ok {
			return commandResultCmd(name, "", errors.New("no active session"))
		}
		return l.commandRPCCmd(name, "session/todos", map[string]string{"session_id": sessionID})
	case "stats":
		params := map[string]string{}
		if len(args) == 1 {
			params["period"] = strings.ToLower(strings.TrimSpace(args[0]))
		}
		return l.commandRPCCmd(name, "stats/tokens", params)
	case "skills":
		if len(args) == 1 {
			return l.commandRPCCmd(name, "skills/get", map[string]string{"name": args[0]})
		}
		return l.commandRPCCmd(name, "skills/list", nil)
	case "mcp":
		if len(args) == 1 {
			return l.commandRPCCmd(name, "settings/mcp/probe", map[string]string{"name": args[0]})
		}
		if len(args) == 2 && strings.EqualFold(strings.TrimSpace(args[0]), "resources") {
			return l.commandRPCCmd(name, "settings/mcp/resources", map[string]string{"name": args[1]})
		}
		if len(args) == 3 && strings.EqualFold(strings.TrimSpace(args[0]), "read") {
			return l.commandRPCCmd(name, "settings/mcp/read", map[string]string{"server": args[1], "uri": args[2]})
		}
		return l.commandRPCCmd(name, "settings/mcp", nil)
	case "files":
		runID := ""
		if len(args) > 0 {
			runID = strings.TrimSpace(args[0])
		} else {
			l.mu.Lock()
			runID = l.runID
			l.mu.Unlock()
		}
		if runID == "" {
			return commandResultCmd(name, "", errors.New("a run_id is required; workspace files are scoped to a run"))
		}
		if len(args) == 2 {
			return l.commandRPCCmd(name, "workspace/read", map[string]string{"run_id": runID, "path": args[1]})
		}
		return l.commandRPCCmd(name, "workspace/list", map[string]string{"run_id": runID})
	case "tools":
		return l.commandRPCCmd(name, "tools/list", nil)
	default:
		return commandResultCmd(name, "", fmt.Errorf("/%s is handled by the shared view or is unavailable", name))
	}
}

func (l *Live) executeImageCommand(args []string) tea.Cmd {
	if len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "clear") {
		l.mu.Lock()
		count := len(l.drafts[l.activeID])
		delete(l.drafts, l.activeID)
		l.mu.Unlock()
		if count == 0 {
			return commandResultCmd("image", "no pending image attachments", nil)
		}
		return commandResultCmd("image", fmt.Sprintf("cleared %d pending image attachment(s)", count), nil)
	}
	if len(args) == 2 && strings.EqualFold(strings.TrimSpace(args[0]), "remove") {
		index, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil || index < 1 {
			return commandResultCmd("image", "", errors.New("image remove index must be a positive number"))
		}
		l.mu.Lock()
		pending := l.drafts[l.activeID]
		if index > len(pending) {
			l.mu.Unlock()
			return commandResultCmd("image", "", fmt.Errorf("image attachment %d is not pending", index))
		}
		pending = append(pending[:index-1], pending[index:]...)
		if len(pending) == 0 {
			delete(l.drafts, l.activeID)
		} else {
			l.drafts[l.activeID] = pending
		}
		l.mu.Unlock()
		return commandResultCmd("image", fmt.Sprintf("removed pending image attachment %d", index), nil)
	}
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return commandResultCmd("image", "", errors.New("usage: /image <relative-path> | /image remove <index> | /image clear"))
	}
	l.mu.Lock()
	sessionID := l.activeID
	contextKnown := l.sidebar.HasContext && l.sidebar.Context.ImageSupportKnown
	imageSupported := contextKnown && l.sidebar.Context.ImageSupported
	l.mu.Unlock()
	if sessionID == "" {
		return commandResultCmd("image", "", errors.New("no active session"))
	}
	if !contextKnown {
		return commandResultCmd("image", "", errors.New("image attachments are unavailable until model image support is known"))
	}
	if !imageSupported {
		return commandResultCmd("image", "", errors.New("the active model does not support image attachments"))
	}
	path := args[0]
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		attachments, err := l.client.resolveAttachments(ctx, []string{path})
		return liveAttachmentResolvedMsg{SessionID: sessionID, Attachments: attachments, Err: err}
	}
}

func (l *Live) applyAttachmentResolved(msg liveAttachmentResolvedMsg) tea.Cmd {
	l.mu.Lock()
	defer l.mu.Unlock()
	if msg.Err != nil {
		l.lastErr = shortErr(msg.Err)
		return nil
	}
	if msg.SessionID == "" || msg.SessionID != l.activeID {
		return nil
	}
	if l.drafts == nil {
		l.drafts = make(map[string][]surface.Attachment)
	}
	if len(l.drafts[msg.SessionID])+len(msg.Attachments) > 4 {
		l.lastErr = "at most 4 image attachments are allowed per message"
		return nil
	}
	l.drafts[msg.SessionID] = append(l.drafts[msg.SessionID], cloneAttachments(msg.Attachments)...)
	l.lastErr = ""
	return nil
}

func commandMutates(name string) bool {
	switch name {
	case "new", "session", "rename", "delete", "permission", "compact", "fork", "rewind":
		return true
	default:
		return false
	}
}

func (l *Live) commandSessionID() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.activeID, l.activeID != ""
}

func (l *Live) commandRPCCmd(name, method string, params any) tea.Cmd {
	sessionID := ""
	if commandMutates(name) {
		l.mu.Lock()
		if l.busy || l.gate != nil || l.loadPending || l.commandInFlight {
			l.mu.Unlock()
			return commandResultCmd(name, "", errors.New("a session operation is already in flight or the session is busy"))
		}
		sessionID = l.activeID
		l.commandInFlight = true
		l.mu.Unlock()
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 20*time.Second)
		defer cancel()
		raw, err := l.client.Call(ctx, method, params)
		return surface.CommandResultMsg{Name: name, Output: command.FormatResult(name, raw), Err: err, Mutation: commandMutates(name), SessionID: sessionID}
	}
}

func nextCommandPermission(current string) string {
	switch current {
	case "cautious":
		return "smart"
	case "smart":
		return "trusted"
	default:
		return "cautious"
	}
}

func commandResultCmd(name, output string, err error) tea.Cmd {
	return func() tea.Msg { return surface.CommandResultMsg{Name: name, Output: output, Err: err} }
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
	toolIndexes := make(map[string]int)
	for _, m := range msgs {
		if m.ToolName != "" {
			if index, ok := toolIndexes[m.ToolCallID]; ok && m.ToolCallID != "" {
				tool := out[index].Tool
				if m.ToolPreview != "" {
					tool.Preview = m.ToolPreview
				}
				if m.Role == surface.RoleTool {
					tool.Status, tool.Result = "done", m.Content
				}
				continue
			}
			status := "pending"
			tool := &surface.ToolCard{ToolName: m.ToolName, ToolCallID: m.ToolCallID, Status: status, Preview: m.ToolPreview}
			if m.Role == surface.RoleTool {
				tool.Status, tool.Result = "done", m.Content
			}
			out = append(out, surface.Message{ID: m.ToolCallID, Role: surface.RoleTool, Tool: tool})
			if m.ToolCallID != "" {
				toolIndexes[m.ToolCallID] = len(out) - 1
			}
			continue
		}
		out = append(out, surface.Message{
			ID:           m.ID,
			Role:         m.Role,
			Content:      m.Content,
			Attachments:  cloneAttachments(m.Attachments),
			FileContexts: cloneFileContexts(mergedFileContexts(m)),
		})
	}
	return out
}

func cloneAttachments(in []surface.Attachment) []surface.Attachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]surface.Attachment, len(in))
	copy(out, in)
	return out
}

func cloneFileContexts(in []surface.FileContext) []surface.FileContext {
	if len(in) == 0 {
		return nil
	}
	out := make([]surface.FileContext, len(in))
	copy(out, in)
	return out
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
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
var _ surface.CommandExecutor = (*Live)(nil)
var _ surface.ContextSender = (*Live)(nil)
var _ surface.ShellExecutor = (*Live)(nil)
