package app

import (
	"agent-vivy/internal/runtime"
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
