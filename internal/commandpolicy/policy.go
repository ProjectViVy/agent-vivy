// Package commandpolicy contains the small, transport-neutral executable
// policy shared by configuration validation, local-process adapters, and the
// governed shell classifier. It deliberately has no runtime or tool imports.
package commandpolicy

import (
	"path/filepath"
	"strings"
)

// deniedExecutables is the common safety source for configured executable
// names. It is intentionally a denylist: MCP configuration authorizes a
// concrete PATH name or absolute path and does not create a second execute
// allowlist.
var deniedExecutables = map[string]struct{}{
	"bash": {}, "cmd": {}, "dash": {}, "diskpart": {}, "eval": {},
	"exec": {}, "fdisk": {}, "fish": {}, "format": {}, "halt": {},
	"ksh": {}, "mkfs": {}, "nohup": {}, "powershell": {}, "pwsh": {},
	"reboot": {}, "rm": {}, "rmdir": {}, "runas": {}, "setsid": {},
	"sh": {}, "shutdown": {}, "source": {}, "su": {}, "sudo": {},
	"zsh": {},
}

// shellEscapeExecutables is the subset whose invocation would turn a
// governed shell command into a host-shell/interpreter escape. The shell
// classifier uses this same policy source; its other deny rules remain
// argument/script-specific.
var shellEscapeExecutables = map[string]struct{}{
	".": {}, "bash": {}, "cmd": {}, "dash": {}, "eval": {}, "exec": {},
	"fish": {}, "ksh": {}, "nohup": {}, "powershell": {}, "pwsh": {},
	"runas": {}, "setsid": {}, "sh": {}, "source": {}, "su": {}, "sudo": {},
	"zsh": {},
}

// NormalizeExecutableName returns a case-insensitive basename with common
// Windows executable suffixes removed. Both slash styles are accepted so
// policy tests are deterministic across host operating systems.
func NormalizeExecutableName(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = filepath.Base(value)
	value = strings.ToLower(value)
	for _, suffix := range []string{".exe", ".cmd", ".bat"} {
		value = strings.TrimSuffix(value, suffix)
	}
	return value
}

// IsDeniedExecutable reports whether an executable is denied by the common
// configured-process policy.
func IsDeniedExecutable(value string) bool {
	_, ok := deniedExecutables[NormalizeExecutableName(value)]
	return ok
}

// IsShellEscapeExecutable reports whether a shell invocation is a direct
// host-shell/interpreter escape. It is narrower than IsDeniedExecutable so
// argument-sensitive bash rules retain their existing behavior.
func IsShellEscapeExecutable(value string) bool {
	_, ok := shellEscapeExecutables[NormalizeExecutableName(value)]
	return ok
}
