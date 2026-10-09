package reportcontract

import (
	"context"

	nb "agent-vivy/internal/notebookcontract"
)

// Service is the sealed report capability owned by the vivy/reports module.
// R0 exposes only trusted admission; generation, get/cancel and settings
// arrive with the module's actions in R1+.
type Service interface {
	// StartReport admits one trusted root workflow Run. The control Session
	// is resolved or created inside the scope's admission namespace; the
	// same operation key rejoins its committed Run.
	StartReport(context.Context, AdmissionContext, ReportRequest) (ReportAdmission, error)
}

// AdmissionPort is the narrow bridge the composition binds to the common
// runtime Service admission authority. The module never constructs Runs,
// locks, or revisions itself.
type AdmissionPort interface {
	StartReport(context.Context, AdmissionContext, ReportRequest) (ReportAdmission, error)
}

// Factory is emitted into a generated RuntimeAssembly only when the
// composition selected vivy/reports. App unwraps the typed value through
// the generated ReportFactoryValue accessor and invokes it once.
type Factory func(context.Context, FactoryInput) (Bundle, error)

// FactoryInput carries the R0 construction seams. The admission bridge is
// bound after the runtime Service exists via Bundle.AttachAdmission.
type FactoryInput struct {
	// Scopes resolves Home or the canonical workspace scope of a
	// server-authorized Session; workspace scopes double as report
	// namespaces.
	Scopes ScopeResolver
	// GenerationID is the sealed Generation identity.
	GenerationID string
}

// ScopeResolver mirrors the notebook scope authority: a report may only
// target a scope the host resolved, never a raw scope string from JSON.
type ScopeResolver interface {
	Home() nb.ScopeID
	ForSession(ctx context.Context, sessionID string) (nb.ScopeID, error)
}

// Bundle is what the factory hands the composition.
type Bundle interface {
	Service() Service
	// AttachAdmission binds the runtime admission authority once it exists;
	// calling Service before attach returns capability_unavailable, never a
	// partial admission.
	AttachAdmission(AdmissionPort)
	Close() error
}
