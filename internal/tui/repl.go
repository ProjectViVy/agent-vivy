package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/surface"
	"agent-vivy/sdk/tui/command"
)

const banner = `vivy tui  —  control-plane client, not a second kernel
connected to the resident gateway. type /help. Ctrl+C or /quit to leave.
`

const helpNotes = `
plain lines are sent as the next user turn.

!<script> requires server shell support (unavailable in this build); @path adds project file context.
Use !! and @@ when a literal leading marker is intended.

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
		client:       client,
		in:           bufio.NewScanner(in),
		out:          out,
		session:      session,
		thinkingMode: "auto",
		events:       make(chan eventNotice, 64),
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
	// thinkingMode is a draft preference and is snapshotted by sendTurn.
	thinkingMode string

	mu      sync.Mutex
	busy    bool
	runID   string
	pending *gatePrompt
	// pendingDelete is a local confirmation barrier. A delete command never
	// mutates a session until the following line is an explicit y/yes.
	pendingDelete string
	// pendingMutation is the same confirmation barrier for session context
	// mutations. The captured session id is an epoch guard: if the operator
	// changes sessions before confirming, the RPC is refused.
	pendingMutation *pendingMutationCommand
	// attachments are metadata-only project image drafts for this session.
	// The bytes remain server-owned until turn/start resolves the paths.
	attachments []surface.Attachment

	events chan eventNotice
}

type pendingMutationCommand struct {
	Name      string
	Args      []string
	SessionID string
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
		line := r.in.Text()
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
	if r.pendingDelete != "" {
		return "delete " + r.pendingDelete + "? [y/n] "
	}
	if r.pendingMutation != nil {
		return r.pendingMutation.Name + "? [y/n] "
	}
	if r.busy {
		return "… "
	}
	prompt := "you> "
	if chips := formatREPLAttachments(r.attachments); chips != "" {
		prompt += chips + " "
	}
	return prompt
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
	r.mu.Lock()
	pendingDelete := r.pendingDelete
	r.mu.Unlock()
	if pendingDelete != "" {
		return r.handleDeleteConfirmation(ctx, pendingDelete, line)
	}
	r.mu.Lock()
	pendingMutation := r.pendingMutation
	r.mu.Unlock()
	if pendingMutation != nil {
		return r.handleMutationConfirmation(ctx, pendingMutation, line)
	}
	parsed, err := command.DefaultRegistry().Parse(line)
	if err != nil {
		fmt.Fprintf(r.out, "vivy: %v\n", err)
		return nil
	}
	if parsed.IsUnavailable() {
		fmt.Fprintf(r.out, "vivy: %s\n", parsed.UnavailableReason)
		return nil
	}
	if parsed.IsShell() {
		if busy {
			fmt.Fprintln(r.out, "(run in flight; /cancel or wait)")
			return nil
		}
		return r.sendShell(ctx, parsed.Shell.Script)
	}
	if parsed.IsCommand() {
		if err := command.DefaultRegistry().Validate(parsed.Invocation); err != nil {
			fmt.Fprintf(r.out, "vivy: %v\n", err)
			return nil
		}
		return r.handleCommand(ctx, parsed.Invocation, busy, runID)
	}
	line = parsed.Text
	if strings.TrimSpace(line) == "" {
		if parsed.IsFile() {
			fmt.Fprintln(r.out, "vivy: @file references require a prompt")
		}
		return nil
	}
	if busy {
		fmt.Fprintln(r.out, "(run in flight; /cancel or wait)")
		return nil
	}
	return r.sendTurnWithContext(ctx, line, parsed.FilePaths())
}

func (r *repl) handleCommand(ctx context.Context, invocation *command.Invocation, busy bool, runID string) error {
	if invocation == nil {
		fmt.Fprintln(r.out, "unknown command (/help)")
		return nil
	}
	spec, ok := command.DefaultRegistry().Lookup(invocation.Name)
	if !ok {
		fmt.Fprintf(r.out, "unknown command /%s  (/help)\n", invocation.Name)
		return nil
	}
	cmd := spec.Name
	args := invocation.Args
	switch cmd {
	case "help":
		fmt.Fprint(r.out, command.DefaultRegistry().Help())
		fmt.Fprint(r.out, helpNotes)
		return nil
	case "quit":
		if len(args) != 0 {
			fmt.Fprintln(r.out, "usage: /quit")
			return nil
		}
		return io.EOF
	case "status":
		if len(args) != 0 {
			fmt.Fprintln(r.out, "usage: /status")
			return nil
		}
		r.mu.Lock()
		active := r.session
		busyNow, currentRun := r.busy, r.runID
		thinking := normalizeREPLThinking(r.thinkingMode)
		r.mu.Unlock()
		state := "idle"
		if busyNow {
			state = "running"
		}
		fmt.Fprintf(r.out, "session: %s (%s)\nrun: %s", active.ID, active.Title, state)
		if currentRun != "" {
			fmt.Fprintf(r.out, " (%s)", currentRun)
		}
		fmt.Fprintln(r.out)
		if active.PermissionPreset != "" {
			fmt.Fprintf(r.out, "permission: %s\n", active.PermissionPreset)
		}
		fmt.Fprintf(r.out, "thinking (next turn): %s\n", thinking)
		return nil
	case "cancel":
		if len(args) != 0 {
			fmt.Fprintln(r.out, "usage: /cancel")
			return nil
		}
		if !busy || runID == "" {
			fmt.Fprintln(r.out, "(nothing to cancel)")
			return nil
		}
		if err := r.client.cancelRun(ctx, runID); err != nil {
			fmt.Fprintf(r.out, "cancel: %v\n", err)
			return nil
		}
		// The line REPL has no background event pump. Keep consuming this run's
		// journal stream until its terminal event so busy/runID are cleared and
		// a cancelled run cannot leave the prompt permanently wedged.
		fmt.Fprint(r.out, "vivy: ")
		return r.drainRun(ctx)
	case "new":
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		title := "TUI"
		if len(args) > 0 {
			title = strings.TrimSpace(strings.Join(args, " "))
		}
		session, err := r.client.createSession(ctx, title)
		if err != nil {
			fmt.Fprintf(r.out, "new session: %v\n", err)
			return nil
		}
		r.mu.Lock()
		r.session = session
		r.attachments = nil
		r.mu.Unlock()
		fmt.Fprintf(r.out, "session %s\n", session.ID)
		return nil
	case "sessions":
		if len(args) != 0 {
			fmt.Fprintln(r.out, "usage: /sessions")
			return nil
		}
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
	case "session":
		if len(args) != 1 {
			fmt.Fprintln(r.out, "usage: /session <id>")
			return nil
		}
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		id := args[0]
		messages, err := r.client.sessionMessages(ctx, id)
		if err != nil {
			fmt.Fprintf(r.out, "session: %v\n", err)
			return nil
		}
		r.mu.Lock()
		r.session = sessionView{ID: id}
		r.attachments = nil
		r.mu.Unlock()
		fmt.Fprint(r.out, formatHistory(messages))
		fmt.Fprintf(r.out, "session %s\n", id)
		return nil
	case "rename":
		if len(args) == 0 {
			fmt.Fprintln(r.out, "usage: /rename <title>")
			return nil
		}
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		r.mu.Lock()
		id := r.session.ID
		r.mu.Unlock()
		if id == "" {
			fmt.Fprintln(r.out, "rename: no active session")
			return nil
		}
		session, err := r.client.renameSession(ctx, id, strings.TrimSpace(strings.Join(args, " ")))
		if err != nil {
			fmt.Fprintf(r.out, "rename: %v\n", err)
			return nil
		}
		r.mu.Lock()
		r.session = session
		r.mu.Unlock()
		fmt.Fprintf(r.out, "session %s renamed\n", session.ID)
		return nil
	case "delete":
		if len(args) > 1 {
			fmt.Fprintln(r.out, "usage: /delete [id]")
			return nil
		}
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		id := ""
		if len(args) == 1 {
			id = strings.TrimSpace(args[0])
		} else {
			r.mu.Lock()
			id = r.session.ID
			r.mu.Unlock()
		}
		if id == "" {
			fmt.Fprintln(r.out, "delete: no session selected")
			return nil
		}
		sessions, err := r.client.listSessions(ctx)
		if err != nil {
			fmt.Fprintf(r.out, "delete: %v\n", err)
			return nil
		}
		found := false
		for _, session := range sessions {
			if session.ID == id {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(r.out, "delete: session not found: %s\n", id)
			return nil
		}
		r.mu.Lock()
		r.pendingDelete = id
		r.mu.Unlock()
		fmt.Fprintf(r.out, "delete %s? type y or n\n", id)
		return nil
	case "queue":
		if len(args) != 1 || !strings.EqualFold(args[0], "clear") {
			fmt.Fprintln(r.out, "usage: /queue clear")
			return nil
		}
		fmt.Fprintln(r.out, "queue is empty")
		return nil
	case "permission":
		if len(args) > 1 {
			fmt.Fprintln(r.out, "usage: /permission [cautious|smart|trusted]")
			return nil
		}
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		preset := ""
		if len(args) == 1 {
			preset = strings.ToLower(strings.TrimSpace(args[0]))
		} else {
			r.mu.Lock()
			preset = nextCommandPermission(r.session.PermissionPreset)
			r.mu.Unlock()
		}
		if preset != "cautious" && preset != "smart" && preset != "trusted" {
			fmt.Fprintln(r.out, "permission must be cautious, smart, or trusted")
			return nil
		}
		r.mu.Lock()
		id := r.session.ID
		r.mu.Unlock()
		session, err := r.client.setSessionPermission(ctx, id, preset)
		if err != nil {
			fmt.Fprintf(r.out, "permission: %v\n", err)
			return nil
		}
		r.mu.Lock()
		r.session = session
		r.mu.Unlock()
		fmt.Fprintf(r.out, "permission: %s\n", session.PermissionPreset)
		return nil
	case "thinking":
		mode := ""
		if len(args) == 1 {
			mode = strings.ToLower(strings.TrimSpace(args[0]))
		}
		r.mu.Lock()
		if mode == "" {
			mode = nextREPLThinking(r.thinkingMode)
		}
		sessionID := r.session.ID
		r.mu.Unlock()
		if mode == "on" {
			contextStatus, err := r.client.sessionContext(ctx, sessionID)
			if err != nil {
				fmt.Fprintf(r.out, "thinking: capability check failed: %v\n", err)
				return nil
			}
			if !contextStatus.ThinkingSupported {
				fmt.Fprintln(r.out, "thinking: extended thinking is unavailable for the active model")
				return nil
			}
		}
		r.mu.Lock()
		r.thinkingMode = mode
		r.mu.Unlock()
		fmt.Fprintf(r.out, "next turn thinking: %s\n", mode)
		return nil
	case "image":
		return r.handleImageCommand(ctx, args)
	case "compact":
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		r.mu.Lock()
		sessionID := r.session.ID
		r.pendingMutation = &pendingMutationCommand{Name: "compact", SessionID: sessionID}
		r.mu.Unlock()
		if sessionID == "" {
			r.mu.Lock()
			r.pendingMutation = nil
			r.mu.Unlock()
			fmt.Fprintln(r.out, "compact: no active session")
			return nil
		}
		fmt.Fprintln(r.out, "compact active session? type y or n")
		return nil
	case "fork":
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		r.mu.Lock()
		sessionID := r.session.ID
		r.pendingMutation = &pendingMutationCommand{Name: "fork", Args: append([]string(nil), args...), SessionID: sessionID}
		r.mu.Unlock()
		if sessionID == "" {
			r.mu.Lock()
			r.pendingMutation = nil
			r.mu.Unlock()
			fmt.Fprintln(r.out, "fork: no active session")
			return nil
		}
		fmt.Fprintf(r.out, "fork at message %s? type y or n\n", args[0])
		return nil
	case "rewind":
		if busy {
			fmt.Fprintln(r.out, "(wait for the current run, or /cancel)")
			return nil
		}
		r.mu.Lock()
		sessionID := r.session.ID
		r.pendingMutation = &pendingMutationCommand{Name: "rewind", Args: append([]string(nil), args...), SessionID: sessionID}
		r.mu.Unlock()
		if sessionID == "" {
			r.mu.Lock()
			r.pendingMutation = nil
			r.mu.Unlock()
			fmt.Fprintln(r.out, "rewind: no active session")
			return nil
		}
		fmt.Fprintf(r.out, "rewind at message %s? type y or n\n", args[0])
		return nil
	case "todos":
		r.mu.Lock()
		sessionID := r.session.ID
		r.mu.Unlock()
		if sessionID == "" {
			fmt.Fprintln(r.out, "todos: no active session")
			return nil
		}
		return r.printCommandRPC(ctx, "todos", "session/todos", map[string]string{"session_id": sessionID})
	case "stats":
		params := map[string]string{}
		if len(args) == 1 {
			params["period"] = strings.ToLower(args[0])
		}
		return r.printCommandRPC(ctx, "stats", "stats/tokens", params)
	case "skills":
		if len(args) == 1 {
			return r.printCommandRPC(ctx, "skills", "skills/get", map[string]string{"name": args[0]})
		}
		return r.printCommandRPC(ctx, "skills", "skills/list", nil)
	case "mcp":
		if len(args) == 1 {
			return r.printCommandRPC(ctx, "mcp", "settings/mcp/probe", map[string]string{"name": args[0]})
		}
		if len(args) == 2 && strings.EqualFold(strings.TrimSpace(args[0]), "resources") {
			return r.printCommandRPC(ctx, "mcp", "settings/mcp/resources", map[string]string{"name": args[1]})
		}
		if len(args) == 3 && strings.EqualFold(strings.TrimSpace(args[0]), "read") {
			return r.printCommandRPC(ctx, "mcp", "settings/mcp/read", map[string]string{"server": args[1], "uri": args[2]})
		}
		return r.printCommandRPC(ctx, "mcp", "settings/mcp", nil)
	case "files":
		fileRunID := ""
		if len(args) > 0 {
			fileRunID = strings.TrimSpace(args[0])
		} else {
			fileRunID = runID
		}
		if fileRunID == "" {
			fmt.Fprintln(r.out, "files: a run_id is required; workspace files are scoped to a run")
			return nil
		}
		if len(args) == 2 {
			return r.printCommandRPC(ctx, "files", "workspace/read", map[string]string{"run_id": fileRunID, "path": args[1]})
		}
		return r.printCommandRPC(ctx, "files", "workspace/list", map[string]string{"run_id": fileRunID})
	case "tools":
		return r.printCommandRPC(ctx, "tools", "tools/list", nil)
	default:
		fmt.Fprintf(r.out, "unknown command /%s  (/help)\n", cmd)
		return nil
	}
}

func (r *repl) printCommandRPC(ctx context.Context, name, method string, params any) error {
	raw, err := r.client.Call(ctx, method, params)
	if err != nil {
		fmt.Fprintf(r.out, "%s: %v\n", name, err)
		return nil
	}
	if output := command.FormatResult(name, raw); output != "" {
		fmt.Fprintln(r.out, output)
	}
	return nil
}

func (r *repl) handleImageCommand(ctx context.Context, args []string) error {
	if len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "clear") {
		r.mu.Lock()
		count := len(r.attachments)
		r.attachments = nil
		r.mu.Unlock()
		if count == 0 {
			fmt.Fprintln(r.out, "no pending image attachments")
		} else {
			fmt.Fprintf(r.out, "cleared %d pending image attachment(s)\n", count)
		}
		return nil
	}
	if len(args) == 2 && strings.EqualFold(strings.TrimSpace(args[0]), "remove") {
		index, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil || index < 1 {
			fmt.Fprintln(r.out, "image remove index must be a positive number")
			return nil
		}
		r.mu.Lock()
		if index > len(r.attachments) {
			r.mu.Unlock()
			fmt.Fprintf(r.out, "image attachment %d is not pending\n", index)
			return nil
		}
		r.attachments = append(r.attachments[:index-1], r.attachments[index:]...)
		r.mu.Unlock()
		fmt.Fprintf(r.out, "removed pending image attachment %d\n", index)
		return nil
	}
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(r.out, "usage: /image <relative-path> | /image remove <index> | /image clear")
		return nil
	}
	r.mu.Lock()
	sessionID := r.session.ID
	currentCount := len(r.attachments)
	r.mu.Unlock()
	if sessionID == "" {
		fmt.Fprintln(r.out, "image: no active session")
		return nil
	}
	if currentCount >= 4 {
		fmt.Fprintln(r.out, "image: at most 4 image attachments are allowed per message")
		return nil
	}
	status, err := r.client.sessionContext(ctx, sessionID)
	if err != nil {
		fmt.Fprintf(r.out, "image: capability check failed: %v\n", err)
		return nil
	}
	if !status.ImageSupportKnown {
		fmt.Fprintln(r.out, "image: unavailable until model image support is known")
		return nil
	}
	if !status.ImageSupported {
		fmt.Fprintln(r.out, "image: the active model does not support image attachments")
		return nil
	}
	attachments, err := r.client.resolveAttachments(ctx, []string{args[0]})
	if err != nil {
		fmt.Fprintf(r.out, "image: %v\n", err)
		return nil
	}
	if len(attachments) != 1 {
		fmt.Fprintln(r.out, "image: resolver returned no attachment")
		return nil
	}
	r.mu.Lock()
	r.attachments = append(r.attachments, attachments[0])
	r.mu.Unlock()
	fmt.Fprintf(r.out, "attached %s\n", formatREPLAttachments(attachments))
	return nil
}

func formatREPLAttachments(attachments []surface.Attachment) string {
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

func (r *repl) handleDeleteConfirmation(ctx context.Context, id, line string) error {
	decision, ok := parseApproval(line)
	if !ok {
		fmt.Fprintln(r.out, "type y or n")
		return nil
	}
	if decision == domain.ApprovalDenied {
		r.mu.Lock()
		r.pendingDelete = ""
		r.mu.Unlock()
		fmt.Fprintln(r.out, "delete cancelled")
		return nil
	}
	if err := r.client.deleteSession(ctx, id); err != nil {
		fmt.Fprintf(r.out, "delete: %v\n", err)
		return nil
	}
	r.mu.Lock()
	active := r.session.ID == id
	r.pendingDelete = ""
	r.mu.Unlock()
	fmt.Fprintf(r.out, "session %s deleted\n", id)
	if !active {
		return nil
	}
	// Keep the REPL usable after deleting its active session. The new session
	// is created only after the delete has succeeded; a failed create leaves a
	// visible error instead of pretending the old session still exists.
	session, err := r.client.createSession(ctx, "TUI")
	if err != nil {
		fmt.Fprintf(r.out, "new session: %v\n", err)
		return nil
	}
	r.mu.Lock()
	r.session = session
	r.attachments = nil
	r.mu.Unlock()
	fmt.Fprintf(r.out, "session %s\n", session.ID)
	return nil
}

func (r *repl) handleMutationConfirmation(ctx context.Context, pending *pendingMutationCommand, line string) error {
	decision, ok := parseApproval(line)
	if !ok {
		fmt.Fprintln(r.out, "type y or n")
		return nil
	}
	if decision == domain.ApprovalDenied {
		r.mu.Lock()
		r.pendingMutation = nil
		r.mu.Unlock()
		fmt.Fprintln(r.out, "command cancelled")
		return nil
	}
	r.mu.Lock()
	currentID := r.session.ID
	busy := r.busy
	r.pendingMutation = nil
	r.mu.Unlock()
	if busy {
		fmt.Fprintln(r.out, "(run started before confirmation; command cancelled)")
		return nil
	}
	if currentID != pending.SessionID {
		fmt.Fprintln(r.out, "active session changed; command cancelled")
		return nil
	}
	params := map[string]string{"session_id": pending.SessionID}
	switch pending.Name {
	case "compact":
		_, ok := r.callCommandRPC(ctx, pending.Name, "context/compact", params)
		if !ok {
			return nil
		}
		// The line face does not retain a context panel, but re-reading the
		// authoritative context makes the next visible snapshot converge with
		// the mutation instead of relying on the command response alone.
		contextRaw, err := r.client.Call(ctx, "session/context", params)
		if err != nil {
			fmt.Fprintf(r.out, "compact refresh: %v\n", err)
			return nil
		}
		fmt.Fprintln(r.out, "refreshed session context")
		fmt.Fprintln(r.out, command.FormatJSON(contextRaw))
		return nil
	case "fork":
		if len(pending.Args) == 0 {
			fmt.Fprintln(r.out, "usage: /fork <message_id> [title]")
			return nil
		}
		params["message_id"] = pending.Args[0]
		if len(pending.Args) == 2 {
			params["title"] = pending.Args[1]
		}
		raw, ok := r.callCommandRPC(ctx, pending.Name, "session/fork", params)
		if !ok {
			return nil
		}
		var result struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(raw, &result); err != nil || strings.TrimSpace(result.SessionID) == "" {
			fmt.Fprintln(r.out, "fork: response did not identify the new session")
			return nil
		}
		session, err := r.client.getSession(ctx, result.SessionID)
		if err != nil {
			fmt.Fprintf(r.out, "fork refresh: %v\n", err)
			return nil
		}
		messages, err := r.client.sessionMessages(ctx, result.SessionID)
		if err != nil {
			fmt.Fprintf(r.out, "fork history: %v\n", err)
			return nil
		}
		r.mu.Lock()
		r.session = session
		r.attachments = nil
		r.mu.Unlock()
		fmt.Fprint(r.out, formatHistory(messages))
		fmt.Fprintf(r.out, "session %s\n", session.ID)
		return nil
	case "rewind":
		if len(pending.Args) == 0 {
			fmt.Fprintln(r.out, "usage: /rewind <message_id>")
			return nil
		}
		params["message_id"] = pending.Args[0]
		if _, ok := r.callCommandRPC(ctx, pending.Name, "session/rewind", params); !ok {
			return nil
		}
		messages, err := r.client.sessionMessages(ctx, pending.SessionID)
		if err != nil {
			fmt.Fprintf(r.out, "rewind refresh: %v\n", err)
			return nil
		}
		fmt.Fprint(r.out, formatHistory(messages))
		fmt.Fprintf(r.out, "session %s refreshed\n", pending.SessionID)
		return nil
	default:
		fmt.Fprintf(r.out, "unknown mutation /%s\n", pending.Name)
		return nil
	}
}

func (r *repl) callCommandRPC(ctx context.Context, name, method string, params any) ([]byte, bool) {
	raw, err := r.client.Call(ctx, method, params)
	if err != nil {
		fmt.Fprintf(r.out, "%s: %v\n", name, err)
		return nil, false
	}
	if output := command.FormatResult(name, raw); output != "" {
		fmt.Fprintln(r.out, output)
	}
	return raw, true
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
	return r.sendTurnWithContext(ctx, text, nil)
}

func (r *repl) sendTurnWithContext(ctx context.Context, text string, contextPaths []string) error {
	r.mu.Lock()
	thinking := normalizeREPLThinking(r.thinkingMode)
	sessionID := r.session.ID
	attachments := append([]surface.Attachment(nil), r.attachments...)
	r.mu.Unlock()
	if thinking == "on" {
		contextStatus, err := r.client.sessionContext(ctx, sessionID)
		if err != nil {
			r.mu.Lock()
			r.thinkingMode = "auto"
			r.mu.Unlock()
			thinking = "auto"
			fmt.Fprintf(r.out, "thinking: capability check failed; using auto: %v\n", err)
		} else if !contextStatus.ThinkingSupported {
			r.mu.Lock()
			r.thinkingMode = "auto"
			r.mu.Unlock()
			thinking = "auto"
			fmt.Fprintln(r.out, "thinking: active model no longer supports on; using auto")
		}
	}
	if len(contextPaths) > 0 {
		if _, err := r.client.resolveProjectContext(ctx, contextPaths); err != nil {
			fmt.Fprintf(r.out, "context: %v\n", err)
			return nil
		}
	}
	accepted, err := r.client.startTurnWithAttachmentsAndContext(ctx, sessionID, text, "code", thinking, attachments, contextPaths)
	if err != nil {
		fmt.Fprintf(r.out, "turn: %v\n", err)
		return nil
	}
	// turn/start is the ownership boundary: the server has accepted and
	// persisted this turn. Never retain the same image draft merely because
	// the follow-up subscription failed, or a retry would duplicate it.
	r.mu.Lock()
	r.attachments = nil
	r.mu.Unlock()
	if err := r.client.subscribe(ctx, accepted.RunID, 0); err != nil {
		fmt.Fprintf(r.out, "turn accepted as %s; subscribe: %v\n", accepted.RunID, err)
		return nil
	}
	r.mu.Lock()
	r.busy = true
	r.runID = accepted.RunID
	r.mu.Unlock()
	fmt.Fprint(r.out, "vivy: ")
	return r.drainRun(ctx)
}

// sendShell starts a governed server-side shell run. The legacy line face has
// no local execution fallback and uses the same run/subscribe/cancel event
// stream as ordinary turns.
func (r *repl) sendShell(ctx context.Context, script string) error {
	if strings.TrimSpace(script) == "" {
		fmt.Fprintln(r.out, "shell: script is required")
		return nil
	}
	r.mu.Lock()
	sessionID := r.session.ID
	r.mu.Unlock()
	if sessionID == "" {
		fmt.Fprintln(r.out, "shell: no active session")
		return nil
	}
	accepted, err := r.client.startShell(ctx, sessionID, script)
	if err != nil {
		fmt.Fprintf(r.out, "shell: %v\n", err)
		return nil
	}
	if err := r.client.subscribe(ctx, accepted.RunID, 0); err != nil {
		fmt.Fprintf(r.out, "shell accepted as %s; subscribe: %v\n", accepted.RunID, err)
		return nil
	}
	r.mu.Lock()
	r.busy = true
	r.runID = accepted.RunID
	r.mu.Unlock()
	fmt.Fprint(r.out, "vivy: ")
	return r.drainRun(ctx)
}

func normalizeREPLThinking(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "on" || mode == "off" {
		return mode
	}
	return "auto"
}

func nextREPLThinking(current string) string {
	switch normalizeREPLThinking(current) {
	case "auto":
		return "on"
	case "on":
		return "off"
	default:
		return "auto"
	}
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
