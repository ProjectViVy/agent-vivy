package plugin

import (
	"context"
	"encoding/json"
	"io"
)

// Face is one mouth of the species body (VIVY-FACE-PACK.md §6): a way for
// a human to open sessions, start turns, watch events, and answer approval
// and question interrupts. A face is a control-plane client — it is never
// a model tool and never hosts the kernel. The kernel FaceHost composes a
// gateway-less app, dials an in-process control plane, and hands the face
// only a FaceEnv.
type Face interface {
	// Kind names the presentation family: "web", "tui", or "headless".
	// It must match the manifest's face.kind.
	Kind() string
	// Run serves one face invocation to completion. The face draws through
	// FaceOptions writers and speaks through env. Returning nil means the
	// invocation ended cleanly (the exit code is FaceResult.Status's job);
	// returning an error is a loud failure of the face itself.
	Run(ctx context.Context, env FaceEnv) (FaceResult, error)
}

// FaceOptions is the invocation payload the launcher hands the face.
// Out is the primary output stream; Err is the diagnostics stream. The
// face must not open os.Stdout/os.Stderr itself — the launcher owns them.
type FaceOptions struct {
	Prompt         string
	ContinueNewest bool
	Out            io.Writer
	Err            io.Writer
}

// FaceResult reports how the served invocation ended. Status mirrors the
// control-plane run status vocabulary ("completed", "cancelled", ...) and
// is empty when the face ended without reaching a run.
type FaceResult struct {
	Status string
}

// FaceConstructor is the signature the generated face register exports.
// The launcher calls it once per face invocation.
type FaceConstructor func(FaceOptions) Face

// FaceEnv is the only world a face may touch. Call issues one control-plane
// request (session/create, session/list, turn/start, run/get, run/log,
// run/cancel, approval/respond, ...) and returns the raw result. OnEvent
// subscribes to server-pushed notifications; the handler receives the
// notification method and params and must not block. Missing grants fail
// closed — but in this first cut every declared face grant maps to the
// same env, and the verifier, not the runtime, arbitrates the vocabulary.
type FaceEnv interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	OnEvent(handler func(method string, params json.RawMessage))
}
