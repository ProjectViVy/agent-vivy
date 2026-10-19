package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"agent-vivy/internal/logging"
	"agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/postgres"
	"agent-vivy/internal/storage/sqlite"
)

// runNotebook implements `vivy notebook export`. It opens the configured
// Storage backend read-only — no App, runtime, module factory, lease, or
// migration work — so export works while a tenant is running and reports an
// upgrade requirement when the deployment predates the notebook schema.
func runNotebook(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "export" {
		_, _ = fmt.Fprintln(stderr, "usage: vivy notebook export --scope <scope> --entry <id> [--revision <id>] --output <new-directory>")
		return 2
	}
	fs := flag.NewFlagSet("notebook export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		scope    = fs.String("scope", string(notebookcontract.HomeScopeID), "scope id (home or ws.v1:<id>)")
		entry    = fs.String("entry", "", "entry id to export")
		revision = fs.String("revision", "", "revision id (default: entry head)")
		output   = fs.String("output", "", "new output directory")
	)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *entry == "" || *output == "" {
		_, _ = fmt.Fprintln(stderr, "notebook export: --entry and --output are required")
		return 2
	}
	cfg, err := loadConfig(logging.NewBootstrap(os.Stderr))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "notebook export: load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	var engine storage.Engine
	switch cfg.Storage.Backend {
	case "postgres":
		engine, err = postgres.OpenExisting(ctx, os.Getenv(cfg.Storage.Postgres.DSNEnv))
	default:
		engine, err = sqlite.OpenExisting(ctx, cfg.Storage.SQLite.Path)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "notebook export: %v\n", err)
		return 1
	}
	defer func() { _ = engine.Close() }()

	path, err := storage.ExportNotebookEntry(ctx, engine.Notebook(), notebookcontract.ScopeID(*scope), *entry, *revision, *output)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "notebook export: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "exported %s\n", path)
	return 0
}
