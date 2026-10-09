package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
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

func admitDefinitionRunFixture(t *testing.T, slot Slot, sessionID domain.SessionID, parentID, runID domain.RunID, createdAt int64) storage.WorkflowStepStore {
	t.Helper()
	descriptorJSON := []byte(`{"definition":"wf-pages"}`)
	authorityJSON := []byte(`{"authority":"test"}`)
	revision := domain.WorkflowRevision{
		RunID: runID, ParentRunID: parentID, ParentSessionID: sessionID, RootRunID: parentID,
		OperationKey: "op-" + string(runID), DescriptorDigest: digestHex(descriptorJSON),
		AuthorityDigest: digestHex(authorityJSON), DescriptorJSON: descriptorJSON,
		AuthorityJSON: authorityJSON, SchemaVersion: 2, CreatedAt: createdAt,
		ProgramDigest: StepDigest("program-" + string(runID)), CatalogDigest: StepDigest("catalog-" + string(runID)),
		CompilerVersion: "inofy@test", EinoBuild: "v0.9.13", InputDigest: StepDigest("input-" + string(runID)),
		InputJSON: []byte(`{}`), EffectiveLimits: []byte(`{"max_nodes":12}`),
		HostBindingID: StepDigest("binding-" + string(runID)), DefinitionID: "wf-pages", DefinitionRevision: 1,
	}
	run := domain.Run{
		ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: createdAt,
		Kind: domain.RunKindWorkflow, ParentID: parentID, RootID: parentID, Depth: 1,
	}
	started := domain.RunEvent{
		RunID: runID, Type: domain.EventRunStarted, CreatedAt: createdAt,
		PayloadVersion: 1, Payload: []byte(`{"provider":"test"}`),
	}
	if _, err := slot.Engine.CommitWorkflowAdmission(context.Background(), storage.WorkflowAdmission{
		Revision: revision, Run: run, Started: started,
	}); err != nil {
		t.Fatalf("admit definition run %s: %v", runID, err)
	}
	return stepStore(t, slot.Engine)
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

	t.Run("ETagRotatesWithRepeatedAndBackwardsTime", func(t *testing.T) {
		etagWorkflowID := "wfdef-etag-" + t.Name()
		d0, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(etagWorkflowID, storage.WorkflowDefinitionETagAbsent, "etag-0", 1000))
		if err != nil {
			t.Fatalf("create draft: %v", err)
		}
		d1, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(etagWorkflowID, d0.ETag, "etag-1", 1000))
		if err != nil {
			t.Fatalf("first edit: %v", err)
		}
		d2, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(etagWorkflowID, d1.ETag, "etag-2", 1000))
		if err != nil {
			t.Fatalf("second edit at repeated time: %v", err)
		}
		d3, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(etagWorkflowID, d2.ETag, "etag-3", 999))
		if err != nil {
			t.Fatalf("edit after clock moved backwards: %v", err)
		}
		if d0.ETag == "" || d1.ETag == "" || d2.ETag == "" || d3.ETag == "" ||
			d0.ETag == d1.ETag || d1.ETag == d2.ETag || d2.ETag == d3.ETag ||
			d0.ETag == d2.ETag || d0.ETag == d3.ETag || d1.ETag == d3.ETag {
			t.Fatalf("ETags must be nonempty and distinct across repeated/backwards times: %q %q %q %q", d0.ETag, d1.ETag, d2.ETag, d3.ETag)
		}
		if d3.CreatedAt != 1000 || d3.UpdatedAt != 999 {
			t.Fatalf("timestamps changed: %+v", d3)
		}
		for _, stale := range []string{d0.ETag, d1.ETag, d2.ETag} {
			if _, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(etagWorkflowID, stale, "stale", 1001)); !errors.Is(err, storage.ErrWorkflowDefinitionConflict) {
				t.Fatalf("reusing prior etag %q: %v", stale, err)
			}
			final, err := s.GetWorkflowDraft(ctx, authorA, etagWorkflowID)
			if err != nil {
				t.Fatalf("read final draft: %v", err)
			}
			if string(final.ArtifactJSON) != string(draftArtifact("etag-3")) {
				t.Fatalf("stale write changed final artifact: %s", final.ArtifactJSON)
			}
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

	t.Run("EmptyExpectedETagRejected", func(t *testing.T) {
		emptyWorkflowID := "wfdef-empty-etag-" + t.Name()
		initial, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(emptyWorkflowID, storage.WorkflowDefinitionETagAbsent, "initial", 5))
		if err != nil {
			t.Fatalf("create draft: %v", err)
		}
		if _, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(emptyWorkflowID, "", "empty-etag", 6)); err == nil {
			t.Fatal("empty expected ETag authorized a draft overwrite")
		}
		current, err := s.GetWorkflowDraft(ctx, authorA, emptyWorkflowID)
		if err != nil {
			t.Fatalf("read current draft: %v", err)
		}
		if current.ETag != initial.ETag || string(current.ArtifactJSON) != string(draftArtifact("initial")) {
			t.Fatalf("empty expected ETag changed draft: %s", current.ArtifactJSON)
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

	t.Run("RunPagingWithTimestampTies", func(t *testing.T) {
		prefix := strings.ReplaceAll(t.Name(), "/", "-")
		sessionID := domain.SessionID("sess-" + prefix)
		parentID := domain.RunID("parent-" + prefix)
		createDefSession(t, slot, string(sessionID))
		if err := slot.Engine.CreateRun(ctx, domain.Run{
			ID: parentID, SessionID: sessionID, Status: domain.RunActive,
			Kind: domain.RunKindPrimary, CreatedAt: 1,
		}); err != nil {
			t.Fatalf("create parent run: %v", err)
		}
		otherSession := domain.SessionID("other-" + prefix)
		otherParent := domain.RunID("other-parent-" + prefix)
		createDefSession(t, slot, string(otherSession))
		if err := slot.Engine.CreateRun(ctx, domain.Run{
			ID: otherParent, SessionID: otherSession, Status: domain.RunActive,
			Kind: domain.RunKindPrimary, CreatedAt: 1,
		}); err != nil {
			t.Fatalf("create other parent run: %v", err)
		}
		wanted := []string{"run-a-" + prefix, "run-b-" + prefix, "run-c-" + prefix, "run-d-" + prefix}
		for i, id := range wanted {
			createdAt := int64(1000)
			if i == 3 {
				createdAt = 999
			}
			admitDefinitionRunFixture(t, slot, sessionID, parentID, domain.RunID(id), createdAt)
		}
		admitDefinitionRunFixture(t, slot, otherSession, otherParent, domain.RunID("run-other-"+prefix), 1000)

		var ids []string
		cursor := ""
		for len(ids) < len(wanted) {
			page, err := s.ListWorkflowDefinitionRuns(ctx, string(sessionID), cursor, 1)
			if err != nil {
				t.Fatalf("list runs after %q: %v", cursor, err)
			}
			if len(page.Runs) != 1 {
				t.Fatalf("page after %q has %d rows: %+v", cursor, len(page.Runs), page)
			}
			ids = append(ids, page.Runs[0].RunID)
			cursor = page.NextCursor
			if cursor == "" {
				break
			}
		}
		if !reflect.DeepEqual(ids, wanted) {
			t.Fatalf("paged ids: %v, want %v", ids, wanted)
		}
		if cursor != "" {
			t.Fatalf("terminal page cursor is not empty: %q", cursor)
		}
	})

	t.Run("RunPagingFromMissingBoundary", func(t *testing.T) {
		prefix := strings.ReplaceAll(t.Name(), "/", "-")
		sessionID := domain.SessionID("sess-" + prefix)
		parentID := domain.RunID("parent-" + prefix)
		createDefSession(t, slot, string(sessionID))
		if err := slot.Engine.CreateRun(ctx, domain.Run{
			ID: parentID, SessionID: sessionID, Status: domain.RunActive,
			Kind: domain.RunKindPrimary, CreatedAt: 1,
		}); err != nil {
			t.Fatalf("create parent run: %v", err)
		}
		admitDefinitionRunFixture(t, slot, sessionID, parentID, domain.RunID("run-b-"+prefix), 1000)
		admitDefinitionRunFixture(t, slot, sessionID, parentID, domain.RunID("run-c-"+prefix), 1000)
		admitDefinitionRunFixture(t, slot, sessionID, parentID, domain.RunID("run-d-"+prefix), 999)
		page, err := s.ListWorkflowDefinitionRuns(ctx, string(sessionID), storage.EncodeWorkflowRunCursor(1000, "run-a-"+prefix), 1)
		if err != nil || len(page.Runs) != 1 || page.Runs[0].RunID != "run-b-"+prefix {
			t.Fatalf("missing-boundary continuation: page=%+v err=%v", page, err)
		}
	})

	t.Run("DefinitionPagingWithColonIDs", func(t *testing.T) {
		const colonWorkflowID = "team:flow"
		draft, err := s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(colonWorkflowID, storage.WorkflowDefinitionETagAbsent, "colon-1", 20))
		if err != nil {
			t.Fatalf("create colon-id draft: %v", err)
		}
		rev1, err := s.PublishWorkflowRevisionCAS(ctx, authorA, colonWorkflowID, draft.ETag, defRevision(colonWorkflowID, "colon-1"))
		if err != nil || rev1.Revision != 1 {
			t.Fatalf("publish colon-id revision 1: revision=%+v err=%v", rev1, err)
		}
		draft, err = s.UpdateWorkflowDraftCAS(ctx, authorA, defDraftUpdate(colonWorkflowID, draft.ETag, "colon-2", 21))
		if err != nil {
			t.Fatalf("edit colon-id draft: %v", err)
		}
		rev2, err := s.PublishWorkflowRevisionCAS(ctx, authorA, colonWorkflowID, draft.ETag, defRevision(colonWorkflowID, "colon-2"))
		if err != nil || rev2.Revision != 2 {
			t.Fatalf("publish colon-id revision 2: revision=%+v err=%v", rev2, err)
		}
		first, err := s.ListWorkflowDefinitions(ctx, "", 1)
		if err != nil || len(first.Revisions) != 1 || first.Revisions[0].WorkflowID != colonWorkflowID || first.Revisions[0].Revision != 1 {
			t.Fatalf("first colon-id page: page=%+v err=%v", first, err)
		}
		second, err := s.ListWorkflowDefinitions(ctx, colonWorkflowID+":1", 1)
		if err != nil || len(second.Revisions) != 1 || second.Revisions[0].WorkflowID != colonWorkflowID || second.Revisions[0].Revision != 2 {
			t.Fatalf("second colon-id page: page=%+v err=%v", second, err)
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
