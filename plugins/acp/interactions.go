package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
)

// Interaction bounds (spec §10, §12.6): at most 16 live reverse calls per
// connection, one non-empty answer of at most 64 KiB UTF-8, presentation text
// clamped to a tool-call-sized block, and a teardown budget that leaves
// headroom inside the launcher's 10 s shutdown limit.
const (
	maxPendingInteractions  = 16
	maxAnswerBytes          = 64 << 10
	maxInteractionTextBytes = 4 << 10
	maxInteractionField     = 1 << 10
	terminalPollInterval    = 25 * time.Millisecond

	optionAllowOnce  = "allow-once"
	optionRejectOnce = "reject-once"

	reviewKindApproval  = "approval"
	reviewStatusPending = "pending"

	decisionApproved = "approved"
	decisionDenied   = "denied"
)

// ownedCleanupBudget bounds the durable settlement wait in closeOwned; it is
// a var so tests can shrink the window.
var ownedCleanupBudget = 5 * time.Second

// interactionKind is the reverse-call family a committed event requires.
type interactionKind int

const (
	interactionApproval interactionKind = iota
	interactionQuestion
)

// interaction is one live reverse call owned by a prompt generation. It is
// never a second run store: identity, expiry and authority stay durable, and
// this struct only fences the wire dialog.
type interaction struct {
	a          *agent
	p          *promptState
	kind       interactionKind
	reviewID   string
	toolCallID string
	expiresAt  time.Time

	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	alive bool
}

// newInteraction derives the bounded dialog lifetime from the committed
// expiry. A zero or past expiry is stale: the durable sweeper owns the
// decision, so the adapter presents nothing.
func newInteraction(a *agent, p *promptState, kind interactionKind, reviewID, toolCallID string, expiresAtMillis int64) *interaction {
	deadline := time.UnixMilli(expiresAtMillis)
	if expiresAtMillis <= 0 || !deadline.After(time.Now()) {
		return nil
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	return &interaction{
		a: a, p: p, kind: kind,
		reviewID: reviewID, toolCallID: toolCallID,
		expiresAt: deadline,
		ctx:       ctx, cancel: cancel,
		alive: true,
	}
}

// settle marks the interaction finished by its own reply. It returns false
// when the interaction was already cancelled or settled, so a late or
// duplicated reply has no effect.
func (in *interaction) settle() bool {
	in.mu.Lock()
	if !in.alive {
		in.mu.Unlock()
		return false
	}
	in.alive = false
	in.mu.Unlock()
	in.cancel()
	return true
}

// cancelLive withdraws a live dialog (cancel, terminal, capacity or
// teardown). A reply racing the withdrawal loses at settle().
func (in *interaction) cancelLive() {
	in.mu.Lock()
	if in.alive {
		in.alive = false
	}
	in.mu.Unlock()
	in.cancel()
}

// startInteraction begins the reverse call for one committed interaction
// event. It never blocks the reducer, never approves by itself, and fails
// closed (cancel the affected run) when capacity is exhausted.
func (a *agent) startInteraction(ctx context.Context, p *promptState, proj *runProjection, ev committedEvent) {
	switch ev.Type {
	case "tool.approval_required":
		var payload struct {
			ApprovalID string `json:"approval_id"`
			ToolCallID string `json:"tool_call_id"`
			ToolName   string `json:"tool_name"`
			ExpiresAt  int64  `json:"expires_at"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.ApprovalID == "" || payload.ToolCallID == "" {
			a.failStream(p, "malformed approval event")
			return
		}
		// Durable visibility before the dialog: a permission request never
		// precedes the tool call it belongs to.
		if payload.ToolName != "" {
			if updates := proj.ensureToolCall(payload.ToolCallID, payload.ToolName); len(updates) > 0 {
				if !a.emitUpdates(ctx, p, updates) {
					return
				}
			}
		}
		in := newInteraction(a, p, interactionApproval, payload.ApprovalID, payload.ToolCallID, payload.ExpiresAt)
		if in == nil {
			return // expired before presentation: the durable decision stands
		}
		if !a.registerInteraction(in) {
			a.failStream(p, "interaction capacity reached")
			return
		}
		go a.runApproval(in)
	case "user.question_required":
		var payload struct {
			QuestionID string `json:"question_id"`
			ToolCallID string `json:"tool_call_id"`
			Prompt     string `json:"prompt"`
			ExpiresAt  int64  `json:"expires_at"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.QuestionID == "" {
			a.failStream(p, "malformed question event")
			return
		}
		in := newInteraction(a, p, interactionQuestion, payload.QuestionID, payload.ToolCallID, payload.ExpiresAt)
		if in == nil {
			return
		}
		if !a.registerInteraction(in) {
			a.failStream(p, "interaction capacity reached")
			return
		}
		go a.runQuestion(in, payload.Prompt)
	}
}

func (a *agent) registerInteraction(in *interaction) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.interactions) >= maxPendingInteractions {
		return false
	}
	a.interactions[in] = struct{}{}
	return true
}

func (a *agent) forgetInteraction(in *interaction) {
	a.mu.Lock()
	delete(a.interactions, in)
	a.mu.Unlock()
}

// cancelPromptInteractions withdraws every live dialog owned by p. The
// durable identity is untouched: the run keeps its committed review row.
func (a *agent) cancelPromptInteractions(p *promptState) {
	a.mu.Lock()
	var owned []*interaction
	for in := range a.interactions {
		if in.p == p {
			owned = append(owned, in)
		}
	}
	a.mu.Unlock()
	for _, in := range owned {
		in.cancelLive()
		a.forgetInteraction(in)
	}
}

// reviewView is the allowlisted subset of Control's review/get result the
// adapter may present. Raw Arguments and every other field stay behind.
type reviewView struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Status       string   `json:"status"`
	SessionID    string   `json:"session_id"`
	RunID        string   `json:"run_id"`
	ToolCallID   string   `json:"tool_call_id"`
	ToolName     string   `json:"tool_name"`
	Action       string   `json:"action"`
	Target       string   `json:"target"`
	Preview      string   `json:"preview"`
	RiskFindings []string `json:"risk_findings"`
	ExpiresAt    int64    `json:"expires_at"`
}

// runApproval resolves one approval dialog: durable ownership first, then a
// single allow-once/reject-once wire choice, then exactly one approval
// response. No path here approves without a live matching selection.
func (a *agent) runApproval(in *interaction) {
	defer a.forgetInteraction(in)
	defer in.cancel()

	view, ok := a.approvalView(in)
	if !ok {
		return
	}
	conn := a.connection()
	if conn == nil {
		return
	}
	body := approvalBody(view)
	if body == "" {
		a.settleApproval(in, decisionDenied, "approval preview unavailable")
		return
	}
	title := view.ToolName
	if title == "" {
		title = view.Action
	}
	resp, err := conn.RequestPermission(in.ctx, acp.RequestPermissionRequest{
		SessionID: acp.SessionID(in.p.scope.SessionID),
		ToolCall: acp.ToolCallUpdate{
			ToolCallID: acp.ToolCallID(opaqueToolID(a.nonce, in.p.scope, in.toolCallID)),
			Title:      clampUTF8(title, maxToolTitleBytes),
			Content: []acp.ToolCallContent{{
				Content: &acp.ToolCallContentContent{
					Content: acp.Content{Content: acp.NewContentBlockText(acp.TextContent{Text: clampUTF8(body, maxInteractionTextBytes)})},
					Type:    "content",
				},
			}},
		},
		Options: []acp.PermissionOption{
			{OptionID: optionAllowOnce, Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce},
			{OptionID: optionRejectOnce, Name: "Reject once", Kind: acp.PermissionOptionKindRejectOnce},
		},
	})
	if err != nil {
		// An unanswerable dialog must not leave the governed run waiting:
		// deny through the existing approval path with a safe reason.
		a.settleApproval(in, decisionDenied, "permission request unavailable")
		return
	}
	decision, reason := decisionDenied, "denied in the ACP client"
	if resp.Outcome.Selected != nil {
		switch string(resp.Outcome.Selected.OptionID) {
		case optionAllowOnce:
			decision, reason = decisionApproved, ""
		case optionRejectOnce:
			// denied
		}
	}
	a.settleApproval(in, decision, reason)
}

// approvalView fetches and validates the durable approval before presenting
// it. A stale or foreign row is left untouched and unapproved; an unusable
// presentation denies so the run cannot hang behind an invisible dialog.
func (a *agent) approvalView(in *interaction) (reviewView, bool) {
	raw, err := a.call(in.ctx, "review/get", map[string]string{"review_id": in.reviewID})
	if err != nil {
		if code, ok := rpcCodeOf(err); ok && code == -32004 {
			return reviewView{}, false // stale/foreign: durable sweep decides
		}
		a.settleApproval(in, decisionDenied, "approval preview unavailable")
		return reviewView{}, false
	}
	var view reviewView
	if err := json.Unmarshal(raw, &view); err != nil {
		a.settleApproval(in, decisionDenied, "approval preview unavailable")
		return reviewView{}, false
	}
	if view.ID != in.reviewID || view.Kind != reviewKindApproval ||
		view.SessionID != in.p.scope.SessionID || view.RunID != in.p.runID() {
		// Identity mismatch: never answer someone else's approval.
		return reviewView{}, false
	}
	if view.Status != reviewStatusPending {
		return reviewView{}, false
	}
	return view, true
}

// approvalBody renders only the allowlisted review fields, each bounded, so
// no raw tool arguments or journal envelope reach the client.
func approvalBody(view reviewView) string {
	var lines []string
	if view.Action != "" {
		lines = append(lines, "Action: "+clampUTF8(view.Action, maxInteractionField))
	}
	if view.Target != "" {
		lines = append(lines, "Target: "+clampUTF8(view.Target, maxInteractionField))
	}
	if view.Preview != "" {
		lines = append(lines, "Preview:\n"+clampUTF8(view.Preview, maxInteractionField))
	}
	if len(view.RiskFindings) > 0 {
		lines = append(lines, "Risk: "+clampUTF8(strings.Join(view.RiskFindings, "; "), maxInteractionField))
	}
	return strings.Join(lines, "\n")
}

func (a *agent) settleApproval(in *interaction, decision, reason string) {
	if !in.settle() {
		return // cancelled or already answered: zero effect
	}
	ctx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
	defer cancel()
	_, _ = a.call(ctx, "approval/respond", map[string]string{
		"approval_id": in.reviewID,
		"decision":    decision,
		"reason":      reason,
	})
}

// runQuestion resolves one question dialog: a fixed single-answer form, one
// validated answer, or an explicit cancel. It never writes an empty answer.
func (a *agent) runQuestion(in *interaction, promptText string) {
	defer a.forgetInteraction(in)
	defer in.cancel()

	conn := a.connection()
	if conn == nil {
		return
	}
	message := clampUTF8(promptText, maxInteractionTextBytes)
	if strings.TrimSpace(message) == "" {
		message = "The assistant needs an answer to continue."
	}
	maxLength := int64(maxAnswerBytes)
	objectType := acp.ElicitationSchemaTypeObject
	scope := &acp.ElicitationFormModeElicitationSessionScope{
		ElicitationSessionScope: acp.ElicitationSessionScope{
			SessionID: acp.SessionID(in.p.scope.SessionID),
		},
		RequestedSchema: acp.ElicitationSchema{
			Type: &objectType,
			Properties: map[string]acp.ElicitationPropertySchema{
				"answer": acp.NewElicitationPropertySchemaString(acp.StringPropertySchema{
					Title:       "Answer",
					Description: "Answer for the pending question.",
					MaxLength:   &maxLength,
				}),
			},
			Required: []string{"answer"},
		},
	}
	if in.toolCallID != "" {
		toolCallID := acp.ToolCallID(opaqueToolID(a.nonce, in.p.scope, in.toolCallID))
		scope.ToolCallID = &toolCallID
	}
	resp, err := conn.UnstableCreateElicitation(in.ctx, acp.CreateElicitationRequest{
		Form: &acp.CreateElicitationRequestForm{
			ElicitationFormMode: acp.ElicitationFormMode{ElicitationSessionScope: scope},
			Message:             &message,
		},
	})
	if err != nil {
		a.cancelQuestion(in, "question prompt unavailable")
		return
	}
	if resp.Accept == nil {
		a.cancelQuestion(in, "declined in the ACP client")
		return
	}
	answer, ok := validateAnswer(resp.Accept.Content)
	if !ok {
		a.cancelQuestion(in, "invalid answer")
		return
	}
	if !in.settle() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
	defer cancel()
	_, _ = a.call(ctx, "question/respond", map[string]string{
		"question_id": in.reviewID,
		"answer":      answer,
	})
}

// validateAnswer accepts exactly one non-blank string under the answer key.
// Extra keys, other value kinds, blank text and oversized values are
// invalid; an invalid answer cancels the question instead of answering.
func validateAnswer(content map[string]acp.ElicitationContentValue) (string, bool) {
	if len(content) != 1 {
		return "", false
	}
	value, ok := content["answer"]
	if !ok || value.String == nil {
		return "", false
	}
	answer := string(*value.String)
	if strings.TrimSpace(answer) == "" || len(answer) > maxAnswerBytes || !utf8.ValidString(answer) {
		return "", false
	}
	return answer, true
}

func (a *agent) cancelQuestion(in *interaction, reason string) {
	if !in.settle() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
	defer cancel()
	_, _ = a.call(ctx, "review/respond", map[string]string{
		"review_id": in.reviewID,
		"action":    "cancel",
		"reason":    reason,
	})
}

// closeOwned runs the bounded connection teardown: admission stops, live
// dialogs withdraw, only owned active runs are cancelled, and their durable
// terminal state is awaited. A run that does not settle is reported, never
// claimed as cancelled.
func (a *agent) closeOwned() error {
	a.mu.Lock()
	a.draining = true
	runs := make(map[string]struct{})
	var live []*interaction
	for in := range a.interactions {
		live = append(live, in)
	}
	for id := range a.ownedRuns {
		runs[id] = struct{}{}
	}
	for _, s := range a.sessions {
		s.mu.Lock()
		if s.active != nil {
			if id := s.active.runID(); id != "" {
				runs[id] = struct{}{}
			}
		}
		s.mu.Unlock()
	}
	a.mu.Unlock()

	for _, in := range live {
		in.cancelLive()
		a.forgetInteraction(in)
	}
	if len(runs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), ownedCleanupBudget)
	defer cancel()
	var wg sync.WaitGroup
	for id := range runs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, _ = a.call(ctx, "run/cancel", map[string]string{"run_id": id})
		}(id)
	}
	wg.Wait()

	var unsettled []string
	for id := range runs {
		if a.awaitTerminal(ctx, id) {
			a.untrackRun(id)
			continue
		}
		unsettled = append(unsettled, id)
	}
	if len(unsettled) > 0 {
		return fmt.Errorf("acp: %d owned run(s) still not terminal after %s: %s",
			len(unsettled), ownedCleanupBudget, strings.Join(unsettled, ","))
	}
	return nil
}

// awaitTerminal polls the durable run state. The ACP transport may be gone,
// so the committed terminal state — not a missing stream — is the evidence.
func (a *agent) awaitTerminal(ctx context.Context, runID string) bool {
	for {
		raw, err := a.call(ctx, "run/get", map[string]string{"run_id": runID})
		if err == nil {
			var run struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(raw, &run) == nil && runTerminal(run.Status) {
				return true
			}
		} else if code, ok := rpcCodeOf(err); ok && code == -32004 {
			return true // no durable row: nothing left to settle
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(terminalPollInterval):
		}
	}
}

func runTerminal(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	}
	return false
}

// connection returns the bound transport, if any (unit tests run without a
// wire and present nothing).
func (a *agent) connection() *acpconn.AgentConnection {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conn
}

// clampUTF8 bounds a string without splitting a rune.
func clampUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:runeFloor(s, maxBytes)]
}
