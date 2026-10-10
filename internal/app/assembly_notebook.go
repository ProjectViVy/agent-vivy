package app

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	nb "agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/storage"
)

// generatedNotebookFactoryBinding is the opaque accessor emitted by the
// generated RuntimeAssembly whenever vivy/notebook-core is selected.
type generatedNotebookFactoryBinding interface {
	NotebookFactoryValue() any
}

// notebookBundleForAssembly resolves the sealed factory seam: a composition
// without vivy/notebook-core returns nil; a selected module whose emitted
// binding is missing or mistyped fails init instead of degrading to a second
// construction path.
func notebookBundleForAssembly(ctx context.Context, assembly *genassembly.RuntimeAssembly, generationID string, engine storage.Engine) (nb.Bundle, error) {
	if !assemblyHasModule(assembly.Manifest.Modules, "vivy/notebook-core") {
		return nil, nil
	}
	binding, ok := any(assembly).(generatedNotebookFactoryBinding)
	if !ok {
		return nil, fmt.Errorf("app: generated Assembly lacks the notebook factory seam")
	}
	factory, ok := binding.NotebookFactoryValue().(nb.Factory)
	if !ok || factory == nil {
		return nil, fmt.Errorf("app: generated notebook factory binding has invalid type")
	}
	bundle, err := factory(ctx, nb.FactoryInput{
		Store:        engine.Notebook(),
		Scopes:       notebookScopeResolver{engine: engine},
		GenerationID: generationID,
	})
	if err != nil {
		return nil, fmt.Errorf("app: notebook factory: %w", err)
	}
	return bundle, nil
}

// notebookScopeResolver is the trusted scope authority: Home always
// resolves; a workspace scope resolves only from a Session the storage
// registry already knows — never from a raw path or arbitrary scope ID.
type notebookScopeResolver struct {
	engine storage.Engine
}

func (r notebookScopeResolver) Home() nb.ScopeID { return nb.HomeScopeID }

func (r notebookScopeResolver) ForSession(ctx context.Context, sessionID string) (nb.ScopeID, error) {
	id := domain.SessionID(sessionID)
	if id == "" {
		return "", &nb.Error{Code: nb.CodeNotFound, Message: "notebook scope requires a session"}
	}
	if _, err := r.engine.GetSession(ctx, id); err != nil {
		return "", &nb.Error{Code: nb.CodeNotFound, Message: "notebook scope requires a known session"}
	}
	return nb.WorkspaceScope(string(id)), nil
}
