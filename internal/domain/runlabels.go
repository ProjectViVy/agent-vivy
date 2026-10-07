package domain

import "context"

// RunLabels carries the provider/model identity of the serving route into
// run-scoped contexts so runtime surfaces (e.g. spawned-process env) can
// report which model the run belongs to without re-reading service state.
type RunLabels struct {
	Provider string
	Model    string
}

type runLabelsContextKey struct{}

// WithRunLabels returns a context carrying the run's provider/model labels.
func WithRunLabels(ctx context.Context, labels RunLabels) context.Context {
	return context.WithValue(ctx, runLabelsContextKey{}, labels)
}

// RunLabelsFromContext reads the run's labels; empty fields when absent.
func RunLabelsFromContext(ctx context.Context) RunLabels {
	labels, _ := ctx.Value(runLabelsContextKey{}).(RunLabels)
	return labels
}
