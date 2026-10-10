package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"
)

const memoryLoopProtocol = "MEMORY_LOOP_REPLY "

type memoryLoopRequest struct {
	Op       string          `json:"op"`
	Method   string          `json:"method,omitempty"`
	Params   json.RawMessage `json:"params,omitempty"`
	Stage    string          `json:"stage,omitempty"`
	RunID    string          `json:"run_id,omitempty"`
	Deadline int64           `json:"deadline"`
}
type memoryLoopResponse struct {
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
}
type memoryLoopRemote struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	in        io.WriteCloser
	out       *bufio.Scanner
	done      chan error
	closed    bool
	pid       int
	closeOnce sync.Once
	closeErr  error
}

// This test transport forwards fixture operations to the same actual App
// and control handlers in an owned child; it adds no product runtime.
func startMemoryLoopRemote(ctx context.Context, options memoryLoopOptions) (*memoryLoopRemote, error) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestMemoryLoopProcessServer$", "-test.v")
	cmd.Env = append(os.Environ(), "VIVY_MEMORY_LOOP_PROCESS_SERVER=1", "VIVY_MEMORY_LOOP_PROCESS_CONFIG="+options.ConfigPath, "VIVY_MEMORY_LOOP_PROCESS_MODEL="+options.ModelMode, fmt.Sprintf("VIVY_MEMORY_LOOP_RECALL_DISABLED=%t", options.RecallDisabled))
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := os.CreateTemp(filepath.Dir(options.ConfigPath), "process-stderr-*.log")
	if err != nil {
		return nil, err
	}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = stderr.Close()
		return nil, err
	}
	r := &memoryLoopRemote{cmd: cmd, in: in, out: bufio.NewScanner(out), done: make(chan error, 1)}
	r.out.Buffer(make([]byte, 4096), 4<<20)
	go func() {
		err := cmd.Wait()
		_ = stderr.Close()
		if err != nil {
			// Preserve owned synthetic-child diagnostics before TempDir cleanup,
			// including race reports otherwise absent from the JSON test log.
			if trace, openErr := os.Open(stderr.Name()); openErr == nil {
				_, _ = io.Copy(os.Stderr, io.LimitReader(trace, 64<<10))
				_ = trace.Close()
			}
		}
		r.done <- err
	}()
	var hello struct {
		PID int `json:"pid"`
	}
	if err := r.exchange(ctx, memoryLoopRequest{Op: "hello"}, &hello); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	if hello.PID != cmd.Process.Pid || hello.PID == os.Getpid() {
		_ = cmd.Process.Kill()
		return nil, errors.New("fixture child identity mismatch")
	}
	r.pid = hello.PID
	return r, nil
}

func (r *memoryLoopRemote) exchange(ctx context.Context, req memoryLoopRequest, dst any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("fixture child closed")
	}
	if deadline, ok := ctx.Deadline(); ok {
		req.Deadline = deadline.UnixMilli()
	}
	done := make(chan error, 1)
	go func() {
		if err := json.NewEncoder(r.in).Encode(req); err != nil {
			done <- err
			return
		}
		for r.out.Scan() {
			line := r.out.Text()
			if !strings.HasPrefix(line, memoryLoopProtocol) {
				continue
			}
			var reply memoryLoopResponse
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, memoryLoopProtocol)), &reply); err != nil {
				done <- err
				return
			}
			if reply.Error != "" {
				done <- errors.New(reply.Error)
				return
			}
			if dst == nil {
				done <- nil
				return
			}
			done <- json.Unmarshal(reply.Value, dst)
			return
		}
		err := r.out.Err()
		if err == nil {
			err = io.EOF
		}
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		r.closed = true
		_ = r.cmd.Process.Kill()
		_ = r.in.Close()
		return ctx.Err()
	}
}

func (r *memoryLoopRemote) close(ctx context.Context) error {
	r.closeOnce.Do(func() { r.closeErr = r.closeProcess(ctx) })
	return r.closeErr
}

func (r *memoryLoopRemote) closeProcess(ctx context.Context) error {
	err := r.exchange(ctx, memoryLoopRequest{Op: "close"}, nil)
	r.mu.Lock()
	r.closed = true
	_ = r.in.Close()
	r.mu.Unlock()
	select {
	case processErr := <-r.done:
		if err != nil {
			return err
		}
		return processErr
	case <-ctx.Done():
		_ = r.cmd.Process.Kill()
		return ctx.Err()
	}
}

func TestMemoryLoopProcessServer(t *testing.T) {
	if os.Getenv("VIVY_MEMORY_LOOP_PROCESS_SERVER") == "" {
		t.Skip("owned subprocess helper")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: os.Getenv("VIVY_MEMORY_LOOP_PROCESS_CONFIG"), ModelMode: os.Getenv("VIVY_MEMORY_LOOP_PROCESS_MODEL"), RecallDisabled: os.Getenv("VIVY_MEMORY_LOOP_RECALL_DISABLED") == "true"})
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 4096), 4<<20)
	for in.Scan() {
		var req memoryLoopRequest
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		cancel := func() {}
		if req.Deadline > 0 {
			ctx, cancel = context.WithDeadline(ctx, time.UnixMilli(req.Deadline))
		}
		var value any
		var err error
		switch req.Op {
		case "hello":
			value = map[string]any{"pid": os.Getpid()}
		case "call":
			value, err = f.Call(ctx, req.Method, req.Params)
		case "wait":
			value, err = f.Wait(ctx, req.Stage, req.RunID)
		case "requests":
			value = f.ModelRequests()
		case "recall-queries":
			value = f.RecallQueries()
		case "close":
			trace := time.AfterFunc(2*time.Second, func() {
				_, _ = fmt.Fprintf(os.Stderr, "slow owned fixture Close stage=%s\n", f.app.currentCloseStage())
				_ = pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
			})
			err = f.Close(ctx)
			trace.Stop()
		default:
			err = fmt.Errorf("unknown fixture operation %q", req.Op)
		}
		cancel()
		var reply memoryLoopResponse
		if err != nil {
			reply.Error = err.Error()
		} else {
			reply.Value, err = json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
		}
		raw, err := json.Marshal(reply)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s%s\n", memoryLoopProtocol, raw); err != nil {
			t.Fatal(err)
		}
		if req.Op == "close" {
			return
		}
	}
	if err := in.Err(); err != nil {
		t.Fatal(err)
	}
}
