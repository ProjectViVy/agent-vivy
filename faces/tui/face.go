// Package tui is the Vivy thin terminal face (VIVY-FACE-PACK.md F3): a
// fullscreen session list, streaming chat, first-class approval/question
// overlay and cancel, over the kernel control plane. It is a face organ:
// compiled into a generation via `vivy-sdk pack --face tui`, never imported
// by the kernel (§14①: the gateway generation does not see TUI deps).
package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"agent-vivy/sdk/plugin"

	"example.com/vivy/faces/tui/view"
)

// FaceKind is the manifest face kind served by this organ.
const FaceKind = "tui"

// New is the seam-face constructor the verifier requires.
func New(opts plugin.FaceOptions) plugin.Face {
	return &face{opts: opts}
}

type face struct {
	opts plugin.FaceOptions
}

func (f *face) Kind() string { return FaceKind }

func (f *face) Run(ctx context.Context, env plugin.FaceEnv) (plugin.FaceResult, error) {
	if !looksTerminal(f.opts.Out) {
		return plugin.FaceResult{Status: "failed"},
			errors.New("tui: this face needs an interactive terminal (stdout is not a tty); pipe prompts to the headless face instead")
	}
	client := newClient(env)
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
		InitialPrompt:  f.opts.Prompt,
		ContinueNewest: f.opts.ContinueNewest,
	})
	defer live.Close()

	configureColor(f.opts.Out)
	program := tea.NewProgram(view.New(live), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(f.opts.Out))
	_, programErr := program.Run()
	shutdownRun(ctx, client, live)
	if programErr != nil {
		return plugin.FaceResult{Status: "failed"}, fmt.Errorf("tui: %w", programErr)
	}
	return plugin.FaceResult{Status: "completed"}, nil
}

func configureColor(out io.Writer) {
	if os.Getenv("NO_COLOR") != "" {
		return
	}
	f, ok := out.(*os.File)
	if !ok {
		return
	}
	stat, err := f.Stat()
	if err == nil && stat.Mode()&os.ModeCharDevice != 0 {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}
}

// shutdownRun cancels a run still mid-flight when the operator quits and
// waits briefly for its terminal status so the Journal closes the run
// instead of leaving it dangling.
func shutdownRun(ctx context.Context, client *client, live *Live) {
	meta := live.Meta()
	if !meta.Busy || meta.RunID == "" {
		return
	}
	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = client.Call(cctx, "run/cancel", map[string]string{"run_id": meta.RunID})
	for i := 0; i < 50; i++ {
		raw, err := client.Call(cctx, "run/get", map[string]string{"run_id": meta.RunID})
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
		case <-cctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// looksTerminal reports whether w is (likely) an interactive terminal.
// Non-*os.File writers (tests, embedders) pass through; bubbletea renders
// the final verdict.
func looksTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return true
	}
	stat, err := f.Stat()
	if err != nil {
		return true
	}
	return stat.Mode()&os.ModeCharDevice != 0
}
