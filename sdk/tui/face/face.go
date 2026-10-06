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

	"agent-vivy/sdk/facerun"
	faceport "agent-vivy/sdk/port/face"
	"agent-vivy/sdk/tui/live"
	"agent-vivy/sdk/tui/surface"
	"agent-vivy/sdk/tui/view"
)

const Kind = "tui"

// New returns the canonical first-party TUI face.
func New(opts faceport.Options) faceport.Runner { return &terminalFace{opts: opts} }

type terminalFace struct {
	opts    faceport.Options
	runView func(surface.Driver, io.Writer, ...view.Options) error
}

func (*terminalFace) Kind() string { return Kind }

func (f *terminalFace) Run(ctx context.Context, env faceport.Host) (faceport.Result, error) {
	switch f.opts.Mode {
	case "", "text":
		// interactive path below
	case "print":
		return facerun.Run(ctx, env, f.headlessOptions(), &facerun.TextSink{Out: f.opts.Out, Err: f.opts.Err})
	case "json":
		return facerun.Run(ctx, env, f.headlessOptions(), facerun.JSONLSink{Out: f.opts.Out})
	default:
		return faceport.Result{Status: "failed"}, faceport.ModeUnavailableError{Mode: f.opts.Mode}
	}
	if !looksTerminal(f.opts.Out) {
		return faceport.Result{Status: "failed"}, errors.New("tui: an interactive terminal is required")
	}
	controller, err := live.New(ctx, newFaceTransport(env), live.Options{
		Host:           "local project",
		Title:          "VIVY CODE",
		InitialPrompt:  f.opts.Prompt,
		ContinueNewest: f.opts.ContinueNewest,
	})
	if err != nil {
		return faceport.Result{Status: "failed"}, err
	}
	defer controller.Close()
	runView := f.runView
	if runView == nil {
		runView = view.RunWithOutput
	}
	if err := runView(controller, f.opts.Out, view.Options{DebugToolOutput: f.opts.DebugToolOutput, Locale: controller.Locale()}); err != nil {
		controller.Shutdown()
		return faceport.Result{Status: "failed"}, fmt.Errorf("tui: %w", err)
	}
	controller.Shutdown()
	return faceport.Result{Status: "completed"}, nil
}

// headlessOptions maps the face launch surface onto the shared non-interactive
// engine used by print and json modes.
func (f *terminalFace) headlessOptions() facerun.Options {
	return facerun.Options{
		Prompt:         f.opts.Prompt,
		ContinueNewest: f.opts.ContinueNewest,
		Resume:         f.opts.Resume,
		SessionID:      f.opts.SessionID,
		Session:        f.opts.Session,
		Fork:           f.opts.Fork,
		SessionDir:     f.opts.SessionDir,
		NoSession:      f.opts.NoSession,
		Export:         f.opts.Export,
		Face:           "code",
		Out:            f.opts.Out,
		Err:            f.opts.Err,
	}
}

type faceTransport struct {
	env faceport.Host

	mu     sync.RWMutex
	notify func(string, json.RawMessage)
}

func newFaceTransport(env faceport.Host) *faceTransport {
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
