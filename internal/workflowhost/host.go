package workflowhost

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	workflow "agent-vivy/internal/workflow"
)

var (
	ErrHostNotConfigured = errors.New("workflow host is not configured")
	ErrInvalidInputs     = errors.New("workflow inputs are invalid")
)

// CapabilitySnapshot is a secret-free, point-in-time view used by both
// validation and invocation. Identity is opaque to workflow definitions but
// is persisted with a future run so capability drift is observable.
type CapabilitySnapshot struct {
	Validation workflow.ValidationContext
	Identity   json.RawMessage
	Hash       string
}

// CapabilitySource supplies a fresh snapshot at each validation boundary.
// Implementations must return declarative state only; credentials and raw
// provider settings never belong in Identity.
type CapabilitySource interface {
	Snapshot(context.Context) (CapabilitySnapshot, error)
}

type CapabilitySourceFunc func(context.Context) (CapabilitySnapshot, error)

func (f CapabilitySourceFunc) Snapshot(ctx context.Context) (CapabilitySnapshot, error) {
	return f(ctx)
}

// Config is the narrow composition seam for WorkflowHost. WorkflowHost does
// not accept a storage.Engine so embedders cannot accidentally hand it a
// second persistence path.
type Config struct {
	Definitions  storage.WorkflowStore
	WorkflowRuns storage.WorkflowRunStore
	Sessions     storage.SessionStore
	Runs         storage.RunStore
	Journal      storage.Journal
	Executor     domain.PlanExecutor
	Capabilities CapabilitySource
	Limits       workflow.Limits
	Now          func() time.Time
}

type Host struct {
	definitions  storage.WorkflowStore
	workflowRuns storage.WorkflowRunStore
	sessions     storage.SessionStore
	runs         storage.RunStore
	journal      storage.Journal
	executor     domain.PlanExecutor
	capabilities CapabilitySource
	limits       workflow.Limits
	now          func() time.Time
}

func New(cfg Config) (*Host, error) {
	if cfg.Definitions == nil {
		return nil, fmt.Errorf("%w: workflow definition store is required", ErrHostNotConfigured)
	}
	limits := cfg.Limits
	defaults := workflow.DefaultLimits()
	if limits.MaxNodes == 0 {
		limits.MaxNodes = defaults.MaxNodes
	}
	if limits.MaxParallelism == 0 {
		limits.MaxParallelism = defaults.MaxParallelism
	}
	if limits.MaxNodeOutputBytes == 0 {
		limits.MaxNodeOutputBytes = defaults.MaxNodeOutputBytes
	}
	if limits.DefinitionMaxBytes == 0 {
		limits.DefinitionMaxBytes = defaults.DefinitionMaxBytes
	}
	if limits.MaxNodes < 0 || limits.MaxParallelism < 0 || limits.MaxNodeOutputBytes < 0 || limits.DefinitionMaxBytes < 0 {
		return nil, fmt.Errorf("%w: workflow limits must not be negative", ErrHostNotConfigured)
	}
	executor := cfg.Executor
	if executor == nil {
		executor = UnavailableExecutor{}
	}
	capabilities := cfg.Capabilities
	if capabilities == nil {
		capabilities = CapabilitySourceFunc(func(context.Context) (CapabilitySnapshot, error) {
			return CapabilitySnapshot{}, nil
		})
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Host{
		definitions: cfg.Definitions, workflowRuns: cfg.WorkflowRuns,
		sessions: cfg.Sessions, runs: cfg.Runs, journal: cfg.Journal,
		executor: executor, capabilities: capabilities, limits: limits, now: now,
	}, nil
}

type DefinitionSummary struct {
	ID        string `json:"id"`
	LatestRev int64  `json:"latest_rev"`
	Hash      string `json:"hash"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
}

type DefinitionView struct {
	ID         string          `json:"id"`
	Rev        int64           `json:"rev"`
	Hash       string          `json:"hash"`
	Definition json.RawMessage `json:"definition"`
	CreatedAt  int64           `json:"created_at"`
}

type DefineResult struct {
	ID   string `json:"id"`
	Rev  int64  `json:"rev"`
	Hash string `json:"hash"`
}

type RunRequest struct {
	ID        string
	Rev       int64
	Inputs    map[string]any
	SessionID domain.SessionID
}

type RunResult struct {
	RunID  domain.RunID     `json:"run_id"`
	Status domain.RunStatus `json:"status"`
}

// RunSummary is the bounded history projection exposed by workflow/runs and
// workflow_runs. Full node outcomes remain behind the durable store and are
// never copied into an agent catalog response.
type RunSummary struct {
	RunID      domain.RunID     `json:"run_id"`
	WorkflowID string           `json:"workflow_id"`
	Rev        int64            `json:"rev"`
	Hash       string           `json:"hash"`
	SessionID  domain.SessionID `json:"session_id"`
	Status     domain.RunStatus `json:"status"`
	Reason     string           `json:"reason,omitempty"`
	CreatedAt  int64            `json:"created_at"`
	UpdatedAt  int64            `json:"updated_at"`
}

type DefinePreview struct {
	ID        string
	NextRev   int64
	Hash      string
	NodeCount int
	Title     string
}

type RunPreview struct {
	ID             string
	Rev            int64
	Hash           string
	CapabilityHash string
	InputsSHA256   string
}

func (h *Host) List(ctx context.Context) ([]DefinitionSummary, error) {
	if h == nil || h.definitions == nil {
		return nil, ErrHostNotConfigured
	}
	records, err := h.definitions.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DefinitionSummary, 0, len(records))
	for _, record := range records {
		title := ""
		if definition, _, _, decodeErr := workflow.Decode(record.Definition); decodeErr == nil {
			title = definition.Title
		}
		out = append(out, DefinitionSummary{ID: record.ID, LatestRev: record.Rev, Hash: record.Hash, Title: title, CreatedAt: record.CreatedAt})
	}
	return out, nil
}

func (h *Host) Get(ctx context.Context, id string, rev int64) (DefinitionView, error) {
	record, err := h.loadDefinition(ctx, id, rev)
	if err != nil {
		return DefinitionView{}, err
	}
	return DefinitionView{
		ID: record.ID, Rev: record.Rev, Hash: record.Hash,
		Definition: append(json.RawMessage(nil), record.Definition...), CreatedAt: record.CreatedAt,
	}, nil
}

func (h *Host) Validate(ctx context.Context, raw []byte) (workflow.ValidationResult, error) {
	snapshot, err := h.capabilitySnapshot(ctx)
	if err != nil {
		return workflow.ValidationResult{}, err
	}
	return workflow.Validate(raw, snapshot.Validation)
}

func (h *Host) ValidateInvocation(ctx context.Context, record storage.WorkflowDefinitionRecord, inputs map[string]any) (workflow.ValidationResult, domain.PlanExecutor, CapabilitySnapshot, error) {
	snapshot, err := h.capabilitySnapshot(ctx)
	if err != nil {
		return workflow.ValidationResult{}, nil, CapabilitySnapshot{}, err
	}
	result, err := workflow.ValidateInvocation(record.Definition, snapshot.Validation)
	if err != nil {
		return result, nil, snapshot, err
	}
	if err := validateInputs(result.Definition, inputs); err != nil {
		return result, nil, snapshot, err
	}
	return result, h.executor, snapshot, nil
}

func (h *Host) Define(ctx context.Context, raw []byte) (DefineResult, error) {
	result, err := h.Validate(ctx, raw)
	if err != nil {
		return DefineResult{}, err
	}
	latest, err := h.definitions.LatestDefinition(ctx, result.Definition.ID)
	nextRev := int64(1)
	if err == nil {
		nextRev = latest.Rev + 1
	} else if !errors.Is(err, storage.ErrNotFound) {
		return DefineResult{}, err
	}
	record := storage.WorkflowDefinitionRecord{
		ID: result.Definition.ID, Rev: nextRev, Hash: result.Hash,
		Definition: append([]byte(nil), result.Canonical...), CreatedAt: h.now().UnixMilli(),
	}
	if err := h.definitions.SaveDefinition(ctx, record); err != nil {
		return DefineResult{}, err
	}
	return DefineResult{ID: record.ID, Rev: record.Rev, Hash: record.Hash}, nil
}

func (h *Host) PreviewDefine(ctx context.Context, raw []byte) (DefinePreview, error) {
	result, err := h.Validate(ctx, raw)
	if err != nil {
		return DefinePreview{}, err
	}
	latest, err := h.definitions.LatestDefinition(ctx, result.Definition.ID)
	nextRev := int64(1)
	if err == nil {
		nextRev = latest.Rev + 1
	} else if !errors.Is(err, storage.ErrNotFound) {
		return DefinePreview{}, err
	}
	return DefinePreview{ID: result.Definition.ID, NextRev: nextRev, Hash: result.Hash, NodeCount: len(result.Definition.Nodes), Title: result.Definition.Title}, nil
}

func (h *Host) PreviewRun(ctx context.Context, request RunRequest) (RunPreview, error) {
	record, err := h.loadDefinition(ctx, request.ID, request.Rev)
	if err != nil {
		return RunPreview{}, err
	}
	result, _, snapshot, err := h.ValidateInvocation(ctx, record, request.Inputs)
	if err != nil {
		return RunPreview{}, err
	}
	if _, err := workflow.Compile(result, record.Rev, snapshot.Validation); err != nil {
		return RunPreview{}, err
	}
	return RunPreview{ID: record.ID, Rev: record.Rev, Hash: record.Hash, CapabilityHash: snapshot.Hash, InputsSHA256: digestJSON(request.Inputs)}, nil
}

// Run performs the complete preflight path. WF-1's unavailable executor is
// rejected before CreateSession/CreateRun, which prevents an accepted orphan
// from pretending that scheduling exists.
func (h *Host) Run(ctx context.Context, request RunRequest) (RunResult, error) {
	record, err := h.loadDefinition(ctx, request.ID, request.Rev)
	if err != nil {
		return RunResult{}, err
	}
	result, executor, snapshot, err := h.ValidateInvocation(ctx, record, request.Inputs)
	if err != nil {
		return RunResult{}, err
	}
	plan, err := workflow.Compile(result, record.Rev, snapshot.Validation)
	if err != nil {
		return RunResult{}, err
	}
	if availability, ok := executor.(interface{ Available() bool }); ok && !availability.Available() {
		return RunResult{}, workflow.ErrExecutionUnavailable
	}
	if h.sessions == nil || h.runs == nil || h.workflowRuns == nil || h.journal == nil {
		return RunResult{}, fmt.Errorf("%w: workflow run stores are incomplete", ErrHostNotConfigured)
	}
	sessionID := request.SessionID
	if sessionID == "" {
		var generatedID string
		generatedID, err = newID("sess_workflow_")
		if err != nil {
			return RunResult{}, err
		}
		sessionID = domain.SessionID(generatedID)
		if err := h.sessions.CreateSession(ctx, domain.Session{ID: sessionID, Title: fmt.Sprintf("workflow %s@%d", record.ID, record.Rev), CreatedAt: h.now().UnixMilli()}); err != nil {
			return RunResult{}, err
		}
	} else if _, err := h.sessions.GetSession(ctx, sessionID); err != nil {
		return RunResult{}, err
	}
	generatedID, err := newID("run_workflow_")
	if err != nil {
		return RunResult{}, err
	}
	runID := domain.RunID(generatedID)
	now := h.now().UnixMilli()
	run := domain.Run{ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: now, Kind: domain.RunKindWorkflow, RootID: runID}
	if err := h.runs.CreateRun(ctx, run); err != nil {
		return RunResult{}, err
	}
	capabilityJSON := append([]byte(nil), snapshot.Identity...)
	if len(capabilityJSON) == 0 {
		capabilityJSON = []byte(`{}`)
	}
	if err := h.workflowRuns.SaveWorkflowRun(ctx, storage.WorkflowRunRecord{RunID: runID, WorkflowID: record.ID, Rev: record.Rev, Hash: record.Hash, CapabilityJSON: capabilityJSON, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: now, UpdatedAt: now}); err != nil {
		return RunResult{}, err
	}
	if err := h.runs.SetRunStatus(ctx, runID, domain.RunActive); err != nil {
		return RunResult{}, err
	}
	if err := h.appendEvent(ctx, runID, domain.EventRunStarted, map[string]any{"provider": "", "model": "workflow", "mode": "normal"}); err != nil {
		return RunResult{}, err
	}
	if err := h.appendEvent(ctx, runID, domain.EventWorkflowInvoked, map[string]any{"definition_id": record.ID, "rev": record.Rev, "hash": record.Hash, "capability_hash": snapshot.Hash, "inputs_sha256": digestJSON(request.Inputs)}); err != nil {
		return RunResult{}, err
	}
	planResult, execErr := executor.ExecutePlan(ctx, runID, plan, h.eventSink(runID))
	outcomes, _ := json.Marshal(planResult.Outcomes)
	if execErr != nil {
		_ = h.workflowRuns.SetWorkflowRunTerminal(ctx, runID, domain.RunFailed, "workflow execution failed", outcomes)
		_ = h.runs.SetRunStatus(ctx, runID, domain.RunFailed)
		return RunResult{}, execErr
	}
	_ = h.workflowRuns.SetWorkflowRunTerminal(ctx, runID, domain.RunCompleted, "", outcomes)
	_ = h.runs.SetRunStatus(ctx, runID, domain.RunCompleted)
	return RunResult{RunID: runID, Status: domain.RunCompleted}, nil
}

func (h *Host) Runs(ctx context.Context, id string, limit int) ([]RunSummary, error) {
	if h == nil || h.workflowRuns == nil {
		return nil, ErrHostNotConfigured
	}
	records, err := h.workflowRuns.ListWorkflowRunsByDefinition(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	out := make([]RunSummary, 0, len(records))
	for _, record := range records {
		out = append(out, RunSummary{RunID: record.RunID, WorkflowID: record.WorkflowID, Rev: record.Rev, Hash: record.Hash, SessionID: record.SessionID, Status: record.Status, Reason: record.Reason, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
	}
	return out, nil
}

func (h *Host) loadDefinition(ctx context.Context, id string, rev int64) (storage.WorkflowDefinitionRecord, error) {
	if h == nil || h.definitions == nil {
		return storage.WorkflowDefinitionRecord{}, ErrHostNotConfigured
	}
	if strings.TrimSpace(id) == "" {
		return storage.WorkflowDefinitionRecord{}, fmt.Errorf("%w: workflow id is required", ErrInvalidInputs)
	}
	if rev > 0 {
		return h.definitions.GetDefinition(ctx, id, rev)
	}
	return h.definitions.LatestDefinition(ctx, id)
}

func (h *Host) capabilitySnapshot(ctx context.Context) (CapabilitySnapshot, error) {
	snapshot, err := h.capabilities.Snapshot(ctx)
	if err != nil {
		return CapabilitySnapshot{}, err
	}
	snapshot.Validation.Limits = h.limits
	if len(snapshot.Identity) == 0 {
		identity, marshalErr := json.Marshal(map[string]any{
			"model_profiles":  snapshot.Validation.ModelProfiles,
			"tools":           snapshot.Validation.Tools,
			"available_tools": snapshot.Validation.AvailableTools,
		})
		if marshalErr != nil {
			return CapabilitySnapshot{}, marshalErr
		}
		snapshot.Identity = identity
	}
	digest := sha256.Sum256(snapshot.Identity)
	snapshot.Hash = hex.EncodeToString(digest[:])
	snapshot.Identity = append(json.RawMessage(nil), snapshot.Identity...)
	return snapshot, nil
}

func validateInputs(definition workflow.Definition, inputs map[string]any) error {
	if inputs == nil {
		inputs = map[string]any{}
	}
	for name, parameter := range definition.Inputs {
		value, present := inputs[name]
		if !present {
			if parameter.Required {
				return fmt.Errorf("%w: input %q is required", ErrInvalidInputs, name)
			}
			continue
		}
		if value == nil {
			return fmt.Errorf("%w: input %q must not be null", ErrInvalidInputs, name)
		}
		switch parameter.Type {
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%w: input %q must be a string", ErrInvalidInputs, name)
			}
		case "number":
			switch value.(type) {
			case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
			default:
				return fmt.Errorf("%w: input %q must be a number", ErrInvalidInputs, name)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("%w: input %q must be a boolean", ErrInvalidInputs, name)
			}
		}
	}
	for name := range inputs {
		if _, declared := definition.Inputs[name]; !declared {
			return fmt.Errorf("%w: input %q is not declared", ErrInvalidInputs, name)
		}
	}
	return nil
}

func digestJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		data = []byte("null")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func newID(prefix string) (string, error) {
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("workflow: generate id: %w", err)
	}
	return prefix + hex.EncodeToString(nonce[:]), nil
}

func (h *Host) appendEvent(ctx context.Context, runID domain.RunID, typ domain.EventType, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = h.journal.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{{RunID: runID, Type: typ, PayloadVersion: 1, CreatedAt: h.now().UnixMilli(), Payload: data}}})
	return err
}

type journalSink struct {
	h     *Host
	runID domain.RunID
}

func (sink journalSink) NodeStarted(ctx context.Context, nodeID string, kind domain.NodeKind) error {
	return sink.h.appendEvent(ctx, sink.runID, domain.EventWorkflowNodeStarted, map[string]any{"node_id": nodeID, "kind": kind})
}

func (sink journalSink) NodeCompleted(ctx context.Context, outcome domain.NodeOutcome) error {
	payload := map[string]any{"node_id": outcome.NodeID, "kind": outcome.Kind, "status": outcome.Status}
	if outcome.Error != "" {
		payload["error"] = outcome.Error
	}
	return sink.h.appendEvent(ctx, sink.runID, domain.EventWorkflowNodeCompleted, payload)
}

func (h *Host) eventSink(runID domain.RunID) domain.PlanEventSink {
	return journalSink{h: h, runID: runID}
}
