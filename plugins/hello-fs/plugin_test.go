package hellofs

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	plugin "agent-vivy/sdk/port/toolworld"
)

type memEnv struct {
	files map[string]string
}

func (m memEnv) Workspace() string { return "workspace" }
func (m memEnv) ModuleID() string  { return "vivy/hello-fs" }

func (m memEnv) OpenRead(path string) (io.ReadCloser, error) {
	body, ok := m.files[path]
	if !ok {
		return nil, plugin.ErrDenied
	}
	return io.NopCloser(strings.NewReader(body)), nil
}

func (m memEnv) OpenWrite(string) (io.WriteCloser, error) {
	return nil, plugin.ErrDenied
}

func (m memEnv) Spawn(context.Context, plugin.SpawnSpec) (plugin.Proc, error) {
	return nil, plugin.ErrDenied
}

func TestHelloStatReadsThroughEnv(t *testing.T) {
	var described module.Module = New()
	if described.Descriptor().Module.ID != "vivy/hello-fs" {
		t.Fatalf("descriptor = %#v", described.Descriptor())
	}
	p := NewProvider()
	tools, err := p.Discover(context.Background(), memEnv{})
	if err != nil || len(tools) != 1 || tools[0].ID != "hello_stat" {
		t.Fatalf("tools = %#v, %v", tools, err)
	}
	got, err := p.Invoke(context.Background(), memEnv{files: map[string]string{"note.txt": "hi"}}, tools[0].ID, []byte(`{"path":"note.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "note.txt 2" {
		t.Fatalf("got %q", got)
	}
}

func TestHelloStatRejectsAbsolutePath(t *testing.T) {
	_, err := NewProvider().Invoke(context.Background(), memEnv{}, "hello_stat", []byte(`{"path":"/etc/passwd"}`))
	if !errors.Is(err, plugin.ErrInvalidArgs) {
		t.Fatalf("err = %v", err)
	}
}
