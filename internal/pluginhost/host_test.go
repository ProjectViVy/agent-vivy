package pluginhost

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	hellofs "agent-vivy/plugins/hello-fs"
	"agent-vivy/sdk/plugin"
)

func TestHelloStatRunsThroughEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapted := Adapt([]plugin.Plugin{hellofs.New()}, func(context.Context) (string, error) {
		return root, nil
	})
	if len(adapted) != 1 || adapted[0].Spec().Name != "hello_stat" || !adapted[0].Spec().Readonly {
		t.Fatalf("adapted = %#v", adapted)
	}
	got, err := adapted[0].InvokableRun(context.Background(), []byte(`{"path":"note.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "note.txt 2" {
		t.Fatalf("got %q", got)
	}
}

func TestMissingGrantDenied(t *testing.T) {
	env := hostedEnv{plugin: hellofs.New(), lookup: func(context.Context) (string, error) { return t.TempDir(), nil }}
	if _, err := env.OpenWrite("x"); err != plugin.ErrDenied {
		t.Fatalf("err = %v", err)
	}
	if _, err := env.Spawn(context.Background(), plugin.SpawnSpec{Command: "cmd"}); err != plugin.ErrDenied {
		t.Fatalf("spawn err = %v, want ErrDenied", err)
	}
}

// spawnStub carries the proc.spawn grant so the host tests can exercise
// the real exec path against a shell child.
type spawnStub struct{}

func (spawnStub) Name() string      { return "spawn-stub" }
func (spawnStub) Seam() plugin.Seam { return plugin.SeamToolWorld }
func (spawnStub) Grants() []plugin.Grant {
	return []plugin.Grant{plugin.GrantFSRead, plugin.GrantProcSpawn}
}
func (spawnStub) Tools() []plugin.Tool { return []plugin.Tool{stubTool{}} }

func TestSpawnRunsChildInWorkspace(t *testing.T) {
	root := t.TempDir()
	env := hostedEnv{plugin: spawnStub{}, lookup: func(context.Context) (string, error) { return root, nil }}

	echo, args := echoCommand()
	proc, err := env.Spawn(context.Background(), plugin.SpawnSpec{Command: echo, Args: args})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	body, err := io.ReadAll(proc.Stdout())
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	_ = proc.Stdin().Close()
	if err := proc.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !strings.Contains(string(body), "hello") {
		t.Fatalf("stdout = %q", string(body))
	}

	cwd, cwdArgs := cwdCommand()
	proc2, err := env.Spawn(context.Background(), plugin.SpawnSpec{Command: cwd, Args: cwdArgs})
	if err != nil {
		t.Fatalf("spawn cwd: %v", err)
	}
	body2, _ := io.ReadAll(proc2.Stdout())
	_ = proc2.Stdin().Close()
	if err := proc2.Wait(); err != nil {
		t.Fatalf("wait cwd: %v", err)
	}
	if !strings.Contains(string(body2), filepath.Base(root)) {
		t.Fatalf("child cwd = %q, want inside workspace %q", string(body2), root)
	}
}

func TestSpawnRejectsWorkspaceEscapes(t *testing.T) {
	env := hostedEnv{plugin: spawnStub{}, lookup: func(context.Context) (string, error) { return t.TempDir(), nil }}
	for _, command := range []string{"", "../evil", `C:\Windows\System32\cmd.exe`, "/bin/sh"} {
		if _, err := env.Spawn(context.Background(), plugin.SpawnSpec{Command: command}); err != plugin.ErrInvalidArgs {
			t.Fatalf("command %q err = %v, want ErrInvalidArgs", command, err)
		}
	}
}

func TestSpawnCloseKillsChild(t *testing.T) {
	root := t.TempDir()
	env := hostedEnv{plugin: spawnStub{}, lookup: func(context.Context) (string, error) { return root, nil }}
	sleep, args := sleepCommand()
	proc, err := env.Spawn(context.Background(), plugin.SpawnSpec{Command: sleep, Args: args})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if err := proc.Close(); err == nil {
		t.Fatal("killed child Wait should report a non-zero exit, got nil")
	}
}

func echoCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo", "hello"}
	}
	return "sh", []string{"-c", "echo hello"}
}

func cwdCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "cd"}
	}
	return "pwd", nil
}

func sleepCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "waitfor", "/t", "5", "vivy-spawn-test"}
	}
	return "sleep", []string{"30"}
}

// channelStub implements plugin.Plugin AND plugin.Channel. Its Tools()
// returns one non-empty tool on purpose: if Adapt ever produced it, the
// test would prove the channel skip is broken, not accidental.
type channelStub struct{}

func (channelStub) Name() string                                           { return "stub-channel" }
func (channelStub) Seam() plugin.Seam                                      { return plugin.SeamChannel }
func (channelStub) Grants() []plugin.Grant                                 { return []plugin.Grant{plugin.GrantChannelPoll} }
func (channelStub) Tools() []plugin.Tool                                   { return []plugin.Tool{stubTool{}} }
func (channelStub) Start(ctx context.Context, env plugin.ChannelEnv) error { return nil }
func (channelStub) Stop(ctx context.Context) error                         { return nil }
func (channelStub) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	return nil, nil
}

type stubTool struct{}

func (stubTool) Name() string            { return "stub_tool" }
func (stubTool) Effect() plugin.Effect   { return plugin.EffectRead }
func (stubTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (stubTool) Run(ctx context.Context, env plugin.Env, args json.RawMessage) (string, error) {
	return "", nil
}

// TestAdaptSkipsChannelSeam: a seam-channel plugin is consumed by the
// kernel ChannelHost (C3) and must never be Adapt-ed into the tool table,
// even when it carries tools.
func TestAdaptSkipsChannelSeam(t *testing.T) {
	adapted := Adapt([]plugin.Plugin{channelStub{}, hellofs.New()}, func(context.Context) (string, error) {
		return t.TempDir(), nil
	})
	for _, tool := range adapted {
		if tool.Spec().Name == "stub_tool" || tool.Spec().Keywords[0] == "stub-channel" {
			t.Fatalf("channel plugin leaked into the tool table: %+v", tool.Spec())
		}
	}
	if len(adapted) != 1 || adapted[0].Spec().Name != "hello_stat" {
		t.Fatalf("hello-fs tool missing after channel skip: adapted = %#v", adapted)
	}
}
