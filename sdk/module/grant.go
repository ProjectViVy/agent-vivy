package module

// CloneGrantBindings returns a deep copy of sealed effective Grant bindings.
// Runtime Hosts use copies so a Module can never mutate Assembly-owned
// constraint slices through a shared reference.
func CloneGrantBindings(bindings []GrantBinding) []GrantBinding {
	out := make([]GrantBinding, 0, len(bindings))
	for _, binding := range bindings {
		constraints := make(map[string][]string, len(binding.Constraints))
		for key, values := range binding.Constraints {
			constraints[key] = append([]string(nil), values...)
		}
		out = append(out, GrantBinding{Name: binding.Name, Constraints: constraints})
	}
	return out
}

// FindGrant returns one copied effective Grant binding. Duplicate bindings are
// invalid compiler output, so callers intentionally consume the first match
// and Host construction performs duplicate rejection at its boundary.
func FindGrant(bindings []GrantBinding, name Grant) (GrantBinding, bool) {
	for _, binding := range bindings {
		if binding.Name != name {
			continue
		}
		cloned := CloneGrantBindings([]GrantBinding{binding})
		return cloned[0], true
	}
	return GrantBinding{}, false
}

// ConstraintValues returns a copied constraint list for one key. A missing
// key is distinguishable from a present-but-empty list through the bool.
func (binding GrantBinding) ConstraintValues(key string) ([]string, bool) {
	values, ok := binding.Constraints[key]
	if !ok {
		return nil, false
	}
	return append([]string(nil), values...), true
}
