package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"agent-vivy/internal/domain"
)

const banner = `vivy tui  —  control-plane client, not a second kernel
connected to the resident gateway. type /help. Ctrl+C or /quit to leave.
`

const helpText = `commands
  /help              this text
  /sessions          list sessions
  /session <id>      switch to an existing session
  /new [title]       start a new session
  /cancel            cancel the in-flight run
  /quit              leave the TTY (gateway keeps running)

plain lines are sent as the next user turn.

when a tool needs approval, the next line is y or n
(approved / denied). a pending question takes the next line as the answer.
`

// Options configure one TTY session against a connected client.
type Options struct {
	Input  io.Reader
	Output io.Writer
	Title  string
}

// RunREPL drives the first-cut terminal face until input ends or /quit.
func RunREPL(ctx context.Context, client *Client, opts Options) error {
	if client == nil {
		return fmt.Errorf("tui: nil client")
	}
	in := opts.Input
	if in == nil {
		in = os.Stdin
	}
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "TUI"
	}

	session, err := client.createSession(ctx, title)
	if err != nil {
		return err
	}

	repl := &repl{
		client:  client,
		in:      bufio.NewScanner(in),
		out:     out,
		session: session,
		events:  make(chan eventNotice, 64),
	}
	client.OnNotify(func(method string, params json.RawMessage) {
		if method != "run/event" {
			return
		}
		event, ok := decodeStreamEvent(params)
		if !ok {
			return
		}
		notice := interpret(event)
		if notice.Kind == "" && notice.Delta == "" && notice.Gate == nil && !notice.Done {
			return
		}
		select {
		case repl.events <- notice:
		case <-ctx.Done():
		}
	})

	fmt.Fprint(out, banner)
	fmt.Fprintf(out, "session %s  (%s)\n", session.ID, session.PermissionPreset)
	return repl.loop(ctx)
}

type repl struct {
	client  *Client
	in      *bufio.Scanner
	out     io.Writer
	session sessionView

	mu      sync.Mutex
	busy    bool
	runID   string
	pending *gatePrompt

	events chan eventNotice
}

func (r *repl) loop(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.mu.Lock()
		prompt := r.promptLocked()
		r.mu.Unlock()
		fmt.Fprint(r.out, prompt)
		if !r.in.Scan() {
			if err := r.in.Err(); err != nil {
				return err
			}
			fmt.Fprintln(r.out)
			return nil
		}
		line := strings.TrimSpace(r.in.Text())
		if err := r.handleLine(ctx, line); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (r *repl) promptLocked() string {
	if r.pending != nil {
		if r.pending.Kind == "approval" {
			return "approve? [y/n] "
		}
		return "answer> "
	}
	if r.busy {
		return "… "
	}
	return "you> "
}

func (r *repl) handleLine(ctx context.Context, line string) error {
	r.mu.Lock()
	pending := r.pending
	busy := r.busy
	runID := r.runID
	r.mu.Unlock()

	if pending != nil {
		if err := r.handleGate(ctx, pending, line); err != nil {
			return err
		}
		r.mu.Lock()
		stillBusy := r.busy && r.pending == nil
		r.mu.Unlock()
		if stillBusy {
			fmt.Fprint(r.out, "vivy: ")
			return r.drainRun(ctx)
		}
		return nil
	}
	if line == "" {
		return nil
	}
	if strings.HasPrefix(line, "/") {
		return r.handleCommand(ctx, line, busy, runID)
	}
	if busy {
		fmt.Fprintln(r.out, "(run in flight; /cancel or wait)")
		return nil
	}
	return r.sendTurn(ctx, line)
}

func (r *repl) handleCommand(ctx context.Context, line string, busy bool, runID string) error {
	fields := strings.Fields(line)
	cmd := strings.ToLower(fields[0])
	switch cmd {
	case "/help", "/?":
		fmt.Fprint(r.out, helpText)
		return nil
	case "/quit", "/exit", "/q":
		return io.EOF
	case "/cancel":
		if !busy || runID == "" {
			fmt.Fprintln(r.out, "(nothing to cancel)")
			return nil
		}
		if err := r.client.cancelRun(ctx, runID); err != nil {
			fmt.Fprintf(r.out, "cancel: %v\n", err)
		}
		return nil
	case "/new":
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		title := "TUI"
		if len(fields) > 1 {
			title = strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		}
		session, err := r.client.createSession(ctx, title)
		if err != nil {
			fmt.Fprintf(r.out, "new session: %v\n", err)
			return nil
		}
		r.mu.Lock()
		r.session = session
		r.mu.Unlock()
		fmt.Fprintf(r.out, "session %s\n", session.ID)
		return nil
	case "/sessions":
		sessions, err := r.client.listSessions(ctx)
		if err != nil {
			fmt.Fprintf(r.out, "sessions: %v\n", err)
			return nil
		}
		for _, session := range sessions {
			mark := " "
			if session.ID == r.session.ID {
				mark = "*"
			}
			fmt.Fprintf(r.out, "%s %s  %s\n", mark, session.ID, session.Title)
		}
		return nil
	case "/session":
		if len(fields) < 2 {
			fmt.Fprintln(r.out, "usage: /session <id>")
			return nil
		}
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		id := fields[1]
		messages, err := r.client.sessionMessages(ctx, id)
		if err != nil {
			fmt.Fprintf(r.out, "session: %v\n", err)
			return nil
		}
		r.mu.Lock()
		r.session = sessionView{ID: id}
		r.mu.Unlock()
		fmt.Fprint(r.out, formatHistory(messages))
		fmt.Fprintf(r.out, "session %s\n", id)
		return nil
	default:
		fmt.Fprintf(r.out, "unknown command %s  (/help)\n", cmd)
		return nil
	}
}

func (r *repl) handleGate(ctx context.Context, pending *gatePrompt, line string) error {
	switch pending.Kind {
	case "approval":
		decision, ok := parseApproval(line)
		if !ok {
			fmt.Fprintln(r.out, "type y or n")
			return nil
		}
		if err := r.client.respondApproval(ctx, pending.ID, decision); err != nil {
			fmt.Fprintf(r.out, "approval: %v\n", err)
			return nil
		}
	case "question":
		if line == "" {
			fmt.Fprintln(r.out, "type an answer")
			return nil
		}
		if err := r.client.respondQuestion(ctx, pending.ID, line); err != nil {
			fmt.Fprintf(r.out, "question: %v\n", err)
			return nil
		}
	}
	r.mu.Lock()
	r.pending = nil
	r.mu.Unlock()
	return nil
}

func parseApproval(line string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes", "ok", domain.ApprovalApproved:
		return domain.ApprovalApproved, true
	case "n", "no", domain.ApprovalDenied:
		return domain.ApprovalDenied, true
	default:
		return "", false
	}
}

func (r *repl) sendTurn(ctx context.Context, text string) error {
	accepted, err := r.client.startTurn(ctx, r.session.ID, text, "code")
	if err != nil {
		fmt.Fprintf(r.out, "turn: %v\n", err)
		return nil
	}
	if err := r.client.subscribe(ctx, accepted.RunID, 0); err != nil {
		fmt.Fprintf(r.out, "subscribe: %v\n", err)
		return nil
	}
	r.mu.Lock()
	r.busy = true
	r.runID = accepted.RunID
	r.mu.Unlock()
	fmt.Fprint(r.out, "vivy: ")
	return r.drainRun(ctx)
}

func (r *repl) drainRun(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case notice := <-r.events:
			if notice.Delta != "" {
				fmt.Fprint(r.out, notice.Delta)
			}
			if notice.Line != "" {
				fmt.Fprintf(r.out, "\n%s", notice.Line)
			}
			if notice.Gate != nil {
				r.mu.Lock()
				r.pending = notice.Gate
				r.mu.Unlock()
				fmt.Fprintln(r.out)
				if notice.Gate.Kind == "approval" {
					fmt.Fprintf(r.out, "%s\n", notice.Gate.Body)
				}
				return nil
			}
			if notice.Done {
				if notice.Failed && notice.Message != "" {
					fmt.Fprintf(r.out, "\n[%s]\n", notice.Message)
				} else {
					fmt.Fprintln(r.out)
				}
				r.mu.Lock()
				r.busy = false
				r.runID = ""
				r.pending = nil
				r.mu.Unlock()
				return nil
			}
		}
	}
}
