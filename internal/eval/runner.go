package eval

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/studio"
)

const defaultProbeTimeout = 45 * time.Second

// Starter launches one air-gapped eval.
type Starter interface {
	Start(ctx context.Context, candidateID, baselineID, suite string) (domain.EvalRun, error)
}

// Runner spawns a candidate species and records the EvalRun on the live
// studio store. The candidate process never receives the production
// Journal handle. The spawn itself lives in Launch; this type only adds
// the store recording on top (the species-side surface).
type Runner struct {
	Studio     *studio.Service
	Executable string
	EvalRoot   string
	Isolation  Isolation
	Timeout    time.Duration
}

func NewRunner(r Runner) *Runner {
	if r.Timeout <= 0 {
		r.Timeout = defaultProbeTimeout
	}
	return &r
}

func (r *Runner) Start(ctx context.Context, candidateID, baselineID, suite string) (domain.EvalRun, error) {
	if r == nil || r.Studio == nil {
		return domain.EvalRun{}, errors.New("eval: runner is not configured")
	}
	if suite != SuiteAirgapProbe {
		return domain.EvalRun{}, ErrInvalidSuite
	}
	gen, err := r.Studio.GetGeneration(ctx, candidateID)
	if err != nil {
		return domain.EvalRun{}, err
	}
	exe, exeOK := CandidateExecutable(gen.SourceRef, r.Executable)
	executable := ""
	if exeOK {
		executable = exe
	}
	res, err := Launch(ctx, LaunchRequest{
		Executable: executable,
		EvalRoot:   r.EvalRoot,
		Isolation:  r.Isolation,
		Timeout:    r.Timeout,
	})
	if err != nil {
		return domain.EvalRun{}, err
	}
	return r.Studio.RecordEval(ctx, domain.EvalRun{
		ID:          res.ID,
		CandidateID: candidateID,
		BaselineID:  baselineID,
		Suite:       suite,
		Verdict:     res.Verdict,
		JournalRef:  filepath.Join(r.EvalRoot, res.ID),
	})
}
