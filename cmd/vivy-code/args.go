package main

import (
	"fmt"
	"strings"

	faceport "agent-vivy/sdk/port/face"
)

// validThinkingLevels mirrors the pi thinking-level surface (RQ-MDL).
var validThinkingLevels = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

// Args is the parsed vivy-code command line. It maps one-to-one onto
// faceport.Options plus process-level switches (help/version) and per-flag
// diagnostics collected during parsing (parity with pi's warn-not-die policy
// on soft-invalid values).
type Args struct {
	Options  faceport.Options
	Help     bool
	Version  bool
	Warnings []string
	Errors   []string
}

func isFlag(s string) bool { return strings.HasPrefix(s, "-") && !strings.HasPrefix(s, "---") }

// next returns the following argument when it exists and is not itself a flag.
func next(args []string, i int) (string, bool) {
	if i+1 < len(args) {
		v := args[i+1]
		if !isFlag(v) {
			return v, true
		}
	}
	return "", false
}

func requireValue(flag string, args []string, i *int, a *Args) (string, bool) {
	v, ok := next(args, *i)
	if !ok {
		a.Errors = append(a.Errors, fmt.Sprintf("%s requires a value", flag))
		return "", false
	}
	*i++
	return v, true
}

func splitList(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// parseArgs parses argv (excluding argv[0]). Unknown flags are hard errors:
// vivy-code has no runtime extensions to forward them to (VCP-O1), so the
// historical contract — usage + exit 2 — is preserved.
func parseArgs(args []string) Args {
	var a Args
	o := &a.Options
	truthy := true
	falsy := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			for _, rest := range args[i+1:] {
				if strings.HasPrefix(rest, "@") {
					o.Files = append(o.Files, rest[1:])
				} else {
					appendMessage(o, rest)
				}
			}
			return a
		case arg == "--help" || arg == "-h":
			a.Help = true
		case arg == "--version" || arg == "-v":
			a.Version = true
		case arg == "--mode":
			if v, ok := requireValue("--mode", args, &i, &a); ok {
				switch v {
				case "text", "json", "rpc", "print":
					o.Mode = v
				default:
					a.Errors = append(a.Errors, fmt.Sprintf("invalid mode %q (valid: text, json, rpc, print)", v))
				}
			}
		case arg == "--print" || arg == "-p":
			// pi parity: -p requests the non-interactive print path but does
			// not override an explicit --mode json/rpc (those are already
			// headless modes with their own output contract).
			if o.Mode == "" || o.Mode == "text" {
				o.Mode = "print"
			}
			if v, ok := next(args, i); ok && !strings.HasPrefix(v, "@") {
				appendMessage(o, v)
				i++
			}
		case arg == "--continue" || arg == "-c":
			o.ContinueNewest = true
		case arg == "--resume" || arg == "-r":
			o.Resume = true
		case arg == "--fork":
			if v, ok := requireValue("--fork", args, &i, &a); ok {
				o.Fork = v
			}
		case arg == "--session":
			if v, ok := requireValue("--session", args, &i, &a); ok {
				o.Session = v
			}
		case arg == "--session-id":
			if v, ok := requireValue("--session-id", args, &i, &a); ok {
				o.SessionID = v
			}
		case arg == "--session-dir":
			if v, ok := requireValue("--session-dir", args, &i, &a); ok {
				o.SessionDir = v
			}
		case arg == "--no-session":
			o.NoSession = true
		case arg == "--name" || arg == "-n":
			if v, ok := requireValue("--name", args, &i, &a); ok {
				o.Name = v
			}
		case arg == "--provider":
			if v, ok := requireValue("--provider", args, &i, &a); ok {
				o.Provider = v
			}
		case arg == "--model":
			if v, ok := requireValue("--model", args, &i, &a); ok {
				o.Model = v
			}
		case arg == "--api-key":
			if v, ok := requireValue("--api-key", args, &i, &a); ok {
				o.APIKey = v
			}
		case arg == "--system-prompt":
			if v, ok := requireValue("--system-prompt", args, &i, &a); ok {
				o.SystemPrompt = v
			}
		case arg == "--append-system-prompt":
			if v, ok := requireValue("--append-system-prompt", args, &i, &a); ok {
				o.AppendSystemPrompt = append(o.AppendSystemPrompt, v)
			}
		case arg == "--thinking":
			if v, ok := requireValue("--thinking", args, &i, &a); ok {
				if validThinkingLevel(v) {
					o.Thinking = v
				} else {
					a.Warnings = append(a.Warnings, fmt.Sprintf("invalid thinking level %q (valid: %s)", v, strings.Join(validThinkingLevels, ", ")))
				}
			}
		case arg == "--models":
			if v, ok := requireValue("--models", args, &i, &a); ok {
				o.Models = splitList(v)
			}
		case arg == "--tools" || arg == "-t":
			if v, ok := requireValue("--tools", args, &i, &a); ok {
				o.Tools = splitList(v)
			}
		case arg == "--exclude-tools" || arg == "-xt":
			if v, ok := requireValue("--exclude-tools", args, &i, &a); ok {
				o.ExcludeTools = splitList(v)
			}
		case arg == "--no-tools" || arg == "-nt":
			o.NoTools = true
		case arg == "--no-builtin-tools" || arg == "-nbt":
			o.NoBuiltinTools = true
		case arg == "--no-mcp":
			o.NoMCP = true
		case arg == "--skill":
			if v, ok := requireValue("--skill", args, &i, &a); ok {
				o.Skills = append(o.Skills, v)
			}
		case arg == "--no-skills" || arg == "-ns":
			o.NoSkills = true
		case arg == "--prompt-template":
			if v, ok := requireValue("--prompt-template", args, &i, &a); ok {
				o.PromptTemplates = append(o.PromptTemplates, v)
			}
		case arg == "--no-prompt-templates" || arg == "-np":
			o.NoPromptTemplates = true
		case arg == "--theme":
			if v, ok := requireValue("--theme", args, &i, &a); ok {
				o.Themes = append(o.Themes, v)
			}
		case arg == "--use-theme":
			if v, ok := requireValue("--use-theme", args, &i, &a); ok {
				o.UseTheme = v
			}
		case arg == "--no-themes":
			o.NoThemes = true
		case arg == "--no-context-files" || arg == "-nc":
			o.NoContextFiles = true
		case arg == "--list-models":
			o.ListModels = true
			if v, ok := next(args, i); ok && !strings.HasPrefix(v, "@") {
				o.ListModelsPattern = v
				i++
			}
		case arg == "--export":
			if v, ok := requireValue("--export", args, &i, &a); ok {
				o.Export = v
			}
		case arg == "--tui-mode":
			if v, ok := requireValue("--tui-mode", args, &i, &a); ok {
				switch v {
				case "regular", "fullscreen":
					o.TUIMode = v
				default:
					a.Errors = append(a.Errors, fmt.Sprintf("invalid tui-mode %q (valid: regular, fullscreen)", v))
				}
			}
		case arg == "--offline":
			o.Offline = true
		case arg == "--verbose":
			o.Verbose = true
		case arg == "--approve" || arg == "-a":
			o.Approve = &truthy
		case arg == "--no-approve" || arg == "-na":
			o.Approve = &falsy
		case arg == "--debug-tools":
			o.DebugToolOutput = true
		case strings.HasPrefix(arg, "@"):
			o.Files = append(o.Files, arg[1:])
		case strings.HasPrefix(arg, "-"):
			a.Errors = append(a.Errors, fmt.Sprintf("unknown argument %q", arg))
		default:
			appendMessage(o, arg)
		}
	}
	return a
}

func appendMessage(o *faceport.Options, msg string) {
	if o.Prompt == "" {
		o.Prompt = msg
		return
	}
	o.Prompt += " " + msg
}

func validThinkingLevel(level string) bool {
	for _, l := range validThinkingLevels {
		if l == level {
			return true
		}
	}
	return false
}
