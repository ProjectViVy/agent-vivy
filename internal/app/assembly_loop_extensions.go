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

type generatedWarmingFactoryBinding interface{ WarmingFactoryValue() any }
type generatedSteeringFactoryBinding interface{ SteeringFactoryValue() any }

func warmingFactoryForAssembly(assembly any) (runtime.WarmingFactory, error) {
	binding, ok := assembly.(generatedWarmingFactoryBinding)
	if !ok {
		return nil, fmt.Errorf("app: generated Assembly lacks the warming factory seam")
	}
	value := binding.WarmingFactoryValue()
	if value == nil {
		return nil, nil
	}
	factory, ok := value.(runtime.WarmingFactory)
	if !ok || factory == nil {
		return nil, fmt.Errorf("app: invalid warming factory binding")
	}
	return factory, nil
}

func steeringFactoryForAssembly(assembly any) (runtime.SteeringFactory, error) {
	binding, ok := assembly.(generatedSteeringFactoryBinding)
	if !ok {
		return nil, fmt.Errorf("app: generated Assembly lacks the steering factory seam")
	}
	value := binding.SteeringFactoryValue()
	if value == nil {
		return nil, nil
	}
	factory, ok := value.(runtime.SteeringFactory)
	if !ok || factory == nil {
		return nil, fmt.Errorf("app: invalid steering factory binding")
	}
	return factory, nil
}

// applyLoopFactoryBindings is shared by startup and configuration rebuilds.
// Resolve every immutable Assembly value before changing the config, and never
// invoke a factory during either composition path.
func applyLoopFactoryBindings(cfg *runtime.EngineConfig, assembly any) error {
	nudge, err := nudgeFactoryForAssembly(assembly)
	if err != nil {
		return err
	}
	warming, err := warmingFactoryForAssembly(assembly)
	if err != nil {
		return err
	}
	steering, err := steeringFactoryForAssembly(assembly)
	if err != nil {
		return err
	}
	cfg.NudgeFactory, cfg.WarmingFactory, cfg.SteeringFactory = nudge, warming, steering
	return nil
}
