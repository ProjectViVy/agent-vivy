package codeclient

import (
	"context"
	"encoding/json"
)

// State mirrors the get_state response data (pi RpcSessionState shape).
type State struct {
	SessionID             string `json:"session_id"`
	IsStreaming           bool   `json:"isStreaming"`
	IsCompacting          bool   `json:"isCompacting"`
	PendingMessageCount   int    `json:"pendingMessageCount"`
	SteeringMode          string `json:"steeringMode"`
	FollowUpMode          string `json:"followUpMode"`
	AutoCompactionEnabled bool   `json:"autoCompactionEnabled"`
}

// Disposition is the prompt/follow_up response data.
type Disposition struct {
	Disposition string `json:"disposition"` // "started" | "queued"
	RunID       string `json:"run_id"`
}

// CommandInfo is one get_commands row.
type CommandInfo struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Source    string `json:"source"`
}

func decode[T any](raw json.RawMessage, err error) (T, error) {
	var v T
	if err != nil {
		return v, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &v); err != nil {
			return v, err
		}
	}
	return v, nil
}

// Prompt starts a turn; while a run is active it queues face-locally (until
// B1 lands kernel steering/follow-up truth).
func (c *Client) Prompt(ctx context.Context, message string) (Disposition, error) {
	return decode[Disposition](c.Call(ctx, "prompt", map[string]any{"message": message}))
}

// FollowUp queues a message behind the active run.
func (c *Client) FollowUp(ctx context.Context, message string) (Disposition, error) {
	return decode[Disposition](c.Call(ctx, "follow_up", map[string]any{"message": message}))
}

// Steer injects a message mid-turn (lands with B1; errors until then).
func (c *Client) Steer(ctx context.Context, message string) (json.RawMessage, error) {
	return c.Call(ctx, "steer", map[string]any{"message": message})
}

// Abort cancels the active run.
func (c *Client) Abort(ctx context.Context) error {
	_, err := c.Call(ctx, "abort", nil)
	return err
}

// ClearQueue drains queued messages; returns them by lane.
func (c *Client) ClearQueue(ctx context.Context) (map[string][]string, error) {
	raw, err := c.Call(ctx, "clear_queue", nil)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{"steering": {}, "followUp": {}}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) == nil {
		for k, v := range m {
			var arr []string
			if json.Unmarshal(v, &arr) == nil {
				out[k] = arr
			}
		}
	}
	return out, nil
}

// NewSession switches the rpc session to a fresh one.
func (c *Client) NewSession(ctx context.Context, title string) (string, error) {
	raw, err := c.Call(ctx, "new_session", map[string]any{"title": title})
	if err != nil {
		return "", err
	}
	var d struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(raw, &d)
	return d.SessionID, nil
}

// SwitchSession attaches the rpc session to an existing session id.
func (c *Client) SwitchSession(ctx context.Context, sessionID string) error {
	_, err := c.Call(ctx, "switch_session", map[string]any{"session_id": sessionID})
	return err
}

func (c *Client) SetSessionName(ctx context.Context, name string) error {
	_, err := c.Call(ctx, "set_session_name", map[string]any{"name": name})
	return err
}

func (c *Client) GetState(ctx context.Context) (State, error) {
	return decode[State](c.Call(ctx, "get_state", nil))
}

// GetMessages returns the raw session/messages result.
func (c *Client) GetMessages(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "get_messages", nil)
}

func (c *Client) LastAssistantText(ctx context.Context) (string, error) {
	raw, err := c.Call(ctx, "get_last_assistant_text", nil)
	if err != nil {
		return "", err
	}
	var d struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(raw, &d)
	return d.Text, nil
}

func (c *Client) GetSessionStats(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "get_session_stats", nil)
}

func (c *Client) Models(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "get_available_models", nil)
}

func (c *Client) SetModel(ctx context.Context, provider, modelID string) error {
	_, err := c.Call(ctx, "set_model", map[string]any{"provider": provider, "modelId": modelID})
	return err
}

// Compact compacts the session context; instructions pass through (kernel
// honors them once D1 lands).
func (c *Client) Compact(ctx context.Context, instructions string) (json.RawMessage, error) {
	return c.Call(ctx, "compact", map[string]any{"customInstructions": instructions})
}

// Fork branches the session at a message (entry) id.
func (c *Client) Fork(ctx context.Context, entryID string) (json.RawMessage, error) {
	return c.Call(ctx, "fork", map[string]any{"entryId": entryID})
}

// Clone forks at the session's last message.
func (c *Client) Clone(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "clone", nil)
}

// Commands describes the child's command surface with availability flags.
func (c *Client) Commands(ctx context.Context) ([]CommandInfo, error) {
	raw, err := c.Call(ctx, "get_commands", nil)
	if err != nil {
		return nil, err
	}
	var d struct {
		Commands []CommandInfo `json:"commands"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return d.Commands, nil
}

// Deferred surfaces — they exist so callers get a typed error now and a
// working method when the backend lands, without an API break.

func (c *Client) CycleModel(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "cycle_model", nil)
}

func (c *Client) SetThinkingLevel(ctx context.Context, level string) (json.RawMessage, error) {
	return c.Call(ctx, "set_thinking_level", map[string]any{"level": level})
}

func (c *Client) ThinkingLevels(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "get_available_thinking_levels", nil)
}

func (c *Client) SetSteeringMode(ctx context.Context, mode string) error {
	_, err := c.Call(ctx, "set_steering_mode", map[string]any{"mode": mode})
	return err
}

func (c *Client) SetFollowUpMode(ctx context.Context, mode string) error {
	_, err := c.Call(ctx, "set_follow_up_mode", map[string]any{"mode": mode})
	return err
}

func (c *Client) SetAutoCompaction(ctx context.Context, enabled bool) error {
	_, err := c.Call(ctx, "set_auto_compaction", map[string]any{"enabled": enabled})
	return err
}

func (c *Client) SetAutoRetry(ctx context.Context, enabled bool) error {
	_, err := c.Call(ctx, "set_auto_retry", map[string]any{"enabled": enabled})
	return err
}

func (c *Client) AbortRetry(ctx context.Context) error {
	_, err := c.Call(ctx, "abort_retry", nil)
	return err
}

func (c *Client) Tree(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "get_tree", nil)
}

func (c *Client) Entries(ctx context.Context) (json.RawMessage, error) {
	return c.Call(ctx, "get_entries", nil)
}

// Export writes the session transcript to path (lands with C1).
func (c *Client) Export(ctx context.Context, path string) (json.RawMessage, error) {
	return c.Call(ctx, "export_html", map[string]any{"outputPath": path})
}
