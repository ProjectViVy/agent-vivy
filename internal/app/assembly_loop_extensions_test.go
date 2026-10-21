package app

import (
	"agent-vivy/internal/runtime"
	"reflect"
	"testing"
)

type nudgeFactoryFixture struct{ value any }

func (f nudgeFactoryFixture) NudgeFactoryValue() any { return f.value }

func TestNudgeFactoryBindingIsLazyAndTyped(t *testing.T) {
	called := false
	factory := runtime.NudgeFactory(func() runtime.NudgeExtension { called = true; return nil })
	got, err := nudgeFactoryForAssembly(nudgeFactoryFixture{value: factory})
	if err != nil || got == nil || called {
		t.Fatalf("lazy binding changed: %v, constructed=%v", err, called)
	}
	if got, err := nudgeFactoryForAssembly(nudgeFactoryFixture{}); err != nil || got != nil {
		t.Fatalf("absent binding: %v", err)
	}
	for _, value := range []any{struct{}{}, nudgeFactoryFixture{value: "wrong"}, nudgeFactoryFixture{value: runtime.NudgeFactory(nil)}} {
		if _, err := nudgeFactoryForAssembly(value); err == nil {
			t.Fatalf("invalid binding accepted: %T", value)
		}
	}
}

type maintenanceFactoryFixture struct{ warming, steering any }

func (f maintenanceFactoryFixture) WarmingFactoryValue() any  { return f.warming }
func (f maintenanceFactoryFixture) SteeringFactoryValue() any { return f.steering }

func TestWarmingSteeringFactoryBindingsAreLazyAndTyped(t *testing.T) {
	called := false
	warming := runtime.WarmingFactory(func(runtime.MaintenanceHost) runtime.ModelSettlement { called = true; return nil })
	steering := runtime.SteeringFactory(func(runtime.SteeringHost) runtime.SteeringController { called = true; return nil })
	fixture := maintenanceFactoryFixture{warming, steering}
	w, err := warmingFactoryForAssembly(fixture)
	if err != nil || w == nil {
		t.Fatalf("warming: %v", err)
	}
	s, err := steeringFactoryForAssembly(fixture)
	if err != nil || s == nil {
		t.Fatalf("steering: %v", err)
	}
	if called {
		t.Fatal("factory was constructed during composition")
	}
	if w, err := warmingFactoryForAssembly(maintenanceFactoryFixture{}); err != nil || w != nil {
		t.Fatalf("absent warming: %v", err)
	}
	if s, err := steeringFactoryForAssembly(maintenanceFactoryFixture{}); err != nil || s != nil {
		t.Fatalf("absent steering: %v", err)
	}
	for _, bad := range []any{struct{}{}, maintenanceFactoryFixture{"wrong", "wrong"}, maintenanceFactoryFixture{runtime.WarmingFactory(nil), runtime.SteeringFactory(nil)}} {
		if _, err := warmingFactoryForAssembly(bad); err == nil {
			t.Fatal("invalid warming accepted")
		}
		if _, err := steeringFactoryForAssembly(bad); err == nil {
			t.Fatal("invalid steering accepted")
		}
	}
}

type loopFactoryFixture struct {
	nudgeFactoryFixture
	maintenanceFactoryFixture
}

func TestLoopFactoryBindingsSurviveConfigRebuild(t *testing.T) {
	called := false
	nudge := runtime.NudgeFactory(func() runtime.NudgeExtension { called = true; return nil })
	warming := runtime.WarmingFactory(func(runtime.MaintenanceHost) runtime.ModelSettlement { called = true; return nil })
	steering := runtime.SteeringFactory(func(runtime.SteeringHost) runtime.SteeringController { called = true; return nil })
	assembly := loopFactoryFixture{nudgeFactoryFixture{nudge}, maintenanceFactoryFixture{warming, steering}}
	for _, contextBytes := range []int{123, 456} {
		cfg := runtime.EngineConfig{MaxContextBytes: contextBytes}
		if err := applyLoopFactoryBindings(&cfg, assembly); err != nil {
			t.Fatal(err)
		}
		if cfg.MaxContextBytes != contextBytes {
			t.Fatal("factory metadata overwrote rebuilt settings")
		}
		for _, pair := range [][2]any{{cfg.NudgeFactory, nudge}, {cfg.WarmingFactory, warming}, {cfg.SteeringFactory, steering}} {
			if reflect.ValueOf(pair[0]).Pointer() != reflect.ValueOf(pair[1]).Pointer() {
				t.Fatal("rebuild lost immutable selected factory")
			}
		}
	}
	if called {
		t.Fatal("rebuild constructed an inert factory")
	}
}
