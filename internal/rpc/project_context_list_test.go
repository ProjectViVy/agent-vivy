package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListProjectContextsReturnsSafeTextMetadataOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"README.md":   []byte("read me"),
		"src/main.go": []byte("package main"),
		"secret.txt":  []byte("private"),
		"binary.dat":  {0, 1, 2},
		".git/config": []byte("git config"),
		".env.local":  []byte("TOKEN=hidden"),
	} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items, truncated, err := listProjectContexts(root, "", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || len(items) != 2 {
		t.Fatalf("listed contexts = %+v, truncated=%v; want README and src/main.go", items, truncated)
	}
	if items[0].Path != "README.md" || items[1].Path != filepath.ToSlash(filepath.Join("src", "main.go")) {
		t.Fatalf("listed paths = %+v", items)
	}
	for _, item := range items {
		if len(item.Content) != 0 || item.Size <= 0 {
			t.Fatalf("listed item leaked body or size = %+v", item)
		}
	}
	one, truncated, err := listProjectContexts(root, "", "", 1)
	if err != nil || len(one) != 1 || !truncated {
		t.Fatalf("limited list = %+v, truncated=%v, err=%v", one, truncated, err)
	}
	nested, truncated, err := listProjectContexts(root, "src", "", 20)
	if err != nil || truncated || len(nested) != 1 || nested[0].Path != "src/main.go" {
		t.Fatalf("prefix list = %+v, truncated=%v, err=%v", nested, truncated, err)
	}
	matched, truncated, err := listProjectContexts(root, "", "main", 20)
	if err != nil || truncated || len(matched) != 1 || matched[0].Path != "src/main.go" {
		t.Fatalf("query list = %+v, truncated=%v, err=%v", matched, truncated, err)
	}
}

func TestProjectContextListErrorsDoNotLeakProjectRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-root")
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.ProjectRoot = missing })
	request := Request{Method: "project-context/list", Params: json.RawMessage(`{}`)}
	_, rpcErr := env.handler.Handle(context.Background(), nil, request)
	if rpcErr == nil || rpcErr.Message != "project context listing failed" {
		t.Fatalf("list error = %+v", rpcErr)
	}
	if strings.Contains(rpcErr.Message, missing) {
		t.Fatalf("list error leaked project root: %q", rpcErr.Message)
	}

	root := t.TempDir()
	env = newControlTestEnv(t, func(deps *ControlDeps) { deps.ProjectRoot = root })
	request.Params = json.RawMessage(`{"prefix":"../private"}`)
	_, rpcErr = env.handler.Handle(context.Background(), nil, request)
	if rpcErr == nil || rpcErr.Message != "invalid project context prefix" || strings.Contains(rpcErr.Message, root) {
		t.Fatalf("prefix error = %+v", rpcErr)
	}
}

func TestProjectContextListQueryFiltersBeforeLimitAndRejectsUnsafeQuery(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 220; i++ {
		name := filepath.Join(root, fmt.Sprintf("file-%03d.txt", i))
		if err := os.WriteFile(name, []byte("text"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "zz-target.txt"), []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, truncated, err := listProjectContexts(root, "", "target", 1)
	if err != nil || truncated || len(items) != 1 || items[0].Path != "zz-target.txt" {
		t.Fatalf("late query match = %+v, truncated=%v, err=%v", items, truncated, err)
	}
	for _, query := range []string{"../secret", `C:\\secret`, "bad\x1bpath", "bad\u202epath", "node_modules/token"} {
		if _, _, err := listProjectContexts(root, "", query, 20); !errors.Is(err, errProjectContextListPrefix) {
			t.Fatalf("unsafe query %q err=%v", query, err)
		}
	}
}
