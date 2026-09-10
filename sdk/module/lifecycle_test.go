package module

import (
	"strings"
	"testing"
)

func TestLifecycleValidate(t *testing.T) {
	tests := []struct {
		name      string
		lifecycle Lifecycle
		wantErr   string
	}{
		{name: "generation", lifecycle: Lifecycle{Scope: ScopeGeneration}},
		{name: "instance", lifecycle: Lifecycle{Scope: ScopeInstance}},
		{name: "unknown", lifecycle: Lifecycle{Scope: Scope("request")}, wantErr: "unsupported lifecycle scope request"},
		{name: "duplicate after", lifecycle: Lifecycle{Scope: ScopeGeneration, After: []string{"example/a", "example/a"}}, wantErr: "duplicate lifecycle.after module example/a"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.lifecycle.Validate()
			if test.wantErr == "" && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}
