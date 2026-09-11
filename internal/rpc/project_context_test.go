package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

func TestResolveProjectContextsRejectsUnsafeSensitiveAndBinaryInputs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n世界\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		".env":       []byte("TOKEN=secret"),
		"secret.txt": []byte("do not read"),
		"binary.dat": {0x7f, 0x00, 0x01},
	} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name  string
		path  string
		cause error
		want  string
	}{
		{name: "parent", path: "../README.md", cause: errProjectContextTraversal, want: "traversal"},
		{name: "absolute", path: `/tmp/README.md`, cause: errProjectContextAbsolute, want: "project-relative"},
		{name: "drive", path: `C:\README.md`, cause: errProjectContextAbsolute, want: "project-relative"},
		{name: "ads", path: `README.md:secret`, cause: errProjectContextAbsolute, want: "project-relative"},
		{name: "unc", path: `\\server\share\README.md`, cause: errProjectContextAbsolute, want: "project-relative"},
		{name: "nul", path: "README.md\x00", cause: errProjectContextNUL, want: "invalid"},
		{name: "newline", path: "README\n.md", cause: errProjectContextPathControl, want: "invalid"},
		{name: "tab", path: "README\t.md", cause: errProjectContextPathControl, want: "invalid"},
		{name: "sensitive", path: ".env", cause: errProjectContextSensitive, want: "sensitive"},
		{name: "naked password", path: "password", cause: errProjectContextSensitive, want: "sensitive"},
		{name: "naked token", path: "token", cause: errProjectContextSensitive, want: "sensitive"},
		{name: "keys file", path: "keys.txt", cause: errProjectContextSensitive, want: "sensitive"},
		{name: "keys", path: "keys", cause: errProjectContextSensitive, want: "sensitive"},
		{name: "binary", path: "binary.dat", cause: errProjectContextBinary, want: "UTF-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveProjectContexts(root, []string{tc.path})
			if err == nil || !errors.Is(err, tc.cause) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("resolve(%q) = %v, want %v/%q", tc.path, err, tc.cause, tc.want)
			}
			if strings.Contains(err.Error(), root) || strings.Contains(err.Error(), tc.path) {
				t.Fatalf("error leaked filesystem input: %q", err)
			}
		})
	}
}

func TestResolveProjectContextsRejectsSensitiveSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".env"), filepath.Join(root, "public.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := resolveProjectContexts(root, []string{"public.txt"})
	if err == nil || !errors.Is(err, errProjectContextSensitive) || !strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("sensitive symlink resolve = %v", err)
	}
	if strings.Contains(err.Error(), root) || strings.Contains(err.Error(), ".env") {
		t.Fatalf("sensitive target leaked through public error: %q", err)
	}
}

func TestResolveProjectContextsKeepsEmptyBodyNonNil(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "empty.txt"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveProjectContexts(root, []string{"empty.txt"})
	if err != nil || len(got) != 1 || got[0].Content == nil || got[0].Size != 0 {
		t.Fatalf("empty context = %+v/%v", got, err)
	}
}

func TestResolveProjectContextsHonorsLiveRequestContext(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolveProjectContextsWithContext(ctx, root, []string{"README.md"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resolve = %v, want context.Canceled", err)
	}
}

func TestTurnStartUsesLiveContextForProjectFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.ProjectRoot = root })
	params, err := json.Marshal(map[string]any{
		"session_id": "missing-session", "text": "inspect", "context_paths": []string{"README.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, rpcErr := env.handler.Handle(ctx, nil, Request{Method: "turn/start", Params: params})
	if rpcErr == nil || rpcErr.Code != InvalidParams || !strings.Contains(rpcErr.Message, "context canceled") {
		t.Fatalf("canceled turn/start = %v, want live-context InvalidParams", rpcErr)
	}
}

func TestResolveProjectContextsBoundsAndSymlinkContainment(t *testing.T) {
	root := t.TempDir()
	inside := []byte("inside")
	if err := os.WriteFile(filepath.Join(root, "README.md"), inside, 0o600); err != nil {
		t.Fatal(err)
	}
	large := bytes.Repeat([]byte{'x'}, maxProjectContextBytes+1)
	if err := os.WriteFile(filepath.Join(root, "large.txt"), large, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveProjectContexts(root, []string{"large.txt"}); err == nil || !errors.Is(err, errProjectContextTooLarge) {
		t.Fatalf("oversize resolve = %v", err)
	}
	paths := make([]string, maxProjectContextCount+1)
	for i := range paths {
		paths[i] = "README.md"
	}
	if _, err := resolveProjectContexts(root, paths); err == nil || !errors.Is(err, errProjectContextTooLarge) {
		t.Fatalf("count resolve = %v", err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "outside.txt")
	if err := os.Symlink(filepath.Join(outside, "outside.txt"), link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := resolveProjectContexts(root, []string{"outside.txt"}); err == nil || !strings.Contains(err.Error(), "escapes the project") {
		t.Fatalf("symlink escape resolve = %v", err)
	}
	got, err := resolveProjectContexts(root, []string{"README.md"})
	if err != nil || len(got) != 1 || string(got[0].Content) != string(inside) || got[0].Size != int64(len(inside)) {
		t.Fatalf("valid resolve = %+v/%v", got, err)
	}
}

func TestProjectContextRPCAndTurnPersistSnapshotMetadataOnly(t *testing.T) {
	root := t.TempDir()
	body := []byte("snapshot body")
	if err := os.WriteFile(filepath.Join(root, "README.md"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	env := newControlTestEnv(t, func(deps *ControlDeps) { deps.ProjectRoot = root })
	resolved, rpcErr := callControl(t, env.handler, "project-context/resolve", map[string]any{"paths": []string{"README.md"}})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var envelope struct {
		Contexts []projectContextResult `json:"contexts"`
	}
	encoded, _ := json.Marshal(resolved)
	if err := json.Unmarshal(encoded, &envelope); err != nil || len(envelope.Contexts) != 1 || envelope.Contexts[0].Size != int64(len(body)) {
		t.Fatalf("resolve metadata = %s/%v", encoded, err)
	}
	if strings.Contains(string(encoded), string(body)) {
		t.Fatalf("resolve leaked body: %s", encoded)
	}
	created, rpcErr := callControl(t, env.handler, "session/create", map[string]string{"title": "context"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var session sessionResult
	createdJSON, _ := json.Marshal(created)
	if err := json.Unmarshal(createdJSON, &session); err != nil {
		t.Fatal(err)
	}
	started, rpcErr := callControl(t, env.handler, "turn/start", map[string]any{
		"session_id": string(session.ID), "text": "inspect", "context_paths": []string{"README.md"},
	})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var accepted struct {
		RunID string `json:"run_id"`
	}
	startedJSON, _ := json.Marshal(started)
	if err := json.Unmarshal(startedJSON, &accepted); err != nil || accepted.RunID == "" {
		t.Fatalf("turn/start = %s/%v", startedJSON, err)
	}
	waitForControlRunTerminal(t, env.backend, accepted.RunID)
	messages, rpcErr := callControl(t, env.handler, "session/messages", map[string]string{"session_id": string(session.ID)})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	var history struct {
		Messages []messageResult `json:"messages"`
	}
	historyJSON, _ := json.Marshal(messages)
	if err := json.Unmarshal(historyJSON, &history); err != nil {
		t.Fatal(err)
	}
	var user *messageResult
	for i := range history.Messages {
		if history.Messages[i].Role == domain.RoleUser && history.Messages[i].Content == "inspect" {
			user = &history.Messages[i]
			break
		}
	}
	if user == nil || len(user.FileContexts) != 1 || user.FileContexts[0].Path != "README.md" || user.FileContexts[0].Size != int64(len(body)) {
		t.Fatalf("history context metadata = %+v", user)
	}
	if strings.Contains(string(historyJSON), string(body)) {
		t.Fatalf("history leaked snapshot body: %s", historyJSON)
	}
	stored, err := env.backend.ListMessages(context.Background(), domain.SessionID(session.ID))
	if err != nil || len(stored) == 0 || len(stored[0].FileContexts) != 1 || string(stored[0].FileContexts[0].Content) != string(body) {
		t.Fatalf("durable snapshot = %+v/%v", stored, err)
	}
}
