// Package face defines the focused std/face@v1 Provider contract.
package face

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"agent-vivy/sdk/module"
)

type Definition struct{ ID, Kind string }
type FaceProvider interface {
	Definition() Definition
	Construct(context.Context, Host) (Instance, error)
}
type Instance interface {
	Run(context.Context, Options) (Result, error)
}

// Runner is the transport-facing implementation shape used inside a Provider.
// It is not a Module entry point; generated assemblies construct Providers.
type Runner interface {
	Kind() string
	Run(context.Context, Host) (Result, error)
}
type Face = Runner
type FaceOptions = Options
type FaceResult = Result
type FaceEnv = Host
type FaceConstructor func(Options) Runner

// Options is the launch surface handed to a face Runner. Mode selects the
// face-internal dispatch ("", "text" = interactive; "print", "json", "rpc" =
// headless); the remaining fields are launch-time overrides parsed from the
// CLI. Faces are free to ignore fields that do not apply to their medium.
type Options struct {
	Prompt                          string // -p/--print value or joined positionals
	Mode                            string // "" or "text" (default) | "print" | "json" | "rpc"
	Model, Provider, Thinking       string
	APIKey                          string // process-scoped override; never logged or persisted
	SystemPrompt                    string
	AppendSystemPrompt              []string
	ContinueNewest, Resume          bool
	Session, SessionID, Fork        string
	SessionDir                      string
	NoSession                       bool
	Name                            string
	Models                          []string // scoped model cycle set
	Tools, ExcludeTools             []string
	NoTools, NoBuiltinTools         bool
	NoMCP                           bool
	Skills, PromptTemplates, Themes []string
	NoSkills, NoPromptTemplates     bool
	NoThemes, NoContextFiles        bool
	UseTheme                        string
	// ThemesDir is the operator theme directory (<agent home>/themes). The
	// TUI face loads <name>.json files from it; embedded themes still win
	// when no file matches.
	ThemesDir string
	// KeybindingsFile is the operator action→chord override file
	// (<agent home>/keybindings.yaml). Missing file = defaults.
	KeybindingsFile string
	// Images gates inline terminal graphics: "auto" (default), "on", "off".
	Images            string
	ListModels        bool
	ListModelsPattern string
	Export            string
	TUIMode           string // "" | "regular" | "fullscreen"
	Offline, Verbose  bool
	Approve           *bool // --approve / --no-approve tri-state; nil = default policy
	DebugToolOutput   bool
	Files             []string  // @file positional arguments
	In                io.Reader // stdin for --mode rpc and interactive input
	Out, Err          io.Writer
}

// ModeUnavailableError reports a face mode that is declared on the CLI
// surface but not yet (or never) implemented by the running face.
type ModeUnavailableError struct{ Mode string }

func (e ModeUnavailableError) Error() string {
	return fmt.Sprintf("face mode %q is not available", e.Mode)
}

type Result struct{ Status string }
type Host interface {
	module.Host
	Call(context.Context, string, any) (json.RawMessage, error)
	OnEvent(func(string, json.RawMessage))
}
