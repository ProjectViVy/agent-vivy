package tools

import (
	"fmt"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// InvocationClass is the per-call risk tier a shell-backed tool assigns to
// its arguments. The zero value fails closed: unclassified calls are treated
// as mutating and go through the normal approval path.
type InvocationClass int

const (
	// InvocationMutating is the default tier: the call may have effects
	// beyond reading, so tiered gating treats it like any effectful tool.
	InvocationMutating InvocationClass = iota
	// InvocationSafe means every invocation in the script is a known
	// read-only command and nothing writes to disk: under the 'auto'
	// approval policy the runtime may run it without interrupting.
	InvocationSafe
	// InvocationDenied hits the deny table: the call never runs, on any
	// profile or approval policy.
	InvocationDenied
)

// safeShellCommands are invocations that only read files, streams, or system
// state. The list is deliberately narrow; anything not listed is mutating.
var safeShellCommands = map[string]struct{}{
	"basename": {}, "cal": {}, "cat": {}, "cmp": {}, "cols": {},
	"comm": {}, "cut": {}, "date": {}, "df": {}, "diff": {}, "dirname": {},
	"du": {}, "echo": {}, "env": {}, "egrep": {}, "expand": {}, "false": {},
	"fgrep": {}, "file": {}, "find": {}, "fold": {}, "free": {}, "grep": {},
	"head": {}, "hexdump": {}, "hostname": {}, "id": {}, "info": {}, "join": {},
	"jq": {}, "less": {}, "ls": {}, "lscpu": {}, "man": {}, "md5sum": {},
	"more": {}, "nl": {}, "nproc": {}, "od": {}, "paste": {}, "printenv": {},
	"printf": {}, "ps": {}, "pwd": {}, "readlink": {}, "realpath": {},
	"rg": {}, "sha1sum": {}, "sha256sum": {}, "sha512sum": {}, "sleep": {},
	"sort": {}, "stat": {}, "strings": {}, "tac": {}, "tail": {}, "test": {},
	"tr": {}, "tree": {}, "true": {}, "tty": {},
	"uname": {}, "unexpand": {}, "uniq": {}, "uptime": {}, "wc": {},
	"whereis": {}, "which": {}, "who": {}, "whoami": {}, "xxd": {},
	"[": {}, ":": {},
}

// safeGitSubcommands restrict git to read-only subcommands; git itself is
// not in safeShellCommands because most of its surface mutates.
var safeGitSubcommands = map[string]struct{}{
	"blame": {}, "cat-file": {}, "check-attr": {}, "check-ignore": {},
	"count-objects": {}, "describe": {}, "diff": {}, "grep": {}, "ls-files": {},
	"log": {}, "shortlog": {}, "show": {}, "status": {}, "rev-parse": {},
	"version": {},
}

// safeGoSubcommands restrict go to read-only subcommands.
var safeGoSubcommands = map[string]struct{}{
	"env": {}, "list": {}, "version": {},
}

// denyReason describes why a deny-table entry fired. Messages are static:
// they never echo the raw script back into events or the model feed.
type denyRule struct {
	name   string
	reason string
	match  func(name string, args []string, script string) bool
}

var denyTable = []denyRule{
	{
		name:   "fork bomb",
		reason: "deny-table: fork bomb pattern",
		match: func(_ string, _ []string, script string) bool {
			return forkBombPattern.MatchString(script)
		},
	},
	{
		name:   "format shell",
		reason: "deny-table: disk format utility",
		match: func(name string, _ []string, _ string) bool {
			switch name {
			case "mkfs", "format", "diskpart", "fdisk":
				return true
			}
			return strings.HasPrefix(name, "mkfs.")
		},
	},
	{
		name:   "network access",
		reason: "deny-table: network access is not allowed from the governed shell",
		match: func(name string, args []string, script string) bool {
			switch name {
			case "curl", "wget", "nc", "ncat", "netcat", "ssh", "scp", "sftp", "ftp", "telnet", "socat":
				return true
			case "git":
				if len(args) > 1 {
					switch args[1] {
					case "clone", "fetch", "pull", "push", "remote", "submodule":
						return true
					}
				}
			}
			return strings.Contains(strings.ToLower(script), "/dev/tcp/") || strings.Contains(strings.ToLower(script), "/dev/udp/")
		},
	},
	{
		name:   "host escape",
		reason: "deny-table: host shell or interpreter escape",
		match: func(name string, _ []string, _ string) bool {
			switch name {
			case "sudo", "su", "runas", "powershell", "pwsh", "cmd", "cmd.exe", "sh", "bash", "zsh", "fish", "ksh", "dash", "eval", "source", ".", "exec", "nohup", "setsid":
				return true
			default:
				return false
			}
		},
	},
	{
		name:   "absolute path",
		reason: "deny-table: absolute or host path is outside the run workspace",
		match: func(name string, args []string, script string) bool {
			if isForbiddenHostPath(name) {
				return true
			}
			for _, arg := range args {
				if isForbiddenHostPath(arg) {
					return true
				}
			}
			upper := strings.ToUpper(script)
			return strings.Contains(upper, "$HOME") || strings.Contains(upper, "${HOME}") ||
				strings.Contains(upper, "$USERPROFILE") || strings.Contains(upper, "${USERPROFILE}") ||
				containsParentTraversal(script)
		},
	},
	{
		name:   "power control",
		reason: "deny-table: host power control",
		match: func(name string, _ []string, _ string) bool {
			switch name {
			case "shutdown", "reboot", "halt", "poweroff":
				return true
			}
			return false
		},
	},
	{
		name:   "destroy fs root",
		reason: "deny-table: recursive force delete of a system root",
		match: func(name string, args []string, _ string) bool {
			if name != "rm" {
				return false
			}
			recursive, force := false, false
			for _, arg := range args {
				if arg == strings.TrimLeft(arg, "-") {
					continue
				}
				flag := strings.TrimLeft(strings.ToLower(arg), "-")
				if strings.ContainsRune(flag, 'r') {
					recursive = true
				}
				if strings.ContainsRune(flag, 'f') {
					force = true
				}
			}
			if !recursive || !force {
				return false
			}
			for _, arg := range args {
				if arg == strings.TrimLeft(arg, "-") && arg != "" && isSystemRootPath(arg) {
					return true
				}
			}
			return false
		},
	},
	{
		name:   "raw disk write",
		reason: "deny-table: write directed at a raw disk device",
		match: func(name string, args []string, _ string) bool {
			for _, arg := range args {
				if strings.HasPrefix(strings.ToLower(arg), "of=/dev/") && rawDevicePattern.MatchString(strings.ToLower(arg)) {
					return true
				}
			}
			return false
		},
	},
}

var (
	forkBombPattern   = regexp.MustCompile(`:\s*\(\s*\)\s*\{[^}]*:[^}]*\|[^}]*&[^}]*\}\s*;\s*:`)
	rawDevicePattern  = regexp.MustCompile(`of=/dev/(sd|hd|vd|nvme|disk)`)
	pipedShellPattern = regexp.MustCompile(`(?i)\b(curl|wget)\b[^;|&` + "`" + `]*\|\s*(sudo\s+)?(ba|z|da|k)?sh\b`)
)

// isSystemRootPath reports whether the target is a path whose deletion
// breaks the host rather than the workspace: filesystem roots, home
// directories, and top-level system prefixes.
func isSystemRootPath(arg string) bool {
	value := strings.TrimSpace(arg)
	if value == "" {
		return false
	}
	// Parameter expansions are unresolvable here; treat $HOME and ~ as roots.
	if strings.HasPrefix(value, "$") || strings.HasPrefix(value, "~") {
		return true
	}
	if value == "/" {
		return true
	}
	value = strings.TrimSuffix(value, "/")
	switch strings.ToLower(value) {
	case "", "/bin", "/boot", "/etc", "/home", "/lib", "/opt", "/root",
		"/sbin", "/srv", "/tmp", "/usr", "/var", "c:", "c:\\", "*":
		return true
	}
	return false
}

// isForbiddenHostPath keeps shell arguments inside the backend-owned run
// workspace. The command backend cannot intercept every path a shell may
// open, so absolute paths, home expansions, drive paths, and UNC paths are
// denied before a script reaches it. The three harmless null/std streams are
// retained for ordinary read-only pipelines.
func isForbiddenHostPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return false
	}
	if value == "/dev/null" || value == "/dev/stdin" || value == "/dev/stdout" || value == "/dev/stderr" {
		return false
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") || strings.HasPrefix(value, "~") {
		return true
	}
	if len(value) >= 3 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':' && (value[2] == '/' || value[2] == '\\') {
		return true
	}
	for _, prefix := range []string{"$HOME", "${HOME}", "$USERPROFILE", "${USERPROFILE}"} {
		if strings.HasPrefix(strings.ToUpper(value), prefix) {
			return true
		}
	}
	return false
}

func containsParentTraversal(value string) bool {
	value = strings.ReplaceAll(value, "\\", "/")
	for _, field := range strings.Fields(value) {
		for _, part := range strings.Split(strings.Trim(field, " \t\r\n\"'"), "/") {
			if strings.Trim(part, " \t\r\n\"'") == ".." {
				return true
			}
		}
	}
	return false
}

// shellCommandName extracts the invocation name when the first word is a
// literal with no expansions; otherwise the command is unclassifiable.
func shellCommandName(word *syntax.Word) (string, bool) {
	value, ok := staticShellWord(word)
	if !ok {
		return "", false
	}
	name := normalizeShellCommand(value)
	if name == "" {
		return "", false
	}
	return name, true
}

func staticShellWord(word *syntax.Word) (string, bool) {
	if word == nil {
		return "", false
	}
	var b strings.Builder
	var appendParts func([]syntax.WordPart) bool
	appendParts = func(parts []syntax.WordPart) bool {
		for _, part := range parts {
			switch value := part.(type) {
			case *syntax.Lit:
				b.WriteString(value.Value)
			case *syntax.SglQuoted:
				if value.Dollar {
					return false
				}
				b.WriteString(value.Value)
			case *syntax.DblQuoted:
				if value.Dollar || !appendParts(value.Parts) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	if !appendParts(word.Parts) {
		return "", false
	}
	return b.String(), true
}

func normalizeShellCommand(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ".exe")
	value = strings.TrimSuffix(value, ".cmd")
	value = strings.TrimSuffix(value, ".bat")
	return strings.ToLower(value)
}

func argsList(words []*syntax.Word) []string {
	args := make([]string, 0, len(words))
	for _, word := range words {
		if value, ok := staticShellWord(word); ok {
			args = append(args, value)
			continue
		}
		args = append(args, "")
	}
	return args
}

// ClassifyShellScript parses a bash script and returns the strictest risk
// tier across every invocation it contains, plus bounded findings for the
// approval preview. A parse error is returned as an error, never a panic.
func ClassifyShellScript(script string) (InvocationClass, []string, error) {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return InvocationMutating, nil, fmt.Errorf("bash: invalid command syntax: %w", err)
	}
	class := InvocationSafe
	var findings []string
	var unknown []string
	syntax.Walk(file, func(node syntax.Node) bool {
		switch item := node.(type) {
		case *syntax.Stmt:
			if item.Background || item.Coprocess {
				class = InvocationDenied
				findings = append(findings, "deny-table: detached or background shell execution")
				return false
			}
			for _, redir := range item.Redirs {
				target := redirectTarget(redir)
				if target == "" || isForbiddenHostPath(target) || containsParentTraversal(target) {
					class = InvocationDenied
					findings = append(findings, "deny-table: redirection outside the run workspace")
					return false
				}
				if !redirectWrites(redir) {
					continue
				}
				if deny, reason := rawDeviceTarget(target); deny {
					class = InvocationDenied
					findings = append(findings, reason)
					return false
				}
				if target == "/dev/null" || target == "/dev/stdout" || target == "/dev/stderr" {
					continue
				}
				if class == InvocationSafe {
					class = InvocationMutating
					findings = append(findings, "mutating: output redirection")
				}
			}
		case *syntax.CallExpr:
			if len(item.Args) == 0 {
				// Assignments without a command have no external effect in a
				// one-shot script.
				return true
			}
			if len(item.Assigns) > 0 && class == InvocationSafe {
				class = InvocationMutating
				findings = append(findings, "mutating: command environment override")
			}
			for _, word := range item.Args {
				if shellWordHasExpansion(word) && class == InvocationSafe {
					class = InvocationMutating
					findings = append(findings, "mutating: dynamic shell expansion")
				}
			}
			name, ok := shellCommandName(item.Args[0])
			if !ok {
				unknown = append(unknown, "expansion")
				if class == InvocationSafe {
					class = InvocationMutating
				}
				return true
			}
			args := argsList(item.Args)
			if denied, reason := matchDenyTable(name, args, script); denied {
				class = InvocationDenied
				findings = append(findings, reason)
				return false
			}
			if pipedShellPattern.MatchString(script) {
				class = InvocationDenied
				findings = append(findings, "deny-table: remote script piped into a shell")
				return false
			}
			if !isSafeInvocation(name, args) {
				unknown = append(unknown, name)
				if class == InvocationSafe {
					class = InvocationMutating
				}
			}
		}
		return true
	})
	if class == InvocationDenied {
		return InvocationDenied, findings, nil
	}
	if class == InvocationMutating && len(findings) == 0 {
		if len(unknown) > 0 {
			findings = append(findings, "mutating command(s): "+strings.Join(dedupe(unknown), ", "))
		} else {
			findings = append(findings, "mutating: shell script with side effects")
		}
	}
	return class, findings, nil
}

func shellWordHasExpansion(word *syntax.Word) bool {
	dynamic := false
	if word == nil {
		return false
	}
	syntax.Walk(word, func(node syntax.Node) bool {
		switch node.(type) {
		case *syntax.ParamExp, *syntax.CmdSubst, *syntax.ArithmExp, *syntax.ProcSubst:
			dynamic = true
			return false
		default:
			return !dynamic
		}
	})
	return dynamic
}

func matchDenyTable(name string, args []string, script string) (bool, string) {
	for _, rule := range denyTable {
		if rule.match(name, args, script) {
			return true, rule.reason
		}
	}
	return false, ""
}

func isSafeInvocation(name string, args []string) bool {
	switch name {
	case "git":
		if len(args) < 2 {
			return false
		}
		_, ok := safeGitSubcommands[args[1]]
		return ok
	case "go":
		if len(args) < 2 {
			return false
		}
		// `go env -w/-u` mutates the user's Go environment. It must not
		// receive the read-only tier merely because the base subcommand is
		// normally observational.
		if args[1] == "env" {
			for _, arg := range args[2:] {
				if arg == "-w" || arg == "--w" || arg == "-u" || arg == "--u" || strings.HasPrefix(arg, "-w=") || strings.HasPrefix(arg, "-u=") {
					return false
				}
			}
		}
		_, ok := safeGoSubcommands[args[1]]
		return ok
	case "find":
		// GNU/BSD find's action predicates can mutate files even though
		// traversal itself is read-only. Keep those calls approval-gated.
		for _, arg := range args[1:] {
			switch arg {
			case "-delete", "-exec", "-execdir", "-ok", "-okdir", "-fls", "-fprint", "-fprint0", "-fprintf":
				return false
			}
		}
		return true
	case "env":
		// Bare env prints the environment; env CMD runs CMD.
		return len(args) == 1
	default:
		_, ok := safeShellCommands[name]
		return ok
	}
}

func redirectWrites(redir *syntax.Redirect) bool {
	switch redir.Op {
	case syntax.RdrOut, syntax.AppOut, syntax.RdrInOut, syntax.ClbOut, syntax.DplOut, syntax.RdrAll, syntax.AppAll:
		return true
	default:
		return false
	}
}

func redirectTarget(redir *syntax.Redirect) string {
	if redir.Word == nil || len(redir.Word.Parts) != 1 {
		return ""
	}
	lit, ok := redir.Word.Parts[0].(*syntax.Lit)
	if !ok {
		return ""
	}
	return strings.TrimSpace(lit.Value)
}

func rawDeviceTarget(target string) (bool, string) {
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "/dev/") && rawDevicePattern.MatchString("of="+lower) {
		return true, "deny-table: redirect into a raw disk device"
	}
	return false, ""
}

func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
