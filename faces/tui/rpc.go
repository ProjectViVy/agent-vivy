package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"agent-vivy/sdk/plugin"
)

// client adapts plugin.FaceEnv to the method set the live driver consumes
// (Call + OnNotify), mirroring the kernel TUI's control-plane Client.
type client struct {
	env plugin.FaceEnv

	mu     sync.Mutex
	notify func(method string, params json.RawMessage)
}

func newClient(env plugin.FaceEnv) *client {
	c := &client{env: env}
	env.OnEvent(func(method string, params json.RawMessage) {
		c.mu.Lock()
		notify := c.notify
		c.mu.Unlock()
		if notify != nil {
			notify(method, params)
		}
	})
	return c
}

// OnNotify registers the callback for server notifications (run/event).
func (c *client) OnNotify(fn func(method string, params json.RawMessage)) {
	c.mu.Lock()
	c.notify = fn
	c.mu.Unlock()
}

// Call issues one JSON-RPC method through the face environment.
func (c *client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c == nil || c.env == nil {
		return nil, fmt.Errorf("tui: client is not connected")
	}
	return c.env.Call(ctx, method, params)
}

type sessionView struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	PermissionPreset string `json:"permission_preset"`
}

type messageView struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
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

func (c *client) sessionMessages(ctx context.Context, sessionID string) ([]messageView, error) {
	raw, err := c.Call(ctx, "session/messages", map[string]string{"session_id": sessionID})
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

func (c *client) startTurn(ctx context.Context, sessionID, text string) (runAccepted, error) {
	raw, err := c.Call(ctx, "turn/start", map[string]string{
		"session_id": sessionID,
		"text":       text,
		"face":       "code",
	})
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

func (c *client) subscribe(ctx context.Context, runID string) error {
	_, err := c.Call(ctx, "run/subscribe", map[string]any{"run_id": runID, "after_seq": 0})
	return err
}

func (c *client) cancelRun(ctx context.Context, runID string) error {
	_, err := c.Call(ctx, "run/cancel", map[string]string{"run_id": runID})
	return err
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
