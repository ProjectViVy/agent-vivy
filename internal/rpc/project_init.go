package rpc

import (
	"errors"
	"os"
	"strings"
)

// projectInitStatus checks the fixed code root on the control plane. A TUI
// project listing can omit large, unreadable, or filtered files, so it is not
// an authoritative overwrite check for /init.
func (h *controlHandler) projectInitStatus() (any, *Error) {
	if strings.TrimSpace(h.deps.ProjectRoot) == "" {
		return nil, &Error{Code: MethodNotFound, Message: "project instructions are not configured"}
	}
	root, err := os.OpenRoot(h.deps.ProjectRoot)
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: "project instructions cannot be inspected"}
	}
	defer root.Close()
	info, err := root.Lstat("AGENTS.md")
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{"exists": false}, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, &Error{Code: InvalidParams, Message: "project instructions cannot be inspected"}
	}
	return map[string]any{"exists": true}, nil
}
