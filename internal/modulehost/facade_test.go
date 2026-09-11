package modulehost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/toolworld"
)

type fakeSecretReader struct {
	value string
	err   error
}

func (reader fakeSecretReader) ReadSecret(context.Context, string) (string, error) {
	return reader.value, reader.err
}

type fakeSpawner struct {
	calls int
}

func (spawner *fakeSpawner) Spawn(context.Context, toolworld.SpawnSpec) (toolworld.Proc, error) {
	spawner.calls++
	return nil, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type fakeRPC struct {
	calls int
}

func (rpc *fakeRPC) Call(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
	rpc.calls++
	return json.RawMessage(`{"method":"` + method + `"}`), nil
}

func TestFacadeRejectsWorkspacePathEscape(t *testing.T) {
	root := t.TempDir()
	facade, err := New(Config{
		ModuleID:      "acme/plugin",
		InstanceID:    "instance-1",
		WorkspaceRoot: root,
		Grants: []module.GrantBinding{{
			Name:        module.GrantFSRead,
			Constraints: map[string][]string{"roots": {"allowed"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := facade.OpenRead("../outside.txt"); !errors.Is(err, ErrDenied) {
		t.Fatalf("OpenRead escape error = %v, want ErrDenied", err)
	}
	if _, err := facade.OpenRead("other/file.txt"); !errors.Is(err, ErrDenied) {
		t.Fatalf("OpenRead root escape error = %v, want ErrDenied", err)
	}
}

func TestFacadeRejectsSymlinkWorkspaceEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}
	facade, err := New(Config{
		ModuleID:      "acme/plugin",
		InstanceID:    "instance-1",
		WorkspaceRoot: root,
		Grants:        []module.GrantBinding{{Name: module.GrantFSRead}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := facade.OpenRead("link/secret.txt"); !errors.Is(err, ErrDenied) {
		t.Fatalf("OpenRead symlink escape error = %v, want ErrDenied", err)
	}
}

func TestFacadeRejectsUndeclaredSecretAndRedactsFailure(t *testing.T) {
	sentinel := errors.New("credential backend exploded with TOP-SECRET")
	facade, err := New(Config{
		ModuleID:   "acme/plugin",
		InstanceID: "instance-1",
		Grants: []module.GrantBinding{{
			Name:        module.GrantSecretRead,
			Constraints: map[string][]string{"names": {"ALLOWED_KEY"}},
		}},
		Secrets: fakeSecretReader{value: "TOP-SECRET", err: sentinel},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := facade.Secret(context.Background(), "OTHER_KEY"); !errors.Is(err, ErrDenied) {
		t.Fatalf("Secret undeclared error = %v, want ErrDenied", err)
	}
	_, err = facade.Secret(context.Background(), "ALLOWED_KEY")
	if !errors.Is(err, sentinel) {
		t.Fatalf("Secret error lost cause chain: %v", err)
	}
	if strings.Contains(err.Error(), "TOP-SECRET") {
		t.Fatalf("Secret error leaked secret material: %v", err)
	}
}

func TestFacadeRejectsUnapprovedProcessSpawn(t *testing.T) {
	spawner := &fakeSpawner{}
	facade, err := New(Config{
		ModuleID:   "acme/plugin",
		InstanceID: "instance-1",
		Grants: []module.GrantBinding{{
			Name:        module.GrantProcSpawn,
			Constraints: map[string][]string{"commands": {"git"}},
		}},
		Spawner: spawner,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := facade.Spawn(context.Background(), toolworld.SpawnSpec{Command: "sh"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("Spawn error = %v, want ErrDenied", err)
	}
	if spawner.calls != 0 {
		t.Fatalf("denied spawn reached backend %d times", spawner.calls)
	}
}

func TestFacadeRejectsHTTPEgressEscape(t *testing.T) {
	calls := 0
	facade, err := New(Config{
		ModuleID:   "acme/plugin",
		InstanceID: "instance-1",
		Grants: []module.GrantBinding{{
			Name: module.GrantNetClient,
			Constraints: map[string][]string{
				"hosts":   {"api.example.com"},
				"schemes": {"https"},
				"ports":   {"443"},
			},
		}},
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, rawURL := range []string{
		"http://api.example.com/v1",
		"https://evil.example.com/v1",
		"https://api.example.com:8443/v1",
	} {
		request, requestErr := http.NewRequest(http.MethodGet, rawURL, nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		if _, err := facade.Do(context.Background(), request); !errors.Is(err, ErrDenied) {
			t.Fatalf("Do(%s) error = %v, want ErrDenied", rawURL, err)
		}
	}
	if calls != 0 {
		t.Fatalf("denied egress reached transport %d times", calls)
	}
}

func TestRPCGrantCannotBeUsedAsGeneralNetwork(t *testing.T) {
	rpc := &fakeRPC{}
	calls := 0
	facade, err := New(Config{
		ModuleID:   "acme/plugin",
		InstanceID: "instance-1",
		Grants:     []module.GrantBinding{{Name: module.GrantRPCClient}},
		RPC:        rpc,
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("unexpected network")
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := facade.Do(context.Background(), request); !errors.Is(err, ErrDenied) {
		t.Fatalf("Do with rpc.client error = %v, want ErrDenied", err)
	}
	if calls != 0 {
		t.Fatalf("rpc.client escaped through HTTP transport %d times", calls)
	}
	if _, err := facade.CallRPC(context.Background(), "status.read", nil); err != nil {
		t.Fatal(err)
	}
	if rpc.calls != 1 {
		t.Fatalf("RPC calls = %d, want 1", rpc.calls)
	}
}
