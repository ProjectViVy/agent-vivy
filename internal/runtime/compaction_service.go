package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Compaction errors. ErrCompactionBusy protects a manual compaction from
// racing an active run (the run already compacts in-run); the caller maps
// it to a 409.
var (
	ErrCompactionBusy            = errors.New("runtime: a run is active; compaction runs inside it")
	ErrCompactionNothingToDo     = errors.New("runtime: session has no messages to compact")
	ErrCompactionNotWired        = errors.New("runtime: compaction store not wired")
	ErrCompactionNotNeeded       = errors.New("runtime: session context is within budget")
	errCompactionCallbackAborted = errors.New("runtime: compaction summary generation failed")
)

// LastCompaction is the most recent context compression of a session
// (process-local observability; the durable record lives in the journal).
type LastCompaction struct {
	Mode         string `json:"mode"`
	BeforeTokens int    `json:"before_tokens"`
	AfterTokens  int    `json:"after_tokens"`
	At           int64  `json:"at"`
}

// ContextStatusResult is the wire shape of session/context — the real
// pressure meter for the chat input ring and the settings compaction card.
type ContextStatusResult struct {
	SessionID domain.SessionID `json:"session_id"`
	// TotalMessages is the full stored history count; FeedMessages is what
	// the next run would actually feed after compaction folding.
	TotalMessages int `json:"total_messages"`
	FeedMessages  int `json:"feed_messages"`
	// FeedBytes / FeedTokens describe the assembled history (excluding the
	// per-run preamble and the next user message).
	FeedBytes  int `json:"feed_bytes"`
	FeedTokens int `json:"feed_tokens"`
	// LimitBytes is runtime.max_context_bytes; ModelLimitTokens is the
	// provider model context window (128000 when unknown).
	LimitBytes       int `json:"limit_bytes"`
	ModelLimitTokens int `json:"model_limit_tokens"`
	// ThinkingSupported mirrors D9 model metadata: whether the active
	// route's model accepts an explicit extended-thinking request. The
	// chat input gates its thinking selector on this flag.
	ThinkingSupported bool `json:"thinking_supported"`
	// CompactionEnabled mirrors the effective compaction policy.
	CompactionEnabled bool `json:"compaction_enabled"`
	// TriggerTokens is the in-run threshold; WouldCompact tells whether the
	// assembled history is already over it.
	TriggerTokens int  `json:"trigger_tokens"`
	WouldCompact  bool `json:"would_compact"`
	// HasCompactionSummary reports whether a durable session summary is
	// currently folded into the feed.
	HasCompactionSummary bool            `json:"has_compaction_summary"`
	LastCompaction       *LastCompaction `json:"last_compaction,omitempty"`
}

// CompactionResult is the outcome of one manual context/compact call.
type CompactionResult struct {
	BeforeTokens int  `json:"before_tokens"`
	AfterTokens  int  `json:"after_tokens"`
	Folded       int  `json:"folded_messages"`
	Skipped      bool `json:"skipped"`
}

// ContextStatus reports the real context pressure of a session: what the
// next run would feed vs the model window and the configured byte budget.
func (s *Service) ContextStatus(ctx context.Context, sessionID domain.SessionID) (ContextStatusResult, error) {
	if s.engine == nil || s.deps.Messages == nil {
		return ContextStatusResult{}, errors.New("runtime: service not wired")
	}
	out := ContextStatusResult{SessionID: sessionID, LimitBytes: s.engine.cfg.MaxContextBytes}
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return out, fmt.Errorf("runtime: list session messages: %w", err)
	}
	out.TotalMessages = len(stored)
	folded, foldedOK := s.foldSessionHistory(ctx, sessionID, stored)
	feed := feedableMessages(folded)
	out.FeedMessages = len(feed)
	out.FeedBytes, out.FeedTokens = historyBytesTokens(feed)
	out.HasCompactionSummary = foldedOK

	info := s.GetModelInfo(ctx)
	out.ThinkingSupported = info.SupportsThinking
	if info.ContextWindow > 0 {
		out.ModelLimitTokens = info.ContextWindow
	} else {
		out.ModelLimitTokens = fallbackContextWindowTokens
	}
	if s.engine.cfg.Compaction != nil && s.engine.cfg.Compaction.Enabled {
		out.CompactionEnabled = true
		out.TriggerTokens = s.engine.cfg.Compaction.TriggerTokens(bytesToTokens(s.engine.cfg.MaxContextBytes))
		out.WouldCompact = out.FeedTokens > out.TriggerTokens
	}
	s.mu.Lock()
	if last, ok := s.lastCompaction[sessionID]; ok && last != nil {
		cp := *last
		out.LastCompaction = &cp
	}
	s.mu.Unlock()
	return out, nil
}

// CompactSession runs one durable, session-level compaction now: it
// summarizes the assembled history with the provider model, records the
// summary (with the kept tail) in the compaction store so future feeds fold
// the covered rows, and journals a context.compacted event. It refuses to
// run while a run is active: live runs already compress in-run.
func (s *Service) CompactSession(ctx context.Context, sessionID domain.SessionID) (CompactionResult, error) {
	if s.engine == nil || s.deps.Messages == nil {
		return CompactionResult{}, errors.New("runtime: service not wired")
	}
	if s.deps.Compactions == nil {
		return CompactionResult{}, ErrCompactionNotWired
	}
	s.mu.Lock()
	busy := len(s.active) > 0 || len(s.pending) > 0
	s.mu.Unlock()
	if busy {
		return CompactionResult{}, ErrCompactionBusy
	}
	if s.engine.cfg.Compaction == nil || !s.engine.cfg.Compaction.Enabled {
		return CompactionResult{}, ErrCompactionNotNeeded
	}
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return CompactionResult{}, fmt.Errorf("runtime: list session messages: %w", err)
	}
	feed := feedableMessages(stored)
	if len(feed) == 0 {
		return CompactionResult{Skipped: true}, nil
	}
	_, tokens := historyBytesTokens(feed)
	trigger := s.engine.cfg.Compaction.TriggerTokens(bytesToTokens(s.engine.cfg.MaxContextBytes))
	if trigger <= 0 || tokens <= trigger {
		return CompactionResult{BeforeTokens: tokens, AfterTokens: tokens, Skipped: true}, nil
	}

	summary, err := s.generateSessionSummary(ctx, feed)
	if err != nil {
		return CompactionResult{}, fmt.Errorf("runtime: generate session summary: %w", err)
	}

	keep := s.engine.cfg.Compaction.KeepRecent
	if keep <= 0 {
		keep = 12
	}
	foldIdx := len(feed) - keep
	if foldIdx < 0 {
		foldIdx = 0
	}
	tailFrom := feed[len(feed)-1].CreatedAt
	if foldIdx > 0 {
		tailFrom = feed[foldIdx-1].CreatedAt
	}
	runID := domain.RunID(newPrefixedID("cmp_"))
	rec := storage.SessionCompaction{
		SessionID:    sessionID,
		RunID:        runID,
		Summary:      summary,
		TailFrom:     tailFrom,
		DroppedCount: foldIdx,
		CreatedAt:    time.Now().UnixMilli(),
	}
	if err := s.deps.Compactions.SaveSessionCompaction(ctx, rec); err != nil {
		return CompactionResult{}, fmt.Errorf("runtime: save session compaction: %w", err)
	}

	foldedFeed := feedableMessages(foldedFor(rec, stored))
	_, afterTokens := historyBytesTokens(foldedFeed)
	result := CompactionResult{BeforeTokens: tokens, AfterTokens: afterTokens, Folded: foldIdx}

	if _, err := s.RecordExternalRunEvent(ctx, runID, domain.EventContextCompacted, payloadContextCompacted{
		Mode: "session", BeforeTokens: tokens, AfterTokens: afterTokens,
		DroppedMessages: foldIdx, RetentionSuffix: keep,
	}); err != nil {
		return CompactionResult{}, fmt.Errorf("runtime: persist compaction event: %w", err)
	}
	s.mu.Lock()
	if s.lastCompaction == nil {
		s.lastCompaction = make(map[domain.SessionID]*LastCompaction)
	}
	s.lastCompaction[sessionID] = &LastCompaction{Mode: "session", BeforeTokens: tokens, AfterTokens: afterTokens, At: rec.CreatedAt}
	s.mu.Unlock()
	return result, nil
}

// foldedFor mirrors the feed folding used at run time for the given
// compaction record, so CompactSession can report honest after-tokens.
func foldedFor(rec storage.SessionCompaction, stored []domain.Message) []domain.Message {
	idx := 0
	for idx < len(stored) && stored[idx].CreatedAt <= rec.TailFrom {
		idx++
	}
	if idx == 0 {
		return stored
	}
	summary := domain.Message{Role: domain.RoleUser, CreatedAt: rec.TailFrom, Content: compactionSummaryPrefix + rec.Summary}
	out := make([]domain.Message, 0, len(stored)-idx+1)
	out = append(out, summary)
	out = append(out, stored[idx:]...)
	return out
}

// generateSessionSummary condenses the assembled history with the same
// provider model the run uses. Returns a plain-text summary.
func (s *Service) generateSessionSummary(ctx context.Context, feed []domain.Message) (string, error) {
	const maxTranscriptBytes = 200 * 1024
	var transcript strings.Builder
	transcript.WriteString("需压缩的会话历史（旧→新）：\n\n")
	remaining := maxTranscriptBytes
	for _, msg := range feed {
		line := fmt.Sprintf("[%s] %s\n", msg.Role, msg.Content)
		if msg.ToolCallID != "" {
			line = fmt.Sprintf("[%s] (%s %s) %s\n", msg.Role, msg.ToolName, msg.ToolCallID, msg.Content)
		}
		// Image attachments are binary and cannot enter the text transcript
		// (VC-1g-2); the placeholder keeps summaries aware they existed.
		for _, attachment := range msg.Attachments {
			name := attachment.Name
			if name == "" {
				name = attachment.MimeType
			}
			line += fmt.Sprintf("[%s] [image attachment: %s]\n", msg.Role, name)
		}
		if len(line) > remaining {
			break
		}
		transcript.WriteString(line)
		remaining -= len(line)
	}
	input := []*schema.Message{
		schema.SystemMessage(sessionSummarySystemPrompt),
		schema.UserMessage(transcript.String()),
	}
	resp, err := s.engine.chatModel.Generate(ctx, input)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", errCompactionCallbackAborted
	}
	out := strings.TrimSpace(resp.Content)
	if out == "" {
		return "", errCompactionCallbackAborted
	}
	return out, nil
}

// sessionSummarySystemPrompt drives the durable session-level summary. It
// mirrors the official summarization middleware's intent: a digest that
// preserves decisions, outstanding work, and essential facts without
// inventing content.
const sessionSummarySystemPrompt = "你是会话压缩助手。把给定的对话与工具调用历史压缩成一段简洁的摘要。必须保留：用户的关键诉求与已达成结论、未完成/待办事项、重要事实（文件路径、命令、关键输出、约束条件）、需要延续的上下文。不要编造历史中不存在的内容。用纯文本输出摘要，控制在 800 字以内。"

// historyBytesTokens estimates the byte and token footprint of an assembled
// history, mirroring context.go's accounting.
func historyBytesTokens(msgs []domain.Message) (int, int) {
	bytes := 0
	for _, msg := range msgs {
		bytes += messageCost(msg.Content, string(msg.Role)) + len(msg.ToolArgs) + len(msg.ToolCallID)
	}
	return bytes, bytes / 4
}

// recordLastCompaction keeps the process-local "last compaction" per
// session for the UI; the durable record is the journal event.
func (s *Service) recordLastCompaction(sessionID domain.SessionID, last *LastCompaction) {
	s.mu.Lock()
	if s.lastCompaction == nil {
		s.lastCompaction = make(map[domain.SessionID]*LastCompaction)
	}
	s.lastCompaction[sessionID] = last
	s.mu.Unlock()
}

// CompactionPolicy returns the engine's current compaction policy snapshot
// (nil when the engine has none). It is observability for the app-level
// settings reload hook.
func (s *Service) CompactionPolicy() *CompactionPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.engine == nil {
		return nil
	}
	return s.engine.cfg.Compaction
}

// ScheduleEngineReload rebuilds the engine with a new config (settings
// save). When runs are in flight the rebuild is deferred to the next idle
// run start; it always lands before any run uses the new thresholds.
func (s *Service) ScheduleEngineReload(next EngineConfig) error {
	if s.deps.RebuildEngine == nil {
		return errors.New("runtime: engine reload not wired")
	}
	s.mu.Lock()
	idle := len(s.active) == 0 && len(s.pending) == 0
	if !idle {
		next := next
		s.pendingEngine = &next
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	return s.applyPendingEngineReload(context.Background(), &next)
}

// applyPendingEngineReload applies a deferred engine rebuild. It only swaps
// while no run is registered, so in-flight iterators and checkpoint ids are
// never disturbed.
func (s *Service) applyPendingEngineReload(ctx context.Context, explicit *EngineConfig) error {
	s.mu.Lock()
	pending := explicit
	if pending == nil {
		pending = s.pendingEngine
	}
	if pending == nil {
		s.mu.Unlock()
		return nil
	}
	if len(s.active) > 0 || len(s.pending) > 0 {
		s.mu.Unlock()
		return nil
	}
	next := *pending
	s.pendingEngine = nil
	s.mu.Unlock()

	eng, err := s.deps.RebuildEngine(ctx, next)
	if err != nil {
		s.mu.Lock()
		s.pendingEngine = &next
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.engine = eng
	s.mu.Unlock()
	return nil
}
