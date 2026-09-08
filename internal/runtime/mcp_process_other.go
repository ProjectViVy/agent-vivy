//go:build !windows

package runtime

import "os/exec"

func configureMCPProcess(_ *exec.Cmd) {}
