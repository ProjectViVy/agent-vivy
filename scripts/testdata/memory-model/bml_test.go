package bml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"
)

type modelRequest struct {
	Op          string `json:"op"`
	ID          string `json:"id"`
	OperationID string `json:"operation_id"`
	Text        string `json:"text"`
	Revision    int64  `json:"revision"`
	Foreign     bool   `json:"foreign"`
}

type modelBML struct {
	root  string
	store *Store
}

func (b *modelBML) reopen() error {
	if b.store != nil {
		if err := b.store.Close(); err != nil {
			return err
		}
	}
	var err error
	b.store, err = Open(context.Background(), b.root, machineMemoryScope)
	return err
}

func modelError(err error) map[string]any {
	code := ""
	if err != nil {
		var typed *StoreError
		if errors.As(err, &typed) {
			code = string(typed.Code)
		} else {
			code = "unexpected_error"
		}
	}
	return map[string]any{"error": code}
}

func (b *modelBML) step(q modelRequest) map[string]any {
	ctx := context.Background()
	owner := OwnershipBinding{SubjectID: "model", DomainID: "model", ScopeKind: "personal", DestinationID: "model"}
	if q.Op == "reopen" {
		return modelError(b.reopen())
	}
	if q.Op == "snapshot" {
		if q.Foreign {
			owner.SubjectID = "foreign"
		}
		hits, err := b.store.SearchBound(ctx, owner, SearchQuery{Text: "needle", Scope: machineScope(nil), Limit: uint32(MaxMemoryRecords)})
		if err != nil {
			return modelError(err)
		}
		type view struct {
			ID       string `json:"id"`
			Text     string `json:"text"`
			Revision int64  `json:"revision"`
		}
		records := []view{}
		for _, hit := range hits {
			records = append(records, view{hit.Record.Record.ID, hit.Record.Record.Content, hit.Record.Revision})
		}
		sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
		meta, err := b.store.Metadata(ctx)
		if err != nil {
			return modelError(err)
		}
		return map[string]any{"error": "", "store_revision": meta.StoreRevision, "records": records}
	}
	if q.Op != "create" && q.Op != "update" && q.Op != "delete" {
		return map[string]any{"error": "invalid_operation"}
	}
	now := time.Unix(1700000000, 0).UTC()
	r := longTermRecord(q.ID, q.Text, nil, now)
	m := BoundMutation{Operation: q.Op, OperationID: q.OperationID, PayloadDigest: "model-" + q.OperationID, Record: r}
	if q.Op == "update" {
		m.ExpectedRecordRevision = &q.Revision
	}
	if q.Op == "delete" {
		m.Operation = "tombstone"
		m.Record = longTermRecord("tomb-"+q.OperationID, "", nil, now)
		m.Record.Supersedes = []string{q.ID}
		m.Record.Tombstone = &Tombstone{TargetRecordID: q.ID, ReasonDigest: MemoryContentDigest([]byte("model removal")), ActorID: "model", CreatedAt: now}
		m.ExpectedTargetRevision = &q.Revision
	}
	outcome, err := b.store.MutateBound(ctx, owner, m)
	if err != nil {
		return modelError(err)
	}
	return map[string]any{"error": "", "revision": outcome.RecordRevision, "store_revision": outcome.StoreRevision}
}

func TestMemoryModelBridge(t *testing.T) {
	root := os.Getenv("VIVY_MEMORY_MODEL_ROOT")
	if root == "" {
		return
	}
	b := modelBML{root: root}
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

func TestMemoryModelNativeReplayAndTombstoneAfterReopen(t *testing.T) {
	b := modelBML{root: t.TempDir()}
	if err := b.reopen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.store.Close() })
	create := modelRequest{Op: "create", ID: "one", OperationID: "create", Text: "needle 初始"}
	first := b.step(create)
	if !reflect.DeepEqual(first, map[string]any{"error": "", "revision": int64(1), "store_revision": int64(1)}) {
		t.Fatalf("create: %v", first)
	}
	update := modelRequest{Op: "update", ID: "one", OperationID: "update", Text: "needle 更新", Revision: 1}
	if got := b.step(update); got["error"] != "" || got["revision"] != int64(2) {
		t.Fatalf("update: %v", got)
	}
	if got := b.step(modelRequest{Op: "delete", ID: "one", OperationID: "remove", Revision: 2}); got["error"] != "" {
		t.Fatalf("delete: %v", got)
	}
	if err := b.reopen(); err != nil {
		t.Fatal(err)
	}
	if got := b.step(create); !reflect.DeepEqual(got, first) {
		t.Fatalf("immutable replay: %v want %v", got, first)
	}
	if got := b.step(modelRequest{Op: "snapshot"}); got["error"] != "" || got["store_revision"] != int64(3) || reflect.ValueOf(got["records"]).Len() != 0 {
		t.Fatalf("deleted memory resurfaced or replay changed state: %v", got)
	}
	// Legacy Get deliberately hides owned rows; inspect retention in-package only.
	var revision int64
	var raw string
	if err := b.store.db.QueryRowContext(context.Background(), selectStoredSQL, "one").Scan(&revision, &raw); err != nil {
		t.Fatal(err)
	}
	retained, err := decodeStored(revision, raw)
	if err != nil || retained.Revision != 2 || retained.Record.Content != "needle 更新" {
		t.Fatalf("tombstone erased raw material: %+v %v", retained, err)
	}
}
