package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	inofy "github.com/ProjectViVy/inofy"
	"github.com/ProjectViVy/inofy/definitions"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

const inofyProductDefinition = `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
	{"id":"a","kind":"call","type":"vivy.child-task@1","config":{"task":"product node","tool_names":["echo_info"]}}
],"edges":[],"exits":["a"],"outputs":{"answer":{"source":"a","pointer":"/result"}}}}`

const inofyProductDefinitionV2 = `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
	{"id":"a","kind":"call","type":"vivy.child-task@1","config":{"task":"product node v2","tool_names":["echo_info"]}}
],"edges":[],"exits":["a"],"outputs":{"answer":{"source":"a","pointer":"/result"}}}}`

func inofyProductArtifact(definition string) json.RawMessage {
	return json.RawMessage(`{"definition":` + definition + `}`)
}

func prepareProductSession(t *testing.T, svc *Service, backend *sqlite.Backend, sessionID domain.SessionID) domain.RunID {
	t.Helper()
	parentRunID := domain.RunID("run-product-" + string(sessionID))
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})
	return parentRunID
}

// TestWorkflowProductDraftLifecycle covers draft CAS, author isolation,
// publish immutability and digest dedup through the host product surface.
func TestWorkflowProductDraftLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	prepareProductSession(t, svc, backend, "sess-prod-author")
	author := domain.SessionID("sess-prod-author")
	other := domain.SessionID("sess-prod-other")
	prepareProductSession(t, svc, backend, other)

	if _, err := svc.INOFYLoadDraft(ctx, author, "wf-product"); err == nil {
		t.Fatal("missing draft loaded")
	}
	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-product", definitions.ETagAbsent, inofyProductArtifact(inofyProductDefinition))
	if err != nil || draft.ETag == "" {
		t.Fatalf("create draft: %v etag=%q", err, draft.ETag)
	}
	if _, err := svc.INOFYSaveDraft(ctx, author, "wf-product", definitions.ETagAbsent, inofyProductArtifact(inofyProductDefinition)); err == nil {
		t.Fatal("create-only sentinel accepted an existing draft")
	}
	if _, err := svc.INOFYSaveDraft(ctx, author, "wf-product", "etag-stale", inofyProductArtifact(inofyProductDefinition)); err == nil {
		t.Fatal("stale etag update was accepted")
	} else {
		var inofyErr *inofy.Error
		if !errors.As(err, &inofyErr) || inofyErr.Code != inofy.ErrRevisionConflict {
			t.Fatalf("stale etag error = %v", err)
		}
	}
	updated, err := svc.INOFYSaveDraft(ctx, author, "wf-product", draft.ETag, inofyProductArtifact(inofyProductDefinitionV2))
	if err != nil || updated.ETag == draft.ETag {
		t.Fatalf("cas update: %v etag=%q", err, updated.ETag)
	}
	loaded, err := svc.INOFYLoadDraft(ctx, author, "wf-product")
	if err != nil || loaded.ETag != updated.ETag {
		t.Fatalf("load draft: %v etag=%q", err, loaded.ETag)
	}
	if _, err := svc.INOFYLoadDraft(ctx, other, "wf-product"); err == nil {
		t.Fatal("foreign author read the draft")
	}
	if _, err := svc.INOFYSaveDraft(ctx, other, "wf-product", loaded.ETag, inofyProductArtifact(inofyProductDefinition)); err == nil {
		t.Fatal("foreign author overwrote the draft")
	}
	if _, err := svc.INOFYPublishDraft(ctx, other, "wf-product", loaded.ETag); err == nil {
		t.Fatal("foreign author published the draft")
	}

	rev1, err := svc.INOFYPublishDraft(ctx, author, "wf-product", loaded.ETag)
	if err != nil || rev1.Revision != 1 {
		t.Fatalf("publish: %v rev=%d", err, rev1.Revision)
	}
	got, err := svc.INOFYGetRevision(ctx, author, "wf-product", 1)
	if err != nil || got.Revision != 1 || got.ArtifactDigest != rev1.ArtifactDigest {
		t.Fatalf("get revision: %v %+v", err, got)
	}
	// Re-publishing the unchanged artifact dedups onto revision 1.
	loaded2, _ := svc.INOFYLoadDraft(ctx, author, "wf-product")
	rev1Again, err := svc.INOFYPublishDraft(ctx, author, "wf-product", loaded2.ETag)
	if err != nil || rev1Again.Revision != 1 {
		t.Fatalf("dedup publish: %v rev=%d", err, rev1Again.Revision)
	}
	// Editing the draft back to the original artifact then publishing allocates
	// the next revision and the stored revision 1 artifact stays immutable.
	if _, err := svc.INOFYSaveDraft(ctx, author, "wf-product", loaded2.ETag, inofyProductArtifact(inofyProductDefinition)); err != nil {
		t.Fatal(err)
	}
	loaded3, _ := svc.INOFYLoadDraft(ctx, author, "wf-product")
	rev2, err := svc.INOFYPublishDraft(ctx, author, "wf-product", loaded3.ETag)
	if err != nil || rev2.Revision != 2 {
		t.Fatalf("publish rev2: %v rev=%d", err, rev2.Revision)
	}
	reread, err := svc.INOFYGetRevision(ctx, author, "wf-product", 1)
	if err != nil || reread.ArtifactDigest != rev1.ArtifactDigest {
		t.Fatalf("revision 1 mutated: %+v", reread)
	}
	page, err := svc.INOFYListWorkflows(ctx, author, "", 10)
	if err != nil || len(page.Revisions) != 2 {
		t.Fatalf("list workflows: %v %+v", err, page)
	}
}

func TestWorkflowProductSaveRequiresExplicitCAS(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-explicit-cas")
	other := domain.SessionID("sess-prod-explicit-cas-other")
	prepareProductSession(t, svc, backend, author)
	prepareProductSession(t, svc, backend, other)
	artifact := inofyProductArtifact(inofyProductDefinition)
	if _, err := svc.INOFYSaveDraft(ctx, author, "wf-explicit-cas", "", artifact); err == nil {
		t.Fatal("empty expected ETag accepted as implicit create or overwrite")
	} else {
		var inofyErr *inofy.Error
		if !errors.As(err, &inofyErr) || inofyErr.Code != inofy.ErrInvalidDefinition {
			t.Fatalf("empty expected ETag error = %v", err)
		}
	}
	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-explicit-cas", definitions.ETagAbsent, artifact)
	if err != nil {
		t.Fatalf("explicit create: %v", err)
	}
	if _, err := svc.INOFYSaveDraft(ctx, author, "wf-explicit-cas", definitions.ETagAbsent, artifact); err == nil {
		t.Fatal("duplicate explicit create succeeded")
	} else {
		var inofyErr *inofy.Error
		if !errors.As(err, &inofyErr) || inofyErr.Code != inofy.ErrRevisionConflict {
			t.Fatalf("duplicate create error = %v", err)
		}
	}
	updated, err := svc.INOFYSaveDraft(ctx, author, "wf-explicit-cas", draft.ETag, inofyProductArtifact(inofyProductDefinitionV2))
	if err != nil || updated.ETag == draft.ETag {
		t.Fatalf("explicit edit: draft=%+v err=%v", updated, err)
	}
	if _, err := svc.INOFYSaveDraft(ctx, other, "wf-explicit-cas", updated.ETag, artifact); err == nil {
		t.Fatal("foreign author overwrote a draft with a current ETag")
	} else {
		var inofyErr *inofy.Error
		if !errors.As(err, &inofyErr) || inofyErr.Code != inofy.ErrAuthorityDenied {
			t.Fatalf("foreign author error = %v", err)
		}
	}
	final, err := svc.INOFYLoadDraft(ctx, author, "wf-explicit-cas")
	if err != nil || final.ETag != updated.ETag || final.ArtifactDigest != updated.ArtifactDigest {
		t.Fatalf("draft after rejected writes: draft=%+v err=%v", final, err)
	}
}

// TestWorkflowProductRootPointerOutput proves an authored root JSON Pointer
// ("pointer":"") survives the typed decode -> marshal -> re-validate cycle
// every save performs: the studio editor writes this form for whole-result
// exit outputs.
func TestWorkflowProductRootPointerOutput(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-rootptr")
	prepareProductSession(t, svc, backend, author)

	def := `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[
	{"id":"a","kind":"call","type":"vivy.child-task@1","config":{"task":"root pointer","tool_names":["echo_info"]}}
],"edges":[],"exits":["a"],"outputs":{"answer":{"source":"a","pointer":""}}}}`
	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-rootptr", definitions.ETagAbsent, inofyProductArtifact(def))
	if err != nil {
		t.Fatalf("save root-pointer draft: %v", err)
	}
	resaved, err := svc.INOFYSaveDraft(ctx, author, "wf-rootptr", draft.ETag, inofyProductArtifact(def))
	if err != nil {
		t.Fatalf("re-save round-tripped root pointer: %v", err)
	}
	rev, err := svc.INOFYPublishDraft(ctx, author, "wf-rootptr", resaved.ETag)
	if err != nil || rev.Revision != 1 {
		t.Fatalf("publish root-pointer draft: %v rev=%+v", err, rev)
	}
}

// TestWorkflowProductPublishRejectsUnknownNode proves publish runs the same
// catalog admission the run path uses: an unknown node type can never become
// an immutable revision.
func TestWorkflowProductPublishRejectsUnknownNode(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-catalog")
	prepareProductSession(t, svc, backend, author)

	bad := `{"schema_version":"inofy.workflow/v1","graph":{"nodes":[{"id":"x","kind":"call","type":"foreign.node@9","config":{}}],"edges":[],"exits":["x"],"outputs":{"o":{"source":"x","pointer":"/result"}}}}`
	if _, err := svc.INOFYSaveDraft(ctx, author, "wf-bad", definitions.ETagAbsent, inofyProductArtifact(bad)); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	draft, err := svc.INOFYLoadDraft(ctx, author, "wf-bad")
	if err != nil {
		t.Fatal(err)
	}
	diags, err := svc.INOFYValidateDraft(ctx, author, "wf-bad", draft.ETag)
	if err == nil && len(diags) == 0 {
		t.Fatal("unknown node type validated clean")
	}
	if _, err := svc.INOFYPublishDraft(ctx, author, "wf-bad", draft.ETag); err == nil {
		t.Fatal("unknown node type published")
	}
}

func TestWorkflowProductPublishRejectsDuplicateTools(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-duplicate-tools")
	prepareProductSession(t, svc, backend, author)
	bad := strings.Replace(inofyProductDefinition, `"tool_names":["echo_info"]`, `"tool_names":["echo_info","echo_info"]`, 1)
	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-duplicate-tools", definitions.ETagAbsent, inofyProductArtifact(bad))
	if err != nil {
		t.Fatalf("save syntactically valid draft: %v", err)
	}
	diagnostics, err := svc.INOFYValidateDraft(ctx, author, "wf-duplicate-tools", draft.ETag)
	if err != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "tool_names") || !strings.Contains(diagnostics[0].Message, "equal") {
		t.Fatalf("duplicate tool diagnostics = %+v, %v", diagnostics, err)
	}
	if _, err := svc.INOFYPublishDraft(ctx, author, "wf-duplicate-tools", draft.ETag); err == nil {
		t.Fatal("duplicate tool names published")
	}
	page, err := svc.INOFYListWorkflows(ctx, author, "", 10)
	if err != nil || len(page.Revisions) != 0 {
		t.Fatalf("duplicate tool publication allocated a revision: page=%+v err=%v", page, err)
	}
}

func TestWorkflowProductHistoricalDuplicateToolsRejectStart(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-historical-duplicate-tools")
	parentRunID := prepareProductSession(t, svc, backend, author)
	workflowID := "wf-historical-duplicate-tools"
	bad := strings.Replace(inofyProductDefinition, `"tool_names":["echo_info"]`, `"tool_names":["echo_info","echo_info"]`, 1)
	artifact := inofyProductArtifact(bad)
	digest := "inofy-normal-v1:sha256:" + strings.Repeat("a", 64)
	draft, err := backend.UpdateWorkflowDraftCAS(ctx, string(author), storage.WorkflowDraftUpdate{
		WorkflowID: workflowID, ExpectedETag: storage.WorkflowDefinitionETagAbsent,
		ArtifactJSON: artifact, DefinitionDigest: digest, ArtifactDigest: digest, Now: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("seed historical draft: %v", err)
	}
	if _, err := backend.PublishWorkflowRevisionCAS(ctx, string(author), workflowID, draft.ETag, storage.WorkflowPublishedRevision{
		WorkflowID: workflowID, ArtifactJSON: artifact, DefinitionDigest: digest, ArtifactDigest: digest,
		UsedCatalogDigest: "sha256:" + strings.Repeat("b", 64), UsedImplementationsJSON: []byte(`{}`),
		AuthorSessionID: string(author), PublishedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("seed immutable historical publication: %v", err)
	}
	if revision, err := svc.INOFYGetRevision(ctx, author, workflowID, 1); err != nil || revision.Revision != 1 {
		t.Fatalf("historical revision should remain readable: revision=%+v err=%v", revision, err)
	}
	if _, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "historical-duplicate-tools-op", WorkflowID: workflowID, Revision: 1,
	}); !errors.Is(err, ErrINOFYInvalidDefinition) {
		t.Fatalf("historical duplicate-tool start = %v", err)
	}
	admissions, err := backend.ListWorkflowRevisions(ctx, parentRunID)
	if err != nil || len(admissions) != 0 {
		t.Fatalf("rejected historical start persisted admission: rows=%+v err=%v", admissions, err)
	}
}

// TestWorkflowProductRunBindsRevision drives a product run from a published
// revision and proves the admitted revision row carries the definition
// identity, operation dedup, per-session scoping and event paging.
func TestWorkflowProductRunBindsRevision(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-run")
	other := domain.SessionID("sess-prod-run-b")
	parentRunID := prepareProductSession(t, svc, backend, author)
	prepareProductSession(t, svc, backend, other)

	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-run", definitions.ETagAbsent, inofyProductArtifact(inofyProductDefinition))
	if err != nil {
		t.Fatal(err)
	}
	rev, err := svc.INOFYPublishDraft(ctx, author, "wf-run", draft.ETag)
	if err != nil {
		t.Fatal(err)
	}
	started, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "prod-op-1", WorkflowID: "wf-run",
		Revision: rev.Revision, Input: json.RawMessage(`{"q":"hello"}`),
	})
	if err != nil || !started.Created {
		t.Fatalf("start: %v %+v", err, started)
	}
	if started.Revision.DefinitionID != "wf-run" || started.Revision.DefinitionRevision != rev.Revision {
		t.Fatalf("definition binding = %+v", started.Revision)
	}
	if len(started.Revision.InputJSON) == 0 {
		t.Fatal("admitted input was not persisted")
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)

	// Same operation key + same revision dedups onto the same Run.
	again, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "prod-op-1", WorkflowID: "wf-run",
		Revision: rev.Revision, Input: json.RawMessage(`{"q":"hello"}`),
	})
	if err != nil || again.Created || again.Run.ID != started.Run.ID {
		t.Fatalf("dedup start: %v %+v", err, again)
	}
	// Same key, different input conflicts.
	if _, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "prod-op-1", WorkflowID: "wf-run",
		Revision: rev.Revision, Input: json.RawMessage(`{"q":"different"}`),
	}); !errors.Is(err, storage.ErrWorkflowRevisionConflict) {
		t.Fatalf("input mismatch under same key = %v", err)
	}
	// A foreign session cannot see the product run.
	if _, err := svc.INOFYGetRun(ctx, other, started.Run.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign get run = %v", err)
	}
	if _, _, _, err := svc.INOFYRunEvents(ctx, other, started.Run.ID, 0, 10); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign events = %v", err)
	}
	if _, err := svc.INOFYNodeOutput(ctx, other, started.Run.ID, "a"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign node output = %v", err)
	}
	if _, err := svc.INOFYCancelRun(ctx, other, started.Run.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign cancel = %v", err)
	}
	// A foreign parent run cannot host a product run.
	foreignParent := prepareProductSession(t, svc, backend, domain.SessionID("sess-prod-foreign"))
	if _, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: foreignParent, OperationKey: "prod-op-foreign", WorkflowID: "wf-run",
		Revision: rev.Revision,
	}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("foreign parent start = %v", err)
	}

	view, err := svc.INOFYGetRun(ctx, author, started.Run.ID)
	if err != nil || view.Revision.DefinitionID != "wf-run" || view.Details.EngineStatus != string(inofy.RunSucceeded) {
		t.Fatalf("get run: %v %+v", err, view)
	}
	output, err := svc.INOFYNodeOutput(ctx, author, started.Run.ID, "a")
	if err != nil || !strings.Contains(string(output), "product node") {
		t.Fatalf("node output: %v %s", err, output)
	}
	// Event paging: first page is bounded, cursor resumes without repeats.
	first, cursor, more, err := svc.INOFYRunEvents(ctx, author, started.Run.ID, 0, 2)
	if err != nil || len(first) != 2 || !more {
		t.Fatalf("events page1: %v n=%d more=%v", err, len(first), more)
	}
	rest, _, _, err := svc.INOFYRunEvents(ctx, author, started.Run.ID, cursor, 100)
	if err != nil || len(rest) == 0 {
		t.Fatalf("events page2: %v n=%d", err, len(rest))
	}
	for _, e := range rest {
		if e.Seq <= first[len(first)-1].Seq {
			t.Fatalf("page2 repeated seq %d", e.Seq)
		}
	}
	page, err := svc.INOFYListRuns(ctx, author, "", 10)
	if err != nil || len(page.Runs) != 1 || page.Runs[0].RunID != string(started.Run.ID) || page.Runs[0].Revision != rev.Revision {
		t.Fatalf("list runs: %v %+v", err, page)
	}
	if page.Runs[0].Status != string(domain.RunCompleted) || page.Runs[0].EngineStatus != string(inofy.RunSucceeded) {
		t.Fatalf("list/detail lifecycle disagree: summary=%+v detail=%+v", page.Runs[0], view.Details)
	}
}

// TestWorkflowProductDraftStartRun covers starting from a live draft etag.
func TestWorkflowProductDraftStartRun(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	author := domain.SessionID("sess-prod-draft-run")
	parentRunID := prepareProductSession(t, svc, backend, author)

	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-draft-run", definitions.ETagAbsent, inofyProductArtifact(inofyProductDefinition))
	if err != nil {
		t.Fatal(err)
	}
	started, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "prod-draft-op", WorkflowID: "wf-draft-run",
		DraftETag: draft.ETag,
	})
	if err != nil || started.Revision.DefinitionID != "wf-draft-run" || started.Revision.DefinitionRevision != 0 {
		t.Fatalf("draft start: %v %+v", err, started.Revision)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
	if _, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "prod-draft-op-2", WorkflowID: "wf-draft-run",
		DraftETag: "etag-stale",
	}); !errors.Is(err, storage.ErrWorkflowDefinitionConflict) {
		t.Fatalf("stale draft etag start = %v", err)
	}
}

// TestWorkflowProductHonestCapabilities proves the advertised capability set
// matches what the catalog actually supports: no waits, no resume.
func TestWorkflowProductHonestCapabilities(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	prepareProductSession(t, svc, backend, "sess-prod-caps")

	caps, err := svc.INOFYCapabilities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if caps["supports_wait"] != false || caps["supports_resume"] != false {
		t.Fatalf("capabilities overpromise waits: %+v", caps)
	}
	if err := svc.INOFYResumeRun(ctx, "sess-prod-caps", domain.RunID("run-any")); err == nil {
		t.Fatal("resume was honored on a wait-free catalog")
	} else {
		var inofyErr *inofy.Error
		if !errors.As(err, &inofyErr) || inofyErr.Code != inofy.ErrUnsupportedFeature {
			t.Fatalf("resume error = %v", err)
		}
	}
	nodeTypes, err := svc.INOFYNodeTypes(ctx)
	if err != nil || len(nodeTypes) != 1 || nodeTypes[0].TypeID != workflowChildType || nodeTypes[0].SupportsWait {
		t.Fatalf("node types: %v %+v", err, nodeTypes)
	}
}

// TestWorkflowProductCancelRun cancels an in-flight product run; the engine
// self-classifies recovery_required instead of a fabricated terminal.
func TestWorkflowProductCancelRun(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, blockingModel{})
	author := domain.SessionID("sess-prod-cancel")
	parentRunID := prepareProductSession(t, svc, backend, author)

	draft, err := svc.INOFYSaveDraft(ctx, author, "wf-cancel", definitions.ETagAbsent, inofyProductArtifact(inofyProductDefinition))
	if err != nil {
		t.Fatal(err)
	}
	rev, err := svc.INOFYPublishDraft(ctx, author, "wf-cancel", draft.ETag)
	if err != nil {
		t.Fatal(err)
	}
	started, err := svc.INOFYStartRun(ctx, author, INOFYStartRunParams{
		ParentRunID: parentRunID, OperationKey: "prod-cancel-op", WorkflowID: "wf-cancel", Revision: rev.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		children, _ := backend.ListChildRuns(ctx, started.Run.ID)
		if len(children) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	run, err := svc.INOFYCancelRun(ctx, author, started.Run.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if run.Status.Terminal() && run.Status != domain.RunCancelled {
		t.Fatalf("cancelled run = %+v", run)
	}
	var view INOFYRunView
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		view, err = svc.INOFYGetRun(ctx, author, started.Run.ID)
		if err == nil && view.Details.EngineStatus == string(inofy.RunRecoveryRequired) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || view.Details.Run.Status != domain.RunActive || view.Details.EngineStatus != string(inofy.RunRecoveryRequired) {
		t.Fatalf("cancelled workflow lifecycle detail = %+v err=%v", view.Details, err)
	}
	page, err := svc.INOFYListRuns(ctx, author, "", 10)
	if err != nil || len(page.Runs) != 1 || page.Runs[0].RunID != string(started.Run.ID) ||
		page.Runs[0].Status != string(domain.RunActive) || page.Runs[0].EngineStatus != string(inofy.RunRecoveryRequired) {
		t.Fatalf("recovery list/detail lifecycle disagree: page=%+v detail=%+v err=%v", page, view.Details, err)
	}
}

// TestWorkflowProductDisabledKeepsCorePath proves a backend without the
// definition store leaves the core task-graph route working.
func TestWorkflowProductDisabledKeepsCorePath(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	sessionID := domain.SessionID("sess-prod-off")
	parentRunID := domain.RunID("run-prod-off-parent")
	prepareChildSessionAuthorizer(t, svc, backend, sessionID, parentRunID, []string{tools.EchoInfoName})
	svc.deps.WorkflowDefinitions = nil

	if _, err := svc.INOFYLoadDraft(ctx, sessionID, "wf"); !errors.Is(err, ErrWorkflowProductUnavailable) {
		t.Fatalf("load draft on disabled product = %v", err)
	}
	if _, err := svc.INOFYCapabilities(ctx); !errors.Is(err, ErrWorkflowProductUnavailable) {
		t.Fatalf("capabilities on disabled product = %v", err)
	}
	started, err := svc.StartINOFYWorkflow(ctx, parentRunID, "wf-op-off", json.RawMessage(inofyTwoNodeDefinition))
	if err != nil {
		t.Fatalf("core workflow start on disabled product: %v", err)
	}
	waitForRunStatus(t, backend, started.Run.ID, domain.RunCompleted)
}
