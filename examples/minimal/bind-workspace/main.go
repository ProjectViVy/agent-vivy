// Command bind-workspace creates a workspace-bound Session on an existing gateway.
// It uses the existing RPC client and never starts or executes a Run.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-vivy/internal/tui"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "existing gateway address")
	workspace := flag.String("workspace", "", "directory to bind to the new Session")
	flag.Parse()
	if err := bind(*addr, *workspace); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bind(addr, workspace string) error {
	if workspace == "" {
		return fmt.Errorf("--workspace is required")
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := tui.Dial(ctx, addr, "")
	if err != nil {
		return fmt.Errorf("connect gateway: %w", err)
	}
	defer client.Close()
	raw, err := client.Call(ctx, "session/create", map[string]string{"title": "Minimal example", "workspace_path": absolute})
	if err != nil {
		return fmt.Errorf("bind Session workspace: %w", err)
	}
	var session struct {
		ID        string `json:"id"`
		Workspace string `json:"workspace_path"`
	}
	if err := json.Unmarshal(raw, &session); err != nil {
		return fmt.Errorf("decode Session: %w", err)
	}
	if session.ID == "" || filepath.Clean(session.Workspace) != absolute {
		return fmt.Errorf("gateway did not confirm the requested workspace binding")
	}
	fmt.Println(string(raw))
	return nil
}
