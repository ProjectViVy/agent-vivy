package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Reusable-workflow-definition contract (S11-F, G9). The CAS/allocation
// semantics mirror INOFY's definitions.Repository: one mutable draft per
// workflow under ETag CAS, immutable monotone published revisions deduped
// by (workflow, artifact+catalog digest), author-scoped drafts and
// organism-visible revisions.

func definitionStore(t *testing.T, slot Slot) storage.WorkflowDefinitionStore {
	t.Helper()
	s, ok := slot.Engine.(storage.WorkflowDefinitionStore)
	if !ok {
		t.Fatal("backend does not implement WorkflowDefinitionStore")
	}
	return s
}

// draftArtifact builds a small valid artifact document.
func draftArtifact(tag string) []byte {
	return []byte(fmt.Sprintf(`{"definition":{"schema_version":"inofy.workflow/v1","graph":{"nodes":[],"edges":[],"exits":[]}},"presentation":{"title":%q}}`, "wf-"+tag))
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func defDraftUpdate(workflowID, etag, tag string, now int64) storage.WorkflowDraftUpdate {
	artifact := draftArtifact(tag)
	return storage.WorkflowDraftUpdate{
		WorkflowID:       workflowID,
		ExpectedETag:     etag,
		ArtifactJSON:     artifact,
		DefinitionDigest: digestOf([]byte("def:" + tag)),
		ArtifactDigest:   digestOf(artifact),
		Now:              now,
	}
}

func defRevision(workflowID, tag string) storage.WorkflowPublishedRevision {
	artifact := draftArtifact(tag)
	return storage.WorkflowPublishedRevision{
		WorkflowID:              workflowID,
		ArtifactJSON:            artifact,
		DefinitionDigest:        digestOf([]byte("def:" + tag)),
		ArtifactDigest:          digestOf(artifact),
		UsedCatalogDigest:       "sha256:" + digestOf([]byte("catalog:"+tag)),
		UsedImplementationsJSON: []byte(`{"vivy.child-task@1":"vivy.child-task@1"}`),
		AuthorSessionID:         "",
		PublishedAt:             time.Now().UnixMilli(),
	}
}

func createDefSession(t *testing.T, slot Slot, id string) {
	t.Helper()
	if err := slot.Engine.CreateSession(context.Background(), domain.Session{ID: domain.SessionID(id), CreatedAt: 1}); err != nil {
		t.Fatalf("create session %s: %v", id, err)
	}
}

// AssertWorkflowDefinitionContract runs every S11-F case against one slot.
func AssertWorkflowDefinitionContract(t *testing.T, slot Slot) {
	t.Helper()
	ctx := context.Background()
	s := definitionStore(t, slot)
	authorA := "sess-def-a-" + t.Name()
	authorB := "sess-def-b-" + t.Name()
	createDefSession(t, slot, authorA)
	createDefSession(t, slot, authorB)
	wf := "wfdef-contract"

	t.Run("CreateRequiresAbsentETag", func(t *testing.T) {
		d, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(wf, storage.WorkflowDefinitionETagAbsent, "a1", 1))
		if err != nil {
			t.Fatalf("create draft: %v", err)
		}
		if d.WorkflowID != wf || d.ETag == "" || d.AuthorSessionID != authorA || d.Archived {
			t.Fatalf("unexpected draft row: %+v", d)
		}
		// Second create with the absent sentinel must conflict.
		if _, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(wf, storage.WorkflowDefinitionETagAbsent, "a2", 2)); !errors.Is(err, storage.ErrWorkflowDefinitionConflict) {
			t.Fatalf("expected conflict on duplicate create, got %v", err)
		}
	})

	var etag string
	t.Run("UpdateCAS", func(t *testing.T) {
		d, err := s.GetWorkflowDraft(ctx, authorA, wf)
		if err != nil {
			t.Fatalf("get draft: %v", err)
		}
		etag = d.ETag
		if _, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(wf, "stale-etag", "a3", 3)); !errors.Is(err, storage.ErrWorkflowDefinitionConflict) {
			t.Fatalf("expected stale-etag conflict, got %v", err)
		}
		next, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(wf, etag, "a3", 4))
		if err != nil {
			t.Fatalf("cas update: %v", err)
		}
		if next.ETag == "" || next.ETag == etag {
			t.Fatalf("etag must rotate on update: %q -> %q", etag, next.ETag)
		}
		etag = next.ETag
	})

	t.Run("AuthorIsolation", func(t *testing.T) {
		if _, err := s.GetWorkflowDraft(ctx, authorB, wf); !errors.Is(err, storage.ErrWorkflowDefinitionNotFound) {
			t.Fatalf("foreign draft read must be not-found, got %v", err)
		}
		if _, err := s.UpdateWorkflowDraftCAS(ctx, authorB, defDraftUpdate(wf, etag, "b1", 5)); !errors.Is(err, storage.ErrWorkflowDefinitionAuthor) {
			t.Fatalf("foreign draft update must be author-denied, got %v", err)
		}
		if _, err := s.PublishWorkflowRevisionCAS(ctx, authorB, wf, etag, defRevision(wf, "b1")); !errors.Is(err, storage.ErrWorkflowDefinitionAuthor) {
			t.Fatalf("foreign publish must be author-denied, got %v", err)
		}
	})

	t.Run("UpdateMissingDraftNotFound", func(t *testing.T) {
		if _, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate("wfdef-missing", "etag", "x", 6)); !errors.Is(err, storage.ErrWorkflowDefinitionNotFound) {
			t.Fatalf("expected not-found on update of missing draft, got %v", err)
		}
	})

	t.Run("PublishMonotoneAndDedup", func(t *testing.T) {
		rev1, err := s.PublishWorkflowRevisionCAS(ctx, authorA, wf, etag, defRevision(wf, "p1"))
		if err != nil {
			t.Fatalf("publish rev1: %v", err)
		}
		if rev1.Revision != 1 {
			t.Fatalf("first revision must be 1, got %d", rev1.Revision)
		}
		// Same artifact+catalog digest republish dedups to the same revision.
		again, err := s.PublishWorkflowRevisionCAS(ctx, authorA, wf, etag, defRevision(wf, "p1"))
		if err != nil {
			t.Fatalf("republish identical artifact: %v", err)
		}
		if again.Revision != rev1.Revision {
			t.Fatalf("dedup must return revision %d, got %d", rev1.Revision, again.Revision)
		}
		// Stale etag publish conflicts.
		if _, err := s.PublishWorkflowRevisionCAS(ctx, authorA, wf, "stale", defRevision(wf, "p2")); !errors.Is(err, storage.ErrWorkflowDefinitionConflict) {
			t.Fatalf("stale-etag publish must conflict, got %v", err)
		}
		// Edit the draft then publish: next revision is monotone.
		next, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(wf, etag, "p2-draft", 7))
		if err != nil {
			t.Fatalf("post-publish edit: %v", err)
		}
		rev2, err := s.PublishWorkflowRevisionCAS(ctx, authorA, wf, next.ETag, defRevision(wf, "p2"))
		if err != nil {
			t.Fatalf("publish rev2: %v", err)
		}
		if rev2.Revision != 2 {
			t.Fatalf("second revision must be 2, got %d", rev2.Revision)
		}
	})

	t.Run("PublishedImmutable", func(t *testing.T) {
		rev1, err := s.GetWorkflowPublishedRevision(ctx, wf, 1)
		if err != nil {
			t.Fatalf("read rev1: %v", err)
		}
		if string(rev1.ArtifactJSON) != string(draftArtifact("p1")) || rev1.UsedCatalogDigest == "" ||
			string(rev1.UsedImplementationsJSON) == "" || rev1.AuthorSessionID != authorA {
			t.Fatalf("revision row drifted: %+v", rev1)
		}
		if _, err := s.GetWorkflowPublishedRevision(ctx, wf, 99); !errors.Is(err, storage.ErrWorkflowDefinitionNotFound) {
			t.Fatalf("missing revision must be not-found, got %v", err)
		}
	})

	t.Run("ListPaging", func(t *testing.T) {
		page, err := s.ListWorkflowDefinitions(ctx, "", 1)
		if err != nil {
			t.Fatalf("list page 1: %v", err)
		}
		if len(page.Revisions) != 1 || page.Revisions[0].WorkflowID != wf || page.Revisions[0].Revision != 1 {
			t.Fatalf("unexpected first page: %+v", page.Revisions)
		}
		if page.NextCursor == "" {
			t.Fatal("expected a next cursor after revision 1")
		}
		page2, err := s.ListWorkflowDefinitions(ctx, page.NextCursor, 10)
		if err != nil {
			t.Fatalf("list page 2: %v", err)
		}
		if len(page2.Revisions) != 1 || page2.Revisions[0].Revision != 2 || page2.NextCursor != "" {
			t.Fatalf("unexpected second page: %+v", page2.Revisions)
		}
	})

	t.Run("ReopenKeepsIdentity", func(t *testing.T) {
		if slot.Reopen == nil {
			t.Skip("slot has no reopen")
		}
		reopened, err := slot.Reopen()
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		rs, ok := reopened.(storage.WorkflowDefinitionStore)
		if !ok {
			t.Fatal("reopened backend does not implement WorkflowDefinitionStore")
		}
		rev1, err := rs.GetWorkflowPublishedRevision(ctx, wf, 1)
		if err != nil {
			t.Fatalf("rev1 after reopen: %v", err)
		}
		if string(rev1.ArtifactJSON) != string(draftArtifact("p1")) {
			t.Fatal("revision bytes differ after reopen")
		}
		d, err := rs.GetWorkflowDraft(ctx, authorA, wf)
		if err != nil {
			t.Fatalf("draft after reopen: %v", err)
		}
		if d.Archived {
			t.Fatal("archived bit must round-trip false")
		}
	})
}
