package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"github.com/dashimaki/laputa/persona"
	"gopkg.in/yaml.v3"
)

type memoryLoopOptions struct {
	ConfigPath string
	ModelMode  string
}
type memoryLoopSnapshot struct {
	RunID            string
	EventSeq         uint64
	IngestionID      string
	CaptureSeq       uint64
	ProcessedThrough uint64
	OperationID      string
	RecordID         string
	Revision         uint64
	CanonicalCount   int
	SourceBody       string
	SourceRole       string
	SourceHash       string
	State            string
}

type memoryLoopFixture struct {
	app      *App
	peer     *controlrpc.Peer
	dataRoot string
	cancel   context.CancelFunc
	mu       sync.Mutex
	requests []json.RawMessage
}

func newMemoryLoopFixture(t *testing.T, opts memoryLoopOptions) *memoryLoopFixture {
	t.Helper()
	if !filepath.IsAbs(opts.ConfigPath) {
		t.Fatal("fixture config must be absolute")
	}
	// Do not fall back to an empty Service or a direct factory fixture.
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Fatal("DIVA generated integration overlay required")
	}
	if opts.ModelMode != "ack" {
		t.Fatal("reflection/recall model modes are pending S06/S08; no synthetic result supplied")
	}
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	f := &memoryLoopFixture{dataRoot: cfg.Storage.DataDir}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, "request read failed", 400)
			return
		}
		var req struct {
			Stream bool `json:"stream"`
		}
		if json.Unmarshal(body, &req) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, append(json.RawMessage(nil), body...))
		f.mu.Unlock()
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"id\":\"memory-loop\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"收到\"},\"finish_reason\":null}]}\n\n")
			_, _ = io.WriteString(w, "data: {\"id\":\"memory-loop\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n")
		} else {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"memory-loop","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"收到"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DEEPSEEK_API_KEY", "synthetic-loopback-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", srv.URL)
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	f.app, err = New(context.Background(), cfg, WithoutEars(), WithoutGateway(), WithInstructionRoot(cfg.Runtime.WorkspaceRoot))
	if err != nil {
		t.Fatalf("real App.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.peer, err = f.app.DialControl(ctx, nil)
	if err != nil {
		_ = f.app.Close()
		t.Fatal(err)
	}
	f.app.StartEmbeddedServices()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := f.Close(ctx); err != nil {
			t.Errorf("fixture close: %v", err)
		}
	})
	return f
}

func (f *memoryLoopFixture) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return f.peer.Call(callCtx, method, params)
}

func (f *memoryLoopFixture) ModelRequests() []json.RawMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]json.RawMessage, len(f.requests))
	for i, req := range f.requests {
		out[i] = append(json.RawMessage(nil), req...)
	}
	return out
}

func (f *memoryLoopFixture) Wait(ctx context.Context, stage, runID string) (memoryLoopSnapshot, error) {
	if stage != "terminal" && stage != "accepted" && stage != "canonical" && stage != "reflected" {
		return memoryLoopSnapshot{}, fmt.Errorf("unobservable stage %q", stage)
	}
	if stage == "reflected" {
		return memoryLoopSnapshot{}, errors.New("reflection observation pending S06")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		snap, ready, err := f.snapshot(ctx, stage, runID)
		if err != nil {
			return snap, err
		}
		if ready {
			return snap, nil
		}
		select {
		case <-ctx.Done():
			return snap, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (f *memoryLoopFixture) snapshot(ctx context.Context, stage, runID string) (memoryLoopSnapshot, bool, error) {
	snap := memoryLoopSnapshot{RunID: runID}
	if stage == "terminal" {
		run, err := f.app.backend.GetRun(ctx, domain.RunID(runID))
		if err != nil {
			return snap, false, err
		}
		snap.State = string(run.Status)
		ready := run.Status == domain.RunCompleted || run.Status == domain.RunFailed || run.Status == domain.RunCancelled
		if !ready {
			return snap, false, nil
		}
		it, err := f.app.backend.Replay(ctx, domain.RunID(runID), 0)
		if err != nil {
			return snap, false, err
		}
		defer it.Close()
		for it.Next() {
			e := it.Value().Event
			if e.Type == domain.EventRunCompleted || e.Type == domain.EventRunFailed || e.Type == domain.EventRunCancelled {
				snap.EventSeq = uint64(e.Seq)
			}
		}
		return snap, true, it.Err()
	}
	// Test-only read-only observation of the real durable source. Role is
	// deliberately left empty if the production source does not expose it.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(f.dataRoot, "garden", "garden.db"))+"?mode=ro")
	if err != nil {
		return snap, false, err
	}
	defer db.Close()
	var memoryID sql.NullString
	terminal, ready, err := f.snapshot(ctx, "terminal", runID)
	if err != nil || !ready {
		return snap, false, err
	}
	snap.EventSeq = terminal.EventSeq
	run, err := f.app.backend.GetRun(ctx, domain.RunID(runID))
	if err != nil {
		return snap, false, err
	}
	// Garden prefixes the stable run/sequence suffix with its encoded
	// trusted binding. Match the suffix and the actual run's session;
	// do not guess that its stored event_id starts with run_id.
	suffix := fmt.Sprintf("/%s:%d", runID, snap.EventSeq)
	err = db.QueryRowContext(ctx, `SELECT rowid,ingestion_id,content,content_hash,status,memory_id FROM ingestions WHERE session_id=? AND substr(event_id,-length(?))=?`, string(run.SessionID), suffix, suffix).Scan(&snap.CaptureSeq, &snap.IngestionID, &snap.SourceBody, &snap.SourceHash, &snap.State, &memoryID)
	if errors.Is(err, sql.ErrNoRows) {
		return snap, false, nil
	}
	if err != nil {
		return snap, false, err
	}
	// Read role from the actual persisted host envelope, never infer it
	// from the test's expected answer or fill a missing role by default.
	var source struct {
		Schema   string `json:"schema"`
		RunID    string `json:"run_id"`
		Messages []struct {
			Role string `json:"role"`
		} `json:"messages"`
	}
	if json.Unmarshal([]byte(snap.SourceBody), &source) == nil && source.Schema == "vivy.conversation-source/v1" && source.RunID == runID {
		for _, message := range source.Messages {
			if message.Role == "user" {
				snap.SourceRole = message.Role
				break
			}
		}
	}
	snap.RecordID = memoryID.String
	if stage == "accepted" {
		return snap, true, nil
	}
	if snap.State == "failed" {
		return snap, false, fmt.Errorf("capture failed: ingestion=%s", snap.IngestionID)
	}
	if snap.State != "completed" {
		return snap, false, nil
	}
	canonical, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(f.dataRoot, "garden", "palace", "palace.db", "canonical.sqlite3"))+"?mode=ro")
	if err != nil {
		return snap, false, err
	}
	defer canonical.Close()
	if err := canonical.QueryRowContext(ctx, `SELECT version FROM memories WHERE id=?`, snap.RecordID).Scan(&snap.Revision); err != nil {
		return snap, false, err
	}
	if err := canonical.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`).Scan(&snap.CanonicalCount); err != nil {
		return snap, false, err
	}
	return snap, true, nil
}

func (f *memoryLoopFixture) Restart(ctx context.Context) error {
	return errors.New("cross-process restart pending S08; use separate close/open smoke explicitly")
}
func (f *memoryLoopFixture) Close(ctx context.Context) error {
	if f.cancel != nil {
		f.cancel()
	}
	if f.peer != nil {
		_ = f.peer.Close()
	}
	if f.app != nil {
		return f.app.CloseContext(ctx)
	}
	return nil
}

func TestMemoryLoopFixtureUsesRealComposition(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("default generation omits DIVA cognition; run the generated integration overlay")
	}
	root := t.TempDir()
	cfg := config.Default()
	cfg.Storage.Backend = "sqlite"
	cfg.Storage.DataDir = root
	cfg.Storage.SQLite.Path = filepath.Join(root, "vivy-test.db")
	cfg.Runtime.WorkspaceRoot = filepath.Join(root, "workspace")
	cfg.Server.Addr = "127.0.0.1:0"
	if err := os.MkdirAll(cfg.Runtime.WorkspaceRoot, 0700); err != nil {
		t.Fatal(err)
	}
	models, err := filepath.Abs(filepath.Join("..", "..", "..", "laputa", "mentle", "models"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(models, "onnx", "model.onnx")); err != nil {
		t.Fatal(err)
	}
	gardenRoot := filepath.Join(root, "garden")
	if err := os.MkdirAll(gardenRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(models, filepath.Join(gardenRoot, "models")); err != nil {
		t.Fatal(err)
	}
	authority, err := persona.Open(filepath.Join(gardenRoot, "persona"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Initialize(persona.Initialization{Identity: "synthetic test agent", Relationship: "test relation", Redline: "test limits", User: "synthetic user", World: "private test world"}, "user", persona.SourceInit, "fixture-init"); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.yaml")
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: configPath, ModelMode: "ack"})
	session, err := f.Call(context.Background(), "session/create", json.RawMessage(`{"title":"memory loop fixture"}`))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(session, &created); err != nil || created.ID == "" {
		t.Fatalf("create: %s %v", session, err)
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	fact := "synthetic random fact " + hex.EncodeToString(nonce)
	args, _ := json.Marshal(map[string]any{"session_id": created.ID, "text": fact})
	started, err := f.Call(context.Background(), "turn/start", args)
	if err != nil {
		t.Fatal(err)
	}
	var run struct {
		ID string `json:"run_id"`
	}
	if err := json.Unmarshal(started, &run); err != nil || run.ID == "" {
		t.Fatalf("start: %s %v", started, err)
	}
	terminal, err := f.Wait(context.Background(), "terminal", run.ID)
	if err != nil || terminal.State != "completed" || terminal.EventSeq == 0 {
		t.Fatalf("terminal: %+v %v", terminal, err)
	}
	accepted, err := f.Wait(context.Background(), "accepted", run.ID)
	if err != nil || accepted.IngestionID == "" || accepted.CaptureSeq == 0 {
		t.Fatalf("accepted: %+v %v", accepted, err)
	}
	canonical, err := f.Wait(context.Background(), "canonical", run.ID)
	if err != nil || canonical.RecordID == "" || canonical.Revision == 0 || canonical.CanonicalCount != 1 {
		t.Fatalf("canonical: %+v %v", canonical, err)
	}
	if runtimeGenerationID(*f.app.assembly) == "" {
		t.Fatal("DIVA integration fixture has no validated generation identity")
	}
	requests := f.ModelRequests()
	if len(requests) == 0 || !strings.Contains(string(requests[0]), fact) {
		t.Fatal("real provider request not recorded")
	}
	if !strings.Contains(canonical.SourceBody, fact) || canonical.SourceRole != "user" {
		t.Fatal("user-only fact or its persisted role lost at real capture boundary")
	}
	t.Logf("real composition run=%s event=%d ingestion=%s capture=%d record=%s revision=%d source=%q", run.ID, terminal.EventSeq, accepted.IngestionID, accepted.CaptureSeq, canonical.RecordID, canonical.Revision, canonical.SourceBody)
	if evidenceDir := os.Getenv("VIVY_MEMORY_LOOP_EVIDENCE_DIR"); evidenceDir != "" {
		if !filepath.IsAbs(evidenceDir) {
			t.Fatal("evidence output must be absolute")
		}
		if err := os.MkdirAll(evidenceDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, "captured-source.txt"), []byte(canonical.SourceBody), 0600); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.MarshalIndent(requests, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, "model-requests.json"), encoded, 0600); err != nil {
			t.Fatal(err)
		}
		snapshot, err := json.MarshalIndent(map[string]any{"generation_id": runtimeGenerationID(*f.app.assembly), "test_root": root, "terminal": terminal, "accepted": accepted, "canonical": canonical}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, "snapshot.json"), snapshot, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.Close(ctx); err != nil {
		t.Fatal(err)
	}
	reopened := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: configPath, ModelMode: "ack"})
	after, err := reopened.Wait(context.Background(), "canonical", run.ID)
	if err != nil || after.RecordID != canonical.RecordID || after.Revision != canonical.Revision || after.CanonicalCount != canonical.CanonicalCount {
		t.Fatalf("reopen: %+v %v", after, err)
	}
	if len(reopened.ModelRequests()) != 0 {
		t.Fatal("request recorder leaked across host instances")
	}
}
