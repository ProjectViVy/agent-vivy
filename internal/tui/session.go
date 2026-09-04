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
	UpdatedAt        int64  `json:"updated_at"`
}

type sidebarView struct {
	Session            sessionView        `json:"session"`
	CWD                string             `json:"cwd"`
	Model              string             `json:"model"`
	Provider           string             `json:"provider"`
	ReasoningKnown     bool               `json:"reasoning_known"`
	ReasoningSupported bool               `json:"reasoning_supported"`
	Context            *contextView       `json:"context"`
	Usage              *sidebarUsageView  `json:"usage"`
	ModifiedFilesKnown bool               `json:"modified_files_known"`
	ModifiedFiles      []sidebarFileView  `json:"modified_files"`
	MCPKnown           bool               `json:"mcp_known"`
	MCP                []sidebarMCPView   `json:"mcp"`
	SkillsKnown        bool               `json:"skills_known"`
	Skills             []sidebarSkillView `json:"skills"`
	LSPKnown           bool               `json:"lsp_known"`
	LSP                []sidebarLSPView   `json:"lsp"`
}

type sidebarMCPView struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type sidebarSkillView struct {
	Name string `json:"name"`
}

type sidebarLSPView struct {
	Language string `json:"language"`
	State    string `json:"state"`
}

type sidebarUsageView struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	ReasoningTokens  int     `json:"reasoning_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	RequestCount     int     `json:"request_count"`
	CostUSD          float64 `json:"cost_usd"`
	CostKnown        bool    `json:"cost_known"`
}

type sidebarDiffView struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type sidebarFileView struct {
	Path      string          `json:"path"`
	Diff      sidebarDiffView `json:"diff"`
	UpdatedAt int64           `json:"updated_at"`
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

func mapSessionView(view sessionView) surface.Session {
	return surface.Session{
		ID: view.ID, Title: view.Title, PermissionPreset: view.PermissionPreset,
		CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
}

func mapSidebarView(view sidebarView) surface.Sidebar {
	snapshot := surface.Sidebar{
		Session:            mapSessionView(view.Session),
		CWD:                view.CWD,
		Model:              view.Model,
		Provider:           view.Provider,
		ReasoningKnown:     view.ReasoningKnown,
		ReasoningSupported: view.ReasoningSupported,
	}
	if view.Context != nil {
		snapshot.Context = mapContextView(*view.Context)
		snapshot.HasContext = true
	}
	if view.Usage != nil {
		snapshot.Usage = surface.SidebarUsage{
			PromptTokens: view.Usage.PromptTokens, CompletionTokens: view.Usage.CompletionTokens,
			TotalTokens: view.Usage.TotalTokens, ReasoningTokens: view.Usage.ReasoningTokens,
			CachedTokens: view.Usage.CachedTokens, RequestCount: view.Usage.RequestCount,
			CostUSD: view.Usage.CostUSD, CostKnown: view.Usage.CostKnown,
		}
		snapshot.HasUsage = true
	}
	if view.ModifiedFilesKnown {
		snapshot.ModifiedFiles = make([]surface.ModifiedFile, 0, len(view.ModifiedFiles))
		for _, file := range view.ModifiedFiles {
			snapshot.ModifiedFiles = append(snapshot.ModifiedFiles, surface.ModifiedFile{
				Path:      file.Path,
				Diff:      surface.SidebarDiff{Additions: file.Diff.Additions, Deletions: file.Diff.Deletions},
				UpdatedAt: file.UpdatedAt,
			})
		}
		snapshot.ModifiedFilesKnown = true
	}
	if view.MCPKnown {
		snapshot.MCPKnown = true
		for _, server := range view.MCP {
			snapshot.MCP = append(snapshot.MCP, surface.MCPServer{Name: server.Name, State: server.State})
		}
	}
	if view.SkillsKnown {
		snapshot.SkillsKnown = true
		for _, skill := range view.Skills {
			snapshot.Skills = append(snapshot.Skills, surface.SidebarSkill{Name: skill.Name})
		}
	}
	if view.LSPKnown {
		snapshot.LSPKnown = true
		for _, server := range view.LSP {
			if server.State != "starting" && server.State != "initialized" {
				continue
			}
			snapshot.LSP = append(snapshot.LSP, surface.LanguageServer{Language: server.Language, State: server.State})
		}
	}
	return snapshot
}

type messageView struct {
	ID           string                `json:"id"`
	Role         string                `json:"role"`
	Content      string                `json:"content"`
	Attachments  []surface.Attachment  `json:"attachments,omitempty"`
	FileContexts []surface.FileContext `json:"file_contexts,omitempty"`
	// ContextFiles is accepted as a compatibility spelling for older server
	// snapshots; both fields are metadata-only and never carry body content.
	ContextFiles []surface.FileContext `json:"context_files,omitempty"`
	ToolName     string                `json:"tool_name,omitempty"`
	ToolCallID   string                `json:"tool_call_id,omitempty"`
	ToolPreview  string                `json:"tool_preview,omitempty"`
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

func (c *Client) sessionSidebar(ctx context.Context, sessionID string) (sidebarView, error) {
	raw, err := c.Call(ctx, "session/sidebar", map[string]string{"session_id": sessionID})
	if err != nil {
		return sidebarView{}, err
	}
	var view sidebarView
	if err := json.Unmarshal(raw, &view); err != nil {
		return sidebarView{}, fmt.Errorf("tui: session/sidebar: %w", err)
	}
	if view.Session.ID == "" {
		return sidebarView{}, fmt.Errorf("tui: session/sidebar returned no session")
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
	return c.startTurnWithAttachmentsAndContext(ctx, sessionID, text, face, thinking, nil, nil)
}

func (c *Client) startTurnWithAttachments(ctx context.Context, sessionID, text, face, thinking string, attachments []surface.Attachment) (runAccepted, error) {
	return c.startTurnWithAttachmentsAndContext(ctx, sessionID, text, face, thinking, attachments, nil)
}

func (c *Client) startTurnWithContext(ctx context.Context, sessionID, text, face, thinking string, paths []string) (runAccepted, error) {
	return c.startTurnWithAttachmentsAndContext(ctx, sessionID, text, face, thinking, nil, paths)
}

func (c *Client) startTurnWithAttachmentsAndContext(ctx context.Context, sessionID, text, face, thinking string, attachments []surface.Attachment, contextPaths []string) (runAccepted, error) {
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
	if len(contextPaths) > 0 {
		params["context_paths"] = append([]string(nil), contextPaths...)
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

func (c *Client) startShell(ctx context.Context, sessionID, script string) (runAccepted, error) {
	// shell/start intentionally accepts only the session and script. Face,
	// policy and filesystem authority are derived by the server-owned runtime.
	raw, err := c.Call(ctx, "shell/start", map[string]string{
		"session_id": sessionID,
		"script":     script,
	})
	if err != nil {
		return runAccepted{}, err
	}
	var accepted runAccepted
	if err := json.Unmarshal(raw, &accepted); err != nil {
		return runAccepted{}, fmt.Errorf("tui: shell/start: %w", err)
	}
	if accepted.RunID == "" {
		return runAccepted{}, fmt.Errorf("tui: shell/start returned no run_id")
	}
	return accepted, nil
}

func (c *Client) resolveProjectContext(ctx context.Context, paths []string) ([]surface.FileContext, error) {
	clean := append([]string(nil), paths...)
	raw, err := c.Call(ctx, "project-context/resolve", map[string]any{"paths": clean})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Contexts     []surface.FileContext `json:"contexts"`
		FileContexts []surface.FileContext `json:"file_contexts"`
		Files        []surface.FileContext `json:"files"`
		Items        []surface.FileContext `json:"items"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		var direct []surface.FileContext
		if directErr := json.Unmarshal(raw, &direct); directErr != nil {
			return nil, fmt.Errorf("tui: project-context/resolve: %w", err)
		}
		if len(direct) != len(clean) {
			return nil, fmt.Errorf("tui: project-context/resolve returned %d contexts, want %d", len(direct), len(clean))
		}
		return direct, nil
	}
	contexts := envelope.Contexts
	if len(contexts) == 0 {
		contexts = envelope.FileContexts
	}
	if len(contexts) == 0 {
		contexts = envelope.Files
	}
	if len(contexts) == 0 {
		contexts = envelope.Items
	}
	if len(contexts) != len(clean) {
		return nil, fmt.Errorf("tui: project-context/resolve returned %d contexts, want %d", len(contexts), len(clean))
	}
	return contexts, nil
}

func (c *Client) listProjectContext(ctx context.Context, query string) ([]surface.FileContext, bool, error) {
	raw, err := c.Call(ctx, "project-context/list", map[string]any{"query": query, "limit": 200})
	if err != nil {
		return nil, false, err
	}
	var envelope struct {
		Contexts     []surface.FileContext `json:"contexts"`
		FileContexts []surface.FileContext `json:"file_contexts"`
		Files        []surface.FileContext `json:"files"`
		Items        []surface.FileContext `json:"items"`
		Truncated    bool                  `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		var direct []surface.FileContext
		if directErr := json.Unmarshal(raw, &direct); directErr != nil {
			return nil, false, fmt.Errorf("tui: project-context/list: %w", err)
		}
		return direct, false, nil
	}
	contexts := envelope.Contexts
	if len(contexts) == 0 {
		contexts = envelope.FileContexts
	}
	if len(contexts) == 0 {
		contexts = envelope.Files
	}
	if len(contexts) == 0 {
		contexts = envelope.Items
	}
	return contexts, envelope.Truncated, nil
}

func (c *Client) subscribe(ctx context.Context, runID string, afterSeq int) (string, error) {
	raw, err := c.Call(ctx, "run/subscribe", map[string]any{"run_id": runID, "after_seq": afterSeq})
	if err != nil {
		return "", err
	}
	var result struct {
		SubscriptionID string `json:"subscription_id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("tui: run/subscribe: %w", err)
	}
	if strings.TrimSpace(result.SubscriptionID) == "" {
		return "", fmt.Errorf("tui: run/subscribe returned no subscription_id")
	}
	return result.SubscriptionID, nil
}

func (c *Client) unsubscribe(ctx context.Context, subscriptionID string) error {
	if strings.TrimSpace(subscriptionID) == "" {
		return nil
	}
	_, err := c.Call(ctx, "run/unsubscribe", map[string]string{"subscription_id": subscriptionID})
	return err
}

func (c *Client) cancelRun(ctx context.Context, runID string) error {
	_, err := c.Call(ctx, "run/cancel", map[string]string{"run_id": runID})
	return err
}

func (c *Client) runStatus(ctx context.Context, runID string) (string, error) {
	raw, err := c.Call(ctx, "run/get", map[string]string{"run_id": runID})
	if err != nil {
		return "", err
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("tui: run/get: %w", err)
	}
	if strings.TrimSpace(result.Status) == "" {
		return "", fmt.Errorf("tui: run/get returned no status")
	}
	return result.Status, nil
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
		if message.ToolName != "" {
			content := message.Content
			if content == "" {
				content = message.ToolPreview
			}
			fmt.Fprintf(&b, "tool %s: %s\n", message.ToolName, strings.TrimSpace(content))
			continue
		}
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
		if chips := formatFileContextMetadata(mergedFileContexts(message)); chips != "" {
			if content != "" {
				content += " "
			}
			content += chips
		}
		fmt.Fprintf(&b, "%s: %s\n", role, content)
	}
	return b.String()
}

func formatFileContextMetadata(contexts []surface.FileContext) string {
	if len(contexts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(contexts))
	for _, context := range contexts {
		name := strings.TrimSpace(context.Name)
		if name == "" {
			name = strings.TrimSpace(context.Path)
		}
		if name == "" {
			name = "file"
		}
		parts = append(parts, "[file: "+name+"]")
	}
	return strings.Join(parts, " ")
}

func mergedFileContexts(message messageView) []surface.FileContext {
	if len(message.FileContexts) > 0 {
		return message.FileContexts
	}
	return message.ContextFiles
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
