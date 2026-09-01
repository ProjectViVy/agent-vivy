package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"agent-vivy/internal/config"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

// The headless face (VIVY-FACE-PACK section 5, VC-2 D11): `vivy run
// "prompt"` assembles the same runtime core as the gateway -- one lease,
// one Journal, one policy authority -- but opens no port, embeds no UI,
// and starts no channel ears. It drives exactly one turn, streams
// assistant text to stdout, keeps diagnostics on stderr, and exits by
// terminal status. A run that needs human approval fails loudly instead
// of bypassing HITL or waiting forever: headless is the mouth of scripts
// and CI, not an approver.

// HeadlessOptions bounds one headless invocation.
type HeadlessOptions struct {
	// Prompt is the user turn text. Required.
	Prompt string
	// ContinueNewest attaches the turn to the most recent session instead
	// of creating one (crush `--continue` parity). The Journal is shared
	// with the web face, so continuation picks up the gateway history.
	ContinueNewest bool
	// Out receives assistant text: model deltas as they arrive, the
	// completed content when the provider did not stream.
	Out io.Writer
	// Err receives tool notices, blocking reports, and failure lines;
	// never assistant text.
	Err io.Writer
}

// HeadlessResult reports how the turn ended. The caller maps it to
// process exit codes: completed -> 0, failed -> 1, cancelled -> 2.
type HeadlessResult struct {
	SessionID   domain.SessionID
	RunID       domain.RunID
	Status      domain.RunStatus
	FailCause   string // cause_category of run.failed
	FailMessage string // user-visible failure message
}

// RunHeadless composes the runtime over the validated config and drives
// one prompt to its terminal event. The lease makes the process
// exclusive: a running gateway holds it and this call fails cleanly.
func RunHeadless(ctx context.Context, cfg config.Config, opts HeadlessOptions) (HeadlessResult, error) {
	if strings.TrimSpace(opts.Prompt) == "" {
		return HeadlessResult{}, errors.New("app: headless prompt is empty")
	}
	if opts.Out == nil || opts.Err == nil {
		return HeadlessResult{}, errors.New("app: headless requires output and error writers")
	}
	sink := newHeadlessSink(opts.Out, opts.Err)
	a, err := New(ctx, cfg, WithoutEars(), WithEventSink(sink))
	if err != nil {
		return HeadlessResult{}, err
	}
	defer func() { _ = a.backend.Close() }()

	sessionID, err := resolveHeadlessSession(ctx, a.backend, opts)
	if err != nil {
		return HeadlessResult{}, err
	}
	return runHeadlessTurn(ctx, a.service, sessionID, opts.Prompt, sink)
}

// resolveHeadlessSession picks the session the prompt lands in: the
// newest one with --continue, or a fresh one titled by the prompt (the
// same Journal the gateway lists, so the turn shows up in the web face).
func resolveHeadlessSession(ctx context.Context, sessions storage.SessionStore, opts HeadlessOptions) (domain.SessionID, error) {
	if opts.ContinueNewest {
		list, err := sessions.ListSessions(ctx)
		if err != nil {
			return "", fmt.Errorf("app: list sessions: %w", err)
		}
		if len(list) == 0 {
			return "", errors.New("app: no sessions to continue; run without --continue to start one")
		}
		return list[0].ID, nil
	}
	id, err := newHeadlessSessionID()
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(opts.Prompt)
	if runes := []rune(title); len(runes) > 60 {
		title = string(runes[:60]) + "..."
	}
	if err := sessions.CreateSession(ctx, domain.Session{ID: id, Title: title, CreatedAt: time.Now().UnixMilli()}); err != nil {
		return "", fmt.Errorf("app: create headless session: %w", err)
	}
	return id, nil
}

// newHeadlessSessionID mints sess_<16hex> with identity-grade randomness,
// the same discipline as every other session namespace.
func newHeadlessSessionID() (domain.SessionID, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("app: crypto/rand unavailable: %w", err)
	}
	return domain.SessionID("sess_" + hex.EncodeToString(b)), nil
}

// runHeadlessTurn starts the run and renders events until the terminal.
// Only Cancel ends a run early (AS-7): a signal cancellation and an
// approval block both go through svc.Cancel so the durable terminal is
// emitted while storage is still open.
func runHeadlessTurn(ctx context.Context, svc *runtime.Service, sessionID domain.SessionID, prompt string, sink *headlessSink) (HeadlessResult, error) {
	result := HeadlessResult{SessionID: sessionID}
	runID, err := svc.RunWithOptions(ctx, sessionID, prompt, runtime.RunOptions{
		Face:       domain.FaceHeadless,
		Provenance: &domain.Provenance{Source: "headless"},
	})
	if err != nil {
		return result, err
	}
	result.RunID = runID

	for {
		select {
		case term := <-sink.terminal:
			result.Status = term.status
			result.FailCause = term.failCause
			result.FailMessage = term.failMessage
			// The drive publishes the terminal as its last act; drain so
			// storage never closes under live persistence (E4).
			drainCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if !svc.WaitIdle(drainCtx) {
				return result, errors.New("app: headless run drain timed out")
			}
			return result, nil
		case <-sink.blocked:
			// Approval/question needed and no human is attached: fail
			// loudly (face-pack section 5). Cancel closes the suspension
			// durably; the terminal then arrives on sink.terminal.
			sink.blocked = nil
			svc.Cancel(runID)
		case <-ctx.Done():
			// Signal cancellation: the drive then emits the cancelled
			// terminal and the loop returns it.
			sink.blocked = nil
			svc.Cancel(runID)
		}
	}
}

// headlessSink renders the event stream for a script consumer. The
// runtime publishes synchronously from the drive goroutine, so the
// writers must not block long; direct writes to os.Stdout/os.Stderr are
// acceptable at this scale. Exactly one terminal reaches the channel.
type headlessSink struct {
	out      io.Writer
	errw     io.Writer
	streamed bool
	terminal chan headlessTerminal
	blocked  chan struct{}
}

type headlessTerminal struct {
	status      domain.RunStatus
	failCause   string
	failMessage string
}

func newHeadlessSink(out, errw io.Writer) *headlessSink {
	return &headlessSink{
		out:      out,
		errw:     errw,
		terminal: make(chan headlessTerminal, 1),
		blocked:  make(chan struct{}, 1),
	}
}

// Publish decodes the wire payloads it renders; everything else is
// journal-side detail a script consumer has no use for.
func (s *headlessSink) Publish(ev domain.RunEvent) {
	switch ev.Type {
	case domain.EventModelDelta:
		var p struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(ev.Payload, &p) == nil && p.Delta != "" {
			_, _ = fmt.Fprint(s.out, p.Delta)
			s.streamed = true
		}
	case domain.EventModelCompleted:
		var p struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(ev.Payload, &p) == nil {
			switch {
			case s.streamed:
				_, _ = fmt.Fprintln(s.out)
			case strings.TrimSpace(p.Content) != "":
				_, _ = fmt.Fprintln(s.out, p.Content)
			}
			s.streamed = false
		}
	case domain.EventToolStarted:
		var p struct {
			ToolName string `json:"tool_name"`
		}
		if json.Unmarshal(ev.Payload, &p) == nil {
			_, _ = fmt.Fprintf(s.errw, "> %s\n", p.ToolName)
		}
	case domain.EventToolFinished:
		var p struct {
			ToolName string `json:"tool_name"`
			Error    string `json:"error"`
		}
		if json.Unmarshal(ev.Payload, &p) == nil && p.Error != "" {
			_, _ = fmt.Fprintf(s.errw, "  %s failed: %s\n", p.ToolName, p.Error)
		}
	case domain.EventToolApprovalRequired:
		var p struct {
			ToolName string `json:"tool_name"`
			Action   string `json:"action"`
			Target   string `json:"target"`
		}
		_ = json.Unmarshal(ev.Payload, &p)
		_, _ = fmt.Fprintf(s.errw, "vivy: blocked: %s (%s %s) requires human approval and the headless face cannot ask for it. Approve in the web UI or adjust the permission preset; the run was cancelled.\n", p.ToolName, p.Action, p.Target)
		s.signalBlocked()
	case domain.EventUserQuestionRequired:
		var p struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(ev.Payload, &p)
		_, _ = fmt.Fprintf(s.errw, "vivy: blocked: the model asked %q and the headless face cannot answer. Re-run in the web UI; the run was cancelled.\n", p.Prompt)
		s.signalBlocked()
	case domain.EventRunFailed:
		var p struct {
			CauseCategory string `json:"cause_category"`
			Message       string `json:"message"`
		}
		_ = json.Unmarshal(ev.Payload, &p)
		_, _ = fmt.Fprintf(s.errw, "vivy: run failed (%s): %s\n", p.CauseCategory, p.Message)
		s.emit(headlessTerminal{status: domain.RunFailed, failCause: p.CauseCategory, failMessage: p.Message})
	case domain.EventRunCancelled:
		_, _ = fmt.Fprintln(s.errw, "vivy: run cancelled")
		s.emit(headlessTerminal{status: domain.RunCancelled})
	case domain.EventRunCompleted:
		s.emit(headlessTerminal{status: domain.RunCompleted})
	}
}

func (s *headlessSink) emit(t headlessTerminal) {
	select {
	case s.terminal <- t:
	default:
	}
}

func (s *headlessSink) signalBlocked() {
	select {
	case s.blocked <- struct{}{}:
	default:
	}
}
