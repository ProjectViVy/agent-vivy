package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

// These are bytes accepted by the real HTTP ResponseWriter. A complete
// handler observation is not an assertion that the client consumed them.
type memoryLoopModelResponse struct {
	RequestIndex int         `json:"request_index"`
	Status       int         `json:"status"`
	Headers      http.Header `json:"headers"`
	Body         string      `json:"body"`
	WriteError   string      `json:"write_error,omitempty"`
	FinishedAt   time.Time   `json:"finished_at"`
}

type memoryLoopObservedResponseWriter struct {
	http.ResponseWriter
	status int
	body   []byte
	err    error
}

func (w *memoryLoopObservedResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *memoryLoopObservedResponseWriter) Write(raw []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(raw)
	w.body = append(w.body, raw[:n]...)
	if err != nil {
		w.err = err
	}
	return n, err
}

func (w *memoryLoopObservedResponseWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (f *memoryLoopFixture) ModelResponses() []memoryLoopModelResponse {
	if f.remote != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var records []memoryLoopModelResponse
		if err := f.remote.exchange(ctx, memoryLoopRequest{Op: "responses"}, &records); err != nil {
			f.t.Fatalf("actual process responses: %v", err)
		}
		return records
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	copy := append([]memoryLoopModelResponse(nil), f.responses...)
	for i := range copy {
		copy[i].Headers = copy[i].Headers.Clone()
	}
	return copy
}

func TestMemoryLoopEvidenceExportsActualLargeModelResponse(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	if os.Getenv("VIVY_MEMORY_LOOP_EVIDENCE_ROOT") == "" {
		t.Setenv("VIVY_MEMORY_LOOP_EVIDENCE_ROOT", t.TempDir())
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "response-echo"})
	session := memoryLoopSession(t, f)
	fact := memoryLoopRandomFact(t)
	run := memoryLoopTurn(t, f, session, fact)
	terminal, err := f.Wait(context.Background(), "terminal", run)
	if err != nil || terminal.State != "completed" {
		t.Fatalf("actual model response did not complete: %+v %v", terminal, err)
	}
	source, err := f.Wait(context.Background(), "canonical", run)
	if err != nil {
		t.Fatal(err)
	}
	responses := f.ModelResponses()
	if len(responses) != 1 || responses[0].RequestIndex != 0 || responses[0].Status != http.StatusOK || len(responses[0].Body) <= 4096 || !strings.Contains(responses[0].Body, fact) || responses[0].WriteError != "" || responses[0].Headers.Get("Content-Type") == "" {
		t.Fatalf("actual full HTTP response missing or truncated: %+v", responses)
	}
	dir := saveMemoryLoopDevelopmentEvidence(t, f, session, "response-complete", source)
	raw, err := os.ReadFile(filepath.Join(dir, "model-responses.json"))
	var exported []memoryLoopModelResponse
	if err != nil || json.Unmarshal(raw, &exported) != nil || len(exported) != 1 || exported[0].Body != responses[0].Body {
		t.Fatalf("export changed actual response bytes: %v", err)
	}
}
