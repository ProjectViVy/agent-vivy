package app

import (
	"context"
	"errors"
	"time"
)

type startupCleanup struct {
	timeout     time.Duration
	steps       []func(context.Context) error
	transferred bool
	ran         bool
}

func newStartupCleanup(timeout time.Duration) *startupCleanup {
	return &startupCleanup{timeout: timeout}
}

func (cleanup *startupCleanup) Add(step func(context.Context) error) {
	if cleanup == nil || step == nil || cleanup.transferred || cleanup.ran {
		return
	}
	cleanup.steps = append(cleanup.steps, step)
}

func (cleanup *startupCleanup) Transfer() {
	if cleanup != nil {
		cleanup.transferred = true
	}
}

func (cleanup *startupCleanup) Run() error {
	if cleanup == nil || cleanup.transferred || cleanup.ran {
		return nil
	}
	cleanup.ran = true
	ctx, cancel := context.WithTimeout(context.Background(), cleanup.timeout)
	defer cancel()
	var cleanupErr error
	for index := len(cleanup.steps) - 1; index >= 0; index-- {
		cleanupErr = errors.Join(cleanupErr, cleanup.steps[index](ctx))
	}
	return cleanupErr
}
