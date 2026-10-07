package faceprocess

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	plugin "agent-vivy/sdk/port/face"
)

// The selected artifact's no-argument invocation drives the sealed Face
// Provider over real stdio pipes. The app-level launch seam is stubbed so
// these tests observe Run/Main behavior without composing a real App.

type selectedCall struct {
	cfg  config.Config
	opts plugin.Options
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// stubSelectedFace replaces the app-level selected launch for the duration
// of one test. body receives the composed config and forwarded launch
// options; its return becomes the call's result.
func stubSelectedFace(t *testing.T, body func(ctx context.Context, cfg config.Config, opts plugin.Options) (plugin.Result, error)) (*atomic.Int32, *[]selectedCall) {
	t.Helper()
	count := &atomic.Int32{}
	calls := &[]selectedCall{}
	prev := runSelectedFace
	runSelectedFace = func(ctx context.Context, cfg config.Config, opts plugin.Options, _ ...app.AppOption) (plugin.Result, error) {
		count.Add(1)
		*calls = append(*calls, selectedCall{cfg: cfg, opts: opts})
		if body == nil {
			return plugin.Result{Status: "completed"}, nil
		}
		return body(ctx, cfg, opts)
	}
	t.Cleanup(func() { runSelectedFace = prev })
	return count, calls
}

func selectedTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := config.Default()
	cfg.Storage.Backend = "sqlite"
	cfg.Storage.DataDir = ""
	cfg.Storage.SQLite.Path = filepath.Join(root, "vivy.db")
	cfg.Logging.Dir = filepath.Join(root, "logs")
	cfg.Runtime.WorkspaceRoot = filepath.Join(root, "caller-workspace")
	cfg.Runtime.Sandbox.WorkspaceRoot = ""
	cfg.Runtime.World = "local"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("seed config invalid: %v", err)
	}
	return cfg
}

func bootConfigEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "storage:\n  sqlite:\n    path: " + filepath.Join(dir, "boot.db") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIVY_CONFIG", path)
}

func TestSelectedFaceWorkspaceComposition(t *testing.T) {
	count, calls := stubSelectedFace(t, nil)

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = inR.Close(); _ = inW.Close() }()

	result, err := Run(context.Background(), selectedTestConfig(t), plugin.Options{
		In:  inR,
		Out: io.Discard,
		Err: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if got := count.Load(); got != 1 {
		t.Fatalf("selected launch ran %d times, want 1", got)
	}
	got := (*calls)[0]
	if got.cfg.Runtime.World != "sandbox" {
		t.Fatalf("world = %q, want sandbox", got.cfg.Runtime.World)
	}
	fallback := got.cfg.Runtime.WorkspaceRoot
	if got.cfg.Runtime.Sandbox.WorkspaceRoot != fallback {
		t.Fatalf("sandbox workspace root %q diverged from %q", got.cfg.Runtime.Sandbox.WorkspaceRoot, fallback)
	}
	if !strings.HasSuffix(fallback, string(filepath.Separator)+"workspace") {
		t.Fatalf("workspace fallback %q is not the per-instance workspace dir", fallback)
	}
	if !strings.Contains(fallback, string(filepath.Separator)+NamespaceFaceInstances+string(filepath.Separator)) {
		t.Fatalf("workspace fallback %q is not under the face-instances namespace", fallback)
	}
	info, err := os.Stat(fallback)
	if err != nil || !info.IsDir() {
		t.Fatalf("workspace fallback %q not created: %v", fallback, err)
	}
	if got.cfg.Storage.DataDir == "" || filepath.Dir(fallback) != got.cfg.Storage.DataDir {
		t.Fatalf("storage DataDir %q is not the instance root owning %q", got.cfg.Storage.DataDir, fallback)
	}
	if got.opts.In != inR {
		t.Fatal("stdin stream was not forwarded to the provider")
	}
}

func TestFaceCLIModes(t *testing.T) {
	t.Run("help prints usage on stderr without protocol startup", func(t *testing.T) {
		count, _ := stubSelectedFace(t, nil)
		bootConfigEnv(t)
		var out, errOut bytes.Buffer
		if code := Main([]string{"--help"}, nil, nil, &errOut); code != 0 {
			t.Fatalf("--help exit = %d, want 0", code)
		}
		if out.Len() != 0 {
			t.Fatalf("--help wrote %d bytes to stdout", out.Len())
		}
		if !strings.Contains(errOut.String(), "usage:") {
			t.Fatalf("--help stderr lacks usage: %q", errOut.String())
		}
		if count.Load() != 0 {
			t.Fatal("--help started the protocol provider")
		}
	})

	t.Run("inspect-generation does not start the protocol", func(t *testing.T) {
		count, _ := stubSelectedFace(t, nil)
		bootConfigEnv(t)
		var out, errOut bytes.Buffer
		// The test binary embeds no sealed manifest, so inspection reports
		// the error — the contract under test is that no ACP startup ran.
		code := Main([]string{"--inspect-generation"}, nil, nopWriteCloser{&out}, &errOut)
		if code != 1 || !strings.Contains(errOut.String(), "sealed") {
			t.Fatalf("inspect exit = %d, stderr = %q", code, errOut.String())
		}
		if count.Load() != 0 {
			t.Fatal("--inspect-generation started the protocol provider")
		}
	})

	t.Run("invalid arguments exit 2", func(t *testing.T) {
		count, _ := stubSelectedFace(t, nil)
		bootConfigEnv(t)
		var errOut bytes.Buffer
		if code := Main([]string{"bogus"}, nil, nil, &errOut); code != 2 {
			t.Fatalf("invalid-arg exit = %d, want 2", code)
		}
		if !strings.Contains(errOut.String(), "usage:") {
			t.Fatalf("invalid-arg stderr lacks usage: %q", errOut.String())
		}
		if count.Load() != 0 {
			t.Fatal("invalid arguments started the protocol provider")
		}
	})

	t.Run("clean idle EOF exits 0", func(t *testing.T) {
		count, calls := stubSelectedFace(t, nil)
		bootConfigEnv(t)
		inR, inW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := inW.Close(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = inR.Close() }()
		var errOut bytes.Buffer
		if code := Main(nil, inR, nopWriteCloser{io.Discard}, &errOut); code != 0 {
			t.Fatalf("idle EOF exit = %d, want 0; stderr: %s", code, errOut.String())
		}
		if count.Load() != 1 {
			t.Fatalf("protocol provider ran %d times, want 1", count.Load())
		}
		if (*calls)[0].opts.In != inR {
			t.Fatal("process stdin was not forwarded into the provider options")
		}
	})

	t.Run("startup failure exits 1", func(t *testing.T) {
		stubSelectedFace(t, func(context.Context, config.Config, plugin.Options) (plugin.Result, error) {
			return plugin.Result{}, context.DeadlineExceeded
		})
		bootConfigEnv(t)
		inR, inW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = inR.Close(); _ = inW.Close() }()
		var errOut bytes.Buffer
		if code := Main(nil, inR, nopWriteCloser{io.Discard}, &errOut); code != 1 {
			t.Fatalf("startup failure exit = %d, want 1", code)
		}
	})
}

func TestFaceCLIProtocolStdout(t *testing.T) {
	// Protocol stdout carries frames only: bootstrap diagnostics and the
	// configured log sink must never interleave with the provider's stream.
	const frame = "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n"
	stubSelectedFace(t, func(ctx context.Context, cfg config.Config, opts plugin.Options) (plugin.Result, error) {
		_, err := io.WriteString(opts.Out, frame)
		return plugin.Result{Status: "completed"}, err
	})
	bootConfigEnv(t)

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := inW.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = inR.Close() }()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	drained := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(outR)
		drained <- string(data)
	}()
	var errOut bytes.Buffer
	if code := Main(nil, inR, outW, &errOut); code != 0 {
		t.Fatalf("protocol run exit = %d, want 0; stderr: %s", code, errOut.String())
	}
	if got := <-drained; got != frame {
		t.Fatalf("protocol stdout = %q, want exactly the provider frame %q", got, frame)
	}
}

func TestFaceLaunchShutdownDeadline(t *testing.T) {
	t.Run("closing owned pipes unblocks a provider stuck on stdin", func(t *testing.T) {
		started := make(chan struct{})
		stubSelectedFace(t, func(ctx context.Context, cfg config.Config, opts plugin.Options) (plugin.Result, error) {
			close(started)
			buf := make([]byte, 8)
			for {
				if _, err := opts.In.Read(buf); err != nil {
					return plugin.Result{Status: "completed"}, nil
				}
			}
		})
		inR, inW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = inW.Close() }()

		ctx, cancel := context.WithCancel(context.Background())
		type outcome struct {
			result plugin.Result
			err    error
		}
		done := make(chan outcome, 1)
		go func() {
			result, err := Run(ctx, selectedTestConfig(t), plugin.Options{In: inR, Out: io.Discard, Err: io.Discard})
			done <- outcome{result, err}
		}()
		<-started
		cancel()
		select {
		case out := <-done:
			if out.err != nil {
				t.Fatalf("run after cancel: %v", out.err)
			}
			if out.result.Status != "completed" {
				t.Fatalf("status = %q, want completed", out.result.Status)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("teardown did not unblock the provider's stdin read")
		}
	})

	t.Run("budget expiry reports incomplete cleanup", func(t *testing.T) {
		prev := teardownBudget
		teardownBudget = 50 * time.Millisecond
		t.Cleanup(func() { teardownBudget = prev })

		release := make(chan struct{})
		stubSelectedFace(t, func(ctx context.Context, cfg config.Config, opts plugin.Options) (plugin.Result, error) {
			<-release
			return plugin.Result{Status: "completed"}, nil
		})
		defer close(release)

		inR, inW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = inR.Close(); _ = inW.Close() }()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		_, err = Run(ctx, selectedTestConfig(t), plugin.Options{In: inR, Out: io.Discard, Err: io.Discard})
		if err == nil || !strings.Contains(err.Error(), "shutdown") {
			t.Fatalf("budget breach error = %v, want shutdown evidence", err)
		}
	})
}
