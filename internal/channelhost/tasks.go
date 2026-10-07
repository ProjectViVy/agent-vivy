// Package channelhost — governed A2A task surface (design §7): the native
// Submit/Get/List/Cancel/Subscribe operations plus the safe service-info
// projection. Everything remote-visible is a journal projection with fixed
// resource bounds; the host owns no status, outputs, or execution of its
// own.
package channelhost

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/channel"
	plugin "agent-vivy/sdk/port/channel"
)

// taskIDMaxBytes is the §10 identity bound: every remote-visible id is at
// most 256 UTF-8 bytes without control characters.
const taskIDMaxBytes = 256

// taskInput bounds (§10): 1..16 text parts, each at most 64 KiB.
const (
	taskMaxParts      = 16
	taskPartMaxBytes  = 64 << 10
	taskHistoryCap    = 64
	taskListScanLimit = 4096 // committed events scanned across one List
	taskListByteLimit = 8 << 20
	taskPageScanLimit = 16384 // committed events scanned for one task
)

// TaskPrincipal is the private task authorization binding carried in the
// request context by the wire adapter (§6.4): the configured Module,
// Provider and Instance triple plus the caller principal and the
// authorization revision the binding was issued under.
type TaskPrincipal struct {
	ModuleID              string
	ProviderID            string
	InstanceID            string
	PrincipalID           string
	AuthorizationRevision int64
}

// InstanceKey is the storage identity of the configured instance triple,
// matching the channel_task_receipts.instance_key encoding (§6.4).
func (p TaskPrincipal) InstanceKey() string {
	raw, _ := json.Marshal([3]string{p.ModuleID, p.ProviderID, p.InstanceID})
	return string(raw)
}

func (p TaskPrincipal) scope() domain.ChannelTaskScope {
	return domain.ChannelTaskScope{
		InstanceKey: p.InstanceKey(),
		PrincipalID: p.PrincipalID,
	}
}

type taskPrincipalKey struct{}

// WithTaskPrincipal binds a request to its private task principal. The wire
// adapter is the only caller; the binding is validated against current
// configured authorization on every operation.
func WithTaskPrincipal(ctx context.Context, p TaskPrincipal) context.Context {
	return context.WithValue(ctx, taskPrincipalKey{}, p)
}

func taskPrincipalFromContext(ctx context.Context) (TaskPrincipal, bool) {
	p, ok := ctx.Value(taskPrincipalKey{}).(TaskPrincipal)
	return p, ok
}

// TaskDeps is the dependency pack for the task surface. The environment
// constructs the task host only when every field is set — a partial pack
// means the assertion on channel.TaskHost fails and the capability stays
// absent (§7.0).
type TaskDeps struct {
	Store    storage.ChannelTaskStore
	Journal  storage.JournalPageReader
	Messages storage.MessageStore
	// Submit runs native channel-task admission (Service.SubmitChannelTask).
	Submit func(context.Context, domain.ChannelTaskInput) (domain.ChannelTaskReceipt, error)
	// Cancel requests cooperative run cancellation; the bool reports
	// whether an active in-memory run was signalled.
	Cancel func(domain.RunID) bool
	// Subscribe registers for live committed events of a run; the journal
	// stays the source of truth and bus closure never proves termination.
	Subscribe func(domain.RunID) (<-chan domain.RunEvent, func())
	// Authorize revalidates the request binding against the current
	// configured allowlist entry and authorization revision; every
	// operation calls it, so revocation takes effect immediately.
	Authorize func(context.Context, TaskPrincipal) bool
	// SafePrompts enables forwarding pending question/approval prompt text
	// into status messages (§7.1). Off, INPUT_REQUIRED carries no prompt.
	SafePrompts bool
	// ServiceInfo is the pre-built safe discovery projection.
	ServiceInfo channel.TaskServiceInfo
}

// taskHost implements channel.TaskHost and channel.TaskServiceInfoHost.
type taskHost struct {
	deps    TaskDeps
	listKey []byte
}

// NewTaskHost builds the task host. It fails closed (returns nil) when any
// dependency is missing so a partial pack never half-enables the surface.
func NewTaskHost(deps TaskDeps) channel.TaskHost {
	if deps.Store == nil || deps.Journal == nil || deps.Messages == nil ||
		deps.Submit == nil || deps.Cancel == nil || deps.Subscribe == nil || deps.Authorize == nil {
		return nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil
	}
	return &taskHost{deps: deps, listKey: key}
}

// NewTaskServiceInfoHost exposes the safe service-info projection on its
// own (TaskServiceInfoHost stays separate so card construction imports no
// Inspect internals).
func NewTaskServiceInfoHost(deps TaskDeps) channel.TaskServiceInfoHost {
	if deps.Authorize == nil {
		return nil
	}
	return &taskHost{deps: deps}
}

func taskErr(code channel.TaskErrorCode, msg string) error {
	return &channel.TaskError{Code: code, Message: msg}
}

// authorize resolves and revalidates the private binding for one op.
func (h *taskHost) authorize(ctx context.Context) (TaskPrincipal, error) {
	p, ok := taskPrincipalFromContext(ctx)
	if !ok || p.InstanceKey() == `["","",""]` || p.PrincipalID == "" {
		return TaskPrincipal{}, taskErr(channel.TaskErrDenied, "unauthenticated task caller")
	}
	if !h.deps.Authorize(ctx, p) {
		return TaskPrincipal{}, taskErr(channel.TaskErrDenied, "task authorization revoked")
	}
	return p, nil
}

func validTaskID(s string) bool {
	if s == "" || len(s) > taskIDMaxBytes || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func mapAdmissionError(err error) error {
	switch {
	case errors.Is(err, storage.ErrWorkRunConflict):
		return taskErr(channel.TaskErrBusy, "run is busy")
	case errors.Is(err, storage.ErrNotFound):
		return taskErr(channel.TaskErrNotFound, "task not found")
	case errors.Is(err, storage.ErrConflict):
		return taskErr(channel.TaskErrConflict, "conflicting channel message")
	case errors.Is(err, storage.ErrRunClosed):
		return taskErr(channel.TaskErrConflict, "run is already closed")
	case errors.Is(err, storage.ErrJournalPageLimit):
		return taskErr(channel.TaskErrLimit, "task exceeds the projection budget")
	default:
		return err
	}
}

// SubmitTask handles both the new-task admission and the ordinary
// pending-input answer (TaskID set): both commit through the scoped
// receipt store before returning.
func (h *taskHost) SubmitTask(ctx context.Context, req channel.TaskRequest) (channel.TaskRef, error) {
	p, err := h.authorize(ctx)
	if err != nil {
		return channel.TaskRef{}, err
	}
	if !validTaskID(req.MessageID) {
		return channel.TaskRef{}, taskErr(channel.TaskErrInvalid, "message_id missing or too long")
	}
	if len(req.Parts) == 0 || len(req.Parts) > taskMaxParts {
		return channel.TaskRef{}, taskErr(channel.TaskErrInvalid, "a task message needs 1..16 text parts")
	}
	parts := make([]string, len(req.Parts))
	for i, part := range req.Parts {
		if len(part.Text) > taskPartMaxBytes {
			return channel.TaskRef{}, taskErr(channel.TaskErrInvalid, "text part exceeds 64 KiB")
		}
		parts[i] = part.Text
	}

	in := domain.ChannelTaskInput{Scope: p.scope(), MessageID: req.MessageID, Parts: parts}
	if req.TaskID != "" {
		// Ordinary answer to the task's pending question: the input hash
		// already binds scope+messageID; TaskID names the target run and
		// QuestionID selects the captured pending input slot.
		owner, ownerErr := h.deps.Store.GetChannelTaskOwner(ctx, p.scope(), domain.RunID(req.TaskID))
		if ownerErr != nil {
			if errors.Is(ownerErr, storage.ErrNotFound) {
				return channel.TaskRef{}, taskErr(channel.TaskErrNotFound, "task not found")
			}
			return channel.TaskRef{}, ownerErr
		}
		if owner == "" {
			return channel.TaskRef{}, taskErr(channel.TaskErrNotFound, "task not found")
		}
		questionID, qErr := h.pendingQuestionID(ctx, domain.RunID(req.TaskID))
		if qErr != nil {
			return channel.TaskRef{}, qErr
		}
		if questionID == "" {
			return channel.TaskRef{}, taskErr(channel.TaskErrConflict, "task is not awaiting input")
		}
		in.RunID = domain.RunID(req.TaskID)
		in.QuestionID = questionID
	}
	receipt, err := h.deps.Submit(ctx, in)
	if err != nil {
		return channel.TaskRef{}, mapAdmissionError(err)
	}
	return channel.TaskRef{
		TaskID:    string(receipt.RunID),
		ContextID: string(receipt.SessionID),
		Replayed:  receipt.Replayed,
	}, nil
}

// GetTask resolves the receipt-owned run and projects one bounded,
// consistent snapshot.
func (h *taskHost) GetTask(ctx context.Context, q channel.TaskQuery) (channel.TaskSnapshot, error) {
	p, err := h.authorize(ctx)
	if err != nil {
		return channel.TaskSnapshot{}, err
	}
	if !validTaskID(q.TaskID) {
		return channel.TaskSnapshot{}, taskErr(channel.TaskErrInvalid, "task_id missing or too long")
	}
	sessionID, err := h.taskSession(ctx, p.scope(), domain.RunID(q.TaskID), q.ContextID)
	if err != nil {
		return channel.TaskSnapshot{}, err
	}
	historyLimit := taskHistoryCap
	if q.HistoryLimit != nil {
		if *q.HistoryLimit < 0 {
			return channel.TaskSnapshot{}, taskErr(channel.TaskErrInvalid, "negative history limit")
		}
		historyLimit = min(*q.HistoryLimit, taskHistoryCap)
	}
	return h.projectTask(ctx, p.scope(), domain.RunID(q.TaskID), sessionID, historyLimit, true)
}

// taskSession authorizes runID inside the scope and applies the optional
// ContextID consistency assertion.
func (h *taskHost) taskSession(ctx context.Context, scope domain.ChannelTaskScope, runID domain.RunID, contextID string) (domain.SessionID, error) {
	sessionID, err := h.deps.Store.GetChannelTaskOwner(ctx, scope, runID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", taskErr(channel.TaskErrNotFound, "task not found")
		}
		return "", err
	}
	if contextID != "" && string(sessionID) != contextID {
		return "", taskErr(channel.TaskErrConflict, "context_id does not match the task")
	}
	return sessionID, nil
}

// CancelTask is idempotent: a committed terminal state returns it as-is; a
// nonterminal run is signalled and the observed projection is reported —
// cancellation is recorded by run.cancelled, never by intent.
func (h *taskHost) CancelTask(ctx context.Context, q channel.TaskQuery) (channel.TaskSnapshot, error) {
	p, err := h.authorize(ctx)
	if err != nil {
		return channel.TaskSnapshot{}, err
	}
	if !validTaskID(q.TaskID) {
		return channel.TaskSnapshot{}, taskErr(channel.TaskErrInvalid, "task_id missing or too long")
	}
	sessionID, err := h.taskSession(ctx, p.scope(), domain.RunID(q.TaskID), q.ContextID)
	if err != nil {
		return channel.TaskSnapshot{}, err
	}
	snap, err := h.projectTask(ctx, p.scope(), domain.RunID(q.TaskID), sessionID, 0, false)
	if err != nil {
		return channel.TaskSnapshot{}, err
	}
	switch snap.Status.State {
	case channel.TaskStateCompleted, channel.TaskStateFailed, channel.TaskStateCanceled:
		return snap, nil // already terminal: idempotent report
	}
	if !h.deps.Cancel(domain.RunID(q.TaskID)) {
		return channel.TaskSnapshot{}, taskErr(channel.TaskErrNotCancelable, "task is not cancelable")
	}
	// Report the observed state; the committed cancellation lands as a
	// journal event and is what GetTask/SubscribeTask will project next.
	snap, err = h.projectTask(ctx, p.scope(), domain.RunID(q.TaskID), sessionID, 0, false)
	if err != nil {
		return channel.TaskSnapshot{}, err
	}
	return snap, nil
}

// SubscribeTask is implemented in A2A-04.3 (task_stream.go). The stub
// fails closed instead of half-enabling the stream.
func (h *taskHost) SubscribeTask(ctx context.Context, sub channel.TaskSubscription) (channel.TaskStream, error) {
	return nil, taskErr(channel.TaskErrUnsupported, "task subscriptions are not enabled")
}

// TaskServiceInfo returns the static safe discovery projection.
func (h *taskHost) TaskServiceInfo(ctx context.Context) (channel.TaskServiceInfo, error) {
	if _, err := h.authorize(ctx); err != nil {
		return channel.TaskServiceInfo{}, err
	}
	info := h.deps.ServiceInfo
	info.Skills = append([]channel.TaskSkill(nil), h.deps.ServiceInfo.Skills...)
	return info, nil
}

// --- bounded journal read under one consistent snapshot -----------------

// journalTail reads every committed event of a run through a fixed
// watermark H captured inside the first page's snapshot (§8.1): appends
// landing after H are invisible, matching the replay-live contract.
func (h *taskHost) journalTail(ctx context.Context, runID domain.RunID, afterSeq domain.EventSeq) ([]domain.RunEvent, domain.EventSeq, error) {
	var (
		out        []domain.RunEvent
		through    domain.EventSeq
		first      = true
		totalBytes int
	)
	for {
		q := storage.JournalPageQuery{
			RunID: runID, AfterSeq: afterSeq, ThroughSeq: through,
			MaxEvents: storage.JournalPageMaxEvents, MaxBytes: storage.JournalPageMaxBytes,
		}
		page, err := h.deps.Journal.ReadJournalPage(ctx, q)
		if err != nil {
			return nil, 0, err
		}
		if first {
			through = page.ThroughSeq
			first = false
		}
		out = append(out, page.Events...)
		for _, e := range page.Events {
			totalBytes += len(e.Payload)
		}
		if len(out) > taskPageScanLimit || totalBytes > taskListByteLimit {
			return nil, 0, taskErr(channel.TaskErrLimit, "task journal exceeds the projection budget")
		}
		if !page.HasMore {
			return out, through, nil
		}
		afterSeq = page.Events[len(page.Events)-1].Seq
	}
}

// --- opaque list page tokens ---------------------------------------------

// taskListToken binds a list page cursor to the exact authorized scope and
// filters so a token is meaningless outside its own listing (§7.3).
type taskListToken struct {
	V         int            `json:"v"`
	Scope     string         `json:"s"`
	Principal string         `json:"p"`
	Filters   [32]byte       `json:"f"`
	IssuedAt  int64          `json:"iat"`
	AfterKey  [2]interface{} `json:"k"` // {updatedAtMilli, runID}
}

const taskListTokenTTL = 5 * 60 // seconds

func (h *taskHost) signListToken(t taskListToken) (string, error) {
	raw, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, h.listKey)
	mac.Write(raw)
	signed := append(raw, mac.Sum(nil)...)
	if len(signed) > 1024 {
		return "", taskErr(channel.TaskErrLimit, "list page token exceeds 1 KiB")
	}
	return base64.RawURLEncoding.EncodeToString(signed), nil
}

func (h *taskHost) openListToken(token string, scope domain.ChannelTaskScope, query channel.TaskListQuery) (*taskListToken, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < sha256.Size+4 || len(raw) > 1024 {
		return nil, taskErr(channel.TaskErrCursorInvalid, "malformed page token")
	}
	body, sig := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, h.listKey)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, taskErr(channel.TaskErrCursorInvalid, "foreign page token")
	}
	var t taskListToken
	if err := json.Unmarshal(body, &t); err != nil || t.V != 1 {
		return nil, taskErr(channel.TaskErrCursorInvalid, "malformed page token")
	}
	if t.Scope != scope.InstanceKey || t.Principal != scope.PrincipalID || t.Filters != listFiltersHash(query) {
		return nil, taskErr(channel.TaskErrCursorInvalid, "page token does not match this listing")
	}
	if now := nowSeconds(); now > t.IssuedAt+taskListTokenTTL || t.IssuedAt > now+60 {
		return nil, taskErr(channel.TaskErrCursorInvalid, "expired page token")
	}
	return &t, nil
}

var nowSeconds = func() int64 { return unixSeconds() }

func listFiltersHash(q channel.TaskListQuery) [32]byte {
	raw, _ := json.Marshal([3]any{q.ContextID, q.State, q.UpdatedAfter})
	return sha256.Sum256(raw)
}

// ListTasks scans receipt-owned primary runs of the authorized scope,
// projects each through a bounded journal read, sorts by descending
// committed status time with the Run ID tie-breaker, and returns an exact
// total only after a complete in-budget scan (§7.3).
func (h *taskHost) ListTasks(ctx context.Context, q channel.TaskListQuery) (channel.TaskPage, error) {
	p, err := h.authorize(ctx)
	if err != nil {
		return channel.TaskPage{}, err
	}
	pageSize := 50
	if q.PageSize != 0 {
		if q.PageSize < 0 || q.PageSize > 100 {
			return channel.TaskPage{}, taskErr(channel.TaskErrInvalid, "page size out of range")
		}
		pageSize = q.PageSize
	}
	scope := p.scope()
	var after *taskListToken
	if q.PageToken != "" {
		after, err = h.openListToken(q.PageToken, scope, q)
		if err != nil {
			return channel.TaskPage{}, err
		}
	}
	historyLimit := 0
	if q.HistoryLimit != nil {
		if *q.HistoryLimit < 0 {
			return channel.TaskPage{}, taskErr(channel.TaskErrInvalid, "negative history limit")
		}
		historyLimit = min(*q.HistoryLimit, taskHistoryCap)
	}

	var (
		snaps    []channel.TaskSnapshot
		cursor   domain.RunID
		budgetEv = taskListScanLimit
	)
	for {
		runs, err := h.deps.Store.ListChannelTaskRuns(ctx, storage.ChannelTaskRunQuery{
			Scope: scope, SessionID: domain.SessionID(q.ContextID), AfterRunID: cursor, Limit: 256,
		})
		if err != nil {
			return channel.TaskPage{}, err
		}
		for _, run := range runs.Runs {
			events, _, err := h.journalTailBudget(ctx, run.ID, 0, &budgetEv)
			if err != nil {
				return channel.TaskPage{}, err
			}
			snap := projectTaskEvents(run.ID, run.SessionID, events, taskProjectionOptions{
				historyLimit:     historyLimit,
				includeArtifacts: q.IncludeArtifacts,
				safePrompts:      h.deps.SafePrompts,
				userMessage:      h.userMessageLookup(ctx, run.SessionID, run.ID),
			})
			if q.State != "" && snap.Status.State != q.State {
				continue
			}
			if q.UpdatedAfter != nil && !snap.Status.UpdatedAt.After(*q.UpdatedAfter) {
				continue
			}
			snaps = append(snaps, snap)
		}
		if !runs.HasMore {
			break
		}
		cursor = runs.NextRunID
		if cursor == "" {
			return channel.TaskPage{}, fmt.Errorf("channelhost: list cursor made no progress")
		}
	}
	sortTaskSnapshots(snaps)
	// Exact authorized total, known only because the whole scope scanned
	// within budget — a budgeted-out scan returns ErrLimit, never a guess.
	total := len(snaps)
	if after != nil && len(after.AfterKey) == 2 {
		// The token names the last emitted position in the projected
		// (committed-status-time, RunID) ordering — receipts order is only
		// a scan vehicle, not the page order.
		marksAt, _ := after.AfterKey[0].(float64)
		marksID, _ := after.AfterKey[1].(string)
		// Sorted order is (updatedAt, taskID) descending; the page
		// continues with entries strictly after the token's key.
		cut := 0
		for i, snap := range snaps {
			at := snap.Status.UpdatedAt.UnixMilli()
			if at > int64(marksAt) || (at == int64(marksAt) && snap.Ref.TaskID >= marksID) {
				cut = i + 1
				continue
			}
			break
		}
		snaps = snaps[cut:]
	}

	out := channel.TaskPage{PageSize: pageSize, TotalSize: total}
	out.Tasks = snaps
	if len(out.Tasks) > pageSize {
		out.Tasks = out.Tasks[:pageSize]
	}
	if len(out.Tasks) < len(snaps) && len(out.Tasks) > 0 {
		last := out.Tasks[len(out.Tasks)-1]
		tok, err := h.signListToken(taskListToken{
			V: 1, Scope: scope.InstanceKey, Principal: scope.PrincipalID,
			Filters: listFiltersHash(q), IssuedAt: nowSeconds(),
			AfterKey: [2]interface{}{last.Status.UpdatedAt.UnixMilli(), last.Ref.TaskID},
		})
		if err != nil {
			return channel.TaskPage{}, err
		}
		out.NextPageToken = tok
	}
	return out, nil
}

// --- Env surface --------------------------------------------------------

// taskCapableEnv mounts the governed task contract on one channel env: the
// §7.0 "task-capable environment wrapper". envFor builds it only for a
// complete TaskDeps pack, so channel.TaskHost assertions fail closed when
// the pack is absent or partial.
type taskCapableEnv struct {
	*hostEnv
	tasks channel.TaskHost
	info  channel.TaskServiceInfoHost
}

// envFor builds the ChannelEnv handed to one adapter's Start; a complete
// Tasks pack additionally mounts the task contract on that env.
func (h *Host) envFor(ch plugin.Channel) plugin.ChannelEnv {
	envelope, ok := h.deps.Config[ch.Name()]
	tokenEnv := ""
	if ok {
		tokenEnv = envelope.TokenEnv
	}
	env := &hostEnv{host: h, seam: ch, client: h.client, tokenEnv: tokenEnv}
	if h.deps.Tasks != nil {
		deps := *h.deps.Tasks
		base := deps.Authorize
		// The env layer additionally pins the binding to this channel's
		// configured instance identity and allow_from principals: a
		// foreign instance or unlisted principal fails here even when the
		// app-level check would pass.
		deps.Authorize = func(ctx context.Context, p TaskPrincipal) bool {
			if p.ProviderID != ch.Name() {
				return false
			}
			listed := false
			if ok {
				for _, id := range envelope.AllowFrom {
					if id == p.PrincipalID {
						listed = true
						break
					}
				}
			}
			return listed && (base == nil || base(ctx, p))
		}
		if tasks, info := NewTaskHost(deps), NewTaskServiceInfoHost(deps); tasks != nil && info != nil {
			return &taskCapableEnv{hostEnv: env, tasks: tasks, info: info}
		}
	}
	return env
}

func (e *taskCapableEnv) SubmitTask(ctx context.Context, req channel.TaskRequest) (channel.TaskRef, error) {
	return e.tasks.SubmitTask(ctx, req)
}
func (e *taskCapableEnv) GetTask(ctx context.Context, q channel.TaskQuery) (channel.TaskSnapshot, error) {
	return e.tasks.GetTask(ctx, q)
}
func (e *taskCapableEnv) ListTasks(ctx context.Context, q channel.TaskListQuery) (channel.TaskPage, error) {
	return e.tasks.ListTasks(ctx, q)
}
func (e *taskCapableEnv) CancelTask(ctx context.Context, q channel.TaskQuery) (channel.TaskSnapshot, error) {
	return e.tasks.CancelTask(ctx, q)
}
func (e *taskCapableEnv) SubscribeTask(ctx context.Context, sub channel.TaskSubscription) (channel.TaskStream, error) {
	return e.tasks.SubscribeTask(ctx, sub)
}
func (e *taskCapableEnv) TaskServiceInfo(ctx context.Context) (channel.TaskServiceInfo, error) {
	return e.info.TaskServiceInfo(ctx)
}
