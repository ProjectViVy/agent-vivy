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
