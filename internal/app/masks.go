package app

import (
	"context"
	"fmt"
	"strings"

	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/moduleport"
	"agent-vivy/internal/storage"
)

// generatedMaskFactoryBinding is emitted by the runtime generator. Its nil
// value in a maskless Assembly keeps the app independent from the optional
// internal/moduleport import while selected builds still expose the typed
// factory.
type generatedMaskFactoryBinding interface {
	MaskServiceFactoryValue() any
}

// primaryAdmissionForComposition selects the primary-run persistence seam.
// Every composition with a proven identity — a packed build's sealed
// Generation identity or the headless default composition's declared
// HeadlessGenerationID — must carry the atomic Core Storage extension; a
// legacy downgrade would silently skip the prompt snapshot contract. The
// legacy sequential branch is reserved for custom embedder Assemblies that
// declare no identity at all.
func primaryAdmissionForComposition(backend any, generationID string, sealed bool) (storage.RunAdmissionStore, error) {
	generationID = strings.TrimSpace(generationID)
	// An unidentified development/test embedder deliberately stays on the
	// legacy sequential path. Core Storage may already implement RunAdmissionStore,
	// but without a proven Generation identity it cannot produce a valid
	// immutable prompt snapshot, so do not opt into that seam accidentally.
	if !sealed {
		return nil, nil
	}
	if generationID == "" {
		return nil, fmt.Errorf("app: sealed first-party composition requires a Generation identity for primary admission")
	}
	admission, ok := backend.(storage.RunAdmissionStore)
	if ok {
		return admission, nil
	}
	return nil, fmt.Errorf("app: sealed first-party composition requires Core Storage RunAdmissionStore")
}

// maskManagerForAssembly wires the selected mask capability. The headless
// default composition carries a declared identity (HeadlessGenerationID) in
// its generated Assembly, so masks arm exactly as in a packed build; a
// sealed composition that selected vivy/masks without proving its identity
// still fails closed.
func maskManagerForAssembly(ctx context.Context, assembly genassembly.RuntimeAssembly, backend storage.Engine, generationID string, sealed bool) (maskcontract.Service, error) {
	if !assemblyHasModule(assembly.Manifest.Modules, "vivy/masks") {
		return nil, nil
	}
	if !sealed {
		return nil, nil
	}
	if generationID == "" {
		return nil, fmt.Errorf("app: selected vivy/masks requires a sealed generation identity")
	}
	binding, ok := any(&assembly).(generatedMaskFactoryBinding)
	if !ok {
		return nil, fmt.Errorf("app: selected vivy/masks Assembly has no typed factory accessor")
	}
	factory, ok := binding.MaskServiceFactoryValue().(moduleport.MaskFactory)
	if !ok || factory == nil {
		return nil, fmt.Errorf("app: selected vivy/masks Assembly has no typed MaskFactory")
	}
	maskStore, ok := backend.(storage.MaskStore)
	if !ok {
		return nil, fmt.Errorf("app: selected vivy/masks requires Core Storage MaskStore")
	}
	// MASK-3 owns admission and prompt snapshot persistence. Do not expose
	// MASK-2 actions in a production composition until that second capability
	// is present, otherwise a successful selection could not be admitted into
	// an immutable run snapshot.
	if _, ok := backend.(storage.RunAdmissionStore); !ok {
		return nil, fmt.Errorf("app: selected vivy/masks requires Core Storage RunAdmissionStore")
	}
	service, err := factory(ctx, moduleport.MaskDependencies{Store: maskStore, GenerationID: generationID})
	if err != nil {
		return nil, fmt.Errorf("app: construct mask service: %w", err)
	}
	if service == nil {
		return nil, fmt.Errorf("app: mask factory returned a nil service")
	}
	return service, nil
}
