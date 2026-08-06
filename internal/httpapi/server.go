package httpapi

import (
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
	Sessions storage.SessionStore
	Messages storage.MessageStore
	Runs     storage.RunStore
	Journal  storage.Journal
	Bus      *events.Bus
	Service  *runtime.Service
}

// New builds the /api handler tree. The app layer mounts it alongside
// /healthz.
func New(d Deps) (http.Handler, error) {
	if d.Sessions == nil || d.Messages == nil || d.Runs == nil || d.Journal == nil || d.Bus == nil || d.Service == nil {
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
	mux.HandleFunc("GET /api/sessions/{id}/messages", s.listMessages)
	mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.cancelRun)
	mux.HandleFunc("GET /api/runs/{id}/events", s.streamRunEvents)
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

type createSessionRequest struct {
	Title string `json:"title"`
}

type renameSessionRequest struct {
	Title string `json:"title"`
}

type postMessageRequest struct {
	Text string `json:"text"`
}

type postMessageResponse struct {
	RunID  domain.RunID     `json:"run_id"`
	Status domain.RunStatus `json:"status"`
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
	runID, err := s.deps.Service.Run(r.Context(), id, req.Text)
	if err != nil {
		writeInternal(w, "start run", err)
		return
	}
	writeJSON(w, http.StatusAccepted, postMessageResponse{RunID: runID, Status: domain.RunAccepted})
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
