package domain

import "context"

type modelOutputLimitKey struct{}

// WithModelOutputLimit carries a trusted runtime request limit, independent of
// physical model metadata. Faces and RunOptions do not select this limit.
func WithModelOutputLimit(ctx context.Context, tokens int) context.Context {
	return context.WithValue(ctx, modelOutputLimitKey{}, tokens)
}
func ModelOutputLimitFromContext(ctx context.Context) int {
	n, _ := ctx.Value(modelOutputLimitKey{}).(int)
	return n
}

type modelRequestScopeKey struct{}

// ModelRequestScope pins the trusted admitted route. It is not a provider
// registry and cannot select another credential, endpoint or model.
type ModelRequestScope struct{ Provider, Model string }

func WithModelRequestScope(ctx context.Context, provider, model string) context.Context {
	return context.WithValue(ctx, modelRequestScopeKey{}, ModelRequestScope{Provider: provider, Model: model})
}
func ModelRequestScopeFromContext(ctx context.Context) (ModelRequestScope, bool) {
	scope, ok := ctx.Value(modelRequestScopeKey{}).(ModelRequestScope)
	return scope, ok
}
