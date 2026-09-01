package runtime

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"agent-vivy/internal/domain"
)

// TitleGenerator produces a short session title from the first exchange.
// The app implements it over an ordered model chain (small model when
// configured, then the main model); runtime only owns the trigger and
// persistence so the titling policy never touches engine internals.
type TitleGenerator interface {
	GenerateTitle(ctx context.Context, userText, assistantText string) (string, error)
}

// titleTimeout bounds one auto-title attempt; it runs detached from the
// request and the run, so it needs its own deadline.
const titleTimeout = 30 * time.Second

// maxSessionTitleRunes bounds the persisted title regardless of what the
// model returned.
const maxSessionTitleRunes = 80

// maybeAutoTitle names an untitled session after its first completed
// exchange (VC-2). Best-effort and fire-and-forget: any failure only logs,
// and a user rename always wins. The goroutine joins the service WaitGroup
// so shutdown drains it before storage closes.
func (s *Service) maybeAutoTitle(ctx context.Context, sessionID domain.SessionID) {
	if s.deps.Titles == nil || s.deps.Sessions == nil || s.deps.Messages == nil {
		return
	}
	titleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), titleTimeout)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		s.autoTitle(titleCtx, sessionID)
	}()
}

func (s *Service) autoTitle(ctx context.Context, sessionID domain.SessionID) {
	sess, err := s.deps.Sessions.GetSession(ctx, sessionID)
	if err != nil || strings.TrimSpace(sess.Title) != "" {
		return // unknown, storage failure, or already titled (user rename wins)
	}
	messages, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		slog.Info("session auto-title skipped", "session", string(sessionID), "err", err)
		return
	}
	var userText, assistantText string
	for _, msg := range messages {
		if userText == "" && msg.Role == domain.RoleUser && msg.ToolCallID == "" {
			userText = msg.Content
			continue
		}
		if userText != "" && assistantText == "" && msg.Role == domain.RoleAssistant && msg.ToolCallID == "" {
			assistantText = msg.Content
			break
		}
	}
	if strings.TrimSpace(userText) == "" {
		return
	}
	title, err := s.deps.Titles.GenerateTitle(ctx, userText, assistantText)
	if err != nil {
		slog.Info("session auto-title skipped", "session", string(sessionID), "err", err)
		return
	}
	title = sanitizeSessionTitle(title)
	if title == "" {
		return
	}
	// Re-check so a rename racing the generation wins; first-writer keeps.
	if fresh, err := s.deps.Sessions.GetSession(ctx, sessionID); err != nil || strings.TrimSpace(fresh.Title) != "" {
		return
	}
	if err := s.deps.Sessions.RenameSession(ctx, sessionID, title); err != nil {
		slog.Info("session auto-title rename failed", "session", string(sessionID), "err", err)
	}
}

// sanitizeSessionTitle collapses whitespace, strips wrapping quotes, and
// caps the length so a runaway model reply still makes a usable label.
func sanitizeSessionTitle(raw string) string {
	title := strings.Join(strings.Fields(raw), " ")
	title = strings.Trim(title, "\"'`“”‘’«»")
	if runes := []rune(title); len(runes) > maxSessionTitleRunes {
		title = string(runes[:maxSessionTitleRunes])
	}
	return title
}
