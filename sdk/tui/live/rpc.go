package live

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"agent-vivy/sdk/tui/surface"
)

// Transport is the complete authority available to the shared TUI client.
// Implementations may be an in-process FaceEnv or a remote JSON-RPC peer.
type Transport interface {
	Call(context.Context, string, any) (json.RawMessage, error)
	OnNotify(func(method string, params json.RawMessage))
}

// client owns the protocol projection shared by every first-party TUI entry.
type client struct {
	transport Transport

	mu   sync.RWMutex
	caps map[string]struct{}
}

func (c *client) setCapabilities(raw json.RawMessage) error {
	var envelope struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	c.mu.Lock()
	c.caps = make(map[string]struct{}, len(envelope.Capabilities))
	for _, capability := range envelope.Capabilities {
		c.caps[capability] = struct{}{}
	}
	c.mu.Unlock()
	return nil
}

func (c *client) SupportsCapability(name string) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.caps[name]
	return ok
}

func newClient(transport Transport) *client {
	return &client{transport: transport}
}

// OnNotify registers the callback for server notifications (run/event).
func (c *client) OnNotify(fn func(method string, params json.RawMessage)) {
	if c != nil && c.transport != nil {
		c.transport.OnNotify(fn)
	}
}

// Call issues one JSON-RPC method through the face environment.
func (c *client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c == nil || c.transport == nil {
		return nil, fmt.Errorf("tui: client is not connected")
	}
	return c.transport.Call(ctx, method, params)
}

type sessionView struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	PermissionPreset string `json:"permission_preset"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type providerEntryView struct {
	DisplayName  string   `json:"display_name"`
	Bundle       string   `json:"bundle"`
	BaseURL      string   `json:"base_url"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
}

type providersView struct {
	Entries        []providerEntryView `json:"entries"`
	Bundles        []providerEntryView `json:"bundles"`
	ActiveProvider string              `json:"active_provider"`
	ActiveModel    string              `json:"active_model"`
	ActiveBaseURL  string              `json:"active_base_url"`
	ReadOnly       bool                `json:"read_only"`
	Frozen         bool                `json:"frozen"`
	ConfigProvider string              `json:"config_provider"`
	ConfigModel    string              `json:"config_model"`
}

func mapProvidersView(view providersView) surface.ModelCatalog {
	currentProvider, currentModel, currentBaseURL := view.ActiveProvider, view.ActiveModel, view.ActiveBaseURL
	if currentProvider == "" || currentModel == "" {
		currentProvider, currentModel, currentBaseURL = view.ConfigProvider, view.ConfigModel, ""
	}
	options := make([]surface.ModelOption, 0)
	seen := make(map[string]struct{})
	add := func(provider, model, baseURL, display string) {
		provider, model, baseURL = strings.TrimSpace(provider), strings.TrimSpace(model), strings.TrimSpace(baseURL)
		if provider == "" || model == "" {
			return
		}
		key := provider + "\x00" + model + "\x00" + baseURL
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		options = append(options, surface.ModelOption{Provider: provider, Model: model, BaseURL: baseURL, DisplayName: strings.TrimSpace(display), Current: provider == currentProvider && model == currentModel && baseURL == currentBaseURL})
	}
	add(view.ConfigProvider, view.ConfigModel, "", view.ConfigProvider)
	for _, bundle := range view.Bundles {
		add(bundle.Bundle, bundle.DefaultModel, "", bundle.DisplayName)
		for _, model := range bundle.Models {
			add(bundle.Bundle, model, "", bundle.DisplayName)
		}
	}
	for _, entry := range view.Entries {
		add(entry.Bundle, entry.DefaultModel, entry.BaseURL, entry.DisplayName)
		for _, model := range entry.Models {
			add(entry.Bundle, model, entry.BaseURL, entry.DisplayName)
		}
	}
	// A legacy active selection may no longer have a registry row. Keep it
	// visible as current, but add it last so matching configured entries retain
	// their operator-facing display name.
	add(currentProvider, currentModel, currentBaseURL, currentProvider)
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Current != options[j].Current {
			return options[i].Current
		}
		left := strings.ToLower(options[i].DisplayName + "\x00" + options[i].Provider + "\x00" + options[i].Model)
		right := strings.ToLower(options[j].DisplayName + "\x00" + options[j].Provider + "\x00" + options[j].Model)
		return left < right
	})
	return surface.ModelCatalog{Options: options, ReadOnly: view.ReadOnly, Frozen: view.Frozen}
}

func (c *client) modelCatalog(ctx context.Context) (surface.ModelCatalog, error) {
	raw, err := c.Call(ctx, "settings/providers", nil)
	if err != nil {
		return surface.ModelCatalog{}, err
	}
	var view providersView
	if err := json.Unmarshal(raw, &view); err != nil {
		return surface.ModelCatalog{}, fmt.Errorf("tui: settings/providers: %w", err)
	}
	return mapProvidersView(view), nil
}

func (c *client) selectModel(ctx context.Context, option surface.ModelOption) (surface.ModelCatalog, error) {
	raw, err := c.Call(ctx, "settings/model/select", map[string]string{"provider": option.Provider, "model": option.Model, "base_url": option.BaseURL})
	if err != nil {
		return surface.ModelCatalog{}, err
	}
	var view providersView
	if err := json.Unmarshal(raw, &view); err != nil {
		return surface.ModelCatalog{}, fmt.Errorf("tui: settings/model/select: %w", err)
	}
	return mapProvidersView(view), nil
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

type dynamicCommandView struct {
	ID          string                       `json:"id"`
	Kind        string                       `json:"kind"`
	Name        string                       `json:"name"`
	Usage       string                       `json:"usage"`
	Description string                       `json:"description"`
	Arguments   []dynamicCommandArgumentView `json:"arguments,omitempty"`
}

type dynamicCommandArgumentView struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type dynamicCommandsView struct {
	Commands []dynamicCommandView `json:"commands"`
}

type dynamicCommandExpansionView struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func (c *client) dynamicCommands(ctx context.Context) ([]surface.DynamicCommand, error) {
	raw, err := c.Call(ctx, "commands/list", nil)
	if err != nil {
		return nil, err
	}
	var view dynamicCommandsView
	if err := json.Unmarshal(raw, &view); err != nil {
		return nil, fmt.Errorf("tui: commands/list: %w", err)
	}
	out := make([]surface.DynamicCommand, 0, len(view.Commands))
	for _, item := range view.Commands {
		arguments := make([]surface.DynamicCommandArgument, 0, len(item.Arguments))
		for _, argument := range item.Arguments {
			arguments = append(arguments, surface.DynamicCommandArgument{Name: argument.Name, Description: argument.Description, Required: argument.Required})
		}
		out = append(out, surface.DynamicCommand{ID: item.ID, Kind: item.Kind, Name: item.Name, Usage: item.Usage, Description: item.Description, Arguments: arguments})
	}
	return out, nil
}

func (c *client) expandDynamicCommand(ctx context.Context, id string, args []string) (dynamicCommandExpansionView, error) {
	raw, err := c.Call(ctx, "commands/expand", map[string]any{"id": id, "args": append([]string(nil), args...)})
	if err != nil {
		return dynamicCommandExpansionView{}, err
	}
	var view dynamicCommandExpansionView
	if err := json.Unmarshal(raw, &view); err != nil {
		return dynamicCommandExpansionView{}, fmt.Errorf("tui: commands/expand: %w", err)
	}
	return view, nil
}

type sidebarMCPView struct {
	Name        string   `json:"name"`
	Transport   string   `json:"transport"`
	State       string   `json:"state"`
	Error       string   `json:"error,omitempty"`
	AuthMissing bool     `json:"auth_missing"`
	EnvMissing  []string `json:"env_missing,omitempty"`
	ToolCount   *int     `json:"tool_count,omitempty"`
}

type sidebarSkillView struct {
	Name   string `json:"name"`
	Origin string `json:"origin,omitempty"`
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

func mapContextView(view contextView) surface.Context {
	return surface.Context{
		FeedTokens: view.FeedTokens, ModelLimitTokens: view.ModelLimitTokens,
		TokenCountsEstimated: view.TokenCountsEstimated, ModelLimitKnown: view.ModelLimitKnown,
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
			toolCount := -1
			if server.ToolCount != nil && *server.ToolCount >= 0 {
				toolCount = *server.ToolCount
			}
			snapshot.MCP = append(snapshot.MCP, surface.MCPServer{
				Name:        server.Name,
				Transport:   server.Transport,
				State:       server.State,
				Error:       server.Error,
				AuthMissing: server.AuthMissing,
				EnvMissing:  append([]string(nil), server.EnvMissing...),
				ToolCount:   toolCount,
			})
		}
	}
	if view.SkillsKnown {
		snapshot.SkillsKnown = true
		for _, skill := range view.Skills {
			snapshot.Skills = append(snapshot.Skills, surface.SidebarSkill{Name: skill.Name, Origin: skill.Origin})
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
	ContextFiles []surface.FileContext `json:"context_files,omitempty"`
	ToolName     string                `json:"tool_name,omitempty"`
	ToolCallID   string                `json:"tool_call_id,omitempty"`
	ToolPreview  string                `json:"tool_preview,omitempty"`
}

type runAccepted struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

func (c *client) createSession(ctx context.Context, title string) (sessionView, error) {
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

func (c *client) listSessions(ctx context.Context) ([]sessionView, error) {
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

func (c *client) getSession(ctx context.Context, sessionID string) (sessionView, error) {
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

func (c *client) sessionContext(ctx context.Context, sessionID string) (contextView, error) {
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

func (c *client) sessionSidebar(ctx context.Context, sessionID string) (sidebarView, error) {
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

func (c *client) renameSession(ctx context.Context, sessionID, title string) (sessionView, error) {
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

func (c *client) deleteSession(ctx context.Context, sessionID string) error {
	_, err := c.Call(ctx, "session/delete", map[string]string{"session_id": sessionID})
	return err
}

func (c *client) sessionMessages(ctx context.Context, sessionID string) ([]messageView, error) {
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

func (c *client) startTurn(ctx context.Context, sessionID, text, thinking string) (runAccepted, error) {
	return c.startTurnWithAttachmentsAndContext(ctx, sessionID, text, thinking, "", nil, nil)
}

func (c *client) startTurnWithAttachments(ctx context.Context, sessionID, text, thinking string, attachments []surface.Attachment) (runAccepted, error) {
	return c.startTurnWithAttachmentsAndContext(ctx, sessionID, text, thinking, "", attachments, nil)
}

func (c *client) startTurnWithContext(ctx context.Context, sessionID, text, thinking string, paths []string) (runAccepted, error) {
	return c.startTurnWithAttachmentsAndContext(ctx, sessionID, text, thinking, "", nil, paths)
}

func (c *client) startTurnWithAttachmentsAndContext(ctx context.Context, sessionID, text, thinking, mode string, attachments []surface.Attachment, contextPaths []string) (runAccepted, error) {
	params := map[string]any{
		"session_id": sessionID,
		"text":       text,
		"face":       "code",
		"thinking":   thinking,
	}
	if mode = strings.TrimSpace(mode); mode != "" && mode != "normal" {
		params["mode"] = mode
	}
	if len(attachments) > 0 {
		paths := make([]string, 0, len(attachments))
		for _, attachment := range attachments {
			if path := strings.TrimSpace(attachment.Path); path != "" {
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

func (c *client) startShell(ctx context.Context, sessionID, script string) (runAccepted, error) {
	// shell/start intentionally accepts only session_id and script. Policy,
	// approval and execution remain runtime-owned by the control plane.
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

func (c *client) resolveProjectContext(ctx context.Context, paths []string) ([]surface.FileContext, error) {
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

func (c *client) listProjectContext(ctx context.Context, query string) ([]surface.FileContext, bool, error) {
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

func (c *client) resolveAttachments(ctx context.Context, paths []string) ([]surface.Attachment, error) {
	raw, err := c.Call(ctx, "attachments/resolve", map[string]any{"attachment_paths": append([]string(nil), paths...)})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Attachments []surface.Attachment `json:"attachments"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("tui: attachments/resolve: %w", err)
	}
	if len(envelope.Attachments) != len(paths) {
		return nil, fmt.Errorf("tui: attachments/resolve returned %d attachments, want %d", len(envelope.Attachments), len(paths))
	}
	return envelope.Attachments, nil
}

func (c *client) setSessionPermission(ctx context.Context, sessionID, preset string) (sessionView, error) {
	raw, err := c.Call(ctx, "session/set_permission", map[string]string{"session_id": sessionID, "preset": preset})
	if err != nil {
		return sessionView{}, err
	}
	var session sessionView
	if err := json.Unmarshal(raw, &session); err != nil {
		return sessionView{}, fmt.Errorf("tui: session/set_permission: %w", err)
	}
	return session, nil
}

func (c *client) subscribe(ctx context.Context, runID string, afterSeq int) (string, error) {
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

func (c *client) unsubscribe(ctx context.Context, subscriptionID string) error {
	if strings.TrimSpace(subscriptionID) == "" {
		return nil
	}
	_, err := c.Call(ctx, "run/unsubscribe", map[string]string{"subscription_id": subscriptionID})
	return err
}

func (c *client) cancelRun(ctx context.Context, runID string) error {
	_, err := c.Call(ctx, "run/cancel", map[string]string{"run_id": runID})
	return err
}

func (c *client) runStatus(ctx context.Context, runID string) (string, error) {
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

func (c *client) respondApproval(ctx context.Context, approvalID, decision string) error {
	_, err := c.Call(ctx, "approval/respond", map[string]string{
		"approval_id": approvalID,
		"decision":    decision,
	})
	return err
}

func (c *client) respondQuestion(ctx context.Context, questionID, answer string) error {
	_, err := c.Call(ctx, "question/respond", map[string]string{
		"question_id": questionID,
		"answer":      answer,
	})
	return err
}
