package main

import (
	"fmt"
	"os"

	"agent-vivy/internal/runtime"
)

func resolveInstructionRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current project: %w", err)
	}
	root, err := runtime.CanonicalInstructionRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("canonicalize current project: %w", err)
	}
	return root, nil
}
