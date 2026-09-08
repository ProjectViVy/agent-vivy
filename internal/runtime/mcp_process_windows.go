//go:build windows

package runtime

import (
	"os/exec"
	"syscall"
)

const mcpCreateNoWindow = 0x08000000

// configureMCPProcess keeps a local MCP helper from creating a visible
// console window in the Windows desktop session. Job Object/process-tree
// ownership is intentionally a separate TODO; this only controls launch UX.
func configureMCPProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: mcpCreateNoWindow,
	}
}
