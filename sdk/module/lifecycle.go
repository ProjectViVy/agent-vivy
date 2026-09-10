package module

import "fmt"

type Scope string

const (
	ScopeGeneration Scope = "generation"
	ScopeInstance   Scope = "instance"
)

type Lifecycle struct {
	Scope Scope    `json:"scope" yaml:"scope"`
	After []string `json:"after,omitempty" yaml:"after,omitempty"`
}

func (lifecycle Lifecycle) Validate() error {
	switch lifecycle.Scope {
	case ScopeGeneration, ScopeInstance:
	default:
		return fmt.Errorf("unsupported lifecycle scope %s", lifecycle.Scope)
	}

	seen := make(map[string]struct{}, len(lifecycle.After))
	for _, moduleID := range lifecycle.After {
		if !moduleIDPattern.MatchString(moduleID) {
			return fmt.Errorf("invalid lifecycle.after module %q", moduleID)
		}
		if _, exists := seen[moduleID]; exists {
			return fmt.Errorf("duplicate lifecycle.after module %s", moduleID)
		}
		seen[moduleID] = struct{}{}
	}
	return nil
}
