package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

type memoryLoopProcessReceipt struct {
	RunID    string             `json:"run_id"`
	Fact     string             `json:"fact"`
	Snapshot memoryLoopSnapshot `json:"snapshot"`
	PID      int                `json:"pid"`
}

// The second App is in a distinct OS process after the first exits. No DB
// rows are seeded; both processes use the full generated composition.
func TestMemoryLoopProcessRestart(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	cfg := memoryLoopConfig(t)
	proof := filepath.Join(filepath.Dir(cfg), "process-receipt.json")
	pids := make(map[int]bool)
	for _, phase := range []string{"capture", "reopen"} {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMemoryLoopProcessHelper$", "-test.v")
		cmd.Env = append(os.Environ(), "VIVY_MEMORY_LOOP_PROCESS_PHASE="+phase, "VIVY_MEMORY_LOOP_PROCESS_CONFIG="+cfg, "VIVY_MEMORY_LOOP_PROCESS_PROOF="+proof)
		out, err := cmd.CombinedOutput()
		cancel()
		t.Logf("%s: %s", phase, out)
		if err != nil {
			t.Fatalf("%s process: %v", phase, err)
		}
		raw, err := os.ReadFile(proof)
		if err != nil {
			t.Fatal(err)
		}
		var receipt memoryLoopProcessReceipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.PID <= 0 || receipt.PID == os.Getpid() || pids[receipt.PID] {
			t.Fatalf("process identity not distinct: %+v", receipt)
		}
		pids[receipt.PID] = true
	}
}

func TestMemoryLoopProcessHelper(t *testing.T) {
	phase := os.Getenv("VIVY_MEMORY_LOOP_PROCESS_PHASE")
	if phase == "" {
		t.Skip("subprocess helper")
	}
	proof := os.Getenv("VIVY_MEMORY_LOOP_PROCESS_PROOF")
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: os.Getenv("VIVY_MEMORY_LOOP_PROCESS_CONFIG"), ModelMode: "ack"})
	var receipt memoryLoopProcessReceipt
	if phase == "capture" {
		raw, err := f.Call(context.Background(), "session/create", json.RawMessage(`{"title":"process memory proof"}`))
		if err != nil {
			t.Fatal(err)
		}
		var session struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &session); err != nil || session.ID == "" {
			t.Fatalf("session: %s %v", raw, err)
		}
		nonce := make([]byte, 16)
		if _, err := rand.Read(nonce); err != nil {
			t.Fatal(err)
		}
		receipt.Fact = "process fact " + hex.EncodeToString(nonce)
		args, _ := json.Marshal(map[string]any{"session_id": session.ID, "text": receipt.Fact})
		raw, err = f.Call(context.Background(), "turn/start", args)
		if err != nil {
			t.Fatal(err)
		}
		var run struct {
			ID string `json:"run_id"`
		}
		if err := json.Unmarshal(raw, &run); err != nil || run.ID == "" {
			t.Fatalf("run: %s %v", raw, err)
		}
		receipt.RunID = run.ID
	} else if phase == "reopen" {
		raw, err := os.ReadFile(proof)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Fatalf("unknown phase %q", phase)
	}
	snap, err := f.Wait(context.Background(), "canonical", receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snap.SourceBody, receipt.Fact) || snap.SourceRole != "user" || snap.CanonicalCount != 1 {
		t.Fatalf("durable source: %+v", snap)
	}
	if phase == "reopen" && (snap.RecordID != receipt.Snapshot.RecordID || snap.IngestionID != receipt.Snapshot.IngestionID || snap.Revision != receipt.Snapshot.Revision) {
		t.Fatalf("identity/revision changed across process: before=%+v after=%+v", receipt.Snapshot, snap)
	}
	receipt.Snapshot, receipt.PID = snap, os.Getpid()
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proof, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
