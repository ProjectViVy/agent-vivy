package domain

// AssemblyRecipe is the named pack bill for one generation. First-party
// units keep their real names; only the user layer is called plugins.
type AssemblyRecipe struct {
	Loop      string   `json:"loop,omitempty"`
	World     string   `json:"world,omitempty"`
	Providers []string `json:"providers,omitempty"`
	Tools     []string `json:"tools,omitempty"`
	Plugins   []string `json:"plugins,omitempty"`
}

type GenerationPhase string

const (
	GenerationBuilt       GenerationPhase = "built"
	GenerationEvalPending GenerationPhase = "eval_pending"
	GenerationEvaluated   GenerationPhase = "evaluated"
	GenerationPromoted    GenerationPhase = "promoted"
	GenerationReleased    GenerationPhase = "released"
	GenerationRejected    GenerationPhase = "rejected"
)

func (p GenerationPhase) Valid() bool {
	switch p {
	case GenerationBuilt, GenerationEvalPending, GenerationEvaluated, GenerationPromoted, GenerationReleased, GenerationRejected:
		return true
	}
	return false
}

// Generation is one packed species body.
type Generation struct {
	ID             string
	ParentID       string
	ArtifactSHA256 string
	SourceRef      string
	Recipe         AssemblyRecipe
	Phase          GenerationPhase
	CreatedAt      int64
}

type EvalVerdict string

const (
	EvalBetter      EvalVerdict = "better"
	EvalWorse       EvalVerdict = "worse"
	EvalMixed       EvalVerdict = "mixed"
	EvalFailedToRun EvalVerdict = "failed_to_run"
)

func (v EvalVerdict) Valid() bool {
	switch v {
	case EvalBetter, EvalWorse, EvalMixed, EvalFailedToRun:
		return true
	}
	return false
}

// EvalRun is one comparison of a candidate generation against a baseline.
type EvalRun struct {
	ID          string
	CandidateID string
	BaselineID  string
	Suite       string
	Verdict     EvalVerdict
	JournalRef  string
	CreatedAt   int64
}

type PromotionPhase string

const (
	PromotionAccepted PromotionPhase = "accepted"
)

func (p PromotionPhase) Valid() bool {
	return p == PromotionAccepted
}

// Promotion is a human-gated next-launch switch from one generation to another.
type Promotion struct {
	ID        string
	FromID    string
	ToID      string
	EvalID    string
	Actor     string
	Phase     PromotionPhase
	AppliesAt string
	CreatedAt int64
}

const PromotionAppliesNextLaunch = "next_launch"
const PromotionActorHuman = "human"

// Worktree is the Studio-owned source tree the engine is pinned to.
type Worktree struct {
	ID        string
	Path      string
	Kind      string // plugin | first-party | kernel
	Dirty     bool
	CreatedAt int64
}

// Release is a human-accepted generation. It is not an install: the daily
// location changes only when an Install is recorded.
type Release struct {
	ID           string
	GenerationID string
	EvalID       string
	Actor        string
	Phase        ReleasePhase
	CreatedAt    int64
}

type ReleasePhase string

const (
	ReleaseAccepted ReleasePhase = "accepted"
)

func (p ReleasePhase) Valid() bool {
	return p == ReleaseAccepted
}

// Install is one write into the daily install location. Rollback is
// expressed as a new Install row whose phase is rolled_back.
type Install struct {
	ID        string
	ReleaseID string
	Target    string
	Phase     InstallPhase
	CreatedAt int64
}

type InstallPhase string

const (
	InstallCurrent    InstallPhase = "current"
	InstallRolledBack InstallPhase = "rolled_back"
)

func (p InstallPhase) Valid() bool {
	switch p {
	case InstallCurrent, InstallRolledBack:
		return true
	}
	return false
}

type StudioEventType string

const (
	StudioGenerationCreated  StudioEventType = "generation.created"
	StudioEvalRunRecorded    StudioEventType = "evalrun.recorded"
	StudioPromotionAccepted  StudioEventType = "promotion.accepted"
	StudioGenerationRejected StudioEventType = "generation.rejected"
	StudioReleaseAccepted    StudioEventType = "release.accepted"
	StudioInstallRecorded    StudioEventType = "install.recorded"
	StudioInstallRolledBack  StudioEventType = "install.rolled_back"
	StudioWorktreePinned     StudioEventType = "worktree.pinned"
)

func (t StudioEventType) Valid() bool {
	switch t {
	case StudioGenerationCreated, StudioEvalRunRecorded, StudioPromotionAccepted, StudioGenerationRejected,
		StudioReleaseAccepted, StudioInstallRecorded, StudioInstallRolledBack, StudioWorktreePinned:
		return true
	}
	return false
}

// StudioEvent is one append-only studio-plane fact. It is not a RunEvent.
type StudioEvent struct {
	Seq       int64
	Type      StudioEventType
	ObjectID  string
	CreatedAt int64
	Payload   []byte
}
