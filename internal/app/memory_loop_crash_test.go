package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/config"
	genassembly "agent-vivy/internal/generated/assembly"
)

// Test-only crash transport. It kills only the fixture-owned process after
// the caller observes an actual held model request; it never invokes Close,
// clears durable state or mutates the production workflow engine.
func (f *memoryLoopFixture) crashAndRestart(ctx context.Context) (int, int, error) {
	r := f.remote
	if r == nil {
		return 0, 0, errors.New("crash requires an owned subprocess")
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return 0, 0, errors.New("owned subprocess already closed")
	}
	r.closed = true
	err := r.cmd.Process.Kill()
	_ = r.in.Close()
	r.mu.Unlock()
	if err != nil {
		return r.pid, 0, err
	}
	select {
	case processErr := <-r.done:
		var exitErr *exec.ExitError
		if !errors.As(processErr, &exitErr) {
			return r.pid, 0, errors.New("owned crash did not produce a process failure")
		}
	case <-ctx.Done():
		return r.pid, 0, ctx.Err()
	}
	r.closeOnce.Do(func() { r.closeErr = nil })
	// SQLite's existing 30-second lease is deliberately not released by a
	// killed process. Observe its real expiry; never delete a lease row or
	// change the clock to manufacture startup availability.
	cfg, err := config.Load(f.options.ConfigPath)
	if err != nil {
		return r.pid, 0, err
	}
	db, err := sql.Open("sqlite", "file:"+url.PathEscape(filepath.ToSlash(cfg.Storage.SQLite.Path))+"?mode=ro")
	if err != nil {
		return r.pid, 0, err
	}
	var owner string
	var expires int64
	err = db.QueryRowContext(ctx, `SELECT owner,expires_at FROM leases WHERE key='vivy/organism'`).Scan(&owner, &expires)
	_ = db.Close()
	if err != nil {
		return r.pid, 0, err
	}
	if !strings.Contains(owner, ":"+strconv.Itoa(r.pid)+":") {
		return r.pid, 0, errors.New("crashed lease is not owned by fixture process")
	}
	delay := time.Until(time.UnixMilli(expires))
	if delay > 35*time.Second {
		return r.pid, 0, fmt.Errorf("unexpected owned lease duration: %s", delay)
	}
	if delay > 0 {
		f.t.Logf("owned crashed process=%d; waiting actual lease expiry for %s", r.pid, delay)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return r.pid, 0, ctx.Err()
		}
	}
	next, err := startMemoryLoopRemote(ctx, f.options)
	if err != nil {
		return r.pid, 0, err
	}
	f.remote = next
	return r.pid, next.pid, nil
}

// A real interrupted inference must keep the original durable window. This
// pre-effect developer probe is not the six-cut, ten-sample crash matrix.
func TestMemoryLoopInterruptedInferenceKeepsOriginalWindow(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "mission-fence"})
	if err := f.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	primary := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	before, err := f.Wait(ctx, "canonical", primary)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for len(f.ModelRequests()) < 3 {
		if time.Now().After(deadline) {
			t.Fatal("actual reflection response gate not reached")
		}
		time.Sleep(10 * time.Millisecond)
	}
	requests := f.ModelRequests()
	window := memoryLoopInferenceWindow(t, requests[2], "reflect")
	var admitted memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &admitted)
	if admitted.Cognition.ActiveRunID == "" || admitted.Cognition.Watermark != 0 || window.After != 0 || window.Through != before.CaptureSeq {
		t.Fatalf("original admission missing: %+v window=%+v", admitted, window)
	}
	oldPID, newPID, err := f.crashAndRestart(ctx)
	if err != nil || oldPID <= 0 || newPID <= 0 || oldPID == newPID {
		t.Fatalf("actual process crash/restart: %d -> %d: %v", oldPID, newPID, err)
	}
	params, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(ctx, "session/get", params); err != nil {
		t.Fatal(err)
	}
	workflowParams, _ := json.Marshal(map[string]any{"run_id": admitted.Cognition.ActiveRunID})
	var workflow struct {
		ID             string `json:"id"`
		EngineStatus   string `json:"engine_status"`
		RevisionDigest string `json:"revision_digest"`
	}
	deadline = time.Now().Add(15 * time.Second)
	for {
		raw, err := f.Call(ctx, "workflow/get", workflowParams)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &workflow); err != nil {
			t.Fatal(err)
		}
		if workflow.EngineStatus == "recovery_required" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("interrupted native engine not classified: %+v", workflow)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if workflow.ID != admitted.Cognition.ActiveRunID || workflow.RevisionDigest == "" {
		t.Fatalf("durable original workflow identity missing: %+v", workflow)
	}
	// Manual requests and actual automatic ticks both keep the same identity.
	assertMemoryLoopTriggerReason(t, f, session, "active")
	assertMemoryLoopNoExtraRequests(t, f, 0, 11*time.Second)
	var after memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &after)
	if after.Cognition.ActiveRunID != admitted.Cognition.ActiveRunID || after.Cognition.Watermark != 0 {
		t.Fatalf("interrupted window was reset or advanced: before=%+v after=%+v", admitted, after)
	}
	source, err := f.Wait(ctx, "canonical", primary)
	if err != nil || source.RecordID != before.RecordID || source.IngestionID != before.IngestionID || source.Revision != before.Revision || source.CanonicalCount != 1 {
		t.Fatalf("interrupted inference duplicated/lost canonical source: %+v %v", source, err)
	}
	t.Logf("real interrupted workflow=%s source=%d window=[%d,%d] processes=%d->%d engine=%s watermark=%d requests=%d", workflow.ID, source.CaptureSeq, window.After, window.Through, oldPID, newPID, workflow.EngineStatus, after.Cognition.Watermark, len(f.ModelRequests()))
}
