// Command vivy-studio is Vivy Studio's own lifecycle tool. It owns the
// Studio ledger (Worktree / Generation / EvalRun / Release / Install),
// invokes vivy-sdk for packing, spawns candidate EXEs itself for eval,
// and writes the daily install location. It is not the daily vivy.exe and
// never touches the species Journal.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent-vivy/internal/studiocore"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	ctx := context.Background()

	// Global flags before the command.
	worktree := ""
	sdkPath := os.Getenv("VIVY_SDK")
	installTarget := os.Getenv("VIVY_INSTALL_DIR")
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "--worktree":
			if len(args) < 2 {
				return fail("--worktree needs a path")
			}
			worktree = args[1]
			args = args[2:]
		case "--sdk":
			if len(args) < 2 {
				return fail("--sdk needs a path")
			}
			sdkPath = args[1]
			args = args[2:]
		case "--target":
			if len(args) < 2 {
				return fail("--target needs a path")
			}
			installTarget = args[1]
			args = args[2:]
		default:
			return fail("unknown global flag %q", args[0])
		}
	}
	if worktree == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fail("resolve worktree: %v", err)
		}
		worktree = wd
	}

	svc, err := studiocore.NewService(ctx, studiocore.Options{
		Worktree:      worktree,
		SDKPath:       sdkPath,
		InstallTarget: installTarget,
	})
	if err != nil {
		return fail("studio: %v", err)
	}
	defer func() { _ = svc.Close() }()

	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "workspace":
		return workspaceCmd(ctx, svc, args[1:])
	case "pack":
		return packCmd(ctx, svc, args[1:])
	case "eval":
		return evalCmd(ctx, svc, args[1:])
	case "release":
		return releaseCmd(ctx, svc, args[1:])
	case "reject":
		return rejectCmd(ctx, svc, args[1:])
	case "install":
		return installCmd(ctx, svc, args[1:])
	case "rollback":
		return rollbackCmd(ctx, svc, args[1:])
	case "inspect":
		return inspectCmd(ctx, svc, args[1:])
	case "list":
		return listCmd(ctx, svc, args[1:])
	default:
		return fail("unknown studio command %q", args[0])
	}
}

func workspaceCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	if len(args) == 0 {
		return fail("usage: vivy-studio workspace pin <path> [--kind kernel|first-party|plugin] | list")
	}
	switch args[0] {
	case "pin":
		if len(args) < 2 {
			return fail("usage: vivy-studio workspace pin <path> [--kind <kind>]")
		}
		path := args[1]
		kind := "kernel"
		for i := 2; i < len(args); i++ {
			if args[i] == "--kind" && i+1 < len(args) {
				kind = args[i+1]
				i++
			}
		}
		w, err := svc.PinWorktree(ctx, path, kind)
		if err != nil {
			return fail("workspace pin: %v", err)
		}
		return printJSON(w)
	case "list":
		ws, err := svc.ListWorktrees(ctx)
		if err != nil {
			return fail("workspace list: %v", err)
		}
		return printJSON(map[string]any{"worktrees": ws})
	default:
		return fail("unknown workspace subcommand %q", args[0])
	}
}

func packCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	var with []string
	out := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--with":
			if i+1 >= len(args) {
				return fail("--with needs a plugin name")
			}
			i++
			with = append(with, args[i])
		case "--out":
			if i+1 >= len(args) {
				return fail("--out needs a directory")
			}
			i++
			out = args[i]
		default:
			return fail("unknown pack flag %q", args[i])
		}
	}
	g, err := svc.Pack(ctx, with, out)
	if err != nil {
		return fail("pack: %v", err)
	}
	return printJSON(g)
}

func evalCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	candidate := ""
	baseline := ""
	suite := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--candidate":
			if i+1 >= len(args) {
				return fail("--candidate needs a generation id")
			}
			i++
			candidate = args[i]
		case "--baseline":
			if i+1 >= len(args) {
				return fail("--baseline needs a generation id")
			}
			i++
			baseline = args[i]
		case "--suite":
			if i+1 >= len(args) {
				return fail("--suite needs an id")
			}
			i++
			suite = args[i]
		default:
			return fail("unknown eval flag %q", args[i])
		}
	}
	if candidate == "" {
		return fail("eval requires --candidate")
	}
	e, err := svc.Eval(ctx, candidate, baseline, suite)
	if err != nil {
		return fail("eval: %v", err)
	}
	return printJSON(e)
}

func releaseCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	generation := ""
	evalID := ""
	actor := ""
	confirmed := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--generation":
			if i+1 >= len(args) {
				return fail("--generation needs a generation id")
			}
			i++
			generation = args[i]
		case "--eval":
			if i+1 >= len(args) {
				return fail("--eval needs an eval run id")
			}
			i++
			evalID = args[i]
		case "--actor":
			if i+1 >= len(args) {
				return fail("--actor needs a value")
			}
			i++
			actor = args[i]
		case "--yes":
			confirmed = true
		default:
			return fail("unknown release flag %q", args[i])
		}
	}
	if generation == "" {
		return fail("release requires --generation")
	}
	if actor != studiocore.PromotionActorHuman {
		return fail("release requires --actor human (only a human may release, NG-25)")
	}
	if !confirmed {
		return fail("release requires explicit human confirmation (--yes)")
	}
	rel, err := svc.Release(ctx, generation, evalID, actor)
	if err != nil {
		return fail("release: %v", err)
	}
	return printJSON(rel)
}

func rejectCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	if len(args) != 2 || args[0] != "--generation" {
		return fail("usage: vivy-studio reject --generation <id>")
	}
	g, err := svc.Reject(ctx, args[1])
	if err != nil {
		return fail("reject: %v", err)
	}
	return printJSON(g)
}

func installCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	releaseID := ""
	target := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--release":
			if i+1 >= len(args) {
				return fail("--release needs a release id")
			}
			i++
			releaseID = args[i]
		case "--target":
			if i+1 >= len(args) {
				return fail("--target needs a path")
			}
			i++
			target = args[i]
		default:
			return fail("unknown install flag %q", args[i])
		}
	}
	if releaseID == "" {
		return fail("install requires --release")
	}
	in, err := svc.Install(ctx, releaseID, target)
	if err != nil {
		return fail("install: %v", err)
	}
	return printJSON(in)
}

func rollbackCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	target := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--target" && i+1 < len(args) {
			target = args[i+1]
			i++
		} else {
			return fail("unknown rollback flag %q", args[i])
		}
	}
	in, err := svc.Rollback(ctx, target)
	if err != nil {
		return fail("rollback: %v", err)
	}
	return printJSON(in)
}

func inspectCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	target := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--target" && i+1 < len(args) {
			target = args[i+1]
			i++
		} else {
			return fail("unknown inspect flag %q", args[i])
		}
	}
	rep, err := svc.InspectInstall(ctx, target)
	if err != nil {
		return fail("inspect: %v", err)
	}
	return printJSON(rep)
}

func listCmd(ctx context.Context, svc *studiocore.Service, args []string) int {
	if len(args) != 1 {
		return fail("usage: vivy-studio list generations|evals|releases|installs|events")
	}
	var (
		out any
		err error
	)
	switch args[0] {
	case "generations":
		out, err = svc.Ledger().ListGenerations(ctx)
	case "evals":
		out, err = svc.Ledger().ListEvalRuns(ctx)
	case "releases":
		out, err = svc.Ledger().ListReleases(ctx)
	case "installs":
		out, err = svc.Ledger().ListInstalls(ctx)
	case "events":
		out, err = svc.Ledger().ListStudioEvents(ctx)
	default:
		return fail("unknown list kind %q", args[0])
	}
	if err != nil {
		return fail("list: %v", err)
	}
	return printJSON(out)
}

func printJSON(v any) int {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fail("encode output: %v", err)
	}
	_, _ = fmt.Fprintln(os.Stdout, string(raw))
	return 0
}

func fail(format string, a ...any) int {
	_, _ = fmt.Fprintf(os.Stderr, "vivy-studio: "+format+"\n", a...)
	return 1
}

func usage(w *os.File) {
	_, _ = fmt.Fprintln(w, `usage: vivy-studio [--worktree <dir>] [--sdk <path>] [--target <dir>] <command>

Commands:
  workspace pin <path> [--kind kernel|first-party|plugin]
  workspace list
  pack --with <plugin>... [--out <dir>]
  eval --candidate <gen> [--baseline <gen>] [--suite airgap.probe]
  release --generation <gen> [--eval <evl>] --actor human --yes
  reject --generation <gen>
  install --release <rel> [--target <dir>]
  rollback [--target <dir>]
  inspect [--target <dir>]
  list generations|evals|releases|installs|events`)
}
