package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

// deliverableFixture composes the real stores and a private-root
// WorkspaceManager the way the app does; the service is the unit under test.
type deliverableFixture struct {
	backend *sqlite.Backend
	manager *WorkspaceManager
	svc     *DeliverableService
	wsRoot  string
	scratch string
	clock   time.Time
}

func newDeliverableFixture(t *testing.T) *deliverableFixture {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "deliverables.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	for _, session := range []domain.Session{
		{ID: "A", Title: "adjacent", CreatedAt: 1},
		{ID: "B", Title: "owner", CreatedAt: 2},
	} {
		if err := backend.CreateSession(ctx, session); err != nil {
			t.Fatalf("create session %s: %v", session.ID, err)
		}
	}
	f := &deliverableFixture{backend: backend, clock: time.UnixMilli(1_700_000_000_000)}
	f.wsRoot = filepath.Join(t.TempDir(), "workspaces")
	f.scratch = filepath.Join(t.TempDir(), "transfers")
	manager, err := NewWorkspaceManager(f.wsRoot)
	if err != nil {
		t.Fatalf("workspace manager: %v", err)
	}
	f.manager = manager
	svc, err := NewDeliverableService(manager, backend, backend, backend, f.scratch)
	if err != nil {
		t.Fatalf("deliverable service: %v", err)
	}
	svc.now = func() time.Time { return f.clock }
	f.svc = svc
	return f
}

func (f *deliverableFixture) createLiveRun(t *testing.T, id string) {
	t.Helper()
	if err := f.backend.CreateRun(context.Background(), domain.Run{ID: domain.RunID(id), SessionID: "B", Status: domain.RunActive, CreatedAt: 20, Kind: domain.RunKindPrimary, RootID: domain.RunID(id)}); err != nil {
		t.Fatalf("create run %s: %v", id, err)
	}
	if err := os.MkdirAll(f.workspaceOf(id), 0o700); err != nil {
		t.Fatalf("create workspace %s: %v", id, err)
	}
}

func (f *deliverableFixture) workspaceOf(runID string) string {
	return filepath.Join(f.wsRoot, runID)
}

func (f *deliverableFixture) writeFile(t *testing.T, runID, rel string, data []byte) string {
	t.Helper()
	full := filepath.Join(f.workspaceOf(runID), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return full
}

// shellWriteFile stands in for shell-created output: bytes the runtime never
// produced itself, which must be presented without any version provenance.
func (f *deliverableFixture) shellWriteFile(t *testing.T, runID, rel string, data []byte) {
	t.Helper()
	full := filepath.Join(f.workspaceOf(runID), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cmd := exec.Command("sh", "-c", "cat > \"$1\"", "sh")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	cmd.Args = append(cmd.Args, full)
	if goruntime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := stdin.Write(data); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("shell write: %v", err)
	}
}

func (f *deliverableFixture) modelCtx(runID domain.RunID, callID string) context.Context {
	ctx := context.Background()
	ctx = tools.WithSessionID(ctx, "B")
	ctx = tools.WithRunID(ctx, runID)
	ctx = tools.WithToolCallID(ctx, callID)
	return ctx
}

func (f *deliverableFixture) operatorCtx(session domain.SessionID) context.Context {
	return WithHistoryOperator(tools.WithSessionID(context.Background(), session))
}

func sha256HexBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func presentOne(t *testing.T, f *deliverableFixture, runID domain.RunID, callID string, files ...domain.PresentFile) domain.DeliverySet {
	t.Helper()
	set, err := f.svc.Present(f.modelCtx(runID, callID), domain.PresentRequest{Files: files})
	if err != nil {
		t.Fatalf("present: %v", err)
	}
	return set
}

func TestDeliverablePresentPartialAndImmutableSets(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-1")
	f.writeFile(t, "run-deliv-1", "out/report.txt", []byte("release notes v1"))
	f.shellWriteFile(t, "run-deliv-1", "bin/blob.bin", []byte{0x00, 0x01, 0x02, 0xff, 0x10})

	set := presentOne(t, f, "run-deliv-1", "call-p1",
		domain.PresentFile{Path: "out/report.txt", Description: "the report"},
		domain.PresentFile{Path: "bin/blob.bin", Description: "binary blob"},
		domain.PresentFile{Path: "gone.txt", Description: "never existed"},
	)
	if set.Status != "partial" || len(set.Items) != 2 || len(set.Failures) != 1 {
		t.Fatalf("dishonest presentation: %+v", set)
	}
	if set.Items[0].SHA256 != sha256HexBytes([]byte("release notes v1")) {
		t.Fatalf("sha = %q", set.Items[0].SHA256)
	}
	if set.Items[0].OriginToolCallID != "call-p1" || set.Items[0].FileVersionID != "" {
		t.Fatalf("fabricated provenance: %+v", set.Items[0])
	}
	if set.Failures[0].Path != "gone.txt" || set.Failures[0].Reason == "" {
		t.Fatalf("missing file must be a recorded failure: %+v", set.Failures)
	}
	// Same call id replays the committed set without re-reading the files.
	replay := presentOne(t, f, "run-deliv-1", "call-p1",
		domain.PresentFile{Path: "out/report.txt", Description: "the report"},
		domain.PresentFile{Path: "bin/blob.bin", Description: "binary blob"},
		domain.PresentFile{Path: "gone.txt", Description: "never existed"},
	)
	if replay.ID != set.ID {
		t.Fatalf("identical call minted a second set: %s vs %s", replay.ID, set.ID)
	}
	// A genuinely new call appends a second immutable set; changing the file
	// between calls proves both fingerprints are retained.
	f.clock = f.clock.Add(time.Second)
	f.writeFile(t, "run-deliv-1", "out/report.txt", []byte("release notes v2"))
	second := presentOne(t, f, "run-deliv-1", "call-p2",
		domain.PresentFile{Path: "out/report.txt", Description: "the report"},
	)
	oldItem, newItem := set.Items[0], second.Items[0]
	if oldItem.SHA256 == newItem.SHA256 {
		t.Fatal("fixture must change bytes")
	}
	page, err := f.svc.List(f.operatorCtx("B"), "B", "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	sets := page.Items
	if len(sets) != 2 {
		t.Fatal("new group replaced prior delivery")
	}
	if sets[0].ID != set.ID || sets[1].ID != second.ID {
		t.Fatalf("list lost immutable ordering: %+v", sets)
	}
}

func TestDeliverablePresentAllFailuresIsFailedSet(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-fail")
	set := presentOne(t, f, "run-deliv-fail", "call-f1",
		domain.PresentFile{Path: "nope.txt", Description: "x"},
	)
	if set.Status != "failed" || len(set.Items) != 0 || len(set.Failures) != 1 {
		t.Fatalf("all-failure set must be failed, not fake success: %+v", set)
	}
}

func TestDeliverablePresentRejectsDuplicatePaths(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-dup")
	f.writeFile(t, "run-deliv-dup", "a.txt", []byte("a"))
	_, err := f.svc.Present(f.modelCtx("run-deliv-dup", "call-dup"), domain.PresentRequest{Files: []domain.PresentFile{
		{Path: "a.txt", Description: "one"},
		{Path: "a.txt", Description: "two"},
	}})
	var derr DeliverableError
	if err == nil || !errors.As(err, &derr) || derr.Status != string(domain.HistoryStatusInvalidArgument) {
		t.Fatalf("duplicate normalized paths accepted: %v", err)
	}
}

func TestDeliverablePresentRefusesEscapesAndSpecialFiles(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("symlink/FIFO fixtures are unix-only")
	}
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-sec")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("host secret bytes"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(f.workspaceOf("run-deliv-sec"), "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	f.writeFile(t, "run-deliv-sec", "ok.txt", []byte("fine"))
	set := presentOne(t, f, "run-deliv-sec", "call-sec",
		domain.PresentFile{Path: "link.txt", Description: "escape"},
		domain.PresentFile{Path: "ok.txt", Description: "ok"},
	)
	if len(set.Items) != 1 || len(set.Failures) != 1 || set.Failures[0].Path != "link.txt" {
		t.Fatalf("outside-root symlink presented: %+v", set)
	}
}

func TestDeliverablePresentRefusesFIFO(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("FIFO fixtures are unix-only")
	}
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-fifo")
	pipe := filepath.Join(f.workspaceOf("run-deliv-fifo"), "pipe")
	if err := mkfifoForTest(pipe); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	set := presentOne(t, f, "run-deliv-fifo", "call-fifo",
		domain.PresentFile{Path: "pipe", Description: "fifo"},
	)
	if len(set.Items) != 0 || len(set.Failures) != 1 || !strings.Contains(set.Failures[0].Reason, "regular") {
		t.Fatalf("fifo presented or reason unsafe: %+v", set)
	}
}

func TestDeliverableReadServesVerifiedSnapshot(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-read")
	body := []byte(strings.Repeat("0123456789", 40)) // 400 bytes
	f.writeFile(t, "run-deliv-read", "data.txt", body)
	set := presentOne(t, f, "run-deliv-read", "call-r1",
		domain.PresentFile{Path: "data.txt", Description: "d"},
	)
	item := set.Items[0]
	ctx := f.operatorCtx("B")
	first, err := f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 150})
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if first.TransferID == "" || first.ItemID != item.ID || first.Digest != item.SHA256 || first.Offset != 0 || first.EOF {
		t.Fatalf("first chunk = %+v", first)
	}
	data, err := base64.StdEncoding.DecodeString(first.DataBase64)
	if err != nil || string(data) != string(body[:150]) {
		t.Fatalf("first chunk bytes mismatch")
	}
	second, err := f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, TransferID: first.TransferID, Offset: 150, Length: 300})
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	data, _ = base64.StdEncoding.DecodeString(second.DataBase64)
	if !second.EOF || string(data) != string(body[150:]) {
		t.Fatalf("second chunk = %+v", second)
	}
	// The same last chunk may be retried after a lost reply.
	replay, err := f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, TransferID: first.TransferID, Offset: 150, Length: 300})
	if err != nil || replay.DataBase64 != second.DataBase64 {
		t.Fatalf("last-chunk replay = %v", err)
	}
	// Offset skipping is refused.
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, TransferID: first.TransferID, Offset: 700, Length: 10})
	var derr DeliverableError
	if err == nil || !errors.As(err, &derr) {
		t.Fatalf("offset skip accepted: %v", err)
	}
}

func TestDeliverableReadRejectsChangedMissingAndLinkedFile(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("symlink fixture is unix-only")
	}
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-drift")
	f.writeFile(t, "run-deliv-drift", "stable.txt", []byte("original bytes"))
	set := presentOne(t, f, "run-deliv-drift", "call-d1",
		domain.PresentFile{Path: "stable.txt", Description: "d"},
	)
	item := set.Items[0]
	ctx := f.operatorCtx("B")
	f.writeFile(t, "run-deliv-drift", "stable.txt", []byte("mutated bytes!!"))
	_, err := f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 100})
	var derr DeliverableError
	if err == nil || !errors.As(err, &derr) || derr.Status != "changed" {
		t.Fatalf("changed file served: %v", err)
	}
	// No leftover snapshot may survive the mismatch.
	entries, _ := os.ReadDir(f.scratch)
	if len(entries) != 0 {
		t.Fatalf("changed-read left %d scratch files", len(entries))
	}

	if err := os.Remove(filepath.Join(f.workspaceOf("run-deliv-drift"), "stable.txt")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 100})
	if err == nil || !errors.As(err, &derr) || derr.Status != "missing" {
		t.Fatalf("removed file reported %+v", derr)
	}

	f.writeFile(t, "run-deliv-drift", "stable.txt", []byte("original bytes"))
	if err := os.Remove(filepath.Join(f.workspaceOf("run-deliv-drift"), "stable.txt")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "replica.txt")
	if err := os.WriteFile(outside, []byte("original bytes"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(f.workspaceOf("run-deliv-drift"), "stable.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 100})
	if err == nil || !errors.As(err, &derr) || derr.Status != "forbidden" {
		t.Fatalf("link replacement followed: %+v", derr)
	}
}

func TestDeliverableReadMissingWorkspace(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-nows")
	f.writeFile(t, "run-deliv-nows", "a.txt", []byte("a"))
	set := presentOne(t, f, "run-deliv-nows", "call-w1", domain.PresentFile{Path: "a.txt", Description: "d"})
	item := set.Items[0]
	if err := os.RemoveAll(f.workspaceOf("run-deliv-nows")); err != nil {
		t.Fatalf("remove workspace: %v", err)
	}
	_, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 10})
	var derr DeliverableError
	if err == nil || !errors.As(err, &derr) || derr.Status != "workspace_unavailable" {
		t.Fatalf("missing workspace = %+v", derr)
	}
	// Metadata stays inspectable even though the bytes are gone.
	view, err := f.svc.Get(f.operatorCtx("B"), "B", set.ID)
	if err != nil || len(view.Items) != 1 {
		t.Fatalf("metadata lost with workspace: %+v err=%v", view, err)
	}
}

func TestDeliverableReadRequiresOwnerAndDigest(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-own")
	f.writeFile(t, "run-deliv-own", "a.txt", []byte("a"))
	set := presentOne(t, f, "run-deliv-own", "call-o1", domain.PresentFile{Path: "a.txt", Description: "d"})
	item := set.Items[0]
	_, err := f.svc.Read(f.operatorCtx("A"), domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 10})
	var derr DeliverableError
	if err == nil || !errors.As(err, &derr) || derr.Status != "forbidden" && derr.Status != "not_found" {
		t.Fatalf("foreign owner read = %+v", derr)
	}
	_, err = f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: strings.Repeat("0", 64), Length: 10})
	if err == nil || !errors.As(err, &derr) || derr.Status != "conflict" {
		t.Fatalf("stale digest read = %+v", derr)
	}
}

func TestDeliverableTransferReplacementAndExpiry(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-xfer")
	f.writeFile(t, "run-deliv-xfer", "one.txt", []byte("one"))
	f.writeFile(t, "run-deliv-xfer", "two.txt", []byte("two"))
	setA := presentOne(t, f, "run-deliv-xfer", "call-t1", domain.PresentFile{Path: "one.txt", Description: "1"})
	setB := presentOne(t, f, "run-deliv-xfer", "call-t2", domain.PresentFile{Path: "two.txt", Description: "2"})
	ctx := f.operatorCtx("B")
	first, err := f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: setA.Items[0].ID, ExpectedDigest: setA.Items[0].SHA256, Length: 3})
	if err != nil {
		t.Fatalf("read one: %v", err)
	}
	// One active transfer per owner: a new first-read replaces it.
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: setB.Items[0].ID, ExpectedDigest: setB.Items[0].SHA256, Length: 3})
	if err != nil {
		t.Fatalf("read two: %v", err)
	}
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: setA.Items[0].ID, ExpectedDigest: setA.Items[0].SHA256, TransferID: first.TransferID, Offset: 3, Length: 1})
	var derr DeliverableError
	if err == nil || !errors.As(err, &derr) {
		t.Fatalf("replaced transfer still serves: %v", err)
	}
	// Idle expiry under a fake clock.
	third, err := f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: setB.Items[0].ID, ExpectedDigest: setB.Items[0].SHA256, Length: 3})
	if err != nil {
		t.Fatalf("read three: %v", err)
	}
	f.clock = f.clock.Add(61 * time.Second)
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: setB.Items[0].ID, ExpectedDigest: setB.Items[0].SHA256, TransferID: third.TransferID, Offset: 3, Length: 1})
	if err == nil || !errors.As(err, &derr) {
		t.Fatalf("expired transfer still serves: %v", err)
	}
}

func TestDeliverableCloseTransferAndSessionHook(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-close")
	f.writeFile(t, "run-deliv-close", "a.txt", []byte("alpha"))
	set := presentOne(t, f, "run-deliv-close", "call-c1", domain.PresentFile{Path: "a.txt", Description: "d"})
	item := set.Items[0]
	ctx := f.operatorCtx("B")
	chunk, err := f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 2})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Another session cannot close B's transfer.
	if err := f.svc.CloseTransfer(f.operatorCtx("A"), chunk.TransferID); err == nil {
		t.Fatal("foreign close accepted")
	}
	if err := f.svc.CloseTransfer(ctx, chunk.TransferID); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, TransferID: chunk.TransferID, Offset: 2, Length: 2})
	var derr DeliverableError
	if err == nil || !errors.As(err, &derr) {
		t.Fatalf("closed transfer still serves: %v", err)
	}
	// Session deletion cancels every live transfer of that session.
	chunk, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 2})
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	f.svc.CloseSessionTransfers("B")
	_, err = f.svc.Read(ctx, domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, TransferID: chunk.TransferID, Offset: 2, Length: 2})
	if err == nil {
		t.Fatal("session-deleted transfer still serves")
	}
}

func TestDeliverableListPagesAndGet(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-page")
	f.writeFile(t, "run-deliv-page", "a.txt", []byte("a"))
	f.writeFile(t, "run-deliv-page", "b.txt", []byte("b"))
	first := presentOne(t, f, "run-deliv-page", "call-g1", domain.PresentFile{Path: "a.txt", Description: "1"})
	f.clock = f.clock.Add(time.Second)
	second := presentOne(t, f, "run-deliv-page", "call-g2", domain.PresentFile{Path: "b.txt", Description: "2"})
	ctx := f.operatorCtx("B")
	page, err := f.svc.List(ctx, "B", "", 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != first.ID || page.NextCursor == "" {
		t.Fatalf("first page = %+v err=%v", page, err)
	}
	page, err = f.svc.List(ctx, "B", page.NextCursor, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != second.ID || page.NextCursor != "" {
		t.Fatalf("second page = %+v err=%v", page, err)
	}
	got, err := f.svc.Get(ctx, "B", second.ID)
	if err != nil || got.ID != second.ID {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	// Adjacent sessions see nothing.
	page, err = f.svc.List(f.operatorCtx("A"), "A", "", 10)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("cross-session leak: %+v", page)
	}
	if _, err := f.svc.Get(f.operatorCtx("A"), "B", second.ID); err == nil {
		t.Fatal("foreign get succeeded")
	}
}

func TestDeliverableMediaTypeAndStartupPurge(t *testing.T) {
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-mime")
	f.writeFile(t, "run-deliv-mime", "page.html", []byte("<!doctype html><html><body>x</body></html>"))
	set := presentOne(t, f, "run-deliv-mime", "call-m1", domain.PresentFile{Path: "page.html", Description: "doc"})
	if !strings.HasPrefix(set.Items[0].MediaType, "text/html") {
		t.Fatalf("html media = %q", set.Items[0].MediaType)
	}
	// Startup cleanup touches only the dedicated transfer scratch.
	stray := filepath.Join(f.scratch, "abandoned.snap")
	if err := os.MkdirAll(f.scratch, 0o700); err != nil {
		t.Fatalf("scratch: %v", err)
	}
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatalf("stray: %v", err)
	}
	fresh, err := NewDeliverableService(f.manager, f.backend, f.backend, f.backend, f.scratch)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_ = fresh
	if _, err := os.Stat(stray); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("startup cleanup left abandoned snapshot")
	}
	if _, err := os.Stat(filepath.Join(f.workspaceOf("run-deliv-mime"), "page.html")); err != nil {
		t.Fatal("startup cleanup touched workspace files")
	}
}

func TestDeliverablePresentDetectsMidReadSwap(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("swap fixture is unix-only")
	}
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-swap")
	f.writeFile(t, "run-deliv-swap", "victim.txt", []byte("stable"))
	f.svc.postReadHook = func() {
		outside := filepath.Join(f.wsRoot, "planted.txt")
		_ = os.WriteFile(outside, []byte("swapped"), 0o600)
		target := filepath.Join(f.workspaceOf("run-deliv-swap"), "victim.txt")
		_ = os.Remove(target)
		_ = os.Symlink(outside, target)
	}
	set := presentOne(t, f, "run-deliv-swap", "call-s1", domain.PresentFile{Path: "victim.txt", Description: "d"})
	if len(set.Items) != 0 || len(set.Failures) != 1 {
		t.Fatalf("swap accepted: %+v", set)
	}
}

func TestDeliverableReadSnapshotWriteFailure(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("permission fixture is unix-only")
	}
	f := newDeliverableFixture(t)
	f.createLiveRun(t, "run-deliv-nospace")
	f.writeFile(t, "run-deliv-nospace", "a.txt", []byte("alpha"))
	set := presentOne(t, f, "run-deliv-nospace", "call-n1", domain.PresentFile{Path: "a.txt", Description: "d"})
	item := set.Items[0]
	if err := os.Chmod(f.scratch, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.scratch, 0o700) })
	_, err := f.svc.Read(f.operatorCtx("B"), domain.DeliveryReadRequest{ItemID: item.ID, ExpectedDigest: item.SHA256, Length: 4})
	if err == nil {
		t.Fatal("read succeeded against unwritable scratch")
	}
	// The failure surfaces as unavailable, never as a served chunk.
	var derr DeliverableError
	if errors.As(err, &derr) && derr.Status == "ok" {
		t.Fatalf("fake success on storage failure: %+v", derr)
	}
}
