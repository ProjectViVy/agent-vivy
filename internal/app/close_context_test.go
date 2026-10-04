package app

// W1 Task 4: deadline-aware close for the composition owner. The timeout
// names the component still unwinding; teardown continues and later close
// calls observe its completion.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCloseContextTimeoutNamesComponent(t *testing.T) {
	release := make(chan struct{})
	a := &App{closeHook: func() { <-release }}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	err := a.CloseContext(ctx)
	if err == nil {
		t.Fatal("CloseContext returned before the deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if !strings.Contains(err.Error(), "automatic work") {
		t.Fatalf("timeout must name the unfinished component, got %v", err)
	}
	close(release)
	if err := a.Close(); err != nil {
		t.Fatalf("Close after released teardown: %v", err)
	}
}

func TestCloseContextSuccessIsIdempotent(t *testing.T) {
	a := &App{}
	if err := a.CloseContext(context.Background()); err != nil {
		t.Fatalf("CloseContext: %v", err)
	}
	if err := a.CloseContext(context.Background()); err != nil {
		t.Fatalf("second CloseContext: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close after CloseContext: %v", err)
	}
}
