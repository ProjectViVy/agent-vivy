package pluginhost

import (
	"context"
	"os"
	"path/filepath"
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
}
