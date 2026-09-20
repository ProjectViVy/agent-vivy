package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestStartupCleanupRunsReverseOnceAndJoinsErrors(t *testing.T) {
	firstErr := errors.New("first cleanup")
	lastErr := errors.New("last cleanup")
	var events []string
	cleanup := newStartupCleanup(time.Second)
	cleanup.Add(func(context.Context) error {
		events = append(events, "first")
		return firstErr
	})
	cleanup.Add(func(context.Context) error {
		events = append(events, "last")
		return lastErr
	})

	err := cleanup.Run()
	if !errors.Is(err, firstErr) || !errors.Is(err, lastErr) {
		t.Fatalf("cleanup error = %v", err)
	}
	if want := []string{"last", "first"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if err := cleanup.Run(); err != nil || !reflect.DeepEqual(events, []string{"last", "first"}) {
		t.Fatalf("second Run() error = %v, events = %v", err, events)
	}
}

func TestStartupCleanupUsesOneBoundedContextForEveryStep(t *testing.T) {
	var events []string
	cleanup := newStartupCleanup(20 * time.Millisecond)
	cleanup.Add(func(ctx context.Context) error {
		events = append(events, "earlier")
		return ctx.Err()
	})
	cleanup.Add(func(ctx context.Context) error {
		events = append(events, "block")
		<-ctx.Done()
		return ctx.Err()
	})

	started := time.Now()
	err := cleanup.Run()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cleanup error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("cleanup took %v, want bounded execution", elapsed)
	}
	if want := []string{"block", "earlier"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}
