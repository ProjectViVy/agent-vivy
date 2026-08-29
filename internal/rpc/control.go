package rpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/eval"
	"agent-vivy/internal/events"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/studio"
	"agent-vivy/internal/tools"
)

const (
	CodeNotFound = -32004
	CodeConflict = -32009
	// CodeBadGateway reports a failed marketplace upstream call (skills.sh)
	// so the UI can offer a retry instead of reading it as a server bug.
	CodeBadGateway = -32010
)

type ControlDeps struct {
	Sessions  storage.SessionStore
	Messages  storage.MessageStore
	Runs      storage.RunStore
	Journal   storage.Journal
	Approvals storage.ApprovalStore
	Questions storage.QuestionStore
	Reviews   storage.ReviewStore
	Todos     storage.TodoStore
	// Skills is the read-only control-plane catalog over skills_root.
	// Nil disables skills/list and skills/get.
	Skills tools.SkillOperations
	// Marketplace adapts the skills.sh directory onto skills_root. Nil
	// disables the skills/marketplace/* methods and the skills.marketplace
	// capability.
	Marketplace tools.SkillsMarketplace
	// SkillRevisions lists staged HITL Skill mutations for
	// skills/revisions/list. Nil disables the method.
	SkillRevisions storage.SkillRevisionStore
	Bus            *events.Bus
	Service        *runtime.Service
	Studio         *studio.Service
	Live           studio.LiveView
	Eval           eval.Starter
	Children       ChildController
	// SettingsPath is the operator-managed model provider settings document.
	// When empty the settings RPCs report the config defaults and reject
	// updates (read-only mode).
	SettingsPath string
	// ConfigProvider is the production config default provider (non-secret),
	// surfaced by settings/get so the UI can show the fallback.
	ConfigProvider string
	// ConfigModel is the production config default model (non-secret).
	ConfigModel string
	// ConfigNetworkSearchProvider is the production config network_search
	// preference (non-secret), surfaced by settings/get.
	ConfigNetworkSearchProvider string
	// ConfigExecuteMaxTimeoutSeconds is the config execute ceiling after the
	// settings overlay, surfaced by settings/get as the UI placeholder.
	ConfigExecuteMaxTimeoutSeconds int
	// DefaultPermissionPreset is the named sandbox/approval bundle applied
	// to newly created sessions.
	DefaultPermissionPreset domain.PermissionPreset
	// SandboxWorkspaceRoot is the live workspace root shown in Settings.
	SandboxWorkspaceRoot string
	// ExecuteAllowedCommands is the command allowlist shown in Settings.
	ExecuteAllowedCommands []string
	// ConfigSandboxDenyPrivateIPs is the production config network default.
	ConfigSandboxDenyPrivateIPs bool
	// ConfigSandboxAllowedDomains is the production config domain allowlist.
	ConfigSandboxAllowedDomains []string
	// ApplySettingsEnv applies a persisted settings document's non-secret
	// overlays to the running process environment (VIVY_API_BASE for
	// base_url, the active bundle's env_key for the resolved api_key). It is
	// wired by the composition root so a settings/providers write updates the
	// environment immediately; the startup overlay replays the same document
	// on the next launch. Nil means no live apply (read-only deployments).
	ApplySettingsEnv func(settings.Settings)
	// TokenUsage provides cross-run usage aggregation for stats/tokens.
	// Nil disables the method.
	TokenUsage storage.TokenUsageStore
	// MCP is the live Streamable HTTP catalog. Writes replace it immediately.
	// Nil disables settings/mcp* methods.
	MCP MCPCatalog
	// Frozen is true when this process is locked to an ENV session. Provider
	// writes are rejected and the UI is read-only for model fields.
	Frozen bool
	// ConfigCompaction is the config-file context compression default
	// (before any settings overlay), surfaced by settings/get as the UI
	// fallback for cleared fields.
	ConfigCompaction runtime.CompactionPolicy
	// OnSettingsChanged is invoked after a successful settings write so the
	// composition root can invalidate the live model cache and refresh live
	// overlays (MCP catalog, sandbox, engine compaction). Nil is a no-op.
	OnSettingsChanged func()
	// ModelLists discovers the upstream OpenAI-compatible /models catalog for
	// settings/providers/refresh. Nil uses the package default 15s client.
	ModelLists *provider.ModelListClient
}

// MCPCatalog is the live MCP backend surface the control plane manages.
type MCPCatalog interface {
	ListTools(context.Context, domain.RunID, string) (tools.MCPListResponse, error)
	ReplaceServers([]runtime.MCPServerConfig)
}

// ChildRequest starts one durable, asynchronous child run under a parent.
// The parent controller derives policy hash, workspace, and budget from the
// durable parent; callers cannot supply a wider authority.
type ChildRequest struct {
	ParentRunID   string   `json:"parent_run_id"`
	Text          string   `json:"text"`
	PolicyProfile string   `json:"policy_profile,omitempty"`
	ToolNames     []string `json:"tool_names,omitempty"`
}

type ChildResult struct {
	ID          string `json:"id"`
	ParentRunID string `json:"parent_run_id"`
	RootRunID   string `json:"root_run_id"`
	SessionID   string `json:"session_id"`
	Status      string `json:"status"`
	Depth       int    `json:"depth"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Result      string `json:"result,omitempty"`
	Error       string `json:"error,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

type ChildController interface {
	StartChild(context.Context, ChildRequest) (ChildResult, error)
	GetChild(context.Context, string) (ChildResult, error)
	ListChildren(context.Context, string, bool) ([]ChildResult, error)
	WaitChild(context.Context, string) (ChildResult, error)
	CancelChild(context.Context, string) (ChildResult, error)
}

func NewControlHandler(deps ControlDeps) (Handler, error) {
	if deps.Sessions == nil || deps.Messages == nil || deps.Runs == nil || deps.Journal == nil ||
		deps.Approvals == nil || deps.Questions == nil || deps.Bus == nil || deps.Service == nil {
		return nil, errors.New("rpc: control dependencies are incomplete")
	}
	return &controlHandler{deps: deps, subscriptions: make(map[string]context.CancelFunc)}, nil
}

type controlHandler struct {
	deps ControlDeps

	mu            sync.Mutex
	subscriptions map[string]context.CancelFunc
}

type sessionParams struct {
	SessionID string `json:"session_id"`
}

type turnParams struct {
	SessionID     string `json:"session_id"`
	Text          string `json:"text"`
	Mode          string `json:"mode,omitempty"`
	PolicyProfile string `json:"policy_profile,omitempty"`
}

type runParams struct {
	RunID string `json:"run_id"`
}

type subscribeParams struct {
	RunID    string `json:"run_id"`
	AfterSeq int64  `json:"after_seq,omitempty"`
}

type approvalParams struct {
	ApprovalID string `json:"approval_id"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason,omitempty"`
}

type questionParams struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

type reviewListParams struct {
	Kind      domain.ReviewKind   `json:"kind,omitempty"`
	Status    domain.ReviewStatus `json:"status,omitempty"`
	SessionID domain.SessionID    `json:"session_id,omitempty"`
	Limit     int                 `json:"limit,omitempty"`
}

type reviewRespondParams struct {
	ReviewID string `json:"review_id"`
	Action   string `json:"action"`
	Decision string `json:"decision,omitempty"`
	Answer   string `json:"answer,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type unsubscribeParams struct {
	SubscriptionID string `json:"subscription_id"`
}

type skillGetParams struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

type skillSummaryResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Context     string   `json:"context,omitempty"`
	Agent       string   `json:"agent,omitempty"`
	Model       string   `json:"model,omitempty"`
	Enabled     bool     `json:"enabled"`
	Hash        string   `json:"hash"`
	Warnings    []string `json:"warnings"`
}

type skillViewResult struct {
	skillSummaryResult
	Content         string   `json:"content"`
	RelativePath    string   `json:"relative_path"`
	SupportingFiles []string `json:"supporting_files"`
}

type skillSetEnabledParams struct {
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	BaseHash string `json:"base_hash"`
}

type marketplaceSearchParams struct {
	Q     string `json:"q"`
	Limit int    `json:"limit,omitempty"`
}

type marketplaceInstallParams struct {
	ID string `json:"id"`
}

type skillRevisionResult struct {
	ID         string   `json:"id"`
	RunID      string   `json:"run_id,omitempty"`
	SkillName  string   `json:"skill_name"`
	Action     string   `json:"action"`
	TargetPath string   `json:"target_path"`
	Preview    string   `json:"preview"`
	Warnings   []string `json:"warnings"`
	Status     string   `json:"status"`
	CreatedAt  int64    `json:"created_at"`
}

type sessionResult struct {
	ID               domain.SessionID        `json:"id"`
	Title            string                  `json:"title"`
	CreatedAt        int64                   `json:"created_at"`
	SandboxMode      domain.SandboxMode      `json:"sandbox_mode"`
	ApprovalPolicy   domain.ApprovalPolicy   `json:"approval_policy"`
	PermissionPreset domain.PermissionPreset `json:"permission_preset"`
}

type messageResult struct {
	ID        string       `json:"id"`
	RunID     domain.RunID `json:"run_id,omitempty"`
	Role      domain.Role  `json:"role"`
	Content   string       `json:"content"`
	CreatedAt int64        `json:"created_at"`
}

type runResult struct {
	ID        domain.RunID     `json:"id"`
	SessionID domain.SessionID `json:"session_id"`
	Status    domain.RunStatus `json:"status"`
	CreatedAt int64            `json:"created_at"`
}

type eventResult struct {
	RunID          domain.RunID     `json:"run_id"`
	Seq            domain.EventSeq  `json:"seq"`
	Type           domain.EventType `json:"type"`
	CreatedAt      int64            `json:"created_at"`
	PayloadVersion int              `json:"payload_version"`
	Payload        json.RawMessage  `json:"payload"`
}

type todoResult struct {
	ID          string            `json:"id"`
	SessionID   domain.SessionID  `json:"session_id"`
	Subject     string            `json:"subject"`
	Description string            `json:"description"`
	Status      domain.TodoStatus `json:"status"`
	Blocks      []string          `json:"blocks"`
	BlockedBy   []string          `json:"blocked_by"`
	ActiveForm  string            `json:"active_form,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Position    int               `json:"position"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

type preflightResult struct {
	Status        runtime.PreflightStatus `json:"status"`
	Mode          domain.RunMode          `json:"mode"`
	PolicyProfile domain.PolicyProfile    `json:"policy_profile"`
	PolicyHash    string                  `json:"policy_hash,omitempty"`
	SelectedTools []string                `json:"selected_tools"`
	ToolDecisions []policyDecisionResult  `json:"tool_decisions"`
	ContextBytes  int                     `json:"context_bytes"`
	HookReady     bool                    `json:"hook_ready"`
	Warnings      []string                `json:"warnings"`
	Blockers      []string                `json:"blockers"`
	NextActions   []string                `json:"next_actions"`
}

type policyDecisionResult struct {
	ToolName string                `json:"tool_name"`
	Decision domain.PolicyDecision `json:"decision"`
	Reason   string                `json:"reason"`
}

type approvalResult struct {
	ID         string       `json:"id"`
	RunID      domain.RunID `json:"run_id"`
	ToolCallID string       `json:"tool_call_id"`
	Decision   string       `json:"decision"`
	ExpiresAt  int64        `json:"expires_at"`
}

type questionResult struct {
	ID         string                `json:"id"`
	RunID      domain.RunID          `json:"run_id"`
	ToolCallID string                `json:"tool_call_id"`
	Prompt     string                `json:"prompt"`
	Status     domain.QuestionStatus `json:"status"`
	ExpiresAt  int64                 `json:"expires_at"`
}

type reviewResult struct {
	ID               string              `json:"id"`
	Kind             domain.ReviewKind   `json:"kind"`
	Status           domain.ReviewStatus `json:"status"`
	SessionID        domain.SessionID    `json:"session_id"`
	SessionTitle     string              `json:"session_title,omitempty"`
	RunID            domain.RunID        `json:"run_id"`
	ToolCallID       string              `json:"tool_call_id,omitempty"`
	ToolName         string              `json:"tool_name,omitempty"`
	Source           string              `json:"source,omitempty"`
	Actor            string              `json:"actor,omitempty"`
	CreatedAt        int64               `json:"created_at"`
	ExpiresAt        int64               `json:"expires_at"`
	DecidedAt        int64               `json:"decided_at,omitempty"`
	Action           string              `json:"action,omitempty"`
	Target           string              `json:"target,omitempty"`
	PreconditionHash string              `json:"precondition_hash,omitempty"`
	Preview          string              `json:"preview,omitempty"`
	RiskFindings     []string            `json:"risk_findings,omitempty"`
	Arguments        json.RawMessage     `json:"arguments,omitempty"`
	Prompt           string              `json:"prompt,omitempty"`
	DecisionReason   string              `json:"decision_reason,omitempty"`
	StaleReason      string              `json:"stale_reason,omitempty"`
	Error            string              `json:"error,omitempty"`
	Effect           string              `json:"effect,omitempty"`
	Reversibility    string              `json:"reversibility,omitempty"`
	Scope            string              `json:"scope,omitempty"`
	Trust            string              `json:"trust,omitempty"`
}

type backgroundResult struct {
	ID          domain.RunID     `json:"id"`
	SessionID   domain.SessionID `json:"session_id"`
	Status      domain.RunStatus `json:"status"`
	CreatedAt   int64            `json:"created_at"`
	WorkspaceID string           `json:"workspace_id,omitempty"`
}

func (h *controlHandler) Handle(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	switch request.Method {
	case "initialize", "capabilities":
		capabilities := []string{
			"session", "session.todos", "session.set_permission", "turn", "run", "preflight", "approval", "question", "review", "run.subscribe",
			"background.recover", "background.list", "background.attach",
			"child.start", "child.get", "child.list", "child.wait", "child.cancel",
			"generations.list", "generations.get", "generations.create", "evals.list", "evals.record", "evals.start", "promotions.list", "promotions.promote",
			"generations.reject", "species.inspect",
			"settings.get", "settings.update",
			"settings.providers", "settings.providers.upsert", "settings.providers.delete", "settings.providers.refresh",
			"settings.mcp", "settings.mcp.upsert", "settings.mcp.delete", "settings.mcp.probe",
			"session.context", "context.compact",
			"stats.tokens",
			"skills.list", "skills.get",
		}
		if h.deps.Marketplace != nil {
			capabilities = append(capabilities, "skills.marketplace")
		}
		if h.deps.SkillRevisions != nil {
			capabilities = append(capabilities, "skills.revisions")
		}
		return map[string]any{
			"protocol_version": ProtocolVersion,
			"capabilities":     capabilities,
		}, nil
	case "session/create":
		return h.createSession(ctx, request)
	case "session/set_permission":
		return h.setSessionPermission(ctx, request)
	case "session/list":
		return h.listSessions(ctx)
	case "session/get":
		return h.getSession(ctx, request)
	case "session/rename":
		return h.renameSession(ctx, request)
	case "session/delete":
		return h.deleteSession(ctx, request)
	case "session/messages":
		return h.listMessages(ctx, request)
	case "session/context":
		return h.sessionContext(ctx, request)
	case "context/compact":
		return h.compactContext(ctx, request)
	case "session/todos":
		return h.listTodos(ctx, request)
	case "preflight/run":
		return h.preflight(ctx, request)
	case "turn/start":
		return h.startTurn(ctx, request)
	case "turn/interrupt", "run/cancel":
		return h.cancelRun(request)
	case "run/get":
		return h.getRun(ctx, request)
	case "run/subscribe":
		return h.subscribe(ctx, peer, request)
	case "run/unsubscribe":
		return h.unsubscribe(request)
	case "run/log":
		return h.runLog(ctx, request)
	case "approval/list":
		return h.listApprovals(ctx)
	case "approval/respond":
		return h.respondApproval(ctx, request)
	case "question/list":
		return h.listQuestions(ctx)
	case "question/respond":
		return h.respondQuestion(ctx, request)
	case "review/list":
		return h.listReviews(ctx, request)
	case "review/get":
		return h.getReview(ctx, request)
	case "review/respond":
		return h.respondReview(ctx, request)
	case "background/recover":
		if err := h.deps.Service.Recover(ctx); err != nil {
			return nil, internalError(err)
		}
		return map[string]any{"recovered": true}, nil
	case "background/list":
		return h.listBackground(ctx)
	case "background/attach":
		return h.attachBackground(ctx, request)
	case "child/start":
		return h.startChild(ctx, request)
	case "child/get":
		return h.getChild(ctx, request)
	case "child/list":
		return h.listChildren(ctx, request)
	case "child/wait":
		return h.waitChild(ctx, request)
	case "child/cancel":
		return h.cancelChild(ctx, request)
	case "generations/list":
		return h.listGenerations(ctx)
	case "generations/get":
		return h.getGeneration(ctx, request)
	case "generations/create":
		return h.createGeneration(ctx, request)
	case "generations/reject":
		return h.rejectGeneration(ctx, request)
	case "evals/list":
		return h.listEvals(ctx)
	case "evals/record":
		return h.recordEval(ctx, request)
	case "evals/start":
		return h.startEval(ctx, request)
	case "promotions/list":
		return h.listPromotions(ctx)
	case "promotions/promote":
		return h.promote(ctx, request)
	case "species/inspect":
		return h.inspectSpecies(ctx)
	case "settings/get":
		return h.getSettings(ctx)
	case "settings/update":
		return h.updateSettings(ctx, request)
	case "settings/providers":
		return h.listProviders(ctx)
	case "settings/providers/upsert":
		return h.upsertProvider(ctx, request)
	case "settings/providers/delete":
		return h.deleteProvider(ctx, request)
	case "settings/providers/refresh":
		return h.refreshProviderModels(ctx, request)
	case "settings/mcp":
		return h.listMCP(ctx)
	case "settings/mcp/upsert":
		return h.upsertMCP(ctx, request)
	case "settings/mcp/delete":
		return h.deleteMCP(ctx, request)
	case "settings/mcp/probe":
		return h.probeMCP(ctx, request)
	case "stats/tokens":
		return h.statsTokens(ctx, request)
	case "skills/list":
		return h.listSkills(ctx)
	case "skills/get":
		return h.getSkill(ctx, request)
	case "skills/set-enabled":
		return h.setSkillEnabled(ctx, request)
	case "skills/revisions/list":
		return h.listSkillRevisions(ctx)
	case "skills/marketplace/search":
		return h.searchMarketplace(ctx, request)
	case "skills/marketplace/featured":
		return h.featuredMarketplace(ctx)
	case "skills/marketplace/install":
		return h.installMarketplace(ctx, request)
	default:
		return nil, &Error{Code: MethodNotFound, Message: "method not found: " + request.Method}
	}
}

func (h *controlHandler) childController() (ChildController, *Error) {
	if h.deps.Children == nil {
		return nil, &Error{Code: MethodNotFound, Message: "child controller is not configured"}
	}
	return h.deps.Children, nil
}

func (h *controlHandler) startChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params ChildRequest
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ParentRunID == "" || params.Text == "" {
		return nil, &Error{Code: InvalidParams, Message: "parent_run_id and text are required"}
	}
	result, err := controller.StartChild(ctx, params)
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) getChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := controller.GetChild(ctx, params.RunID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "child run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) listChildren(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	var params struct {
		ParentRunID string `json:"parent_run_id"`
		Tree        bool   `json:"tree,omitempty"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ParentRunID == "" {
		return nil, &Error{Code: InvalidParams, Message: "parent_run_id is required"}
	}
	result, err := controller.ListChildren(ctx, params.ParentRunID, params.Tree)
	if err != nil {
		return nil, internalError(err)
	}
	return map[string]any{"children": result}, nil
}

func (h *controlHandler) waitChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := controller.WaitChild(ctx, params.RunID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "child run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) cancelChild(ctx context.Context, request Request) (any, *Error) {
	controller, rpcErr := h.childController()
	if rpcErr != nil {
		return nil, rpcErr
	}
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := controller.CancelChild(ctx, params.RunID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "child run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) createSession(ctx context.Context, request Request) (any, *Error) {
	var params struct {
		Title string `json:"title"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.Title == "" {
		params.Title = "New session"
	}
	session := domain.Session{ID: domain.SessionID(newControlID("sess_")), Title: params.Title, CreatedAt: nowMillis()}
	if mode, policy, ok := h.defaultPreset().Bundle(); ok {
		session.SandboxMode = string(mode)
		session.ApprovalPolicy = string(policy)
	}
	if err := h.deps.Sessions.CreateSession(ctx, session); err != nil {
		return nil, internalError(err)
	}
	return toSessionResult(session), nil
}

func (h *controlHandler) defaultPreset() domain.PermissionPreset {
	if h.deps.DefaultPermissionPreset.ValidSwitch() {
		return h.deps.DefaultPermissionPreset
	}
	return domain.PermissionPresetSmart
}

func toSessionResult(session domain.Session) sessionResult {
	mode, policy := session.EffectiveSandbox()
	return sessionResult{
		ID: session.ID, Title: session.Title, CreatedAt: session.CreatedAt,
		SandboxMode: mode, ApprovalPolicy: policy, PermissionPreset: domain.PermissionPresetOf(mode, policy),
	}
}

func (h *controlHandler) setSessionPermission(ctx context.Context, request Request) (any, *Error) {
	var params struct {
		SessionID string `json:"session_id"`
		Preset    string `json:"preset"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.SessionID == "" || params.Preset == "" {
		return nil, &Error{Code: InvalidParams, Message: "session_id and preset are required"}
	}
	preset := domain.PermissionPreset(params.Preset)
	mode, policy, ok := preset.Bundle()
	if !ok {
		return nil, &Error{Code: InvalidParams, Message: "preset must be cautious, smart, or trusted"}
	}
	if err := h.deps.Sessions.UpdateSandboxPolicy(ctx, domain.SessionID(params.SessionID), mode, policy); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &Error{Code: CodeNotFound, Message: "session not found"}
		}
		return nil, internalError(err)
	}
	session, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(params.SessionID))
	if err != nil {
		return nil, internalError(err)
	}
	return toSessionResult(session), nil
}

func (h *controlHandler) listSessions(ctx context.Context) (any, *Error) {
	sessions, err := h.deps.Sessions.ListSessions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]sessionResult, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, toSessionResult(session))
	}
	return map[string]any{"sessions": out}, nil
}

func (h *controlHandler) getSession(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	session, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(params.SessionID))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "session not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	messages, err := h.deps.Messages.ListMessages(ctx, session.ID)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]messageResult, 0, len(messages))
	for _, message := range messages {
		out = append(out, messageResult{ID: message.ID, RunID: message.RunID, Role: message.Role, Content: message.Content, CreatedAt: message.CreatedAt})
	}
	return map[string]any{
		"session":  toSessionResult(session),
		"messages": out,
	}, nil
}

func (h *controlHandler) deleteSession(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if err := h.deps.Sessions.DeleteSession(ctx, domain.SessionID(params.SessionID)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &Error{Code: CodeNotFound, Message: "session not found"}
		}
		return nil, internalError(err)
	}
	return map[string]any{"deleted": true}, nil
}

func (h *controlHandler) renameSession(ctx context.Context, request Request) (any, *Error) {
	var params struct {
		SessionID string `json:"session_id"`
		Title     string `json:"title"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.SessionID == "" || params.Title == "" {
		return nil, &Error{Code: InvalidParams, Message: "session_id and title are required"}
	}
	if err := h.deps.Sessions.RenameSession(ctx, domain.SessionID(params.SessionID), params.Title); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, &Error{Code: CodeNotFound, Message: "session not found"}
		}
		return nil, internalError(err)
	}
	session, err := h.deps.Sessions.GetSession(ctx, domain.SessionID(params.SessionID))
	if err != nil {
		return nil, internalError(err)
	}
	return toSessionResult(session), nil
}

func (h *controlHandler) listMessages(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	messages, err := h.deps.Messages.ListMessages(ctx, domain.SessionID(params.SessionID))
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]messageResult, 0, len(messages))
	for _, message := range messages {
		out = append(out, messageResult{ID: message.ID, RunID: message.RunID, Role: message.Role, Content: message.Content, CreatedAt: message.CreatedAt})
	}
	return map[string]any{"messages": out}, nil
}

// sessionContext reports the real context pressure of a session (feed
// bytes/tokens vs model window and byte budget, compaction trigger state).
func (h *controlHandler) sessionContext(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if h.deps.Service == nil {
		return nil, &Error{Code: MethodNotFound, Message: "runtime service is not configured"}
	}
	status, err := h.deps.Service.ContextStatus(ctx, domain.SessionID(params.SessionID))
	if err != nil {
		return nil, internalError(err)
	}
	return status, nil
}

// compactContext runs one durable session-level compaction immediately and
// reports before/after tokens. A busy session (run in flight) is a 409: the
// run already compresses in-run.
func (h *controlHandler) compactContext(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if h.deps.Service == nil {
		return nil, &Error{Code: MethodNotFound, Message: "runtime service is not configured"}
	}
	result, err := h.deps.Service.CompactSession(ctx, domain.SessionID(params.SessionID))
	if errors.Is(err, runtime.ErrCompactionBusy) {
		return nil, &Error{Code: CodeConflict, Message: err.Error()}
	}
	if errors.Is(err, runtime.ErrCompactionNotNeeded) || errors.Is(err, runtime.ErrCompactionNothingToDo) {
		return result, nil
	}
	if err != nil {
		return nil, internalError(err)
	}
	return result, nil
}

func (h *controlHandler) listSkills(ctx context.Context) (any, *Error) {
	if h.deps.Skills == nil {
		return nil, &Error{Code: MethodNotFound, Message: "skills backend is not configured"}
	}
	items, err := h.deps.Skills.ListSkills(ctx, "")
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]skillSummaryResult, 0, len(items))
	for _, item := range items {
		out = append(out, toSkillSummaryResult(item))
	}
	return map[string]any{"skills": out}, nil
}

func (h *controlHandler) getSkill(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Skills == nil {
		return nil, &Error{Code: MethodNotFound, Message: "skills backend is not configured"}
	}
	var params skillGetParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, &Error{Code: InvalidParams, Message: "name is required"}
	}
	view, err := h.deps.Skills.ViewSkill(ctx, "", params.Name, params.Path)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, &Error{Code: CodeNotFound, Message: err.Error()}
		}
		return nil, internalError(err)
	}
	return toSkillViewResult(view), nil
}

func toSkillSummaryResult(item tools.SkillSummary) skillSummaryResult {
	warnings := item.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return skillSummaryResult{
		Name: item.Name, Description: item.Description, Context: item.Context,
		Agent: item.Agent, Model: item.Model, Enabled: item.Enabled, Hash: item.Hash, Warnings: warnings,
	}
}

func toSkillViewResult(view tools.SkillView) skillViewResult {
	files := view.SupportingFiles
	if files == nil {
		files = []string{}
	}
	return skillViewResult{
		skillSummaryResult: toSkillSummaryResult(view.SkillSummary),
		Content:            view.Content,
		RelativePath:       view.RelativePath,
		SupportingFiles:    files,
	}
}

// setSkillEnabled flips the frontmatter enabled flag of one installed Skill
// under compare-and-swap on the current content hash. A stale hash is a 409:
// the document changed under the caller, so it must re-read and re-decide.
func (h *controlHandler) setSkillEnabled(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Skills == nil {
		return nil, &Error{Code: MethodNotFound, Message: "skills backend is not configured"}
	}
	var params skillSetEnabledParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, &Error{Code: InvalidParams, Message: "name is required"}
	}
	if strings.TrimSpace(params.BaseHash) == "" {
		return nil, &Error{Code: InvalidParams, Message: "base_hash is required"}
	}
	summary, err := h.deps.Skills.SetSkillEnabled(ctx, params.Name, params.Enabled, params.BaseHash)
	if err != nil {
		return nil, skillToggleError(err)
	}
	return toSkillSummaryResult(summary), nil
}

func skillToggleError(err error) *Error {
	if strings.Contains(err.Error(), "changed since it was read") {
		return &Error{Code: CodeConflict, Message: err.Error()}
	}
	if strings.Contains(err.Error(), "not found") {
		return &Error{Code: CodeNotFound, Message: err.Error()}
	}
	return internalError(err)
}

// listSkillRevisions surfaces staged HITL Skill mutations (skill_manage
// proposals) that are still pending human review. Read-only: the decision
// itself stays in the run review flow.
func (h *controlHandler) listSkillRevisions(ctx context.Context) (any, *Error) {
	if h.deps.SkillRevisions == nil {
		return nil, &Error{Code: MethodNotFound, Message: "skill revision store is not configured"}
	}
	revisions, err := h.deps.SkillRevisions.ListPendingSkillRevisions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]skillRevisionResult, 0, len(revisions))
	for _, revision := range revisions {
		var warnings []string
		_ = json.Unmarshal(revision.WarningsJSON, &warnings)
		if warnings == nil {
			warnings = []string{}
		}
		out = append(out, skillRevisionResult{
			ID: revision.ID, RunID: string(revision.RunID), SkillName: revision.SkillName,
			Action: revision.Action, TargetPath: revision.TargetPath, Preview: revision.Preview,
			Warnings: warnings, Status: string(revision.Status), CreatedAt: revision.CreatedAt,
		})
	}
	return map[string]any{"revisions": out}, nil
}

func (h *controlHandler) searchMarketplace(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Marketplace == nil {
		return nil, &Error{Code: MethodNotFound, Message: "skills marketplace is not configured"}
	}
	var params marketplaceSearchParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if len([]rune(strings.TrimSpace(params.Q))) < 2 {
		return nil, &Error{Code: InvalidParams, Message: "q must be at least 2 characters"}
	}
	skills, err := h.deps.Marketplace.SearchMarketplace(ctx, params.Q, params.Limit)
	if err != nil {
		return nil, marketplaceError(err)
	}
	return map[string]any{"skills": skills}, nil
}

func (h *controlHandler) featuredMarketplace(ctx context.Context) (any, *Error) {
	if h.deps.Marketplace == nil {
		return nil, &Error{Code: MethodNotFound, Message: "skills marketplace is not configured"}
	}
	featured, err := h.deps.Marketplace.FeaturedMarketplace(ctx)
	if err != nil {
		return nil, marketplaceError(err)
	}
	return featured, nil
}

func (h *controlHandler) installMarketplace(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Marketplace == nil {
		return nil, &Error{Code: MethodNotFound, Message: "skills marketplace is not configured"}
	}
	var params marketplaceInstallParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, &Error{Code: InvalidParams, Message: "id is required"}
	}
	result, err := h.deps.Marketplace.InstallMarketplace(ctx, params.ID)
	if err != nil {
		return nil, marketplaceError(err)
	}
	return result, nil
}

func marketplaceError(err error) *Error {
	var upstream *runtime.MarketplaceUpstreamError
	if errors.As(err, &upstream) {
		return &Error{Code: CodeBadGateway, Message: err.Error()}
	}
	if strings.Contains(err.Error(), "already exists") {
		return &Error{Code: CodeConflict, Message: err.Error()}
	}
	if strings.Contains(err.Error(), "not found") {
		return &Error{Code: CodeNotFound, Message: err.Error()}
	}
	if strings.Contains(err.Error(), "must be at least 2 characters") || strings.Contains(err.Error(), "must look like owner/repo/slug") {
		return &Error{Code: InvalidParams, Message: err.Error()}
	}
	return internalError(err)
}

func (h *controlHandler) listTodos(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Todos == nil {
		return nil, &Error{Code: MethodNotFound, Message: "todo store is not configured"}
	}
	params, rpcErr := parseSessionParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	sessionID := domain.SessionID(params.SessionID)
	if _, err := h.deps.Sessions.GetSession(ctx, sessionID); errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "session not found"}
	} else if err != nil {
		return nil, internalError(err)
	}
	todos, err := h.deps.Todos.ListTodos(ctx, sessionID)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]todoResult, 0, len(todos))
	for _, todo := range todos {
		blocks := todo.Blocks
		if blocks == nil {
			blocks = []string{}
		}
		blockedBy := todo.BlockedBy
		if blockedBy == nil {
			blockedBy = []string{}
		}
		out = append(out, todoResult{
			ID:          todo.ID,
			SessionID:   todo.SessionID,
			Subject:     todo.Subject,
			Description: todo.Description,
			Status:      todo.Status,
			Blocks:      blocks,
			BlockedBy:   blockedBy,
			ActiveForm:  todo.ActiveForm,
			Owner:       todo.Owner,
			Position:    todo.Position,
			CreatedAt:   todo.CreatedAt,
			UpdatedAt:   todo.UpdatedAt,
		})
	}
	return map[string]any{"todos": out}, nil
}

func (h *controlHandler) listBackground(ctx context.Context) (any, *Error) {
	runs, err := h.deps.Runs.ListActiveRuns(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]backgroundResult, 0, len(runs))
	for _, run := range runs {
		workspaceID := ""
		if workspace, workspaceErr := h.deps.Service.Workspace(ctx, run.ID); workspaceErr == nil {
			workspaceID = workspace.ID
		}
		out = append(out, backgroundResult{ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt, WorkspaceID: workspaceID})
	}
	return map[string]any{"runs": out}, nil
}

func (h *controlHandler) attachBackground(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	run, err := h.deps.Runs.GetRun(ctx, domain.RunID(params.RunID))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	workspace, err := h.deps.Service.Workspace(ctx, run.ID)
	if err != nil {
		return nil, internalError(err)
	}
	return backgroundResult{ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt, WorkspaceID: workspace.ID}, nil
}

func (h *controlHandler) preflight(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseTurnParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	result, err := h.deps.Service.Preflight(ctx, domain.SessionID(params.SessionID), params.Text, runtime.RunOptions{
		Mode: domain.RunMode(params.Mode), Profile: domain.PolicyProfile(params.PolicyProfile),
	})
	if err != nil {
		return nil, runtimeError(err)
	}
	return toPreflightResult(result), nil
}

func (h *controlHandler) startTurn(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseTurnParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if params.Text == "" {
		return nil, &Error{Code: InvalidParams, Message: "text must not be empty"}
	}
	runID, err := h.deps.Service.RunWithOptions(ctx, domain.SessionID(params.SessionID), params.Text, runtime.RunOptions{
		Mode: domain.RunMode(params.Mode), Profile: domain.PolicyProfile(params.PolicyProfile),
	})
	if err != nil {
		return nil, runtimeError(err)
	}
	return map[string]any{"run_id": runID, "status": domain.RunAccepted}, nil
}

func (h *controlHandler) cancelRun(request Request) (any, *Error) {
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if !h.deps.Service.Cancel(domain.RunID(params.RunID)) {
		return nil, &Error{Code: CodeNotFound, Message: "run is not active in this process"}
	}
	return map[string]any{"run_id": params.RunID, "status": "cancelling"}, nil
}

func (h *controlHandler) getRun(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseRunParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	run, err := h.deps.Runs.GetRun(ctx, domain.RunID(params.RunID))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "run not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return toRunResult(run), nil
}

func (h *controlHandler) runLog(ctx context.Context, request Request) (any, *Error) {
	params, rpcErr := parseSubscribeParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	entries, err := h.replayEvents(ctx, domain.RunID(params.RunID), domain.EventSeq(params.AfterSeq))
	if err != nil {
		return nil, internalError(err)
	}
	return map[string]any{"events": entries}, nil
}

func (h *controlHandler) listApprovals(ctx context.Context) (any, *Error) {
	approvals, err := h.deps.Approvals.ListPendingApprovals(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]approvalResult, 0, len(approvals))
	for _, approval := range approvals {
		out = append(out, approvalResult{ID: approval.ID, RunID: approval.RunID, ToolCallID: approval.ToolCallID, Decision: approval.Decision, ExpiresAt: approval.ExpiresAt})
	}
	return map[string]any{"approvals": out}, nil
}

func (h *controlHandler) respondApproval(ctx context.Context, request Request) (any, *Error) {
	var params approvalParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ApprovalID == "" || params.Decision == "" {
		return nil, &Error{Code: InvalidParams, Message: "approval_id and decision are required"}
	}
	if err := h.deps.Service.DecideApprovalWithReason(ctx, params.ApprovalID, params.Decision, params.Reason); err != nil {
		return nil, runtimeError(err)
	}
	return map[string]any{"approval_id": params.ApprovalID, "decision": params.Decision}, nil
}

func (h *controlHandler) listQuestions(ctx context.Context) (any, *Error) {
	questions, err := h.deps.Questions.ListPendingQuestions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]questionResult, 0, len(questions))
	for _, question := range questions {
		out = append(out, questionResult{ID: question.ID, RunID: question.RunID, ToolCallID: question.ToolCallID, Prompt: question.Prompt, Status: question.Status, ExpiresAt: question.ExpiresAt})
	}
	return map[string]any{"questions": out}, nil
}

func (h *controlHandler) respondQuestion(ctx context.Context, request Request) (any, *Error) {
	var params questionParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.QuestionID == "" || params.Answer == "" {
		return nil, &Error{Code: InvalidParams, Message: "question_id and answer are required"}
	}
	if err := h.deps.Service.AnswerQuestion(ctx, params.QuestionID, params.Answer); err != nil {
		return nil, runtimeError(err)
	}
	return map[string]any{"question_id": params.QuestionID, "answer": params.Answer}, nil
}

func (h *controlHandler) listReviews(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Reviews == nil {
		return nil, &Error{Code: MethodNotFound, Message: "review store is not configured"}
	}
	var params reviewListParams
	if request.Params != nil {
		if err := decodeParams(request, &params); err != nil {
			return nil, err
		}
	}
	items, err := h.deps.Reviews.ListReviews(ctx, storage.ReviewFilter{
		Kind: params.Kind, Status: params.Status, SessionID: params.SessionID, Limit: params.Limit,
	})
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]reviewResult, 0, len(items))
	for _, item := range items {
		out = append(out, toReviewResult(item))
	}
	return map[string]any{"reviews": out}, nil
}

func (h *controlHandler) getReview(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Reviews == nil {
		return nil, &Error{Code: MethodNotFound, Message: "review store is not configured"}
	}
	var params struct {
		ReviewID string `json:"review_id"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ReviewID == "" {
		return nil, &Error{Code: InvalidParams, Message: "review_id is required"}
	}
	item, err := h.deps.Reviews.GetReview(ctx, params.ReviewID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "review not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return toReviewResult(item), nil
}

func (h *controlHandler) respondReview(ctx context.Context, request Request) (any, *Error) {
	var params reviewRespondParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ReviewID == "" || params.Action == "" {
		return nil, &Error{Code: InvalidParams, Message: "review_id and action are required"}
	}
	if h.deps.Reviews == nil {
		return nil, &Error{Code: MethodNotFound, Message: "review store is not configured"}
	}
	item, err := h.deps.Reviews.GetReview(ctx, params.ReviewID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "review not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	switch item.Kind {
	case domain.ReviewKindApproval:
		if params.Action != "approve" && params.Action != "deny" {
			return nil, &Error{Code: InvalidParams, Message: "approval action must be approve or deny"}
		}
		decision := domain.ApprovalDenied
		if params.Action == "approve" {
			decision = domain.ApprovalApproved
		}
		if err := h.deps.Service.DecideApprovalWithReason(ctx, item.ID, decision, params.Reason); err != nil {
			return nil, runtimeError(err)
		}
		return map[string]any{"review_id": item.ID, "status": decision}, nil
	case domain.ReviewKindQuestion:
		switch params.Action {
		case "answer":
			if strings.TrimSpace(params.Answer) == "" {
				return nil, &Error{Code: InvalidParams, Message: "answer is required"}
			}
			if err := h.deps.Service.AnswerQuestion(ctx, item.ID, params.Answer); err != nil {
				return nil, runtimeError(err)
			}
			return map[string]any{"review_id": item.ID, "status": domain.ReviewAnswered}, nil
		case "cancel":
			if err := h.deps.Service.CancelQuestion(ctx, item.ID, params.Reason); err != nil {
				return nil, runtimeError(err)
			}
			return map[string]any{"review_id": item.ID, "status": domain.ReviewCancelled}, nil
		default:
			return nil, &Error{Code: InvalidParams, Message: "question action must be answer or cancel"}
		}
	default:
		return nil, &Error{Code: InvalidParams, Message: "unsupported review kind"}
	}
}

func toReviewResult(item domain.ReviewItem) reviewResult {
	return reviewResult{
		ID: item.ID, Kind: item.Kind, Status: item.Status, SessionID: item.SessionID, SessionTitle: item.SessionTitle,
		RunID: item.RunID, ToolCallID: item.ToolCallID, ToolName: item.ToolName, Source: item.Source, Actor: item.Actor,
		CreatedAt: item.CreatedAt, ExpiresAt: item.ExpiresAt, DecidedAt: item.DecidedAt, Action: item.Action, Target: item.Target,
		PreconditionHash: item.PreconditionHash, Preview: item.Preview, RiskFindings: item.RiskFindings, Arguments: item.Arguments,
		Prompt: item.Prompt, DecisionReason: item.DecisionReason, StaleReason: item.StaleReason, Error: item.Error,
		Effect: item.Effect, Reversibility: item.Reversibility, Scope: item.Scope, Trust: item.Trust,
	}
}

func (h *controlHandler) subscribe(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	params, rpcErr := parseSubscribeParams(request)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if peer == nil {
		return nil, &Error{Code: InternalError, Message: "subscription requires a connected peer"}
	}
	streamCtx, cancel := context.WithCancel(ctx)
	subscriptionID := newControlID("sub_")
	h.mu.Lock()
	h.subscriptions[subscriptionID] = cancel
	h.mu.Unlock()
	stream := func() {
		defer func() {
			h.mu.Lock()
			delete(h.subscriptions, subscriptionID)
			h.mu.Unlock()
			cancel()
		}()
		h.streamRun(streamCtx, peer, subscriptionID, domain.RunID(params.RunID), domain.EventSeq(params.AfterSeq))
	}
	peer.AfterResponse(request.ID, stream)
	return map[string]any{"subscription_id": subscriptionID, "run_id": params.RunID, "after_seq": params.AfterSeq}, nil
}

func (h *controlHandler) unsubscribe(request Request) (any, *Error) {
	var params unsubscribeParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	h.mu.Lock()
	cancel := h.subscriptions[params.SubscriptionID]
	delete(h.subscriptions, params.SubscriptionID)
	h.mu.Unlock()
	if cancel == nil {
		return nil, &Error{Code: CodeNotFound, Message: "subscription not found"}
	}
	cancel()
	return map[string]any{"unsubscribed": true}, nil
}

func (h *controlHandler) streamRun(ctx context.Context, peer *Peer, subscriptionID string, runID domain.RunID, after domain.EventSeq) {
	ch, cancel := h.deps.Bus.Subscribe(runID)
	defer cancel()
	last := after
	send := func(event domain.RunEvent) bool {
		if event.Seq <= last {
			return true
		}
		if err := peer.Notify("run/event", map[string]any{
			"subscription_id": subscriptionID,
			"event":           toEventResult(event),
		}); err != nil {
			return false
		}
		last = event.Seq
		return true
	}
	entries, err := h.replayEvents(ctx, runID, last)
	if err != nil {
		_ = peer.Notify("run/stream_error", map[string]any{"subscription_id": subscriptionID, "message": "event replay failed"})
		return
	}
	for _, entry := range entries {
		if !send(domain.RunEvent{RunID: entry.RunID, Seq: entry.Seq, Type: entry.Type, CreatedAt: entry.CreatedAt, PayloadVersion: entry.PayloadVersion, Payload: entry.Payload}) {
			return
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if ok {
				if !send(event) {
					return
				}
				continue
			}
			// Bus closes before sending terminal events. Replay the tail
			// so the JSON-RPC client still receives the terminal record.
			tail, replayErr := h.replayEvents(ctx, runID, last)
			if replayErr != nil {
				return
			}
			for _, entry := range tail {
				if !send(domain.RunEvent{RunID: entry.RunID, Seq: entry.Seq, Type: entry.Type, CreatedAt: entry.CreatedAt, PayloadVersion: entry.PayloadVersion, Payload: entry.Payload}) {
					return
				}
			}
			return
		}
	}
}

func (h *controlHandler) replayEvents(ctx context.Context, runID domain.RunID, after domain.EventSeq) ([]eventResult, error) {
	it, err := h.deps.Journal.Replay(ctx, runID, after)
	if err != nil {
		return nil, err
	}
	defer func() { _ = it.Close() }()
	var out []eventResult
	for it.Next() {
		out = append(out, toEventResult(it.Value().Event))
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeParams(request Request, target any) *Error {
	if len(request.Params) == 0 || string(request.Params) == "null" {
		return nil
	}
	if err := json.Unmarshal(request.Params, target); err != nil {
		return &Error{Code: InvalidParams, Message: "params must be a valid JSON object"}
	}
	return nil
}

func parseSessionParams(request Request) (sessionParams, *Error) {
	var params sessionParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.SessionID == "" {
		return params, &Error{Code: InvalidParams, Message: "session_id is required"}
	}
	return params, nil
}

func parseTurnParams(request Request) (turnParams, *Error) {
	var params turnParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.SessionID == "" {
		return params, &Error{Code: InvalidParams, Message: "session_id is required"}
	}
	return params, nil
}

func parseRunParams(request Request) (runParams, *Error) {
	var params runParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.RunID == "" {
		return params, &Error{Code: InvalidParams, Message: "run_id is required"}
	}
	return params, nil
}

func parseSubscribeParams(request Request) (subscribeParams, *Error) {
	var params subscribeParams
	if err := decodeParams(request, &params); err != nil {
		return params, err
	}
	if params.RunID == "" || params.AfterSeq < 0 {
		return params, &Error{Code: InvalidParams, Message: "run_id is required and after_seq must not be negative"}
	}
	return params, nil
}

func toRunResult(run domain.Run) runResult {
	return runResult{ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt}
}

func toPreflightResult(result runtime.PreflightResult) preflightResult {
	decisions := make([]policyDecisionResult, 0, len(result.ToolDecisions))
	for _, decision := range result.ToolDecisions {
		decisions = append(decisions, policyDecisionResult{ToolName: decision.ToolName, Decision: decision.Decision, Reason: decision.Reason})
	}
	return preflightResult{
		Status: result.Status, Mode: result.Mode, PolicyProfile: result.PolicyProfile, PolicyHash: result.PolicyHash,
		SelectedTools: result.SelectedTools, ToolDecisions: decisions, ContextBytes: result.ContextBytes,
		HookReady: result.HookReady, Warnings: result.Warnings, Blockers: result.Blockers, NextActions: result.NextActions,
	}
}

func toEventResult(event domain.RunEvent) eventResult {
	return eventResult{RunID: event.RunID, Seq: event.Seq, Type: event.Type, CreatedAt: event.CreatedAt, PayloadVersion: event.PayloadVersion, Payload: json.RawMessage(append([]byte(nil), event.Payload...))}
}

type generationParams struct {
	ID             string                `json:"id,omitempty"`
	ParentID       string                `json:"parent_id,omitempty"`
	ArtifactSHA256 string                `json:"artifact_sha256"`
	SourceRef      string                `json:"source_ref,omitempty"`
	Recipe         domain.AssemblyRecipe `json:"recipe"`
}

type evalParams struct {
	CandidateID string `json:"candidate_id"`
	BaselineID  string `json:"baseline_id,omitempty"`
	Suite       string `json:"suite"`
	Verdict     string `json:"verdict"`
	JournalRef  string `json:"journal_ref,omitempty"`
}

type promoteParams struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	EvalID string `json:"eval_id,omitempty"`
	Actor  string `json:"actor,omitempty"`
}

func (h *controlHandler) listGenerations(ctx context.Context) (any, *Error) {
	if h.deps.Studio == nil {
		return map[string]any{"generations": []generationResult{}}, nil
	}
	gens, err := h.deps.Studio.ListGenerations(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]generationResult, 0, len(gens))
	for _, g := range gens {
		out = append(out, toGenerationResult(g))
	}
	return map[string]any{"generations": out}, nil
}

func (h *controlHandler) getGeneration(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Studio == nil {
		return nil, &Error{Code: CodeNotFound, Message: "generation not found"}
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ID == "" {
		return nil, &Error{Code: InvalidParams, Message: "id is required"}
	}
	g, err := h.deps.Studio.GetGeneration(ctx, params.ID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, &Error{Code: CodeNotFound, Message: "generation not found"}
	}
	if err != nil {
		return nil, internalError(err)
	}
	return toGenerationResult(g), nil
}

func (h *controlHandler) createGeneration(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Studio == nil {
		return nil, internalError(errors.New("studio is not configured"))
	}
	var params generationParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	g, err := h.deps.Studio.CreateGeneration(ctx, domain.Generation{
		ID: params.ID, ParentID: params.ParentID, ArtifactSHA256: params.ArtifactSHA256,
		SourceRef: params.SourceRef, Recipe: params.Recipe,
	})
	if err != nil {
		return nil, studioError(err)
	}
	return toGenerationResult(g), nil
}

func (h *controlHandler) rejectGeneration(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Studio == nil {
		return nil, internalError(errors.New("studio is not configured"))
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.ID == "" {
		return nil, &Error{Code: InvalidParams, Message: "id is required"}
	}
	g, err := h.deps.Studio.Reject(ctx, params.ID)
	if err != nil {
		return nil, studioError(err)
	}
	return toGenerationResult(g), nil
}

func (h *controlHandler) listEvals(ctx context.Context) (any, *Error) {
	if h.deps.Studio == nil {
		return map[string]any{"evals": []evalResult{}}, nil
	}
	evals, err := h.deps.Studio.ListEvalRuns(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]evalResult, 0, len(evals))
	for _, e := range evals {
		out = append(out, toEvalResult(e))
	}
	return map[string]any{"evals": out}, nil
}

func (h *controlHandler) recordEval(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Studio == nil {
		return nil, internalError(errors.New("studio is not configured"))
	}
	var params evalParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	e, err := h.deps.Studio.RecordEval(ctx, domain.EvalRun{
		CandidateID: params.CandidateID, BaselineID: params.BaselineID,
		Suite: params.Suite, Verdict: domain.EvalVerdict(params.Verdict), JournalRef: params.JournalRef,
	})
	if err != nil {
		return nil, studioError(err)
	}
	return toEvalResult(e), nil
}

func (h *controlHandler) startEval(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Eval == nil {
		return nil, internalError(errors.New("eval is not configured"))
	}
	var params struct {
		CandidateID string `json:"candidate_id"`
		BaselineID  string `json:"baseline_id,omitempty"`
		Suite       string `json:"suite"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	e, err := h.deps.Eval.Start(ctx, params.CandidateID, params.BaselineID, params.Suite)
	if err != nil {
		return nil, studioError(err)
	}
	return toEvalResult(e), nil
}

func (h *controlHandler) listPromotions(ctx context.Context) (any, *Error) {
	if h.deps.Studio == nil {
		return map[string]any{"promotions": []promotionResult{}}, nil
	}
	promos, err := h.deps.Studio.ListPromotions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	out := make([]promotionResult, 0, len(promos))
	for _, p := range promos {
		out = append(out, toPromotionResult(p))
	}
	return map[string]any{"promotions": out}, nil
}

func (h *controlHandler) promote(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Studio == nil {
		return nil, internalError(errors.New("studio is not configured"))
	}
	var params promoteParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	p, err := h.deps.Studio.Promote(ctx, params.FromID, params.ToID, params.EvalID, params.Actor)
	if err != nil {
		return nil, studioError(err)
	}
	return toPromotionResult(p), nil
}

func (h *controlHandler) inspectSpecies(ctx context.Context) (any, *Error) {
	svc := h.deps.Studio
	if svc == nil {
		svc = studio.NewService(nil)
	}
	rep, err := svc.Inspect(ctx, h.deps.Live)
	if err != nil {
		return nil, internalError(err)
	}
	rep.ProtocolVersion = ProtocolVersion
	return rep, nil
}

// settingsResult is the operator-managed model provider selection and
// network tool preferences surfaced in the Settings UI. Secret values are
// never included: only the api_key_set flag is exposed.
type settingsResult struct {
	// Provider is the active bundle name (openai|anthropic), or empty
	// when the config default applies.
	Provider string `json:"provider"`
	// Frozen reports an ENV-session lock. The UI must treat model fields as
	// read-only for this process.
	Frozen bool `json:"frozen"`
	// DefaultModel is the selected model id, or empty for bundle default.
	DefaultModel string `json:"default_model"`
	// BaseURL is an optional OpenAI-compatible gateway, or empty.
	BaseURL string `json:"base_url"`
	// APIKeySet reports whether an api_key overlay is stored. The value
	// itself is never returned.
	APIKeySet bool `json:"api_key_set"`
	// ExecuteMaxTimeoutSeconds is the effective execute/commandline ceiling;
	// 0 means the config value applies. Editable in Settings → General.
	ExecuteMaxTimeoutSeconds int `json:"execute_max_timeout_seconds"`
	// ReadOnly reports whether updates are accepted. When the settings
	// document path is not configured, the UI shows values but cannot save.
	ReadOnly bool `json:"read_only"`
	// ConfigProvider is the production config default provider, for display.
	ConfigProvider string `json:"config_provider"`
	// ConfigModel is the production config default model, for display.
	ConfigModel string `json:"config_model"`
	// ConfigExecuteMaxTimeoutSeconds is the config execute ceiling the UI
	// falls back to when the override is cleared, for display.
	ConfigExecuteMaxTimeoutSeconds int `json:"config_execute_max_timeout_seconds"`
	// NetworkSearch is the network_search provider preference plus the
	// per-provider availability (env key presence, never values).
	NetworkSearch networkSearchSettingsResult `json:"network_search"`
	// Sandbox is the operator-managed default permission preset and network
	// policy, plus the live workspace root / command allowlist for display.
	Sandbox sandboxSettingsResult `json:"sandbox"`
	// Compaction is the effective context compression policy plus the config
	// defaults the UI falls back to when a field is cleared.
	Compaction compactionSettingsResult `json:"compaction"`
}

// compactionSettingsResult is the wire shape of the compaction overlay:
// effective values plus the config-file fallbacks for display.
type compactionSettingsResult struct {
	Enabled              bool `json:"enabled"`
	MaxTokens            int  `json:"max_tokens"`
	TriggerPercent       int  `json:"trigger_percent"`
	KeepRecent           int  `json:"keep_recent"`
	ConfigEnabled        bool `json:"config_enabled"`
	ConfigMaxTokens      int  `json:"config_max_tokens"`
	ConfigTriggerPercent int  `json:"config_trigger_percent"`
	ConfigKeepRecent     int  `json:"config_keep_recent"`
}

type sandboxSettingsResult struct {
	DefaultPreset          domain.PermissionPreset `json:"default_preset"`
	ConfigDefaultPreset    domain.PermissionPreset `json:"config_default_preset"`
	DenyPrivateIPs         bool                    `json:"deny_private_ips"`
	AllowedDomains         []string                `json:"allowed_domains"`
	WorkspaceRoot          string                  `json:"workspace_root"`
	ExecuteAllowedCommands []string                `json:"execute_allowed_commands"`
}

// networkSearchSettingsResult is the non-secret network_search section of
// settings/get. Providers is the availability roster in preference order.
type networkSearchSettingsResult struct {
	// Provider is the saved preference, or empty for automatic.
	Provider string `json:"provider"`
	// ConfigProvider is the production config network_search default.
	ConfigProvider string `json:"config_provider"`
	// Providers is the allowlisted roster with env-key presence status.
	Providers []runtime.NetworkSearchProviderInfo `json:"providers"`
}

func networkSearchView(saved, configDefault string) networkSearchSettingsResult {
	return networkSearchSettingsResult{
		Provider:       saved,
		ConfigProvider: configDefault,
		Providers:      runtime.NetworkSearchProviderAvailability(),
	}
}

// compactionView merges the saved overlay over the config default for the
// Settings UI. A nil overlay (or zero fields inside it) keeps the config
// value, mirroring the ExecuteMaxTimeoutSeconds pattern.
func (h *controlHandler) compactionView(saved *settings.CompactionSettings) compactionSettingsResult {
	base := h.deps.ConfigCompaction
	if base.TriggerPercent == 0 {
		base.TriggerPercent = 80
	}
	if base.KeepRecent == 0 {
		base.KeepRecent = 12
	}
	out := compactionSettingsResult{
		Enabled:              base.Enabled,
		MaxTokens:            base.MaxTokens,
		TriggerPercent:       base.TriggerPercent,
		KeepRecent:           base.KeepRecent,
		ConfigEnabled:        base.Enabled,
		ConfigMaxTokens:      base.MaxTokens,
		ConfigTriggerPercent: base.TriggerPercent,
		ConfigKeepRecent:     base.KeepRecent,
	}
	if saved != nil {
		if saved.Enabled != nil {
			out.Enabled = *saved.Enabled
		}
		if saved.MaxTokens != 0 {
			out.MaxTokens = saved.MaxTokens
		}
		if saved.TriggerPercent != 0 {
			out.TriggerPercent = saved.TriggerPercent
		}
		if saved.KeepRecent != 0 {
			out.KeepRecent = saved.KeepRecent
		}
	}
	return out
}

// settingsActiveKey reports whether the persisted document resolves a key
// overlay for the given selection (registry entry match wins, the legacy
// api_key overlay falls back). The value itself is never returned.
func settingsActiveKey(s settings.Settings, provider, baseURL string) bool {
	return settings.ActiveKey(s, provider, baseURL) != ""
}

func (h *controlHandler) getSettings(ctx context.Context) (any, *Error) {
	out := settingsResult{
		Provider:       "",
		DefaultModel:   "",
		BaseURL:        "",
		APIKeySet:      false,
		Frozen:         h.deps.Frozen,
		ReadOnly:       h.deps.SettingsPath == "" || h.deps.Frozen,
		ConfigProvider: "",
		ConfigModel:    "",
	}
	savedSearchProvider := ""
	var savedSandbox settings.SandboxSettings
	var savedCompaction *settings.CompactionSettings
	if h.deps.SettingsPath != "" {
		if s, err := settings.Load(h.deps.SettingsPath); err == nil {
			out.Provider = s.Provider
			out.DefaultModel = s.DefaultModel
			out.BaseURL = s.BaseURL
			out.APIKeySet = settingsActiveKey(s, s.Provider, s.BaseURL)
			savedSearchProvider = s.NetworkSearch.Provider
			out.ExecuteMaxTimeoutSeconds = s.ExecuteMaxTimeoutSeconds
			savedSandbox = s.Sandbox
			savedCompaction = s.Compaction
		}
	}
	// Reflect the production config defaults so the UI can show what a
	// cleared field falls back to. The runtime never exposes secret values.
	out.ConfigProvider = h.deps.ConfigProvider
	out.ConfigModel = h.deps.ConfigModel
	out.ConfigExecuteMaxTimeoutSeconds = h.deps.ConfigExecuteMaxTimeoutSeconds
	out.NetworkSearch = networkSearchView(savedSearchProvider, h.deps.ConfigNetworkSearchProvider)
	out.Sandbox = h.sandboxView(savedSandbox)
	out.Compaction = h.compactionView(savedCompaction)
	return out, nil
}

func (h *controlHandler) sandboxView(saved settings.SandboxSettings) sandboxSettingsResult {
	preset := h.defaultPreset()
	if saved.DefaultPreset.ValidSwitch() {
		preset = saved.DefaultPreset
	}
	denyPrivate := h.deps.ConfigSandboxDenyPrivateIPs
	if saved.Network.DenyPrivateIPs != nil {
		denyPrivate = *saved.Network.DenyPrivateIPs
	}
	domains := append([]string(nil), h.deps.ConfigSandboxAllowedDomains...)
	if saved.Network.AllowedDomains != nil {
		domains = append([]string(nil), saved.Network.AllowedDomains...)
	}
	if domains == nil {
		domains = []string{}
	}
	return sandboxSettingsResult{
		DefaultPreset:          preset,
		ConfigDefaultPreset:    h.defaultPreset(),
		DenyPrivateIPs:         denyPrivate,
		AllowedDomains:         domains,
		WorkspaceRoot:          h.deps.SandboxWorkspaceRoot,
		ExecuteAllowedCommands: append([]string(nil), h.deps.ExecuteAllowedCommands...),
	}
}

func (h *controlHandler) updateSettings(ctx context.Context, request Request) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return nil, &Error{Code: CodeConflict, Message: "settings are read-only in this deployment"}
	}
	if h.deps.Frozen {
		return nil, &Error{Code: CodeConflict, Message: "this process is locked to an environment-variable provider session and cannot change models"}
	}
	var params struct {
		Provider     string `json:"provider"`
		DefaultModel string `json:"default_model"`
		BaseURL      string `json:"base_url"`
		// ApiKey replaces the optional api_key overlay; empty clears it.
		// The value is never echoed back. The registry (providers list) is
		// preserved as-is: this method selects, it does not redefine.
		ApiKey string `json:"api_key"`
		// NetworkSearch carries the network_search provider preference;
		// empty clears it back to automatic.
		NetworkSearch struct {
			Provider string `json:"provider"`
		} `json:"network_search"`
		// ExecuteMaxTimeoutSeconds overrides the execute ceiling; 0 keeps
		// the config value.
		ExecuteMaxTimeoutSeconds int `json:"execute_max_timeout_seconds"`
		Sandbox                  *struct {
			DefaultPreset  string   `json:"default_preset"`
			DenyPrivateIPs *bool    `json:"deny_private_ips"`
			AllowedDomains []string `json:"allowed_domains"`
		} `json:"sandbox"`
		// Compaction is the context compression overlay; absent keeps the
		// previous value, explicit zeros inside a present block keep the
		// config value.
		Compaction *struct {
			Enabled        *bool `json:"enabled"`
			MaxTokens      int   `json:"max_tokens"`
			TriggerPercent int   `json:"trigger_percent"`
			KeepRecent     int   `json:"keep_recent"`
		} `json:"compaction"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	// Load-then-merge keeps the registry entries and any other section the
	// UI did not send; only the active selection fields are replaced.
	cur, err := settings.Load(h.deps.SettingsPath)
	if err != nil {
		return nil, internalError(err)
	}
	cur.Provider = params.Provider
	cur.DefaultModel = params.DefaultModel
	cur.BaseURL = params.BaseURL
	// Empty api_key on select leaves the registry / overlay keys alone.
	// A non-empty value still writes the legacy overlay for older clients.
	if params.ApiKey != "" {
		cur.ApiKey = params.ApiKey
	}
	cur.NetworkSearch = settings.NetworkSearchSettings{Provider: params.NetworkSearch.Provider}
	cur.ExecuteMaxTimeoutSeconds = params.ExecuteMaxTimeoutSeconds
	if params.Sandbox != nil {
		cur.Sandbox.DefaultPreset = domain.PermissionPreset(params.Sandbox.DefaultPreset)
		cur.Sandbox.Network.DenyPrivateIPs = params.Sandbox.DenyPrivateIPs
		if params.Sandbox.AllowedDomains != nil {
			cur.Sandbox.Network.AllowedDomains = append([]string(nil), params.Sandbox.AllowedDomains...)
		}
	}
	if params.Compaction != nil {
		if cur.Compaction == nil {
			cur.Compaction = &settings.CompactionSettings{}
		}
		if params.Compaction.Enabled != nil {
			cur.Compaction.Enabled = params.Compaction.Enabled
		}
		if params.Compaction.MaxTokens != 0 {
			cur.Compaction.MaxTokens = params.Compaction.MaxTokens
		}
		if params.Compaction.TriggerPercent != 0 {
			cur.Compaction.TriggerPercent = params.Compaction.TriggerPercent
		}
		if params.Compaction.KeepRecent != 0 {
			cur.Compaction.KeepRecent = params.Compaction.KeepRecent
		}
		if cur.Compaction.Enabled == nil && cur.Compaction.MaxTokens == 0 &&
			cur.Compaction.TriggerPercent == 0 && cur.Compaction.KeepRecent == 0 {
			// Everything cleared again: config default stands.
			cur.Compaction = nil
		}
	}
	saved, err := settings.Save(h.deps.SettingsPath, cur)
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	// Write-through env apply: the resolved active key (registry entry
	// wins, legacy overlay falls back) and base_url reach the running
	// process now; the startup overlay replays the same document.
	if h.deps.ApplySettingsEnv != nil {
		h.deps.ApplySettingsEnv(saved)
	}
	h.notifySettingsChanged()
	_ = ctx
	// Echo the config fallbacks too so the UI keeps its display values
	// (provider/model/execute ceiling) consistent right after a save,
	// instead of flashing empty/zero until the next settings/get.
	return settingsResult{
		Provider:                       saved.Provider,
		DefaultModel:                   saved.DefaultModel,
		BaseURL:                        saved.BaseURL,
		APIKeySet:                      settingsActiveKey(saved, saved.Provider, saved.BaseURL),
		ExecuteMaxTimeoutSeconds:       saved.ExecuteMaxTimeoutSeconds,
		Frozen:                         h.deps.Frozen,
		ReadOnly:                       h.deps.Frozen,
		ConfigProvider:                 h.deps.ConfigProvider,
		ConfigModel:                    h.deps.ConfigModel,
		ConfigExecuteMaxTimeoutSeconds: h.deps.ConfigExecuteMaxTimeoutSeconds,
		NetworkSearch:                  networkSearchView(saved.NetworkSearch.Provider, h.deps.ConfigNetworkSearchProvider),
		Sandbox:                        h.sandboxView(saved.Sandbox),
		Compaction:                     h.compactionView(saved.Compaction),
	}, nil
}

// providerEntryResult is one registry entry surfaced in the Settings UI.
// Secret values are never included: only the api_key_set flag is exposed.
type providerEntryResult struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	Bundle       string   `json:"bundle"`
	BaseURL      string   `json:"base_url"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	APIKeySet    bool     `json:"api_key_set"`
}

func toProviderEntryResult(e settings.ProviderEntry) providerEntryResult {
	// The wire contract is models: [] for an empty list — never null — so the
	// UI validator keeps an entry with no models yet visible (a custom
	// provider created before its first refresh must still render).
	models := e.Models
	if models == nil {
		models = []string{}
	}
	return providerEntryResult{
		ID:           e.ID,
		DisplayName:  e.DisplayName,
		Bundle:       e.Bundle,
		BaseURL:      e.BaseURL,
		DefaultModel: e.DefaultModel,
		Models:       models,
		APIKeySet:    e.ApiKey != "",
	}
}

// providersResult is the full registry view: entries (redacted), the active
// selection, and the config defaults the UI falls back to.
type providersResult struct {
	Entries        []providerEntryResult `json:"entries"`
	ActiveProvider string                `json:"active_provider"`
	ActiveModel    string                `json:"active_model"`
	ActiveBaseURL  string                `json:"active_base_url"`
	ReadOnly       bool                  `json:"read_only"`
	Frozen         bool                  `json:"frozen"`
	ConfigProvider string                `json:"config_provider"`
	ConfigModel    string                `json:"config_model"`
}

func (h *controlHandler) providersView(s settings.Settings) providersResult {
	entries := make([]providerEntryResult, 0, len(s.Providers))
	for _, e := range s.Providers {
		entries = append(entries, toProviderEntryResult(e))
	}
	return providersResult{
		Entries:        entries,
		ActiveProvider: s.Provider,
		ActiveModel:    s.DefaultModel,
		ActiveBaseURL:  s.BaseURL,
		ReadOnly:       h.deps.SettingsPath == "" || h.deps.Frozen,
		Frozen:         h.deps.Frozen,
		ConfigProvider: h.deps.ConfigProvider,
		ConfigModel:    h.deps.ConfigModel,
	}
}

func (h *controlHandler) loadSettingsOrError() (settings.Settings, *Error) {
	s, err := settings.Load(h.deps.SettingsPath)
	if err != nil {
		return settings.Settings{}, internalError(err)
	}
	return s, nil
}

// listProviders returns the registry plus the active selection and config
// defaults. Read-only deployments report read_only=true with an empty list.
func (h *controlHandler) listProviders(ctx context.Context) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return h.providersView(settings.Settings{}), nil
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}
	return h.providersView(s), nil
}

// upsertProvider creates or updates one registry entry by id. The api_key is
// write-only: it is persisted into the runtime document and applied to the
// environment when this entry is the active selection, but never returned.
func (h *controlHandler) upsertProvider(ctx context.Context, request Request) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return nil, &Error{Code: CodeConflict, Message: "settings are read-only in this deployment"}
	}
	if h.deps.Frozen {
		return nil, &Error{Code: CodeConflict, Message: "this process is locked to an environment-variable provider session and cannot change models"}
	}
	var params struct {
		ID           string   `json:"id"`
		DisplayName  string   `json:"display_name"`
		Bundle       string   `json:"bundle"`
		BaseURL      string   `json:"base_url"`
		DefaultModel string   `json:"default_model"`
		Models       []string `json:"models"`
		ApiKey       string   `json:"api_key"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}
	entry := settings.ProviderEntry{
		ID:           params.ID,
		DisplayName:  params.DisplayName,
		Bundle:       params.Bundle,
		BaseURL:      params.BaseURL,
		DefaultModel: params.DefaultModel,
		Models:       params.Models,
		ApiKey:       params.ApiKey,
	}
	if entry.ID == "" {
		entry.ID = "custom-" + providerIDNonce()
	}
	saved, err := settings.Save(h.deps.SettingsPath, s.UpsertProvider(entry))
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	// The persisted active selection resolves its key from this entry (by
	// bundle+base_url); apply it to the environment at write time.
	if h.deps.ApplySettingsEnv != nil {
		h.deps.ApplySettingsEnv(saved)
	}
	h.notifySettingsChanged()
	_ = ctx
	// Echo back the redacted saved entry so the UI can confirm the result.
	for _, e := range saved.Providers {
		if e.ID == entry.ID {
			return toProviderEntryResult(e), nil
		}
	}
	return nil, internalError(fmt.Errorf("provider upsert did not persist entry"))
}

// deleteProvider removes one registry entry by id. If it was the active
// selection its key overlay is cleared in the document; the environment
// keeps the current value until the next launch (no hot-swap), then falls
// back to the bundle's env_key.
func (h *controlHandler) deleteProvider(ctx context.Context, request Request) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return nil, &Error{Code: CodeConflict, Message: "settings are read-only in this deployment"}
	}
	if h.deps.Frozen {
		return nil, &Error{Code: CodeConflict, Message: "this process is locked to an environment-variable provider session and cannot change models"}
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}
	next := make([]settings.ProviderEntry, 0, len(s.Providers))
	found := false
	for _, e := range s.Providers {
		if e.ID == params.ID {
			found = true
			continue
		}
		next = append(next, e)
	}
	if !found {
		return nil, &Error{Code: CodeNotFound, Message: "provider entry not found"}
	}
	s.Providers = next
	saved, err := settings.Save(h.deps.SettingsPath, s)
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	if h.deps.ApplySettingsEnv != nil {
		h.deps.ApplySettingsEnv(saved)
	}
	h.notifySettingsChanged()
	_ = ctx
	return map[string]any{"deleted": true, "id": params.ID}, nil
}

// refreshProviderModels fetches the upstream OpenAI-compatible model list
// for one provider and persists it into the provider registry (models only;
// any configured api_key is kept untouched). A catalog vendor with no
// registry row is cloned into a new custom entry so there is a persist
// target; the clone's display name comes from the request (the frontend
// knows the catalog label). Anthropic-native providers are rejected: their
// API does not implement GET /models. The response is the redacted saved
// entry — the key never crosses the wire.
func (h *controlHandler) refreshProviderModels(ctx context.Context, request Request) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return nil, &Error{Code: CodeConflict, Message: "settings are read-only in this deployment"}
	}
	if h.deps.Frozen {
		return nil, &Error{Code: CodeConflict, Message: "this process is locked to an environment-variable provider session and cannot change models"}
	}
	var params struct {
		ID           string `json:"id"`
		Bundle       string `json:"bundle"`
		BaseURL      string `json:"base_url"`
		DisplayName  string `json:"display_name"`
		DefaultModel string `json:"default_model"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Resolve the target entry: by registry id, or by (bundle, base_url)
	// cloning a catalog vendor into the registry on first refresh.
	var entry settings.ProviderEntry
	switch {
	case params.ID != "":
		found := false
		for _, e := range s.Providers {
			if e.ID == params.ID {
				entry, found = e, true
				break
			}
		}
		if !found {
			return nil, &Error{Code: CodeNotFound, Message: "provider entry not found"}
		}
	default:
		if params.Bundle != settings.ProviderOpenAI {
			return nil, &Error{Code: InvalidParams, Message: "model refresh is only supported for OpenAI-compatible providers"}
		}
		baseURL := strings.TrimSpace(params.BaseURL)
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			return nil, &Error{Code: InvalidParams, Message: "model refresh requires an http(s) base_url"}
		}
		existing, ok := s.FindProvider(settings.ProviderOpenAI, baseURL)
		if ok {
			entry = existing
		} else {
			displayName := strings.TrimSpace(params.DisplayName)
			if displayName == "" {
				if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
					displayName = u.Host
				} else {
					displayName = baseURL
				}
			}
			entry = settings.ProviderEntry{
				ID:           "custom-" + providerIDNonce(),
				DisplayName:  displayName,
				Bundle:       settings.ProviderOpenAI,
				BaseURL:      baseURL,
				DefaultModel: strings.TrimSpace(params.DefaultModel),
			}
		}
	}
	if entry.Bundle != settings.ProviderOpenAI {
		return nil, &Error{Code: InvalidParams, Message: "model refresh is only supported for OpenAI-compatible providers"}
	}

	apiKey := settings.ActiveKey(s, entry.Bundle, entry.BaseURL)
	models, err := h.deps.ModelLists.List(ctx, entry.BaseURL, apiKey)
	if err != nil {
		// List already sanitizes the cause; never echo the key or URL.
		return nil, &Error{Code: InternalError, Message: "refresh provider models: " + err.Error()}
	}

	// Union policy: upstream ids first (gateway order), then locally added
	// ids upstream omits, so a manual "新增" entry survives a refresh.
	merged := append([]string(nil), models...)
	seen := make(map[string]bool, len(merged))
	for _, m := range merged {
		seen[m] = true
	}
	for _, m := range entry.Models {
		if !seen[m] {
			seen[m] = true
			merged = append(merged, m)
		}
	}
	entry.Models = merged

	saved, err := settings.Save(h.deps.SettingsPath, s.UpsertProvider(entry))
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	if h.deps.ApplySettingsEnv != nil {
		h.deps.ApplySettingsEnv(saved)
	}
	h.notifySettingsChanged()
	_ = ctx
	for _, e := range saved.Providers {
		if e.ID == entry.ID {
			return toProviderEntryResult(e), nil
		}
	}
	return nil, internalError(fmt.Errorf("provider refresh did not persist entry"))
}

// providerIDNonce supplies a short random suffix for auto-generated entry
// ids when the UI omits one. crypto/rand keeps the id unguessable, but the
// id is not a secret; uniqueness is what matters.
func providerIDNonce() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (h *controlHandler) notifySettingsChanged() {
	if h.deps.OnSettingsChanged != nil {
		h.deps.OnSettingsChanged()
	}
}

type mcpServerResult struct {
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	AuthEnv    string `json:"auth_env,omitempty"`
	AuthEnvSet bool   `json:"auth_env_set"`
	Enabled    bool   `json:"enabled"`
	ToolCount  int    `json:"tool_count"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type mcpListResult struct {
	Servers  []mcpServerResult `json:"servers"`
	ReadOnly bool              `json:"read_only"`
}

func toMCPServerResult(server settings.MCPServer) mcpServerResult {
	return mcpServerResult{
		Name:       server.Name,
		Endpoint:   server.Endpoint,
		AuthEnv:    server.AuthEnv,
		AuthEnvSet: server.AuthEnv != "" && os.Getenv(server.AuthEnv) != "",
		Enabled:    settings.MCPServerEnabled(server),
		Status:     "idle",
	}
}

func (h *controlHandler) mcpView(s settings.Settings) mcpListResult {
	servers := s.MCPServersOrEmpty()
	out := make([]mcpServerResult, 0, len(servers))
	for _, server := range servers {
		out = append(out, toMCPServerResult(server))
	}
	return mcpListResult{Servers: out, ReadOnly: h.deps.SettingsPath == ""}
}

func (h *controlHandler) listMCP(ctx context.Context) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return h.mcpView(settings.Settings{}), nil
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}
	_ = ctx
	return h.mcpView(s), nil
}

func (h *controlHandler) upsertMCP(ctx context.Context, request Request) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return nil, &Error{Code: CodeConflict, Message: "settings are read-only in this deployment"}
	}
	var params struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		AuthEnv  string `json:"auth_env"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}
	entry := settings.MCPServer{
		Name:     strings.TrimSpace(params.Name),
		Endpoint: strings.TrimSpace(params.Endpoint),
		AuthEnv:  strings.TrimSpace(params.AuthEnv),
		Enabled:  params.Enabled,
	}
	saved, err := settings.Save(h.deps.SettingsPath, s.UpsertMCPServer(entry))
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	h.notifySettingsChanged()
	_ = ctx
	for _, server := range saved.MCPServersOrEmpty() {
		if strings.EqualFold(server.Name, entry.Name) {
			return toMCPServerResult(server), nil
		}
	}
	return nil, internalError(fmt.Errorf("mcp upsert did not persist entry"))
}

func (h *controlHandler) deleteMCP(ctx context.Context, request Request) (any, *Error) {
	if h.deps.SettingsPath == "" {
		return nil, &Error{Code: CodeConflict, Message: "settings are read-only in this deployment"}
	}
	var params struct {
		Name string `json:"name"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	s, rpcErr := h.loadSettingsOrError()
	if rpcErr != nil {
		return nil, rpcErr
	}
	next, ok := s.DeleteMCPServer(params.Name)
	if !ok {
		return nil, &Error{Code: CodeNotFound, Message: "mcp server not found"}
	}
	if _, err := settings.Save(h.deps.SettingsPath, next); err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	h.notifySettingsChanged()
	_ = ctx
	return map[string]any{"deleted": true, "name": strings.TrimSpace(params.Name)}, nil
}

func (h *controlHandler) probeMCP(ctx context.Context, request Request) (any, *Error) {
	if h.deps.MCP == nil {
		return nil, &Error{Code: MethodNotFound, Message: "mcp catalog is not configured"}
	}
	var params struct {
		Name string `json:"name"`
	}
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, &Error{Code: InvalidParams, Message: "name is required"}
	}
	s := settings.Settings{}
	if h.deps.SettingsPath != "" {
		loaded, rpcErr := h.loadSettingsOrError()
		if rpcErr != nil {
			return nil, rpcErr
		}
		s = loaded
	}
	var found settings.MCPServer
	ok := false
	for _, server := range s.MCPServersOrEmpty() {
		if strings.EqualFold(server.Name, name) {
			found = server
			ok = true
			break
		}
	}
	if !ok {
		return nil, &Error{Code: CodeNotFound, Message: "mcp server not found"}
	}
	result := toMCPServerResult(found)
	if !result.Enabled {
		result.Status = "idle"
		return result, nil
	}
	listed, err := h.deps.MCP.ListTools(ctx, "", found.Name)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, nil
	}
	result.Status = "ok"
	result.ToolCount = len(listed.Tools)
	return result, nil
}

type generationResult struct {
	ID             string                 `json:"id"`
	ParentID       string                 `json:"parent_id,omitempty"`
	ArtifactSHA256 string                 `json:"artifact_sha256"`
	SourceRef      string                 `json:"source_ref,omitempty"`
	Recipe         domain.AssemblyRecipe  `json:"recipe"`
	Phase          domain.GenerationPhase `json:"phase"`
	CreatedAt      int64                  `json:"created_at"`
}

type evalResult struct {
	ID          string             `json:"id"`
	CandidateID string             `json:"candidate_id"`
	BaselineID  string             `json:"baseline_id,omitempty"`
	Suite       string             `json:"suite"`
	Verdict     domain.EvalVerdict `json:"verdict"`
	JournalRef  string             `json:"journal_ref,omitempty"`
	CreatedAt   int64              `json:"created_at"`
}

type promotionResult struct {
	ID        string                `json:"id"`
	FromID    string                `json:"from_id"`
	ToID      string                `json:"to_id"`
	EvalID    string                `json:"eval_id"`
	Actor     string                `json:"actor"`
	Phase     domain.PromotionPhase `json:"phase"`
	AppliesAt string                `json:"applies_at"`
	CreatedAt int64                 `json:"created_at"`
}

func toGenerationResult(g domain.Generation) generationResult {
	return generationResult{
		ID: g.ID, ParentID: g.ParentID, ArtifactSHA256: g.ArtifactSHA256,
		SourceRef: g.SourceRef, Recipe: g.Recipe, Phase: g.Phase, CreatedAt: g.CreatedAt,
	}
}

func toEvalResult(e domain.EvalRun) evalResult {
	return evalResult{
		ID: e.ID, CandidateID: e.CandidateID, BaselineID: e.BaselineID,
		Suite: e.Suite, Verdict: e.Verdict, JournalRef: e.JournalRef, CreatedAt: e.CreatedAt,
	}
}

func toPromotionResult(p domain.Promotion) promotionResult {
	return promotionResult{
		ID: p.ID, FromID: p.FromID, ToID: p.ToID, EvalID: p.EvalID,
		Actor: p.Actor, Phase: p.Phase, AppliesAt: p.AppliesAt, CreatedAt: p.CreatedAt,
	}
}

func studioError(err error) *Error {
	switch {
	case errors.Is(err, studio.ErrInvalid), errors.Is(err, studio.ErrNotHuman), errors.Is(err, eval.ErrInvalidSuite):
		return &Error{Code: InvalidParams, Message: err.Error()}
	case errors.Is(err, studio.ErrNotReady), errors.Is(err, studio.ErrAlreadyDecided), errors.Is(err, eval.ErrBlockedPath):
		return &Error{Code: CodeConflict, Message: err.Error()}
	case errors.Is(err, storage.ErrConflict):
		return &Error{Code: CodeConflict, Message: err.Error()}
	case errors.Is(err, storage.ErrNotFound):
		return &Error{Code: CodeNotFound, Message: err.Error()}
	default:
		return internalError(err)
	}
}

func runtimeError(err error) *Error {
	switch {
	case errors.Is(err, runtime.ErrInvalidRunMode), errors.Is(err, runtime.ErrInvalidPolicyProfile), errors.Is(err, runtime.ErrQuestionInvalidAnswer), errors.Is(err, runtime.ErrApprovalInvalidDecision), errors.Is(err, runtime.ErrApprovalInvalidReason):
		return &Error{Code: InvalidParams, Message: err.Error()}
	case errors.Is(err, runtime.ErrApprovalAlreadyDecided), errors.Is(err, runtime.ErrApprovalExpired), errors.Is(err, runtime.ErrQuestionAlreadyAnswered), errors.Is(err, runtime.ErrQuestionExpired), errors.Is(err, runtime.ErrRecoveryBusy):
		return &Error{Code: CodeConflict, Message: err.Error()}
	case errors.Is(err, runtime.ErrApprovalNotFound), errors.Is(err, runtime.ErrQuestionNotFound):
		return &Error{Code: CodeNotFound, Message: err.Error()}
	default:
		return internalError(err)
	}
}

func internalError(err error) *Error {
	if err == nil {
		return &Error{Code: InternalError, Message: "internal error"}
	}
	return &Error{Code: InternalError, Message: "internal error"}
}

func newControlID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return prefix + "fallback"
	}
	return prefix + hex.EncodeToString(bytes)
}

func nowMillis() int64 {
	return timeNow().UnixMilli()
}

var timeNow = func() time.Time { return time.Now() }
