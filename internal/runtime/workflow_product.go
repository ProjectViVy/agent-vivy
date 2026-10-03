package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ProjectViVy/inofy"
	"github.com/ProjectViVy/inofy/definitions"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// ErrWorkflowProductUnavailable reports that the reusable-workflow product
// surface (drafts, publishing, product runs) is not wired — e.g. the selected
// Core Storage backend does not implement WorkflowDefinitionStore. The core
// task-graph path (workflow/start on a raw definition) keeps working.
var ErrWorkflowProductUnavailable = errors.New("runtime: workflow product is not available")

// ErrWorkflowProductResumeUnsupported is the honest wait/resume answer for a
// catalog whose node descriptors do not implement waits.
var ErrWorkflowProductResumeUnsupported = &inofy.Error{
	Code: inofy.ErrUnsupportedFeature, Message: "wait/resume is not supported by this node catalog",
}

// workflowDefinitionService resolves the INOFY definitions service bound to
// one author session. Draft authority is session-scoped; published revisions
// are organism-visible.
func (s *Service) workflowDefinitionRepo(sessionID domain.SessionID) (*inofyDefinitionRepository, error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return nil, ErrWorkflowProductUnavailable
	}
	if strings.TrimSpace(string(sessionID)) == "" {
		return nil, errors.New("runtime: workflow product requires a session context")
	}
	if _, err := s.deps.Sessions.GetSession(context.Background(), sessionID); err != nil {
		return nil, err
	}
	return &inofyDefinitionRepository{store: s.deps.WorkflowDefinitions, session: sessionID}, nil
}

func (s *Service) workflowDefinitionService(sessionID domain.SessionID) (*definitions.Service, error) {
	repo, err := s.workflowDefinitionRepo(sessionID)
	if err != nil {
		return nil, err
	}
	return definitions.NewService(repo), nil
}

// workflowHostToolUniverse is the widest honest tool ceiling for draft-time
// validation: every workflow-eligible tool the host can grant. Admission still
// narrows to the calling parent's authority.
func (s *Service) workflowHostToolUniverse() []string {
	if s == nil || s.engine == nil {
		return nil
	}
	names := make([]string, 0, len(s.engine.SelectTools().Specs))
	for _, spec := range s.engine.SelectTools().Specs {
		names = append(names, spec.Name)
	}
	return s.workflowChildTools(names)
}

func decodeArtifactDefinition(artifactJSON []byte) (json.RawMessage, error) {
	a, diags, err := inofy.DecodeArtifact(artifactJSON)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrINOFYInvalidDefinition, err)
	}
	if len(diags) != 0 {
		return nil, fmt.Errorf("%w: %v", ErrINOFYInvalidDefinition, diags)
	}
	defJSON, err := json.Marshal(a.Definition)
	if err != nil {
		return nil, err
	}
	return defJSON, nil
}

// INOFYCapabilities reports the host action surface honestly: drafts,
// publishing, runs, event paging/subscription and cancel exist; the trusted
// catalog has no wait-capable node so resume is advertised false.
func (s *Service) INOFYCapabilities(ctx context.Context) (map[string]any, error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return nil, ErrWorkflowProductUnavailable
	}
	return map[string]any{
		"schema_version":  inofy.SchemaVersionV1,
		"features":        []string{"draft", "publish", "validate", "runs", "events", "cancel", "node_types", "connections"},
		"supports_wait":   false,
		"supports_resume": false,
	}, nil
}

// INOFYNodeTypes returns the trusted catalog's node descriptors.
func (s *Service) INOFYNodeTypes(ctx context.Context) ([]inofy.NodeDescriptor, error) {
	catalog, err := trustedINOFYCatalog()
	if err != nil {
		return nil, err
	}
	descriptors := make([]inofy.NodeDescriptor, 0, 1)
	for _, typeID := range catalog.Types() {
		if descriptor, ok := catalog.Lookup(typeID); ok {
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors, nil
}

// INOFYLoadDraft returns the caller's draft for one workflow.
func (s *Service) INOFYLoadDraft(ctx context.Context, sessionID domain.SessionID, workflowID string) (definitions.Draft, error) {
	repo, err := s.workflowDefinitionRepo(sessionID)
	if err != nil {
		return definitions.Draft{}, err
	}
	return repo.GetDraft(ctx, workflowID)
}

// INOFYSaveDraft stores one artifact under CAS. expectedETag empty means
// "create or overwrite unconditionally"; use definitions.ETagAbsent to require
// create-only semantics.
func (s *Service) INOFYSaveDraft(ctx context.Context, sessionID domain.SessionID, workflowID, expectedETag string, artifactJSON json.RawMessage) (definitions.Draft, error) {
	svc, err := s.workflowDefinitionService(sessionID)
	if err != nil {
		return definitions.Draft{}, err
	}
	a, err := decodeArtifactValue(artifactJSON)
	if err != nil {
		return definitions.Draft{}, err
	}
	return svc.SaveDraft(ctx, workflowID, expectedETag, a)
}

func decodeArtifactValue(raw json.RawMessage) (inofy.Artifact, error) {
	a, diags, err := inofy.DecodeArtifact(raw)
	if err != nil {
		return inofy.Artifact{}, fmt.Errorf("%w: %v", ErrINOFYInvalidDefinition, err)
	}
	if len(diags) != 0 {
		return inofy.Artifact{}, fmt.Errorf("%w: %v", ErrINOFYInvalidDefinition, diags)
	}
	return a, nil
}

// INOFYValidateDraft compiles the stored draft against the trusted catalog
// and additionally applies host admission rules (kind/retry/limits/tools vs
// the host tool universe). Catalog diagnostics are returned as diagnostics;
// a host-rule violation surfaces as one capability diagnostic.
func (s *Service) INOFYValidateDraft(ctx context.Context, sessionID domain.SessionID, workflowID, expectedETag string) ([]inofy.Diagnostic, error) {
	svc, err := s.workflowDefinitionService(sessionID)
	if err != nil {
		return nil, err
	}
	catalog, err := trustedINOFYCatalog()
	if err != nil {
		return nil, err
	}
	diags, err := svc.ValidateDraft(ctx, workflowID, expectedETag, catalog)
	if err != nil || len(diags) != 0 {
		return diags, err
	}
	repo, err := s.workflowDefinitionRepo(sessionID)
	if err != nil {
		return nil, err
	}
	draft, err := repo.GetDraft(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	defJSON, err := json.Marshal(draft.Artifact.Definition)
	if err != nil {
		return nil, err
	}
	if _, err := validateINOFYDefinition(ctx, defJSON, s.workflowHostToolUniverse()); err != nil {
		return []inofy.Diagnostic{{Check: inofy.CheckCapability, Path: "", Code: "host_admission", Message: err.Error()}}, nil
	}
	return nil, nil
}

// INOFYPublishDraft compiles and publishes the caller's draft under CAS. A
// definition that would fail admission is rejected here rather than becoming
// an unstartable immutable revision.
func (s *Service) INOFYPublishDraft(ctx context.Context, sessionID domain.SessionID, workflowID, expectedETag string) (definitions.Revision, error) {
	svc, err := s.workflowDefinitionService(sessionID)
	if err != nil {
		return definitions.Revision{}, err
	}
	repo, err := s.workflowDefinitionRepo(sessionID)
	if err != nil {
		return definitions.Revision{}, err
	}
	draft, err := repo.GetDraft(ctx, workflowID)
	if err != nil {
		return definitions.Revision{}, err
	}
	if expectedETag != "" && draft.ETag != expectedETag {
		return definitions.Revision{}, &inofy.Error{Code: inofy.ErrRevisionConflict, Path: workflowID, Message: "draft etag changed"}
	}
	defJSON, err := json.Marshal(draft.Artifact.Definition)
	if err != nil {
		return definitions.Revision{}, err
	}
	if _, err := validateINOFYDefinition(ctx, defJSON, s.workflowHostToolUniverse()); err != nil {
		return definitions.Revision{}, fmt.Errorf("%w: %w", ErrINOFYInvalidDefinition, err)
	}
	catalog, err := trustedINOFYCatalog()
	if err != nil {
		return definitions.Revision{}, err
	}
	return svc.Publish(ctx, workflowID, expectedETag, catalog)
}

// INOFYGetRevision returns one immutable published revision.
func (s *Service) INOFYGetRevision(ctx context.Context, sessionID domain.SessionID, workflowID string, revision uint64) (definitions.Revision, error) {
	svc, err := s.workflowDefinitionService(sessionID)
	if err != nil {
		return definitions.Revision{}, err
	}
	return svc.GetRevision(ctx, workflowID, revision)
}

// INOFYListWorkflows pages published revisions in key order.
func (s *Service) INOFYListWorkflows(ctx context.Context, sessionID domain.SessionID, cursor string, limit int) (definitions.Page, error) {
	svc, err := s.workflowDefinitionService(sessionID)
	if err != nil {
		return definitions.Page{}, err
	}
	return svc.List(ctx, cursor, limit)
}

// INOFYStartRunParams binds a product run to a published revision (Revision>0)
// or a draft snapshot (Revision==0, DraftETag pins the current draft etag).
// OperationKey is the caller-supplied admission dedup key (same semantics as
// workflow/start: a repeat returns the same Run, a mismatch conflicts).
type INOFYStartRunParams struct {
	ParentRunID  domain.RunID
	OperationKey string
	WorkflowID   string
	Revision     uint64
	DraftETag    string
	Input        json.RawMessage
}

func (s *Service) INOFYStartRun(ctx context.Context, sessionID domain.SessionID, params INOFYStartRunParams) (WorkflowStartResult, error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return WorkflowStartResult{}, ErrWorkflowProductUnavailable
	}
	if _, err := s.workflowDefinitionService(sessionID); err != nil {
		return WorkflowStartResult{}, err
	}
	if strings.TrimSpace(params.WorkflowID) == "" || params.Revision == 0 && strings.TrimSpace(params.DraftETag) == "" {
		return WorkflowStartResult{}, errors.New("runtime: workflow id plus revision or draft_etag is required")
	}
	var artifactJSON []byte
	source := WorkflowDefinitionSource{DefinitionID: params.WorkflowID}
	if params.Revision > 0 {
		rec, err := s.deps.WorkflowDefinitions.GetWorkflowPublishedRevision(ctx, params.WorkflowID, params.Revision)
		if err != nil {
			return WorkflowStartResult{}, err
		}
		artifactJSON = rec.ArtifactJSON
		source.DefinitionRevision = rec.Revision
	} else {
		rec, err := s.deps.WorkflowDefinitions.GetWorkflowDraft(ctx, string(sessionID), params.WorkflowID)
		if err != nil {
			return WorkflowStartResult{}, err
		}
		if rec.ETag != params.DraftETag {
			return WorkflowStartResult{}, storage.ErrWorkflowDefinitionConflict
		}
		artifactJSON = rec.ArtifactJSON
	}
	defJSON, err := decodeArtifactDefinition(artifactJSON)
	if err != nil {
		return WorkflowStartResult{}, err
	}
	// The parent run scopes the product run: it must live in the caller's
	// session, so a foreign parent id is indistinguishable from missing.
	parent, err := s.deps.Runs.GetRun(ctx, params.ParentRunID)
	if err != nil {
		return WorkflowStartResult{}, err
	}
	if parent.SessionID != sessionID {
		return WorkflowStartResult{}, storage.ErrNotFound
	}
	return s.startINOFYWorkflow(ctx, params.ParentRunID, params.OperationKey, defJSON, params.Input, &source, nil)
}

// INOFYListRuns pages the session's product runs (runs admitted with a
// definition binding) newest-first.
func (s *Service) INOFYListRuns(ctx context.Context, sessionID domain.SessionID, cursor string, limit int) (storage.WorkflowRunPage, error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return storage.WorkflowRunPage{}, ErrWorkflowProductUnavailable
	}
	return s.deps.WorkflowDefinitions.ListWorkflowDefinitionRuns(ctx, string(sessionID), cursor, limit)
}

// INOFYRunView pairs the run inspection detail with its definition binding.
type INOFYRunView struct {
	Details  WorkflowDetails
	Revision domain.WorkflowRevision
}

// productRunScope resolves a workflow run and verifies the caller's session
// owns it; foreign run ids answer not-found.
func (s *Service) productRunScope(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) (domain.Run, error) {
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return domain.Run{}, err
	}
	if run.SessionID != sessionID {
		return domain.Run{}, storage.ErrNotFound
	}
	return run, nil
}

func (s *Service) INOFYGetRun(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) (INOFYRunView, error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return INOFYRunView{}, ErrWorkflowProductUnavailable
	}
	if _, err := s.productRunScope(ctx, sessionID, runID); err != nil {
		return INOFYRunView{}, err
	}
	revision, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID)
	if err != nil {
		return INOFYRunView{}, err
	}
	details, err := s.GetWorkflow(ctx, runID)
	if err != nil {
		return INOFYRunView{}, err
	}
	return INOFYRunView{Details: details, Revision: revision}, nil
}

// INOFYRunEvents pages journaled events for the session's own run starting
// after `after` (event sequence). nextCursor is empty when the page is the
// last; otherwise it is the last returned sequence, so the caller resumes at
// or before the first unseen event.
func (s *Service) INOFYRunEvents(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, after uint64, limit int) (events []domain.RunEvent, nextCursor uint64, more bool, err error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return nil, 0, false, ErrWorkflowProductUnavailable
	}
	if _, err := s.productRunScope(ctx, sessionID, runID); err != nil {
		return nil, 0, false, err
	}
	if limit <= 0 {
		limit = 100
	}
	iterator, err := s.deps.Journal.Replay(ctx, runID, domain.EventSeq(after))
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { _ = iterator.Close() }()
	for len(events) < limit && iterator.Next() {
		events = append(events, iterator.Value().Event)
	}
	if err := iterator.Err(); err != nil {
		return nil, 0, false, err
	}
	if len(events) == 0 || !iterator.Next() {
		return events, 0, false, nil
	}
	return events, uint64(events[len(events)-1].Seq), true, nil
}

// INOFYNodeOutput returns the committed output blob of one node, addressed by
// its graph id or its full engine path.
func (s *Service) INOFYNodeOutput(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, node string) (json.RawMessage, error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return nil, ErrWorkflowProductUnavailable
	}
	if _, err := s.productRunScope(ctx, sessionID, runID); err != nil {
		return nil, err
	}
	node = strings.TrimSpace(node)
	if node == "" {
		return nil, storage.ErrNotFound
	}
	if !strings.HasPrefix(node, "/") {
		node = "/graph/nodes/" + node
	}
	engine, err := s.inofyEngine()
	if err != nil {
		return nil, err
	}
	state, err := engine.LoadWorkflowStep(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("runtime: load workflow engine state: %w", err)
	}
	var result *storage.WorkflowStepResult
	for i := range state.Results {
		if state.Results[i].Path == node && (result == nil || state.Results[i].Attempt > result.Attempt) {
			result = &state.Results[i]
		}
	}
	if result == nil || result.BlobID == "" {
		return nil, storage.ErrNotFound
	}
	blob, found, err := engine.Blobs().Get(ctx, result.BlobID)
	if err != nil || !found {
		return nil, storage.ErrNotFound
	}
	return json.RawMessage(blob), nil
}

// INOFYCancelRun cancels the session's own workflow run through the governed
// cancellation path.
func (s *Service) INOFYCancelRun(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) (domain.Run, error) {
	if s == nil || s.deps.WorkflowDefinitions == nil {
		return domain.Run{}, ErrWorkflowProductUnavailable
	}
	if _, err := s.productRunScope(ctx, sessionID, runID); err != nil {
		return domain.Run{}, err
	}
	return s.CancelWorkflow(ctx, runID)
}

// INOFYResumeRun honestly reports that the trusted catalog has no
// wait-capable node; nothing to resume.
func (s *Service) INOFYResumeRun(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) error {
	return ErrWorkflowProductResumeUnsupported
}
