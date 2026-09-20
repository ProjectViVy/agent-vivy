package rpccontract

import (
	"fmt"
	"sort"
	"strings"
)

// MethodBinding is one build-owned attachment to the control-plane
// dispatcher.
type MethodBinding struct {
	Method     string
	Capability string
	Handler    HandlerFunc
}

// Contribution exposes typed method bindings from a selected internal Module.
// It is not a public or runtime-discovered registration surface.
type Contribution interface {
	RPCBindings() []MethodBinding
}

// ValidateMethodBindings rejects malformed contributions before they can be
// attached to a dispatcher. coreMethods is the immutable method vocabulary
// owned by the control plane.
func ValidateMethodBindings(coreMethods map[string]struct{}, bindings []MethodBinding) error {
	type problem struct {
		method  string
		message string
	}

	seen := make(map[string]struct{}, len(bindings))
	problems := make([]problem, 0)
	for _, binding := range bindings {
		if binding.Method == "" {
			problems = append(problems, problem{message: "empty method"})
			continue
		}
		if binding.Capability == "" {
			problems = append(problems, problem{method: binding.Method, message: fmt.Sprintf("method %q has empty capability", binding.Method)})
		}
		if binding.Handler == nil {
			problems = append(problems, problem{method: binding.Method, message: fmt.Sprintf("method %q has nil handler", binding.Method)})
		}
		if _, exists := seen[binding.Method]; exists {
			problems = append(problems, problem{method: binding.Method, message: fmt.Sprintf("duplicate method %q", binding.Method)})
		} else {
			seen[binding.Method] = struct{}{}
		}
		if _, exists := coreMethods[binding.Method]; exists {
			problems = append(problems, problem{method: binding.Method, message: fmt.Sprintf("binding collides with core method %q", binding.Method)})
		}
	}
	if len(problems) == 0 {
		return nil
	}

	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].method == problems[j].method {
			return problems[i].message < problems[j].message
		}
		return problems[i].method < problems[j].method
	})
	messages := make([]string, len(problems))
	for i, item := range problems {
		messages[i] = item.message
	}
	return fmt.Errorf("rpc: invalid method bindings: %s", strings.Join(messages, "; "))
}
