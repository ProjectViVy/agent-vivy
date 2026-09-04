package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"agent-vivy/internal/tui/view"
	"agent-vivy/sdk/plugin"
)

// NewFace is the built-in interactive code face used by `vivy tui`.
func NewFace(opts plugin.FaceOptions) plugin.Face { return &codeFace{opts: opts} }

type codeFace struct{ opts plugin.FaceOptions }

func (*codeFace) Kind() string { return "tui" }

func (f *codeFace) Run(ctx context.Context, env plugin.FaceEnv) (plugin.FaceResult, error) {
	if !looksTerminal(f.opts.Out) {
		return plugin.FaceResult{Status: "failed"}, errors.New("tui: an interactive terminal is required")
	}
	client := AttachFaceEnv(env)
	initialized, err := client.Call(ctx, "initialize", nil)
	if err != nil {
		return plugin.FaceResult{Status: "failed"}, fmt.Errorf("tui: initialize: %w", err)
	}
	if err := client.setCapabilities(initialized); err != nil {
		return plugin.FaceResult{Status: "failed"}, fmt.Errorf("tui: initialize capabilities: %w", err)
	}
	live := NewLive(client, LiveOptions{
		Host:           "local project",
		Title:          "VIVY CODE",
		Face:           "code",
		InitialPrompt:  f.opts.Prompt,
		ContinueNewest: f.opts.ContinueNewest,
	})
	defer live.Close()
	defer shutdownLiveRun(client, live)
	if err := view.RunWithOutput(live, f.opts.Out); err != nil {
		return plugin.FaceResult{Status: "failed"}, fmt.Errorf("tui: %w", err)
	}
	return plugin.FaceResult{Status: "completed"}, nil
}

func shutdownLiveRun(client *Client, live *Live) {
	meta := live.Meta()
	if !meta.Busy || meta.RunID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.cancelRun(ctx, meta.RunID)
	for i := 0; i < 50; i++ {
		raw, err := client.Call(ctx, "run/get", map[string]string{"run_id": meta.RunID})
		if err != nil {
			return
		}
		var payload struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(raw, &payload) == nil {
			switch payload.Status {
			case "completed", "cancelled", "failed":
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func looksTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return true
	}
	stat, err := f.Stat()
	return err != nil || stat.Mode()&os.ModeCharDevice != 0
}
