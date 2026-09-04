package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/surface"
)

type sessionView struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	PermissionPreset string `json:"permission_preset"`
	CreatedAt        int64  `json:"created_at"`
}

type contextView struct {
	FeedTokens           int  `json:"feed_tokens"`
	ModelLimitTokens     int  `json:"model_limit_tokens"`
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

func mapContextView(view contextView) surface.Context {
	return surface.Context{
		FeedTokens: view.FeedTokens, ModelLimitTokens: view.ModelLimitTokens,
		TriggerTokens: view.TriggerTokens, TotalMessages: view.TotalMessages,
		FeedMessages: view.FeedMessages, ThinkingSupported: view.ThinkingSupported,
		ImageSupportKnown: view.ImageSupportKnown, ImageSupported: view.ImageSupported,
		CompactionEnabled: view.CompactionEnabled, WouldCompact: view.WouldCompact,
		HasCompactionSummary: view.HasCompactionSummary,
	}
}

type messageView struct {
	ID          string               `json:"id"`
	Role        string               `json:"role"`
	Content     string               `json:"content"`
	Attachments []surface.Attachment `json:"attachments,omitempty"`
}

type runAccepted struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

func (c *Client) createSession(ctx context.Context, title string) (sessionView, error) {
	raw, err := c.Call(ctx, "session/create", map[string]string{"title": title})
	if err != nil {
		return sessionView{}, err
	}
	var session sessionView
	if err := json.Unmarshal(raw, &session); err != nil {
		return sessionView{}, fmt.Errorf("tui: session/create: %w", err)
	}
	if session.ID == "" {
		return sessionView{}, fmt.Errorf("tui: session/create returned no id")
	}
	return session, nil
}

func (c *Client) listSessions(ctx context.Context) ([]sessionView, error) {
	raw, err := c.Call(ctx, "session/list", nil)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Sessions []sessionView `json:"sessions"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("tui: session/list: %w", err)
	}
	return envelope.Sessions, nil
}

func (c *Client) getSession(ctx context.Context, sessionID string) (sessionView, error) {
	raw, err := c.Call(ctx, "session/get", map[string]any{"session_id": sessionID, "include_attachment_data": false})
	if err != nil {
		return sessionView{}, err
	}
	var envelope struct {
		Session sessionView `json:"session"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return sessionView{}, fmt.Errorf("tui: session/get: %w", err)
	}
	if envelope.Session.ID == "" {
		return sessionView{}, fmt.Errorf("tui: session/get returned no id")
	}
	return envelope.Session, nil
}

func (c *Client) sessionContext(ctx context.Context, sessionID string) (contextView, error) {
	raw, err := c.Call(ctx, "session/context", map[string]string{"session_id": sessionID})
	if err != nil {
		return contextView{}, err
	}
	var view contextView
	if err := json.Unmarshal(raw, &view); err != nil {
		return contextView{}, fmt.Errorf("tui: session/context: %w", err)
	}
	return view, nil
}

func (c *Client) renameSession(ctx context.Context, sessionID, title string) (sessionView, error) {
	raw, err := c.Call(ctx, "session/rename", map[string]string{"session_id": sessionID, "title": title})
	if err != nil {
		return sessionView{}, err
	}
	var session sessionView
	if err := json.Unmarshal(raw, &session); err != nil {
		return sessionView{}, fmt.Errorf("tui: session/rename: %w", err)
	}
	return session, nil
}

func (c *Client) deleteSession(ctx context.Context, sessionID string) error {
	_, err := c.Call(ctx, "session/delete", map[string]string{"session_id": sessionID})
	return err
}

func (c *Client) sessionMessages(ctx context.Context, sessionID string) ([]messageView, error) {
	raw, err := c.Call(ctx, "session/messages", map[string]any{"session_id": sessionID, "include_attachment_data": false})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Messages []messageView `json:"messages"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("tui: session/messages: %w", err)
	}
	return envelope.Messages, nil
}

func (c *Client) startTurn(ctx context.Context, sessionID, text, face, thinking string) (runAccepted, error) {
	return c.startTurnWithAttachments(ctx, sessionID, text, face, thinking, nil)
}

func (c *Client) startTurnWithAttachments(ctx context.Context, sessionID, text, face, thinking string, attachments []surface.Attachment) (runAccepted, error) {
	params := map[string]any{"session_id": sessionID, "text": text}
	if strings.TrimSpace(face) != "" {
		params["face"] = face
	}
	if strings.TrimSpace(thinking) != "" {
		params["thinking"] = thinking
	}
	if len(attachments) > 0 {
		paths := make([]string, 0, len(attachments))
		for _, attachment := range attachments {
			path := strings.TrimSpace(attachment.Path)
			if path != "" {
				paths = append(paths, path)
			}
		}
		if len(paths) > 0 {
			params["attachment_paths"] = paths
		}
	}
	raw, err := c.Call(ctx, "turn/start", params)
	if err != nil {
		return runAccepted{}, err
	}
	var accepted runAccepted
	if err := json.Unmarshal(raw, &accepted); err != nil {
		return runAccepted{}, fmt.Errorf("tui: turn/start: %w", err)
	}
	if accepted.RunID == "" {
		return runAccepted{}, fmt.Errorf("tui: turn/start returned no run_id")
	}
	return accepted, nil
}

func (c *Client) subscribe(ctx context.Context, runID string, afterSeq int) error {
	_, err := c.Call(ctx, "run/subscribe", map[string]any{"run_id": runID, "after_seq": afterSeq})
	return err
}

func (c *Client) cancelRun(ctx context.Context, runID string) error {
	_, err := c.Call(ctx, "run/cancel", map[string]string{"run_id": runID})
	return err
}

func (c *Client) respondApproval(ctx context.Context, approvalID, decision string) error {
	_, err := c.Call(ctx, "approval/respond", map[string]string{
		"approval_id": approvalID,
		"decision":    decision,
	})
	return err
}

func (c *Client) respondQuestion(ctx context.Context, questionID, answer string) error {
	_, err := c.Call(ctx, "question/respond", map[string]string{
		"question_id": questionID,
		"answer":      answer,
	})
	return err
}

func (c *Client) setSessionPermission(ctx context.Context, sessionID, preset string) (sessionView, error) {
	raw, err := c.Call(ctx, "session/set_permission", map[string]string{
		"session_id": sessionID,
		"preset":     preset,
	})
	if err != nil {
		return sessionView{}, err
	}
	var session sessionView
	if err := json.Unmarshal(raw, &session); err != nil {
		return sessionView{}, fmt.Errorf("tui: session/set_permission: %w", err)
	}
	return session, nil
}

func formatHistory(messages []messageView) string {
	if len(messages) == 0 {
		return ""
	}
	var b strings.Builder
	for _, message := range messages {
		role := message.Role
		if role == string(domain.RoleUser) {
			role = "you"
		}
		content := strings.TrimSpace(message.Content)
		if chips := formatAttachmentMetadata(message.Attachments); chips != "" {
			if content != "" {
				content += " "
			}
			content += chips
		}
		fmt.Fprintf(&b, "%s: %s\n", role, content)
	}
	return b.String()
}

func formatAttachmentMetadata(attachments []surface.Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	parts := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = strings.TrimSpace(attachment.Path)
		}
		if name == "" {
			name = "image"
		}
		parts = append(parts, "[image: "+name+"]")
	}
	return strings.Join(parts, " ")
}
