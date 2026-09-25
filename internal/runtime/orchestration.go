package runtime

import (
	"context"
	"errors"
)

// ErrNativeOrchestrationUnimplemented marks the approved G0 proof boundary.
var ErrNativeOrchestrationUnimplemented = errors.New("runtime: native orchestration unimplemented")

type nativeOrchestrationRequest struct {
	Task string
}

type nativeOrchestrationResult struct {
	Output string
}

func (s *Service) runNativeOrchestration(context.Context, nativeOrchestrationRequest) (nativeOrchestrationResult, error) {
	return nativeOrchestrationResult{}, ErrNativeOrchestrationUnimplemented
}
