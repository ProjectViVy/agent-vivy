package actmem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
)

type modelRequest struct {
	Op          string `json:"op"`
	OperationID string `json:"operation_id"`
	Text        string `json:"text"`
	Revision    uint64 `json:"revision"`
	Boundary    string `json:"boundary"`
}

type modelRecap struct {
	root  string
	store *Store
}

var modelBinding = RecapBinding{SubjectID: "model", DomainID: "model"}

func (b *modelRecap) reopen() error {
	if b.store != nil {
		if err := b.store.Close(); err != nil {
			return err
		}
	}
	var err error
	b.store, err = Open(b.root)
	return err
}

func modelError(err error) map[string]any {
	code := ""
	if err != nil {
		code = CodeOf(err)
		if code != "recap_commit_unknown" && errors.Is(err, context.Canceled) {
			code = "context_canceled"
		}
	}
	return map[string]any{"error": code}
}

func (b *modelRecap) step(q modelRequest) map[string]any {
	ctx := context.Background()
	var head RecapHead
	var err error
	switch q.Op {
	case "reopen":
		return modelError(b.reopen())
	case "read":
		head, err = b.store.ReadRecap(ctx, modelBinding)
	case "commit":
		head, err = b.store.CommitRecap(ctx, RecapCommit{Binding: modelBinding, BaseRevision: q.Revision, OperationID: q.OperationID, Text: q.Text, Cause: "edit"})
	case "cancel_recovery":
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		fired := false
		b.store.recapFault = func(point string) error {
			if point == q.Boundary {
				fired = true
				cancel()
				if point == "recovery_after_replace" {
					return cancelCtx.Err()
				}
			}
			return nil
		}
		_, err = b.store.ReadRecap(cancelCtx, modelBinding)
		b.store.recapFault = nil
		if !fired {
			return map[string]any{"error": "fault_boundary_not_reached"}
		}
		return modelError(err)
	default:
		return map[string]any{"error": "invalid_operation"}
	}
	if err != nil {
		return modelError(err)
	}
	return map[string]any{"error": "", "revision": head.Revision, "text": head.Text}
}

func TestMemoryModelBridge(t *testing.T) {
	root := os.Getenv("VIVY_MEMORY_MODEL_ROOT")
	if root == "" {
		return
	}
	b := modelRecap{root: root}
	if err := b.reopen(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	decoder.DisallowUnknownFields()
	for {
		var q modelRequest
		if err := decoder.Decode(&q); err != nil {
			if errors.Is(err, io.EOF) {
				err = b.store.Close()
				if err == nil {
					os.Exit(0)
				}
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		response := map[string]any{"error": ""}
		if q.Op == "restart" {
			response = modelError(b.store.Close())
		} else if q.Op != "crash" {
			response = b.step(q)
		}
		if err := encoder.Encode(response); err != nil {
			os.Exit(2)
		}
		if q.Op == "crash" {
			os.Exit(23)
		}
		if q.Op == "restart" {
			os.Exit(0)
		}
	}
}

func TestMemoryModelNativeRecoveryCancellation(t *testing.T) {
	for _, boundary := range []string{"recovery_before_replace", "recovery_after_replace"} {
		t.Run(boundary, func(t *testing.T) {
			b := modelRecap{root: t.TempDir()}
			if err := b.reopen(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = b.store.Close() })
			if got := b.step(modelRequest{Op: "commit", OperationID: "one", Text: "kept"}); got["error"] != "" {
				t.Fatalf("seed: %v", got)
			}
			if err := b.reopen(); err != nil {
				t.Fatal(err)
			}
			want := "context_canceled"
			if boundary == "recovery_after_replace" {
				want = "recap_commit_unknown"
			}
			if got := b.step(modelRequest{Op: "cancel_recovery", Boundary: boundary}); got["error"] != want {
				t.Errorf("cancellation: %v want %s", got, want)
			}
			if boundary == "recovery_after_replace" {
				if got := b.step(modelRequest{Op: "read"}); got["error"] != "recap_commit_unknown" {
					t.Errorf("uncertain owner served a head: %v", got)
				}
				if got := b.step(modelRequest{Op: "commit", OperationID: "two", Text: "next", Revision: 1}); got["error"] != "recap_commit_unknown" {
					t.Errorf("uncertain owner accepted a mutation: %v", got)
				}
				if err := b.reopen(); err != nil {
					t.Fatal(err)
				}
			}
			if got := b.step(modelRequest{Op: "read"}); got["error"] != "" || got["revision"] != uint64(1) || got["text"] != "kept" {
				t.Fatalf("recovery: %v", got)
			}
			if got := b.step(modelRequest{Op: "commit", OperationID: "two", Text: "next", Revision: 1}); got["error"] != "" || got["revision"] != uint64(2) {
				t.Fatalf("commit after recovery: %v", got)
			}
		})
	}
}
