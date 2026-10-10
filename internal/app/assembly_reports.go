package app

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
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

// reportScopeResolver maps the authenticated session identity onto the
// canonical workspace scope (agent-origin runs) and Home (human-origin
// actions) — mirroring the notebook resolver on the same backend authority.
type reportScopeResolver struct {
	engine storage.Engine
}

func (r reportScopeResolver) Home() nb.ScopeID { return nb.HomeScopeID }

func (r reportScopeResolver) ForSession(ctx context.Context, sessionID string) (nb.ScopeID, error) {
	id := domain.SessionID(sessionID)
	if id == "" {
		return "", &rc.Error{Code: rc.CodeNotFound, Message: "report scope requires a session"}
	}
	if _, err := r.engine.GetSession(ctx, id); err != nil {
		return "", &rc.Error{Code: rc.CodeNotFound, Message: "report scope requires a known session"}
	}
	return nb.WorkspaceScope(string(id)), nil
}
