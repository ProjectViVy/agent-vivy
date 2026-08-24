package runtime

import "context"

type selectedToolsContextKey struct{}

// withSelectedTools binds the request-scoped callable surface to the engine
// context. An explicit empty set means that no tool may execute; absence of
// the value keeps direct low-level engine tests backwards compatible.
func withSelectedTools(ctx context.Context, names []string) context.Context {
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	return context.WithValue(ctx, selectedToolsContextKey{}, allowed)
}

func selectedToolSet(ctx context.Context) (map[string]struct{}, bool) {
	allowed, ok := ctx.Value(selectedToolsContextKey{}).(map[string]struct{})
	return allowed, ok
}
