package app

import (
	"agent-vivy/internal/runtime"
	"fmt"
)

type generatedNudgeFactoryBinding interface{ NudgeFactoryValue() any }

// nudgeFactoryForAssembly unwraps metadata only; construction belongs to the
// future core-to-plugin cutover, never to this inert seam.
func nudgeFactoryForAssembly(assembly any) (runtime.NudgeFactory, error) {
	binding, ok := assembly.(generatedNudgeFactoryBinding)
	if !ok {
		return nil, fmt.Errorf("app: generated Assembly lacks the nudge factory seam")
	}
	value := binding.NudgeFactoryValue()
	if value == nil {
		return nil, nil
	}
	factory, ok := value.(runtime.NudgeFactory)
	if !ok || factory == nil {
		return nil, fmt.Errorf("app: invalid nudge factory binding")
	}
	return factory, nil
}
