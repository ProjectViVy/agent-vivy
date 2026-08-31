package runtime

import (
	"context"
	"errors"

	"agent-vivy/internal/domain"
)

// ErrInvalidFace is returned before a run is persisted when a caller
// supplies an unsupported face.
var ErrInvalidFace = errors.New("runtime: invalid face")

// normalizeFace defaults "" to the web face and rejects unknown values.
// The empty value keeps callers and journals written before faces existed
// meaning web.
func normalizeFace(face domain.Face) (domain.Face, error) {
	if face == "" {
		return domain.FaceWeb, nil
	}
	if face.Valid() {
		return face, nil
	}
	return "", errors.Join(ErrInvalidFace, errors.New("face must be web, tui, or code"))
}

type faceContextKey struct{}

// withFace binds the run's serving face to the run context. The prompt
// composer reads it for face framing; suspend/resume paths rebind it so
// a resumed run keeps its face.
func withFace(ctx context.Context, face domain.Face) context.Context {
	return context.WithValue(ctx, faceContextKey{}, face)
}

// runFace reports the run's serving face; runs without the binding are web.
func runFace(ctx context.Context) domain.Face {
	if face, ok := ctx.Value(faceContextKey{}).(domain.Face); ok && face != "" {
		return face
	}
	return domain.FaceWeb
}
