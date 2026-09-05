// Package face provides the single first-party fullscreen TUI face. Product
// entry points and packed organs delegate here instead of carrying copies of
// the controller, protocol projection, and Bubble Tea lifecycle.
package face

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"agent-vivy/sdk/plugin"
	"agent-vivy/sdk/tui/live"
	"agent-vivy/sdk/tui/view"
)

const Kind = "tui"

// New returns the canonical first-party TUI face.
func New(opts plugin.FaceOptions) plugin.Face { return &terminalFace{opts: opts} }

type terminalFace struct{ opts plugin.FaceOptions }

func (*terminalFace) Kind() string { return Kind }

func (f *terminalFace) Run(ctx context.Context, env plugin.FaceEnv) (plugin.FaceResult, error) {
	if !looksTerminal(f.opts.Out) {
		return plugin.FaceResult{Status: "failed"}, errors.New("tui: an interactive terminal is required")
	}
	controller, err := live.New(ctx, newFaceTransport(env), live.Options{
		Host:           "local project",
		Title:          "VIVY CODE",
		InitialPrompt:  f.opts.Prompt,
		ContinueNewest: f.opts.ContinueNewest,
	})
	if err != nil {
		return plugin.FaceResult{Status: "failed"}, err
	}
	defer controller.Close()
	if err := view.RunWithOutput(controller, f.opts.Out); err != nil {
		controller.Shutdown()
		return plugin.FaceResult{Status: "failed"}, fmt.Errorf("tui: %w", err)
	}
	controller.Shutdown()
	return plugin.FaceResult{Status: "completed"}, nil
}

type faceTransport struct {
	env plugin.FaceEnv

	mu     sync.RWMutex
	notify func(string, json.RawMessage)
}

func newFaceTransport(env plugin.FaceEnv) *faceTransport {
	transport := &faceTransport{env: env}
	if env != nil {
		env.OnEvent(transport.dispatch)
	}
	return transport
}

func (t *faceTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if t == nil || t.env == nil {
		return nil, errors.New("tui: client is not connected")
	}
	return t.env.Call(ctx, method, params)
}

func (t *faceTransport) OnNotify(fn func(string, json.RawMessage)) {
	t.mu.Lock()
	t.notify = fn
	t.mu.Unlock()
}

func (t *faceTransport) dispatch(method string, params json.RawMessage) {
	t.mu.RLock()
	notify := t.notify
	t.mu.RUnlock()
	if notify != nil {
		notify(method, params)
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
