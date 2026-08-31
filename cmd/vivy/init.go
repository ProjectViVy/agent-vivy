package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// vivy init generates a starter AGENTS.md in the current directory (VC-1e,
// D6: AGENTS.md is the one injected context file). Behavior follows the
// ratified initialize essentials: refuse an empty directory, refuse to
// overwrite an existing file, probe existing rule files, and template only
// non-obvious knowledge.

// probedRuleFiles are the existing instruction sources the template mentions
// so the author keeps them in sync with AGENTS.md.
var probedRuleFiles = []string{
	".cursorrules",
	filepath.Join(".cursor", "rules"),
	filepath.Join(".github", "copilot-instructions.md"),
}

func runInit(_ []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vivy init: resolve current directory: %v\n", err)
		return 1
	}
	path, hints, err := initAgentsMD(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vivy init: %v\n", err)
		return 1
	}
	fmt.Printf("created %s\n", path)
	for _, hint := range hints {
		fmt.Printf("found existing instructions in %s — keep them in sync or reference them from AGENTS.md\n", hint)
	}
	fmt.Println("record only what is non-obvious: an agent can read the code, but it cannot guess intent.")
	return 0
}

// initAgentsMD writes the starter AGENTS.md into dir and returns its path
// plus the probed rule files that already exist.
func initAgentsMD(dir string) (string, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, fmt.Errorf("read directory: %w", err)
	}
	visible := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		visible++
	}
	if visible == 0 {
		return "", nil, fmt.Errorf("directory has no visible files; vivy init describes an existing project, not an empty one")
	}

	path := filepath.Join(dir, "AGENTS.md")
	if _, err := os.Stat(path); err == nil {
		return "", nil, fmt.Errorf("%s already exists; refusing to overwrite", path)
	} else if !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("inspect %s: %w", path, err)
	}

	var hints []string
	for _, rule := range probedRuleFiles {
		if _, err := os.Stat(filepath.Join(dir, rule)); err == nil {
			hints = append(hints, rule)
		}
	}
	sort.Strings(hints)

	if err := os.WriteFile(path, []byte(renderAgentsMDTemplate(hints)), 0o600); err != nil {
		return "", nil, fmt.Errorf("write %s: %w", path, err)
	}
	return path, hints, nil
}

func renderAgentsMDTemplate(hints []string) string {
	var sb strings.Builder
	sb.WriteString(`# AGENTS.md

Guidance for Vivy and any coding agent working in this repository.

Record only what is non-obvious here: agents can read the code, but they
cannot guess intent. Skip anything a quick file listing or search already
reveals — no file trees, no restated public APIs.

## Project overview

<!-- What this project is, in a few sentences — and what it deliberately is not. -->

## Build, test, verify

<!-- The exact commands to run, in order, and what a passing result looks like. -->

## Conventions the code does not show

<!-- Decisions a newcomer would get wrong: layout rules, naming, what must never change. -->

## Known pitfalls

<!-- Traps: misleading names, legacy quirks, required environment, slow or dangerous commands. -->
`)
	if len(hints) > 0 {
		sb.WriteString(`
## Existing instruction files

This repository also carries agent rules in:
`)
		for _, hint := range hints {
			sb.WriteString("- " + hint + "\n")
		}
		sb.WriteString(`
Keep them in sync with this file, or move their content here and reference it.
`)
	}
	return sb.String()
}
