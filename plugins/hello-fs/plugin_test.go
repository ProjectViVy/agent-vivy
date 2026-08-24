package hellofs

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"agent-vivy/sdk/plugin"
)

type memEnv struct {
	files map[string]string
}

func (m memEnv) Workspace() string { return "workspace" }

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

func TestHelloStatReadsThroughEnv(t *testing.T) {
	p := New()
	if p.Name() != "hello-fs" || p.Seam() != plugin.SeamToolWorld {
		t.Fatalf("identity = %s %s", p.Name(), p.Seam())
	}
	tools := p.Tools()
	if len(tools) != 1 || tools[0].Name() != "hello_stat" {
		t.Fatalf("tools = %#v", tools)
	}
	got, err := tools[0].Run(context.Background(), memEnv{files: map[string]string{"note.txt": "hi"}}, []byte(`{"path":"note.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "note.txt 2" {
		t.Fatalf("got %q", got)
	}
}

func TestHelloStatRejectsAbsolutePath(t *testing.T) {
	_, err := statTool{}.Run(context.Background(), memEnv{}, []byte(`{"path":"/etc/passwd"}`))
	if !errors.Is(err, plugin.ErrInvalidArgs) {
		t.Fatalf("err = %v", err)
	}
}
