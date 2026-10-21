package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type unavailablePresentationStore struct{ storage.ContinuityStore }

func (unavailablePresentationStore) ReadDeliverablePresentation(context.Context, domain.SessionID, string) ([]byte, error) {
	return nil, storage.ErrNotFound
}

func TestD09PresentationRequiresRetainedBytes(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "capture-run")
	f.writeFile(t, "capture-run", "private.txt", []byte("private output"))
	set := presentOne(t, f, "capture-run", "capture-call", domain.PresentFile{Path: "private.txt"})
	f.svc.continuity = unavailablePresentationStore{f.backend}
	_, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, Length: 32})
	var public DeliverableError
	if !errors.As(err, &public) || public.Status != string(domain.HistoryStatusUnavailable) {
		t.Fatalf("missing historical capture must require re-presentation: %v", err)
	}
}

func TestD09PresentationWireContainsNoDigestOrPrivateBytes(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "capture-run")
	f.writeFile(t, "capture-run", "private.txt", []byte("private output"))
	set := presentOne(t, f, "capture-run", "capture-call", domain.PresentFile{Path: "private.txt"})
	wire, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{`"sha256"`, `"digest"`, "private output"} {
		if bytes.Contains(wire, []byte(secret)) {
			t.Fatalf("presentation wire leaked %q", secret)
		}
	}
}

func TestD09PresentationExactStateAndImmutableTransfer(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "capture-run")
	full := f.writeFile(t, "capture-run", "private.txt", []byte("abcdef"))
	original, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	set := presentOne(t, f, "capture-run", "capture-call", domain.PresentFile{Path: "private.txt"})
	item := set.Items[0]
	f.writeFile(t, "capture-run", "private.txt", []byte("abcdeg"))
	if err := os.Chtimes(full, original.ModTime(), original.ModTime()); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: item.ID, Length: 3})
	var public DeliverableError
	if !errors.As(err, &public) || public.Status != DeliveryReadChanged {
		t.Fatalf("same-sized restored-time edit accepted: %v", err)
	}
	f.writeFile(t, "capture-run", "private.txt", []byte("abcdef"))
	first, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: item.ID, Length: 3})
	if err != nil {
		t.Fatal(err)
	}
	f.writeFile(t, "capture-run", "private.txt", []byte("xxxxxx"))
	next, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: item.ID, TransferID: first.TransferID, Offset: 3, Length: 3})
	if err != nil {
		t.Fatal(err)
	}
	content, err := base64.StdEncoding.DecodeString(next.DataBase64)
	if err != nil || string(content) != "def" {
		t.Fatalf("mutable transfer bytes=%q err=%v", content, err)
	}
}

func TestD09PresentationRestartEmptyAndTransferExpiry(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "capture-run")
	f.writeFile(t, "capture-run", "empty.txt", []byte{})
	set := presentOne(t, f, "capture-run", "capture-call", domain.PresentFile{Path: "empty.txt"})
	restarted, err := NewDeliverableService(f.manager, f.backend, f.backend, f.backend, filepath.Join(t.TempDir(), "fresh-transfers"))
	if err != nil {
		t.Fatal(err)
	}
	f.svc = restarted
	first, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, Length: 32})
	if err != nil || !first.EOF || first.DataBase64 != "" {
		t.Fatalf("restarted empty capture=%+v err=%v", first, err)
	}
	if err := f.svc.CloseTransfer(f.operatorCtx("B"), first.TransferID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, Length: 32})
	if err != nil {
		t.Fatalf("closing transfer removed durable bytes: %v", err)
	}
}

func TestD09PresentationConcurrentChunkAndClose(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "capture-run")
	f.writeFile(t, "capture-run", "out.txt", []byte("abcdef"))
	set := presentOne(t, f, "capture-run", "capture-call", domain.PresentFile{Path: "out.txt"})
	first, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, Length: 3})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chunk, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, TransferID: first.TransferID, Offset: 3, Length: 3})
			if err == nil {
				content, _ := base64.StdEncoding.DecodeString(chunk.DataBase64)
				if string(content) != "def" {
					t.Errorf("changed concurrent bytes=%q", content)
				}
			}
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); _ = f.svc.CloseTransfer(f.operatorCtx("B"), first.TransferID) }()
	wg.Wait()
	if _, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, TransferID: first.TransferID, Offset: 3, Length: 3}); err == nil {
		t.Fatal("closed transfer remained available")
	}
}

func TestD09PresentationReplayPinsExactWindow(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "capture-run")
	f.writeFile(t, "capture-run", "out.txt", []byte("abcdef"))
	set := presentOne(t, f, "capture-run", "capture-call", domain.PresentFile{Path: "out.txt"})
	first, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, Length: 3})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, TransferID: first.TransferID, Length: 6})
	var public DeliverableError
	if !errors.As(err, &public) || public.Status != string(domain.HistoryStatusInvalidArgument) {
		t.Fatalf("changed retry window accepted: %v", err)
	}
	retry, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: set.Items[0].ID, TransferID: first.TransferID, Length: 3})
	if err != nil || retry.DataBase64 != first.DataBase64 {
		t.Fatalf("identical retry changed: %v", err)
	}
}
