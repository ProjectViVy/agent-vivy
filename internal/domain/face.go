package domain

// Face attributes one run to the entry assembly serving it. It is per-run
// metadata (like RunMode), not a process mode: one serving process runs
// any face. The base tool surface stays mainline-shared across faces
// (VC-0 D1 ratification); the face steers prompt framing and future
// face-scoped capabilities, never a hidden tool narrowing.
type Face string

const (
	FaceWeb      Face = "web"
	FaceTui      Face = "tui"
	FaceCode     Face = "code"
	FaceHeadless Face = "headless"
)

// Valid reports whether the face is supported by the current harness.
func (f Face) Valid() bool {
	return f == FaceWeb || f == FaceTui || f == FaceCode || f == FaceHeadless
}
