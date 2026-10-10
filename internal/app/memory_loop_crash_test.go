package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/config"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	laputaevolution "github.com/dashimaki/laputa/evolution"
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
	saveMemoryLoopDevelopmentEvidence(t, f, session, "before-crash", before)
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
	assertMemoryLoopTriggerReason(t, f, session, "blocked:unknown_outcome")
	assertMemoryLoopNoExtraRequests(t, f, 0, 11*time.Second)
	var after memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &after)
	if after.Cognition.ActiveRunID != admitted.Cognition.ActiveRunID || after.Cognition.Watermark != 0 || after.Cognition.Phase != "blocked" || after.Cognition.BlockReason != "unknown_outcome" {
		t.Fatalf("interrupted window was reset or advanced: before=%+v after=%+v", admitted, after)
	}
	source, err := f.Wait(ctx, "canonical", primary)
	if err != nil || source.RecordID != before.RecordID || source.IngestionID != before.IngestionID || source.Revision != before.Revision || source.CanonicalCount != 1 {
		t.Fatalf("interrupted inference duplicated/lost canonical source: %+v %v", source, err)
	}
	t.Logf("real interrupted workflow=%s source=%d window=[%d,%d] processes=%d->%d engine=%s watermark=%d requests=%d", workflow.ID, source.CaptureSeq, window.After, window.Through, oldPID, newPID, workflow.EngineStatus, after.Cognition.Watermark, len(f.ModelRequests()))
	saveMemoryLoopDevelopmentEvidence(t, f, session, "after-crash", source)
}

type memoryLoopCaptureHandshake struct {
	PID        int    `json:"pid"`
	Ingestion  string `json:"ingestion_id"`
	CaptureSeq uint64 `json:"capture_seq"`
	RunID      string `json:"run_id,omitempty"`
	EventID    string `json:"event_id,omitempty"`
}

type memoryLoopCaptureAttemptGate struct {
	runtime.CognitiveCaptureSink
	path string
}

func memoryLoopCaptureAttemptHandshake(path string) AppOption {
	return func(options *appOptions) {
		options.cognitiveCaptureSinkWrapper = func(sink runtime.CognitiveCaptureSink) runtime.CognitiveCaptureSink {
			return &memoryLoopCaptureAttemptGate{CognitiveCaptureSink: sink, path: path}
		}
	}
}

func (g *memoryLoopCaptureAttemptGate) LookupCapture(ctx context.Context, capture runtime.CognitiveCapture) (runtime.CognitiveCaptureReceipt, bool, error) {
	lookup, ok := g.CognitiveCaptureSink.(interface {
		LookupCapture(context.Context, runtime.CognitiveCapture) (runtime.CognitiveCaptureReceipt, bool, error)
	})
	if !ok {
		return runtime.CognitiveCaptureReceipt{}, false, nil
	}
	return lookup.LookupCapture(ctx, capture)
}

func (g *memoryLoopCaptureAttemptGate) Capture(ctx context.Context, capture runtime.CognitiveCapture) (runtime.CognitiveCaptureReceipt, error) {
	claim, err := os.OpenFile(g.path+".claimed", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return g.CognitiveCaptureSink.Capture(ctx, capture)
	}
	if err != nil {
		return runtime.CognitiveCaptureReceipt{}, fmt.Errorf("claim C01 handshake: %w", err)
	}
	if err := claim.Close(); err != nil {
		return runtime.CognitiveCaptureReceipt{}, fmt.Errorf("close C01 handshake claim: %w", err)
	}
	raw, err := json.Marshal(memoryLoopCaptureHandshake{PID: os.Getpid(), RunID: string(capture.RunID), EventID: capture.EventID})
	if err != nil {
		return runtime.CognitiveCaptureReceipt{}, fmt.Errorf("encode C01 handshake: %w", err)
	}
	temporary := fmt.Sprintf("%s.tmp-%d", g.path, os.Getpid())
	if err := os.WriteFile(temporary, raw, 0600); err != nil {
		return runtime.CognitiveCaptureReceipt{}, fmt.Errorf("write C01 handshake: %w", err)
	}
	if err := os.Rename(temporary, g.path); err != nil {
		return runtime.CognitiveCaptureReceipt{}, fmt.Errorf("publish C01 handshake: %w", err)
	}
	<-ctx.Done() // Test parent kills this process after observing the durable terminal.
	return runtime.CognitiveCaptureReceipt{}, ctx.Err()
}

func memoryLoopCaptureReceiptHandshake(path string) AppOption {
	return func(options *appOptions) {
		options.cognitiveCaptureReceiptHook = func(receipt runtime.CognitiveCaptureReceipt) {
			claim, err := os.OpenFile(path+".claimed", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if errors.Is(err, os.ErrExist) {
				return // Restart already consumed the one-shot crash handshake.
			}
			if err != nil {
				panic(fmt.Errorf("claim C02 handshake: %w", err))
			}
			if err := claim.Close(); err != nil {
				panic(fmt.Errorf("close C02 handshake claim: %w", err))
			}
			raw, err := json.Marshal(memoryLoopCaptureHandshake{PID: os.Getpid(), Ingestion: receipt.IngestionID, CaptureSeq: receipt.Seq})
			if err != nil {
				panic(fmt.Errorf("encode C02 handshake: %w", err))
			}
			temporary := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
			if err := os.WriteFile(temporary, raw, 0600); err != nil {
				panic(fmt.Errorf("write C02 handshake: %w", err))
			}
			if err := os.Rename(temporary, path); err != nil {
				panic(fmt.Errorf("publish C02 handshake: %w", err))
			}
			select {} // Parent kills this real process after reading durable state.
		}
	}
}

type memoryLoopEffectReceiptHandshake struct {
	PID         int    `json:"pid"`
	OperationID string `json:"operation_id"`
	TargetRef   string `json:"target_ref"`
	Revision    uint64 `json:"revision"`
	Status      string `json:"status"`
}

// memoryLoopEffectReceiptGate waits after the actual domain has committed and
// returned its atomic receipt, but before the strategy's caller can observe it.
type memoryLoopEffectReceiptGate struct {
	laputaevolution.Domain
	path string
}

func memoryLoopEffectReceiptHandshakeOption(path string) AppOption {
	return func(options *appOptions) {
		options.cognitiveDomainWrapper = func(domain laputaevolution.Domain) laputaevolution.Domain {
			return &memoryLoopEffectReceiptGate{Domain: domain, path: path}
		}
	}
}

func (g *memoryLoopEffectReceiptGate) BindForRun(ctx context.Context, binding laputaevolution.RunBinding) (laputaevolution.Domain, error) {
	binder, ok := g.Domain.(interface {
		BindForRun(context.Context, laputaevolution.RunBinding) (laputaevolution.Domain, error)
	})
	if !ok {
		return nil, errors.New("C03 domain does not support run binding")
	}
	bound, err := binder.BindForRun(ctx, binding)
	if err != nil {
		return nil, err
	}
	return &memoryLoopEffectReceiptGate{Domain: bound, path: g.path}, nil
}

func (g *memoryLoopEffectReceiptGate) Apply(ctx context.Context, effect laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	receipt, err := g.Domain.Apply(ctx, effect)
	if err != nil || effect.Kind != laputaevolution.KindMemoryMutation || receipt.Status != laputaevolution.StatusApplied {
		return receipt, err
	}
	claim, err := os.OpenFile(g.path+".claimed", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return receipt, nil // Restart consumes the recorded receipt without blocking again.
	}
	if err != nil {
		return laputaevolution.EffectReceipt{}, fmt.Errorf("claim C03 handshake: %w", err)
	}
	if err := claim.Close(); err != nil {
		return laputaevolution.EffectReceipt{}, fmt.Errorf("close C03 handshake claim: %w", err)
	}
	handshake := memoryLoopEffectReceiptHandshake{PID: os.Getpid(), OperationID: receipt.OperationID, TargetRef: receipt.TargetRef, Revision: receipt.Revision, Status: string(receipt.Status)}
	raw, err := json.Marshal(handshake)
	if err != nil {
		return laputaevolution.EffectReceipt{}, fmt.Errorf("encode C03 handshake: %w", err)
	}
	temporary := fmt.Sprintf("%s.tmp-%d", g.path, os.Getpid())
	if err := os.WriteFile(temporary, raw, 0600); err != nil {
		return laputaevolution.EffectReceipt{}, fmt.Errorf("write C03 handshake: %w", err)
	}
	if err := os.Rename(temporary, g.path); err != nil {
		return laputaevolution.EffectReceipt{}, fmt.Errorf("publish C03 handshake: %w", err)
	}
	<-ctx.Done() // The parent kills this process while the caller lacks the receipt.
	return laputaevolution.EffectReceipt{}, ctx.Err()
}

// C03 proves that canonical memory and its atomic receipt are durable while
// the DIVA caller is still blocked before acknowledgement of Apply's result.
func TestMemoryLoopCrashC03CanonicalBeforeCallerReceipt(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 130*time.Second)
	defer cancel()
	handshakePath := filepath.Join(t.TempDir(), "effect-receipt.json")
	f := newMemoryLoopFixture(t, memoryLoopOptions{
		ConfigPath: memoryLoopConfig(t), ModelMode: "reflection", EffectHandshakePath: handshakePath,
	})
	if err := f.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	fact := memoryLoopRandomFact(t)
	runID := memoryLoopTurn(t, f, session, fact)
	primary, err := f.Wait(ctx, "canonical", runID)
	if err != nil {
		t.Fatal(err)
	}
	handshake := waitMemoryLoopEffectHandshake(t, handshakePath, 40*time.Second)
	if f.remote == nil || handshake.PID != f.remote.pid || handshake.OperationID == "" || handshake.TargetRef == "" || handshake.Revision == 0 || handshake.Status != string(laputaevolution.StatusApplied) {
		t.Fatalf("C03 handshake does not identify a committed memory effect: %+v", handshake)
	}
	var receipt struct {
		OperationID string `json:"operation_id"`
		TargetRef   string `json:"target_ref"`
		Revision    uint64 `json:"revision"`
		Status      string `json:"status"`
	}
	memoryLoopAction(t, f, "diva.cognitive.memory.receipt", map[string]any{"session_id": session, "operation_id": handshake.OperationID}, &receipt)
	if receipt.OperationID != handshake.OperationID || receipt.TargetRef != handshake.TargetRef || receipt.Revision != handshake.Revision || receipt.Status != handshake.Status {
		t.Fatalf("public atomic receipt differs from the unreturned Apply receipt: handshake=%+v public=%+v", handshake, receipt)
	}
	canonical, err := f.readOnlyDB("garden", "palace", "palace.db", "canonical.sqlite3")
	if err != nil {
		t.Fatal(err)
	}
	var body string
	var revision uint64
	var count int
	if err := canonical.QueryRowContext(ctx, `SELECT version,content FROM memories WHERE id=?`, handshake.TargetRef).Scan(&revision, &body); err != nil {
		_ = canonical.Close()
		t.Fatal(err)
	}
	if err := canonical.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`).Scan(&count); err != nil {
		_ = canonical.Close()
		t.Fatal(err)
	}
	_ = canonical.Close()
	if revision != handshake.Revision || count != 2 || !strings.Contains(body, fact) || primary.CanonicalCount != 1 {
		t.Fatalf("C03 canonical commit mismatch: primary=%+v canonical=%+v revision=%d count=%d body=%q", primary, handshake, revision, count, body)
	}
	var cognitionBefore memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &cognitionBefore)
	if cognitionBefore.Cognition.ActiveRunID == "" || cognitionBefore.Cognition.Phase != "running" {
		t.Fatalf("C03 caller completed before returning from the blocked Apply: %+v", cognitionBefore.Cognition)
	}
	requestsBefore := len(f.ModelRequests())
	oldPID, newPID, err := f.crashAndRestart(ctx)
	if err != nil || oldPID != handshake.PID || newPID <= 0 || newPID == oldPID {
		t.Fatalf("crash/restart at C03 handshake: %d -> %d: %v", oldPID, newPID, err)
	}
	sessionParams, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(ctx, "session/get", sessionParams); err != nil {
		t.Fatalf("rebind user session after process restart: %v", err)
	}
	workflowParams, _ := json.Marshal(map[string]any{"run_id": cognitionBefore.Cognition.ActiveRunID})
	var workflow struct {
		ID             string `json:"id"`
		EngineStatus   string `json:"engine_status"`
		RevisionDigest string `json:"revision_digest"`
	}
	deadline := time.Now().Add(20 * time.Second)
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
			t.Fatalf("C03 interrupted caller was neither resumed safely nor classified unknown: %+v", workflow)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if workflow.ID != cognitionBefore.Cognition.ActiveRunID || workflow.RevisionDigest == "" {
		t.Fatalf("C03 recovery lost original workflow identity: %+v", workflow)
	}
	var afterReceipt struct {
		OperationID string `json:"operation_id"`
		TargetRef   string `json:"target_ref"`
		Revision    uint64 `json:"revision"`
		Status      string `json:"status"`
	}
	memoryLoopAction(t, f, "diva.cognitive.memory.receipt", map[string]any{"session_id": session, "operation_id": handshake.OperationID}, &afterReceipt)
	if afterReceipt != receipt {
		t.Fatalf("C03 original atomic receipt changed after restart: before=%+v after=%+v", receipt, afterReceipt)
	}
	assertMemoryLoopNoExtraRequests(t, f, 0, 11*time.Second)
	if requestsBefore < 3 {
		t.Fatalf("C03 never reached reflection inference before the effect: requests=%d", requestsBefore)
	}
	canonical, err = f.readOnlyDB("garden", "palace", "palace.db", "canonical.sqlite3")
	if err != nil {
		t.Fatal(err)
	}
	defer canonical.Close()
	if err := canonical.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("C03 recovery duplicated canonical memory: count=%d err=%v", count, err)
	}
	t.Logf("C03 operation=%s canonical=%s revision=%d processes=%d->%d workflow=%s requests-before=%d requests-after=0", handshake.OperationID, handshake.TargetRef, handshake.Revision, oldPID, newPID, workflow.EngineStatus, requestsBefore)
}

func waitMemoryLoopEffectHandshake(t *testing.T, path string, timeout time.Duration) memoryLoopEffectReceiptHandshake {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := os.ReadFile(path)
		if err == nil {
			var handshake memoryLoopEffectReceiptHandshake
			if err := json.Unmarshal(raw, &handshake); err != nil {
				t.Fatalf("decode C03 handshake: %v", err)
			}
			return handshake
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read C03 handshake: %v", err)
		}
		select {
		case <-deadline.C:
			t.Fatalf("C03 receipt handshake not reached in %s", timeout)
		case <-ticker.C:
		}
	}
}

// C01 stops at the actual sink boundary: the terminal Journal event exists,
// but the Capture sink has not received it yet.
func TestMemoryLoopCrashC01TerminalBeforeCapture(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	handshakePath := filepath.Join(t.TempDir(), "capture-attempt.json")
	f := newMemoryLoopFixture(t, memoryLoopOptions{
		ConfigPath: memoryLoopConfig(t), ModelMode: "nochange", CaptureAttemptHandshakePath: handshakePath,
	})
	if err := f.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	fact := memoryLoopRandomFact(t)
	runID := memoryLoopTurn(t, f, session, fact)
	handshake := waitMemoryLoopCaptureHandshake(t, handshakePath, 35*time.Second)
	if f.remote == nil || handshake.PID != f.remote.pid || handshake.RunID != runID || handshake.EventID == "" {
		t.Fatalf("pre-capture handshake does not identify the terminal event: receipt=%+v run=%s", handshake, runID)
	}
	terminal, err := f.Wait(ctx, "terminal", runID)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.ProcessID != handshake.PID || !strings.HasSuffix(handshake.EventID, fmt.Sprintf(":%d", terminal.EventSeq)) {
		t.Fatalf("handshake differs from durable terminal event: handshake=%+v terminal=%+v", handshake, terminal)
	}
	if count, err := f.memoryLoopIngestionCountForEvent(ctx, runID, terminal.EventSeq); err != nil || count != 0 {
		t.Fatalf("Capture sink already received C01 event: rows=%d err=%v", count, err)
	}
	cursorBefore, err := f.memoryLoopObserverCursor(ctx, runID)
	if err != nil || cursorBefore >= terminal.EventSeq {
		t.Fatalf("C01 cursor already acknowledged event %d: cursor=%d err=%v", terminal.EventSeq, cursorBefore, err)
	}
	oldPID, newPID, err := f.crashAndRestart(ctx)
	if err != nil || oldPID != handshake.PID || newPID <= 0 || newPID == oldPID {
		t.Fatalf("crash/restart at C01 handshake: %d -> %d: %v", oldPID, newPID, err)
	}
	sessionParams, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(ctx, "session/get", sessionParams); err != nil {
		t.Fatalf("rebind user session after process restart: %v", err)
	}
	after, err := f.Wait(ctx, "canonical", runID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EventSeq != terminal.EventSeq || after.CaptureSeq == 0 || after.IngestionID == "" || after.CanonicalCount != 1 || after.Revision != 1 || !strings.Contains(after.SourceBody, fact) || after.SourceRole != "user" {
		t.Fatalf("C01 replay did not capture the original terminal source exactly once: before=%+v after=%+v", terminal, after)
	}
	count, err := f.memoryLoopIngestionCountForEvent(ctx, runID, terminal.EventSeq)
	if err != nil || count != 1 {
		t.Fatalf("C01 replay produced %d ingestion rows, err=%v", count, err)
	}
	deadline := time.Now().Add(20 * time.Second)
	var cursorAfter uint64
	for {
		cursorAfter, err = f.memoryLoopObserverCursor(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if cursorAfter >= terminal.EventSeq {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("replayed terminal event was not acknowledged: event=%d cursor=%d", terminal.EventSeq, cursorAfter)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Logf("C01 event=%s cursor=%d->%d processes=%d->%d receipt=%s capture=%d canonical=%s revision=%d count=%d", handshake.EventID, cursorBefore, cursorAfter, oldPID, newPID, after.IngestionID, after.CaptureSeq, after.RecordID, after.Revision, after.CanonicalCount)
}

func (f *memoryLoopFixture) memoryLoopIngestionCountForEvent(ctx context.Context, runID string, eventSeq uint64) (int, error) {
	db, err := f.readOnlyDB("garden", "garden.db")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	suffix := fmt.Sprintf("/%s:%d", runID, eventSeq)
	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ingestions WHERE substr(event_id,-length(?))=?`, suffix, suffix).Scan(&count)
	return count, err
}

// C02 kills the real host after the capture receipt is durable but before
// ObserverHost can advance its cursor. The file is a condition handshake
// emitted by the actual receipt callback, not a timing guess.
func TestMemoryLoopCrashC02CaptureBeforeObserverAck(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	handshakePath := filepath.Join(t.TempDir(), "capture-receipt.json")
	f := newMemoryLoopFixture(t, memoryLoopOptions{
		ConfigPath: memoryLoopConfig(t), ModelMode: "nochange", CaptureHandshakePath: handshakePath,
	})
	if err := f.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	fact := memoryLoopRandomFact(t)
	runID := memoryLoopTurn(t, f, session, fact)
	handshake := waitMemoryLoopCaptureHandshake(t, handshakePath, 35*time.Second)
	if f.remote == nil || handshake.PID != f.remote.pid || handshake.Ingestion == "" || handshake.CaptureSeq == 0 {
		t.Fatalf("capture receipt handshake does not identify the owned child: receipt=%+v", handshake)
	}
	before, err := f.Wait(ctx, "canonical", runID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ProcessID != handshake.PID || before.CaptureSeq != handshake.CaptureSeq || before.IngestionID != handshake.Ingestion || before.CanonicalCount != 1 || before.Revision == 0 || !strings.Contains(before.SourceBody, fact) {
		t.Fatalf("C02 durable canonical snapshot differs from receipt: receipt=%+v snapshot=%+v", handshake, before)
	}
	var cognitionBefore memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &cognitionBefore)
	if cognitionBefore.Cognition.Watermark != 0 || cognitionBefore.Cognition.PendingThrough != 0 {
		t.Fatalf("source was processed before ObserverHost ACK: %+v", cognitionBefore.Cognition)
	}
	cursorBefore, err := f.memoryLoopObserverCursor(ctx, runID)
	if err != nil || cursorBefore >= before.EventSeq {
		t.Fatalf("C02 cursor already acknowledged event %d: cursor=%d err=%v", before.EventSeq, cursorBefore, err)
	}
	oldPID, newPID, err := f.crashAndRestart(ctx)
	if err != nil || oldPID != handshake.PID || newPID <= 0 || newPID == oldPID {
		t.Fatalf("crash/restart at C02 handshake: %d -> %d: %v", oldPID, newPID, err)
	}
	sessionParams, _ := json.Marshal(map[string]any{"session_id": session})
	if _, err := f.Call(ctx, "session/get", sessionParams); err != nil {
		t.Fatalf("rebind user session after process restart: %v", err)
	}
	after, err := f.Wait(ctx, "canonical", runID)
	if err != nil {
		t.Fatal(err)
	}
	if after.IngestionID != before.IngestionID || after.CaptureSeq != before.CaptureSeq || after.RecordID != before.RecordID || after.Revision != before.Revision || after.CanonicalCount != 1 || after.SourceHash != before.SourceHash {
		t.Fatalf("C02 redelivery did not reuse original receipt/source: before=%+v after=%+v", before, after)
	}
	deadline := time.Now().Add(20 * time.Second)
	var cursorAfter uint64
	for {
		cursorAfter, err = f.memoryLoopObserverCursor(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if cursorAfter >= after.EventSeq {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("replayed terminal event was not acknowledged: event=%d cursor=%d", after.EventSeq, cursorAfter)
		}
		time.Sleep(10 * time.Millisecond)
	}
	var cognitionAfter memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &cognitionAfter)
	if cognitionAfter.Cognition.Watermark > after.CaptureSeq || cognitionAfter.Cognition.PendingThrough > after.CaptureSeq {
		t.Fatalf("C02 restart advanced beyond committed source: %+v capture=%d", cognitionAfter.Cognition, after.CaptureSeq)
	}
	t.Logf("C02 receipt=%s capture=%d event=%d cursor=%d->%d processes=%d->%d canonical=%s revision=%d watermark=%d pending=%d", after.IngestionID, after.CaptureSeq, after.EventSeq, cursorBefore, cursorAfter, oldPID, newPID, after.RecordID, after.Revision, cognitionAfter.Cognition.Watermark, cognitionAfter.Cognition.PendingThrough)
}

func waitMemoryLoopCaptureHandshake(t *testing.T, path string, timeout time.Duration) memoryLoopCaptureHandshake {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := os.ReadFile(path)
		if err == nil {
			var receipt memoryLoopCaptureHandshake
			if err := json.Unmarshal(raw, &receipt); err != nil {
				t.Fatalf("decode C02 receipt handshake: %v", err)
			}
			return receipt
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read C02 receipt handshake: %v", err)
		}
		select {
		case <-deadline.C:
			t.Fatalf("C02 receipt handshake not reached in %s", timeout)
		case <-ticker.C:
		}
	}
}

func (f *memoryLoopFixture) memoryLoopObserverCursor(ctx context.Context, runID string) (uint64, error) {
	if f.remote != nil {
		var value uint64
		err := f.remote.exchange(ctx, memoryLoopRequest{Op: "observer-cursor", RunID: runID}, &value)
		return value, err
	}
	key := memoryLoopObserverCursorKey(runID)
	value, _, err := f.app.backend.Snapshot().Get(ctx, key)
	if err != nil || len(value) == 0 {
		return 0, err
	}
	return strconv.ParseUint(string(value), 10, 64)
}

func memoryLoopObserverCursorKey(runID string) string {
	sum := sha256.Sum256([]byte(runtime.CognitiveCaptureProviderID + "\x00" + runID))
	return "observer/run/" + hex.EncodeToString(sum[:])
}
