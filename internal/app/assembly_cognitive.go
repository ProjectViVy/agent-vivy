package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"
)

// generatedCognitiveFactoryBinding is the opaque accessor emitted by the
// generated RuntimeAssembly whenever vivy/diva-cognitive is selected.
type generatedCognitiveFactoryBinding interface {
	CognitiveFactoryValue() any
}

// cognitiveBundleForAssembly resolves the sealed factory seam: a composition
// without vivy/diva-cognitive returns nil; a selected module whose emitted
// binding is missing or mistyped fails init instead of degrading to a second
// construction path.
func cognitiveBundleForAssembly(ctx context.Context, assembly *genassembly.RuntimeAssembly, cfg config.Config, generationID string, store storage.SnapshotStore) (cognitivecontract.Bundle, error) {
	if !assemblyHasModule(assembly.Manifest.Modules, "vivy/diva-cognitive") {
		return nil, nil
	}
	binding, ok := any(assembly).(generatedCognitiveFactoryBinding)
	if !ok {
		return nil, fmt.Errorf("app: generated Assembly lacks the cognitive factory seam")
	}
	factory, ok := binding.CognitiveFactoryValue().(cognitivecontract.Factory)
	if !ok || factory == nil {
		return nil, fmt.Errorf("app: generated cognitive factory binding has invalid type")
	}
	// Config paths may be relative to the process working directory. The
	// owner requires an absolute root; resolve it before crossing its seam.
	dataRoot, err := filepath.Abs(cfg.DataDirectory())
	if err != nil {
		return nil, fmt.Errorf("app: resolve cognitive data directory: %w", err)
	}
	cfg.Storage.DataDir = dataRoot
	bundle, err := factory(ctx, cognitivecontract.FactoryInput{
		Config:       cfg,
		GenerationID: generationID,
		Store:        store,
	})
	if err != nil {
		return nil, fmt.Errorf("app: cognitive factory: %w", err)
	}
	return bundle, nil
}

// lazyDomain resolves the bound evolution Domain on first use so a
// selected-but-unavailable memory writer fails the affected effect, not the
// whole composition. The resolution is memoized.
type lazyDomain struct {
	binding laputaevolution.RunBinding
	bundle  cognitivecontract.Bundle
	once    sync.Once
	domain  laputaevolution.Domain
	err     error
}

func (d *lazyDomain) bound(ctx context.Context) (laputaevolution.Domain, error) {
	d.once.Do(func() {
		d.domain, d.err = d.bundle.BoundDomain(ctx, d.binding)
	})
	return d.domain, d.err
}

func (d *lazyDomain) Collect(ctx context.Context, window laputaevolution.Window) (laputaevolution.EvidenceBatch, error) {
	bound, err := d.bound(ctx)
	if err != nil {
		return laputaevolution.EvidenceBatch{}, err
	}
	return bound.Collect(ctx, window)
}

func (d *lazyDomain) Apply(ctx context.Context, effect laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	bound, err := d.bound(ctx)
	if err != nil {
		return laputaevolution.EffectReceipt{}, err
	}
	return bound.Apply(ctx, effect)
}

func (d *lazyDomain) ApplyAtMissionRevision(ctx context.Context, revision uint64, effect laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	bound, err := d.bound(ctx)
	if err != nil {
		return laputaevolution.EffectReceipt{}, err
	}
	if guarded, ok := bound.(interface {
		ApplyAtMissionRevision(context.Context, uint64, laputaevolution.Effect) (laputaevolution.EffectReceipt, error)
	}); ok {
		return guarded.ApplyAtMissionRevision(ctx, revision, effect)
	}
	return laputaevolution.EffectReceipt{}, fmt.Errorf("app: atomic Mission apply gate unavailable")
}

func (d *lazyDomain) Lookup(ctx context.Context, operationID string) (laputaevolution.EffectReceipt, error) {
	bound, err := d.bound(ctx)
	if err != nil {
		return laputaevolution.EffectReceipt{}, err
	}
	return bound.Lookup(ctx, operationID)
}

// cognitiveControlPort maps the bound bundle's armed control cell onto the
// runtime service callbacks. It is constructed once per composition and
// attached to the bundle after Service construction (single-use).
type cognitiveControlPort struct {
	svc    *runtime.Service
	bundle cognitivecontract.Bundle
}

func (c *cognitiveControlPort) GetState(ctx context.Context) (cognitivecontract.ControlState, error) {
	if c.svc == nil {
		return cognitivecontract.ControlState{}, runtime.ErrCognitiveUnavailable
	}
	state, watermark, err := c.svc.CognitiveStatus(ctx)
	if err != nil {
		return cognitivecontract.ControlState{}, err
	}
	policy, revision, err := c.svc.CognitivePolicyState(ctx)
	if err != nil {
		return cognitivecontract.ControlState{}, err
	}
	return cognitivecontract.ControlState{
		Enabled:        policy.Enabled,
		MinIntervalMS:  policy.MinIntervalMS,
		PolicyRevision: revision,
		ActiveRunID:    state.ActiveRunID,
		SourceID:       c.bundle.SourceID(),
		Watermark:      watermark,
	}, nil
}

func (c *cognitiveControlPort) SetPolicyCAS(ctx context.Context, policy laputaevolution.TriggerPolicy, baseRevision uint64) (cognitivecontract.ControlState, error) {
	if c.svc == nil {
		return cognitivecontract.ControlState{}, runtime.ErrCognitiveUnavailable
	}
	if err := c.svc.UpdateCognitivePolicyCAS(ctx, policy, baseRevision); err != nil {
		return cognitivecontract.ControlState{}, err
	}
	return c.GetState(ctx)
}

func (c *cognitiveControlPort) Trigger(ctx context.Context) (cognitivecontract.ControlState, error) {
	if c.svc == nil {
		return cognitivecontract.ControlState{}, runtime.ErrCognitiveUnavailable
	}
	elig, err := c.svc.TriggerCognitive(ctx)
	if err != nil {
		return cognitivecontract.ControlState{}, err
	}
	state, getErr := c.GetState(ctx)
	if getErr != nil {
		return cognitivecontract.ControlState{}, getErr
	}
	state.Eligibility = &elig
	return state, nil
}

func (c *cognitiveControlPort) Cancel(ctx context.Context, runID domain.RunID) (cognitivecontract.ControlState, error) {
	if c.svc == nil {
		return cognitivecontract.ControlState{}, runtime.ErrCognitiveUnavailable
	}
	cancelled, err := c.svc.CancelRun(ctx, runID)
	if err != nil {
		return cognitivecontract.ControlState{}, err
	}
	state, getErr := c.GetState(ctx)
	if getErr != nil {
		return cognitivecontract.ControlState{}, getErr
	}
	if !cancelled {
		state.BlockReason = "run_not_active"
	}
	return state, nil
}

// The per-module ActionHost wrapper that hands the armed dispatcher to the
// generated providers lands with DN-4C (same pattern as actionhost's
// newMaskActionHost); until then every cognitive action fails closed at its
// cognitivecontract.ActionHost type assertion.
