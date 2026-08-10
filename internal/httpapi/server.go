package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

// Error codes of the {"error":{"code","message"}} envelope (FR-11).
const (
	codeNotFound       = "not_found"
	codeInvalidRequest = "invalid_request"
	codeConflict       = "conflict"
)

// defaultSessionTitle names sessions the UI did not title yet.
const defaultSessionTitle = "New session"

// Deps wires the handlers to the composed process. All fields are
// mandatory; New validates them.
type Deps struct {
	Sessions  storage.SessionStore
	Messages  storage.MessageStore
	Runs      storage.RunStore
	Journal   storage.Journal
	Approvals storage.ApprovalStore
	Questions storage.QuestionStore
	Bus       *events.Bus
	Service   *runtime.Service
}

// New builds the /api handler tree. The app layer mounts it alongside
// /healthz.
func New(d Deps) (http.Handler, error) {
	if d.Sessions == nil || d.Messages == nil || d.Runs == nil || d.Journal == nil || d.Approvals == nil || d.Questions == nil || d.Bus == nil || d.Service == nil {
		return nil, errors.New("httpapi: deps not wired")
	}
	s := &server{deps: d}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/sessions", s.createSession)
	mux.HandleFunc("GET /api/sessions", s.listSessions)
	mux.HandleFunc("GET /api/sessions/{id}", s.getSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", s.renameSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.deleteSession)
	mux.HandleFunc("POST /api/sessions/{id}/messages", s.postMessage)
	mux.HandleFunc("POST /api/sessions/{id}/preflight", s.preflight)
	mux.HandleFunc("GET /api/sessions/{id}/messages", s.listMessages)
	mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.cancelRun)
	mux.HandleFunc("GET /api/runs/{id}/events", s.streamRunEvents)
	mux.HandleFunc("GET /api/background/runs", s.listBackgroundRuns)
	mux.HandleFunc("POST /api/background/recover", s.recoverBackgroundRuns)
	mux.HandleFunc("POST /api/background/runs/{id}/attach", s.attachBackgroundRun)
	mux.HandleFunc("GET /api/background/runs/{id}/logs", s.backgroundRunLogs)
	mux.HandleFunc("GET /api/approvals", s.listApprovals)
	mux.HandleFunc("POST /api/approvals/{id}/decision", s.decideApproval)
	mux.HandleFunc("GET /api/questions", s.listQuestions)
	mux.HandleFunc("POST /api/questions/{id}/answer", s.answerQuestion)
	return mux, nil
}

type server struct {
	deps Deps
}

// DTO shapes. Everything is snake_case JSON; ids are prefixed by kind.

type sessionDTO struct {
	ID        domain.SessionID `json:"id"`
	Title     string           `json:"title"`
	CreatedAt int64            `json:"created_at"`
}

func toSessionDTO(s domain.Session) sessionDTO {
	return sessionDTO{ID: s.ID, Title: s.Title, CreatedAt: s.CreatedAt}
}

type messageDTO struct {
	ID        string       `json:"id"`
	RunID     domain.RunID `json:"run_id,omitempty"`
	Role      domain.Role  `json:"role"`
	Content   string       `json:"content"`
	CreatedAt int64        `json:"created_at"`
}

func toMessageDTO(m domain.Message) messageDTO {
	return messageDTO{ID: m.ID, RunID: m.RunID, Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt}
}

type runDTO struct {
	ID        domain.RunID     `json:"id"`
	SessionID domain.SessionID `json:"session_id"`
	Status    domain.RunStatus `json:"status"`
	CreatedAt int64            `json:"created_at"`
}

type backgroundRunDTO struct {
	ID          domain.RunID     `json:"id"`
	SessionID   domain.SessionID `json:"session_id"`
	Status      domain.RunStatus `json:"status"`
	CreatedAt   int64            `json:"created_at"`
	WorkspaceID string           `json:"workspace_id,omitempty"`
	EventsURL   string           `json:"events_url"`
	LogsURL     string           `json:"logs_url"`
}

type backgroundLogDTO struct {
	Seq       domain.EventSeq  `json:"seq"`
	Type      domain.EventType `json:"type"`
	CreatedAt int64            `json:"created_at"`
	Payload   json.RawMessage  `json:"payload"`
}

type createSessionRequest struct {
	Title string `json:"title"`
}

type renameSessionRequest struct {
	Title string `json:"title"`
}

type postMessageRequest struct {
	Text          string `json:"text"`
	Mode          string `json:"mode,omitempty"`
	PolicyProfile string `json:"policy_profile,omitempty"`
}

type postMessageResponse struct {
	RunID  domain.RunID     `json:"run_id"`
	Status domain.RunStatus `json:"status"`
}

type preflightRequest struct {
	Text          string `json:"text"`
	Mode          string `json:"mode,omitempty"`
	PolicyProfile string `json:"policy_profile,omitempty"`
}

type preflightResponse struct {
	Status        runtime.PreflightStatus `json:"status"`
	Mode          domain.RunMode          `json:"mode"`
	PolicyProfile domain.PolicyProfile    `json:"policy_profile"`
	PolicyHash    string                  `json:"policy_hash,omitempty"`
	SelectedTools []string                `json:"selected_tools"`
	ToolDecisions []policyPreviewDTO      `json:"tool_decisions"`
	ContextBytes  int                     `json:"context_bytes"`
	HookReady     bool                    `json:"hook_ready"`
	Warnings      []string                `json:"warnings"`
	Blockers      []string                `json:"blockers"`
	NextActions   []string                `json:"next_actions"`
}

type policyPreviewDTO struct {
	ToolName string                `json:"tool_name"`
	Decision domain.PolicyDecision `json:"decision"`
	Reason   string                `json:"reason"`
}

// approvalDTO exposes the decision-critical fields only; tool_name and
// args travel in the tool.approval_required event payload instead.
type approvalDTO struct {
	ID         string       `json:"id"`
	RunID      domain.RunID `json:"run_id"`
	ToolCallID string       `json:"tool_call_id"`
	ExpiresAt  int64        `json:"expires_at"`
}

type decideApprovalRequest struct {
	Decision string `json:"decision"`
}

type decideApprovalResponse struct {
	ApprovalID string       `json:"approval_id"`
	RunID      domain.RunID `json:"run_id"`
	Decision   string       `json:"decision"`
}

type questionDTO struct {
	ID         string       `json:"id"`
	RunID      domain.RunID `json:"run_id"`
	ToolCallID string       `json:"tool_call_id"`
	Prompt     string       `json:"prompt"`
	ExpiresAt  int64        `json:"expires_at"`
}

type answerQuestionRequest struct {
	Answer string `json:"answer"`
}

type answerQuestionResponse struct {
	QuestionID string       `json:"question_id"`
	RunID      domain.RunID `json:"run_id"`
	Answer     string       `json:"answer"`
}

// --- sessions ---

func (s *server) createSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "body must be a JSON object")
			return
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = defaultSessionTitle
	}
	sess := domain.Session{ID: domain.SessionID(newID("sess_")), Title: title, CreatedAt: time.Now().UnixMilli()}
	if err := s.deps.Sessions.CreateSession(r.Context(), sess); err != nil {
		writeInternal(w, "create session", err)
		return
	}
	writeJSON(w, http.StatusCreated, toSessionDTO(sess))
}

func (s *server) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.deps.Sessions.ListSessions(r.Context())
	if err != nil {
		writeInternal(w, "list sessions", err)
		return
	}
	out := make([]sessionDTO, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, toSessionDTO(sess))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *server) getSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.deps.Sessions.GetSession(r.Context(), domain.SessionID(r.PathValue("id")))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "session not found")
		return
	}
	if err != nil {
		writeInternal(w, "get session", err)
		return
	}
	writeJSON(w, http.StatusOK, toSessionDTO(sess))
}

func (s *server) renameSession(w http.ResponseWriter, r *http.Request) {
	var req renameSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "body must be a JSON object")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "title must not be empty")
		return
	}
	id := domain.SessionID(r.PathValue("id"))
	if err := s.deps.Sessions.RenameSession(r.Context(), id, title); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "session not found")
			return
		}
		writeInternal(w, "rename session", err)
		return
	}
	sess, err := s.deps.Sessions.GetSession(r.Context(), id)
	if err != nil {
		writeInternal(w, "get session after rename", err)
		return
	}
	writeJSON(w, http.StatusOK, toSessionDTO(sess))
}

// deleteSession cancels the session's in-flight runs first so their
// terminal events land in the journal, then cascades the delete.
func (s *server) deleteSession(w http.ResponseWriter, r *http.Request) {
	id := domain.SessionID(r.PathValue("id"))
	if _, err := s.deps.Sessions.GetSession(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "session not found")
			return
		}
		writeInternal(w, "get session", err)
		return
	}
	active, err := s.deps.Runs.ListActiveRuns(r.Context())
	if err != nil {
		writeInternal(w, "list active runs", err)
		return
	}
	for _, run := range active {
		if run.SessionID == id {
			s.deps.Service.Cancel(run.ID)
		}
	}
	if err := s.deps.Sessions.DeleteSession(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "session not found")
			return
		}
		writeInternal(w, "delete session", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- messages / run submission ---

func (s *server) postMessage(w http.ResponseWriter, r *http.Request) {
	id := domain.SessionID(r.PathValue("id"))
	if _, err := s.deps.Sessions.GetSession(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "session not found")
			return
		}
		writeInternal(w, "get session", err)
		return
	}
	var req postMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "body must be a JSON object")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "text must not be empty")
		return
	}
	runID, err := s.deps.Service.RunWithOptions(r.Context(), id, req.Text, runtime.RunOptions{
		Mode: domain.RunMode(req.Mode), Profile: domain.PolicyProfile(req.PolicyProfile),
	})
	if err != nil {
		if errors.Is(err, runtime.ErrInvalidRunMode) {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "mode must be normal or plan")
			return
		}
		if errors.Is(err, runtime.ErrInvalidPolicyProfile) {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "policy_profile is invalid")
			return
		}
		writeInternal(w, "start run", err)
		return
	}
	writeJSON(w, http.StatusAccepted, postMessageResponse{RunID: runID, Status: domain.RunAccepted})
}

// preflight performs the same context/tool policy checks as a run without
// persisting a message, run row, event, or provider/tool call.
func (s *server) preflight(w http.ResponseWriter, r *http.Request) {
	id := domain.SessionID(r.PathValue("id"))
	if _, err := s.deps.Sessions.GetSession(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "session not found")
			return
		}
		writeInternal(w, "get session", err)
		return
	}
	var req preflightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "body must be a JSON object")
		return
	}
	result, err := s.deps.Service.Preflight(r.Context(), id, req.Text, runtime.RunOptions{
		Mode: domain.RunMode(req.Mode), Profile: domain.PolicyProfile(req.PolicyProfile),
	})
	if errors.Is(err, runtime.ErrInvalidRunMode) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "mode must be normal or plan")
		return
	}
	if errors.Is(err, runtime.ErrInvalidPolicyProfile) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "policy_profile is invalid")
		return
	}
	if err != nil {
		writeInternal(w, "preflight", err)
		return
	}
	writeJSON(w, http.StatusOK, preflightResponse{
		Status: result.Status, Mode: result.Mode, PolicyProfile: result.PolicyProfile,
		PolicyHash: result.PolicyHash, SelectedTools: result.SelectedTools,
		ToolDecisions: mapPolicyPreviews(result.ToolDecisions), ContextBytes: result.ContextBytes,
		HookReady: result.HookReady, Warnings: result.Warnings, Blockers: result.Blockers,
		NextActions: result.NextActions,
	})
}

func mapPolicyPreviews(previews []runtime.PolicyPreview) []policyPreviewDTO {
	out := make([]policyPreviewDTO, 0, len(previews))
	for _, preview := range previews {
		out = append(out, policyPreviewDTO{ToolName: preview.ToolName, Decision: preview.Decision, Reason: preview.Reason})
	}
	return out
}

func (s *server) listMessages(w http.ResponseWriter, r *http.Request) {
	id := domain.SessionID(r.PathValue("id"))
	if _, err := s.deps.Sessions.GetSession(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "session not found")
			return
		}
		writeInternal(w, "get session", err)
		return
	}
	msgs, err := s.deps.Messages.ListMessages(r.Context(), id)
	if err != nil {
		writeInternal(w, "list messages", err)
		return
	}
	out := make([]messageDTO, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toMessageDTO(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": out})
}

// --- runs ---

func (s *server) getRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.deps.Runs.GetRun(r.Context(), domain.RunID(r.PathValue("id")))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "run not found")
		return
	}
	if err != nil {
		writeInternal(w, "get run", err)
		return
	}
	writeJSON(w, http.StatusOK, runDTO{ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt})
}

func (s *server) cancelRun(w http.ResponseWriter, r *http.Request) {
	runID := domain.RunID(r.PathValue("id"))
	run, err := s.deps.Runs.GetRun(r.Context(), runID)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "run not found")
		return
	}
	if err != nil {
		writeInternal(w, "get run", err)
		return
	}
	if run.Status.Terminal() {
		writeError(w, http.StatusConflict, codeConflict, fmt.Sprintf("run is already %s", run.Status))
		return
	}
	if !s.deps.Service.Cancel(runID) {
		// Row says non-terminal but this process holds no live cancel
		// handle (e.g. after a restart; recovery lands in E2).
		writeError(w, http.StatusConflict, codeConflict, "run is not active in this process")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run_id": runID, "status": "cancelling"})
}

// streamRunEvents serves the SSE replay-then-live stream. after_seq is the
// reconnect cursor (events with seq <= after_seq are skipped); default 0
// replays everything.
func (s *server) streamRunEvents(w http.ResponseWriter, r *http.Request) {
	runID := domain.RunID(r.PathValue("id"))
	if _, err := s.deps.Runs.GetRun(r.Context(), runID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "run not found")
			return
		}
		writeInternal(w, "get run", err)
		return
	}
	afterSeq := int64(0)
	if raw := r.URL.Query().Get("after_seq"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "after_seq must be a non-negative integer")
			return
		}
		afterSeq = v
	}
	if err := events.ServeSSE(r.Context(), w, s.deps.Journal, s.deps.Bus, runID, domain.EventSeq(afterSeq)); err != nil {
		// Mid-stream failures cannot become JSON errors: headers and
		// frames are already on the wire. The journal keeps the truth.
		slog.Warn("sse stream ended with error", "run", string(runID), "err", err)
	}
}

// listBackgroundRuns lists non-terminal runs that survive the submitting
// request. Completed history remains available through the session journal;
// this endpoint is the operational background-run inbox.
func (s *server) listBackgroundRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.deps.Runs.ListActiveRuns(r.Context())
	if err != nil {
		writeInternal(w, "list background runs", err)
		return
	}
	out := make([]backgroundRunDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, s.toBackgroundRunDTO(r.Context(), run))
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

// attachBackgroundRun is a read-only capability handshake. The client can
// immediately consume the replay-safe SSE URL and bounded JSON log URL after
// a refresh or process restart.
func (s *server) attachBackgroundRun(w http.ResponseWriter, r *http.Request) {
	runID := domain.RunID(r.PathValue("id"))
	run, err := s.deps.Runs.GetRun(r.Context(), runID)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "run not found")
		return
	}
	if err != nil {
		writeInternal(w, "get background run", err)
		return
	}
	workspace, err := s.deps.Service.Workspace(r.Context(), runID)
	if err != nil {
		writeInternal(w, "attach background workspace", err)
		return
	}
	writeJSON(w, http.StatusOK, s.toBackgroundRunDTOWithWorkspace(run, workspace.ID))
}

// backgroundRunLogs returns the durable event log for headless workers that
// cannot consume SSE. The cap keeps one response bounded; callers can use
// the SSE after_seq cursor for a complete replay.
func (s *server) backgroundRunLogs(w http.ResponseWriter, r *http.Request) {
	runID := domain.RunID(r.PathValue("id"))
	if _, err := s.deps.Runs.GetRun(r.Context(), runID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "run not found")
			return
		}
		writeInternal(w, "get background run", err)
		return
	}
	const maxLogEvents = 1024
	it, err := s.deps.Journal.Replay(r.Context(), runID, 0)
	if err != nil {
		writeInternal(w, "replay background run logs", err)
		return
	}
	defer func() { _ = it.Close() }()
	logs := make([]backgroundLogDTO, 0, 32)
	truncated := false
	for it.Next() {
		if len(logs) == maxLogEvents {
			truncated = true
			break
		}
		ev := it.Value().Event
		logs = append(logs, backgroundLogDTO{
			Seq: ev.Seq, Type: ev.Type, CreatedAt: ev.CreatedAt,
			Payload: append(json.RawMessage(nil), ev.Payload...),
		})
	}
	if err := it.Err(); err != nil {
		writeInternal(w, "replay background run logs", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run_id": runID, "events": logs, "truncated": truncated})
}

// recoverBackgroundRuns is the manual counterpart to startup recovery. It
// refuses to run while this process owns live work, preventing an operator
// refresh from reclassifying a healthy in-process run as an orphan.
func (s *server) recoverBackgroundRuns(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Service.RecoverBackground(r.Context()); err != nil {
		if errors.Is(err, runtime.ErrRecoveryBusy) {
			writeError(w, http.StatusConflict, codeConflict, "background runs are active in this process")
			return
		}
		writeInternal(w, "recover background runs", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "recovered"})
}

func (s *server) toBackgroundRunDTO(ctx context.Context, run domain.Run) backgroundRunDTO {
	workspaceID := ""
	if workspace, err := s.deps.Service.Workspace(ctx, run.ID); err == nil {
		workspaceID = workspace.ID
	}
	return s.toBackgroundRunDTOWithWorkspace(run, workspaceID)
}

func (s *server) toBackgroundRunDTOWithWorkspace(run domain.Run, workspaceID string) backgroundRunDTO {
	id := string(run.ID)
	return backgroundRunDTO{
		ID: run.ID, SessionID: run.SessionID, Status: run.Status, CreatedAt: run.CreatedAt,
		WorkspaceID: workspaceID,
		EventsURL:   "/api/runs/" + id + "/events",
		LogsURL:     "/api/background/runs/" + id + "/logs",
	}
}

// --- approvals ---

// listApprovals returns every pending approval, latest expiry first. The
// UI judges expiry itself off expires_at; the server re-checks on
// decision (D-009).
func (s *server) listApprovals(w http.ResponseWriter, r *http.Request) {
	rows, err := s.deps.Approvals.ListPendingApprovals(r.Context())
	if err != nil {
		writeInternal(w, "list approvals", err)
		return
	}
	out := make([]approvalDTO, 0, len(rows))
	for _, a := range rows {
		out = append(out, approvalDTO{ID: a.ID, RunID: a.RunID, ToolCallID: a.ToolCallID, ExpiresAt: a.ExpiresAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": out})
}

// decideApproval settles a pending approval and triggers the resume. All
// refusals are 4xx with distinct messages so the UI can tell an already
// decided row from an expired one (D-009 server-enforced).
func (s *server) decideApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req decideApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "body must be a JSON object")
		return
	}
	// Read the row first so the success response can carry run_id;
	// existence is re-checked inside the service call.
	approval, err := s.deps.Approvals.GetApproval(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "approval not found")
		return
	}
	if err != nil {
		writeInternal(w, "get approval", err)
		return
	}
	if err := s.deps.Service.DecideApproval(r.Context(), id, req.Decision); err != nil {
		switch {
		case errors.Is(err, runtime.ErrApprovalNotFound):
			writeError(w, http.StatusNotFound, codeNotFound, "approval not found")
		case errors.Is(err, runtime.ErrApprovalInvalidDecision):
			writeError(w, http.StatusConflict, codeConflict, "decision must be approved or denied")
		case errors.Is(err, runtime.ErrApprovalAlreadyDecided):
			writeError(w, http.StatusConflict, codeConflict, "approval already decided")
		case errors.Is(err, runtime.ErrApprovalExpired):
			writeError(w, http.StatusConflict, codeConflict, "approval expired")
		default:
			writeInternal(w, "decide approval", err)
		}
		return
	}
	writeJSON(w, http.StatusAccepted, decideApprovalResponse{ApprovalID: id, RunID: approval.RunID, Decision: req.Decision})
}

// listQuestions returns pending ask_user interactions. Questions have their
// own endpoint and DTO so the UI cannot mistake an answer for approval.
func (s *server) listQuestions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.deps.Questions.ListPendingQuestions(r.Context())
	if err != nil {
		writeInternal(w, "list questions", err)
		return
	}
	out := make([]questionDTO, 0, len(rows))
	for _, q := range rows {
		out = append(out, questionDTO{ID: q.ID, RunID: q.RunID, ToolCallID: q.ToolCallID, Prompt: q.Prompt, ExpiresAt: q.ExpiresAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"questions": out})
}

// answerQuestion validates and persists a user answer, then resumes the
// suspended run through the runtime question path.
func (s *server) answerQuestion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req answerQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "body must be a JSON object")
		return
	}
	question, err := s.deps.Questions.GetQuestion(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "question not found")
		return
	}
	if err != nil {
		writeInternal(w, "get question", err)
		return
	}
	if err := s.deps.Service.AnswerQuestion(r.Context(), id, req.Answer); err != nil {
		switch {
		case errors.Is(err, runtime.ErrQuestionNotFound):
			writeError(w, http.StatusNotFound, codeNotFound, "question not found")
		case errors.Is(err, runtime.ErrQuestionInvalidAnswer):
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "answer must not be empty")
		case errors.Is(err, runtime.ErrQuestionAlreadyAnswered):
			writeError(w, http.StatusConflict, codeConflict, "question already answered")
		case errors.Is(err, runtime.ErrQuestionExpired):
			writeError(w, http.StatusConflict, codeConflict, "question expired")
		default:
			writeInternal(w, "answer question", err)
		}
		return
	}
	writeJSON(w, http.StatusAccepted, answerQuestionResponse{QuestionID: id, RunID: question.RunID, Answer: strings.TrimSpace(req.Answer)})
}

// --- response helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write response", "err", err)
	}
}

// writeError renders the FR-11 error envelope. Codes never leak internals;
// the message stays user-visible.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeInternal(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "the request could not be completed")
}

// newID mints "<prefix>" + 8 random hex bytes, matching the runtime's
// run_ style.
func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("httpapi: crypto/rand unavailable: %v", err))
	}
	return prefix + hex.EncodeToString(b)
}
