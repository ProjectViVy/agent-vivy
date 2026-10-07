// Package headless is the first seam-face organ (VIVY-FACE-PACK.md §6):
// `vivy run` as a packed face. It speaks only control-plane JSON-RPC
// through plugin.FaceEnv — no kernel import, no listener, no engine. The
// run loop itself lives in sdk/facerun, shared with the vivy-code
// print/json modes, so every headless face verifies the stream and fails
// blocked runs the same way.
package headless

import (
	"context"

	"agent-vivy/sdk/facerun"
	faceport "agent-vivy/sdk/port/face"
)

// FaceKind is the presentation family this organ serves.
const FaceKind = "headless"

func newRunner(opts faceport.Options) faceport.Runner {
	return &face{opts: opts}
}

type face struct {
	opts faceport.Options
}

func (f *face) Kind() string { return FaceKind }

func (f *face) Run(ctx context.Context, env faceport.Host) (faceport.Result, error) {
	return facerun.Run(ctx, env, facerun.Options{
		Prompt:         f.opts.Prompt,
		ContinueNewest: f.opts.ContinueNewest,
		Resume:         f.opts.Resume,
		SessionID:      f.opts.SessionID,
		Session:        f.opts.Session,
		Fork:           f.opts.Fork,
		SessionDir:     f.opts.SessionDir,
		NoSession:      f.opts.NoSession,
		Export:         f.opts.Export,
		Face:           FaceKind,
		Out:            f.opts.Out,
		Err:            f.opts.Err,
	}, &facerun.TextSink{Out: f.opts.Out, Err: f.opts.Err})
}
