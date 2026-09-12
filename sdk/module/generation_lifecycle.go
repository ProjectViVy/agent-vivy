package module

import (
	"context"
	"errors"
	"sync"
)

// Generation owns one successfully started set of Module Instances.
type Generation struct {
	mu     sync.Mutex
	owners []Instance
	closed bool
}

// StartGeneration starts and readies Instances in order. Failure rolls back
// every possibly-started Instance and closes all constructed Instances in
// reverse order while retaining every cause in the returned error chain.
func StartGeneration(ctx context.Context, owners []Instance) (*Generation, error) {
	for index, owner := range owners {
		if owner == nil {
			return nil, errors.Join(errors.New("module lifecycle: nil Instance"), rollbackGeneration(ctx, owners, index))
		}
		if err := owner.Start(ctx); err != nil {
			return nil, errors.Join(err, rollbackGeneration(ctx, owners, index+1))
		}
		if err := owner.Ready(ctx); err != nil {
			return nil, errors.Join(err, rollbackGeneration(ctx, owners, index+1))
		}
	}
	return &Generation{owners: append([]Instance(nil), owners...)}, nil
}

// Close stops and closes Instances in reverse order exactly once.
func (generation *Generation) Close(ctx context.Context) error {
	if generation == nil {
		return nil
	}
	generation.mu.Lock()
	if generation.closed {
		generation.mu.Unlock()
		return nil
	}
	generation.closed = true
	owners := generation.owners
	generation.owners = nil
	generation.mu.Unlock()

	var failures []error
	for index := len(owners) - 1; index >= 0; index-- {
		failures = append(failures, owners[index].Stop(ctx), owners[index].Close(ctx))
	}
	return errors.Join(failures...)
}

// CloseConstructed closes Instances that were constructed before a later
// constructor failed.
func CloseConstructed(ctx context.Context, owners []Instance) error {
	var failures []error
	for index := len(owners) - 1; index >= 0; index-- {
		if owners[index] != nil {
			failures = append(failures, owners[index].Close(ctx))
		}
	}
	return errors.Join(failures...)
}

func rollbackGeneration(ctx context.Context, owners []Instance, started int) error {
	var failures []error
	for index := started - 1; index >= 0; index-- {
		if owners[index] != nil {
			failures = append(failures, owners[index].Stop(ctx))
		}
	}
	failures = append(failures, CloseConstructed(ctx, owners))
	return errors.Join(failures...)
}
