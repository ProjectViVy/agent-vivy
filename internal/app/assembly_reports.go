package app

import (
	"context"
	"fmt"

	genassembly "agent-vivy/internal/generated/assembly"
	rc "agent-vivy/internal/reportcontract"
)

// generatedReportFactoryBinding is the opaque accessor emitted by the
// generated RuntimeAssembly whenever vivy/reports is selected.
type generatedReportFactoryBinding interface {
	ReportFactoryValue() any
}

// reportsBundleForAssembly resolves the sealed factory seam: a composition
// without vivy/reports returns nil; a selected module whose emitted binding
// is missing or mistyped fails init. The bundle's admission bridge is
// attached after the runtime Service exists via AttachAdmission.
func reportsBundleForAssembly(ctx context.Context, assembly *genassembly.RuntimeAssembly, generationID string, scopes rc.ScopeResolver) (rc.Bundle, error) {
	if !assemblyHasModule(assembly.Manifest.Modules, "vivy/reports") {
		return nil, nil
	}
	binding, ok := any(assembly).(generatedReportFactoryBinding)
	if !ok {
		return nil, fmt.Errorf("app: generated Assembly lacks the report factory seam")
	}
	factory, ok := binding.ReportFactoryValue().(rc.Factory)
	if !ok || factory == nil {
		return nil, fmt.Errorf("app: generated report factory binding has invalid type")
	}
	bundle, err := factory(ctx, rc.FactoryInput{Scopes: scopes, GenerationID: generationID})
	if err != nil {
		return nil, fmt.Errorf("app: report factory: %w", err)
	}
	return bundle, nil
}
