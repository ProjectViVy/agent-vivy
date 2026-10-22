package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

// WorkflowStepStatus is the host-side projection state of one admitted INOFY
// execution. The values mirror the engine lifecycle but stay host-owned.
type WorkflowStepStatus string

const (
	WorkflowStepAdmitted         WorkflowStepStatus = "admitted"
	WorkflowStepRunning          WorkflowStepStatus = "running"
	WorkflowStepWaiting          WorkflowStepStatus = "waiting"
	WorkflowStepSucceeded        WorkflowStepStatus = "succeeded"
	WorkflowStepFailed           WorkflowStepStatus = "failed"
	WorkflowStepCancelled        WorkflowStepStatus = "cancelled"
	WorkflowStepRecoveryRequired WorkflowStepStatus = "recovery_required"
)

// Terminal reports whether the step status is a terminal lifecycle state.
func (s WorkflowStepStatus) Terminal() bool {
	switch s {
	case WorkflowStepSucceeded, WorkflowStepFailed, WorkflowStepCancelled:
		return true
	}
	return false
}

// NativeRunStatus maps a terminal step status to the native Run status.
func (s WorkflowStepStatus) NativeRunStatus() (domain.RunStatus, bool) {
	switch s {
	case WorkflowStepSucceeded:
		return domain.RunCompleted, true
	case WorkflowStepFailed:
		return domain.RunFailed, true
	case WorkflowStepCancelled:
		return domain.RunCancelled, true
	}
	return "", false
}

// WorkflowStepEvent is one redacted Journal event committed by a step. The
// Journal assigns the contiguous sequence inside the commit transaction.
type WorkflowStepEvent struct {
	Payload        []byte
	Type           domain.EventType
	CreatedAt      int64
	PayloadVersion int
}

// WorkflowStepResult is the transaction-local reference to a protected node
// output. The protected bytes commit atomically with their path and attempt.
type WorkflowStepResult struct {
	Output  []byte `json:"output,omitempty"`
	Path    string `json:"path"`
	Attempt int    `json:"attempt"`
	Bytes   int64  `json:"bytes"`
}

// WorkflowStepCheckpoint references the latest committed checkpoint: the
// complete native serialized envelope, including its private payload.
type WorkflowStepCheckpoint struct {
	Envelope []byte `json:"envelope"`
}

// WorkflowStepCommit is one atomic INOFY commit as host storage state.
// Expected "" means the projection row does not exist yet (the initial
// admit transition); every later commit must name the current state.
type WorkflowStepCommit struct {
	RunID             domain.RunID
	CommitID          string
	Epoch             uint64
	ProgramID         string
	HostBindingID     string
	Expected          WorkflowStepStatus
	Target            WorkflowStepStatus
	Events            []WorkflowStepEvent
	Results           []WorkflowStepResult
	AdmittedInput     json.RawMessage `json:"-"`
	ResumeKey         string          `json:"-"`
	ResumeAnswersJSON json.RawMessage `json:"-"`
	Checkpoint        *WorkflowStepCheckpoint
}

// WorkflowStepReceipt is the durable acknowledgement of a committed step.
type WorkflowStepReceipt struct {
	FirstSequence domain.EventSeq
	LastSequence  domain.EventSeq
	Revision      uint64
	// Replayed reports an idempotent re-commit of an already-stored
	// commit-id: the returned sequences are the original rows', nothing was
	// written again.
	Replayed bool
}

// WorkflowStepProjection is the mutable execution projection row.
type WorkflowStepProjection struct {
	Status            WorkflowStepStatus `json:"status"`
	Epoch             uint64             `json:"epoch"`
	Revision          uint64             `json:"revision"`
	UsageJSON         []byte             `json:"usage,omitempty"`
	CheckpointJSON    []byte             `json:"checkpoint,omitempty"`
	WaitsJSON         []byte             `json:"waits,omitempty"`
	InterruptsJSON    []byte             `json:"interrupts,omitempty"`
	GatesJSON         []byte             `json:"gates,omitempty"`
	ResumeAnswersJSON []byte             `json:"-"`
	ResumeKey         string             `json:"resume_key,omitempty"`
	UnresolvedJSON    []byte             `json:"unresolved,omitempty"`
	UpdatedAt         int64              `json:"updated_at"`
}

// WorkflowStepState is the full recovery view returned by Load.
type WorkflowStepState struct {
	Revision   domain.WorkflowRevision
	Run        domain.Run
	Projection *WorkflowStepProjection
	Results    []WorkflowStepResult
}

// WorkflowStepStore is the narrow step-commit surface the INOFY RunStore
// adapter maps to. Every commit is one atomic SQL transaction.
type WorkflowStepStore interface {
	CommitWorkflowStep(ctx context.Context, commit WorkflowStepCommit) (WorkflowStepReceipt, error)
	LoadWorkflowStep(ctx context.Context, runID domain.RunID) (WorkflowStepState, error)
}

// ValidateWorkflowStepCommit performs cheap caller-side validation; the
// drivers re-verify everything inside the commit transaction.
func ValidateWorkflowStepCommit(c WorkflowStepCommit) error {
	if c.RunID == "" || c.CommitID == "" {
		return fmt.Errorf("storage: workflow step commit requires run id and commit id")
	}

	if c.ProgramID == "" || c.HostBindingID == "" {
		return ErrWorkflowStepIdentity
	}
	if c.Epoch == 0 {
		return fmt.Errorf("storage: workflow step epoch must be >= 1")
	}
	switch c.Target {
	case WorkflowStepAdmitted, WorkflowStepRunning, WorkflowStepWaiting,
		WorkflowStepSucceeded, WorkflowStepFailed, WorkflowStepCancelled,
		WorkflowStepRecoveryRequired:
	default:
		return fmt.Errorf("storage: unknown workflow step target %q", c.Target)
	}
	if c.Expected == "" && c.Target != WorkflowStepAdmitted {
		return fmt.Errorf("storage: initial workflow step must target admitted")
	}
	terminals := 0
	for _, e := range c.Events {
		if e.Type == "" {
			return fmt.Errorf("storage: workflow step event requires a type")
		}
		if e.Type.Terminal() {
			terminals++
		}
	}
	if terminals > 1 {
		return ErrCommitInvalid
	}
	if c.Target.Terminal() && terminals != 1 {
		return fmt.Errorf("storage: terminal workflow step requires exactly one terminal event")
	}
	if !c.Target.Terminal() && terminals != 0 {
		return fmt.Errorf("storage: non-terminal workflow step carries a terminal event")
	}
	for _, r := range c.Results {
		if r.Path == "" || r.Attempt < 1 || len(r.Output) == 0 || r.Bytes != int64(len(r.Output)) {
			return fmt.Errorf("storage: workflow step result requires path and output")
		}
	}
	if c.Checkpoint != nil {
		if len(c.Checkpoint.Envelope) == 0 {
			return fmt.Errorf("storage: workflow step checkpoint requires envelope")
		}
	}
	return nil
}

// workflowStepAdmissionMeta is the identity data parsed from the
// workflow.admitted event payload.
type workflowStepAdmissionMeta struct {
	Limits json.RawMessage `json:"limits"`
}

// workflowStepWaitingMeta is parsed from the workflow.waiting event payload.
type workflowStepWaitingMeta struct {
	Waits      json.RawMessage            `json:"waits"`
	Interrupts map[string]json.RawMessage `json:"interrupts"`
	Gates      json.RawMessage            `json:"gates"`
}

// workflowStepNodeMeta is parsed from every workflow.node.* event payload.
type workflowStepNodeMeta struct {
	NodeKey string `json:"node_key"`
	Attempt int    `json:"attempt"`
}

// workflowStepUsage is the cumulative usage projection.
type workflowStepUsage struct {
	Attempts             int   `json:"attempts"`
	CompletedOutputBytes int64 `json:"completed_output_bytes"`
	Activations          int   `json:"activations"`
	ActiveMS             int64 `json:"active_ms"`
}

// workflowStepUnresolved is one started-but-unresolved effect.
type workflowStepUnresolved struct {
	OperationKey string `json:"operation_key"`
	Path         string `json:"path"`
	Attempt      int    `json:"attempt"`
}

// WorkflowStepDerive applies one commit's events to the current projection
// in memory. The drivers call it inside the commit transaction so waits,
// interrupts, unresolved operations and usage stay derived from committed
// events. Input/resume claims come only from the private commit fields.
func WorkflowStepDerive(proj *WorkflowStepProjection, c WorkflowStepCommit, revision domain.WorkflowRevision) (*WorkflowStepProjection, error) {
	if (c.Expected == "" || len(c.AdmittedInput) > 0) && !jsonBytesEqual(c.AdmittedInput, revision.InputJSON) {
		return nil, ErrWorkflowStepIdentity
	}
	hasResume := false
	for _, event := range c.Events {
		if event.Type == domain.EventWorkflowResumed {
			hasResume = true
		}
	}
	if (c.ResumeKey != "" || len(c.ResumeAnswersJSON) > 0) && !hasResume {
		return nil, ErrWorkflowStepIdentity
	}

	next := WorkflowStepProjection{
		Status:            c.Target,
		Epoch:             c.Epoch,
		Revision:          proj.Revision + 1,
		UsageJSON:         proj.UsageJSON,
		CheckpointJSON:    proj.CheckpointJSON,
		WaitsJSON:         proj.WaitsJSON,
		InterruptsJSON:    proj.InterruptsJSON,
		GatesJSON:         proj.GatesJSON,
		ResumeKey:         proj.ResumeKey,
		ResumeAnswersJSON: append([]byte(nil), proj.ResumeAnswersJSON...),
		UnresolvedJSON:    proj.UnresolvedJSON,
	}
	usage := workflowStepUsage{}
	if len(proj.UsageJSON) > 0 {
		if err := json.Unmarshal(proj.UsageJSON, &usage); err != nil {
			return nil, fmt.Errorf("storage: decode workflow usage: %w", err)
		}
	}
	unresolved := map[string]workflowStepUnresolved{}
	if len(proj.UnresolvedJSON) > 0 {
		var prior []workflowStepUnresolved
		if err := json.Unmarshal(proj.UnresolvedJSON, &prior); err != nil {
			return nil, fmt.Errorf("storage: decode workflow unresolved: %w", err)
		}
		for _, op := range prior {
			unresolved[unresolvedKey(op.Path, op.Attempt)] = op
		}
	}
	for _, e := range c.Events {
		switch e.Type {
		case domain.EventWorkflowAdmitted:
			var meta workflowStepAdmissionMeta
			if err := json.Unmarshal(e.Payload, &meta); err != nil {
				return nil, fmt.Errorf("storage: decode workflow.admitted payload: %w", err)
			}
			if !jsonBytesEqual(c.AdmittedInput, revision.InputJSON) {
				return nil, ErrWorkflowStepIdentity
			}
			if len(meta.Limits) > 0 && len(revision.EffectiveLimits) > 0 &&
				!jsonBytesEqual(meta.Limits, revision.EffectiveLimits) {
				return nil, ErrWorkflowStepIdentity
			}
		case domain.EventWorkflowWaiting:
			var meta workflowStepWaitingMeta
			if err := json.Unmarshal(e.Payload, &meta); err != nil {
				return nil, fmt.Errorf("storage: decode workflow.waiting payload: %w", err)
			}
			next.WaitsJSON = append([]byte(nil), meta.Waits...)
			if meta.Interrupts != nil {
				raw, err := json.Marshal(meta.Interrupts)
				if err != nil {
					return nil, fmt.Errorf("storage: encode workflow interrupts: %w", err)
				}
				next.InterruptsJSON = raw
			}
			next.GatesJSON = append([]byte(nil), meta.Gates...)
		case domain.EventWorkflowResumed:
			if c.ResumeKey == "" || len(c.ResumeAnswersJSON) == 0 || !json.Valid(c.ResumeAnswersJSON) {
				return nil, ErrWorkflowStepIdentity
			}
			if proj.ResumeKey == c.ResumeKey && !jsonBytesEqual(proj.ResumeAnswersJSON, c.ResumeAnswersJSON) {
				return nil, ErrWorkflowStepIdempotency
			}
			next.ResumeKey = c.ResumeKey
			next.ResumeAnswersJSON = append([]byte(nil), c.ResumeAnswersJSON...)
			next.WaitsJSON = nil
			next.InterruptsJSON = nil
			next.GatesJSON = nil
		case domain.EventWorkflowNodeAttempt:
			var meta workflowStepNodeMeta
			if err := json.Unmarshal(e.Payload, &meta); err != nil {
				return nil, fmt.Errorf("storage: decode workflow.node.attempt payload: %w", err)
			}
			key := unresolvedKey(meta.NodeKey, meta.Attempt)
			unresolved[key] = workflowStepUnresolved{
				OperationKey: fmt.Sprintf("%s/%s", c.RunID, meta.NodeKey),
				Path:         meta.NodeKey,
				Attempt:      meta.Attempt,
			}
			usage.Attempts++
		case domain.EventWorkflowNodeCompleted, domain.EventWorkflowNodeFailed,
			domain.EventWorkflowNodeDegraded, domain.EventWorkflowNodeWaiting:
			var meta workflowStepNodeMeta
			if err := json.Unmarshal(e.Payload, &meta); err != nil {
				return nil, fmt.Errorf("storage: decode workflow node payload: %w", err)
			}
			delete(unresolved, unresolvedKey(meta.NodeKey, meta.Attempt))
		}
	}
	for _, r := range c.Results {
		key := unresolvedKey(r.Path, r.Attempt)
		delete(unresolved, key)
		usage.CompletedOutputBytes += r.Bytes
	}
	if len(unresolved) > 0 {
		ops := make([]workflowStepUnresolved, 0, len(unresolved))
		for _, op := range unresolved {
			ops = append(ops, op)
		}
		sortWorkflowStepUnresolved(ops)
		raw, err := json.Marshal(ops)
		if err != nil {
			return nil, fmt.Errorf("storage: encode workflow unresolved: %w", err)
		}
		next.UnresolvedJSON = raw
	} else {
		next.UnresolvedJSON = nil
	}
	raw, err := json.Marshal(usage)
	if err != nil {
		return nil, fmt.Errorf("storage: encode workflow usage: %w", err)
	}
	next.UsageJSON = raw
	if c.Checkpoint != nil {
		next.CheckpointJSON = append([]byte(nil), c.Checkpoint.Envelope...)

	}
	return &next, nil
}

func unresolvedKey(path string, attempt int) string {
	return fmt.Sprintf("%s/%d", path, attempt)
}

func sortWorkflowStepUnresolved(ops []workflowStepUnresolved) {
	for i := 1; i < len(ops); i++ {
		for j := i; j > 0 && ops[j].OperationKey < ops[j-1].OperationKey; j-- {
			ops[j], ops[j-1] = ops[j-1], ops[j]
		}
	}
}

func jsonBytesEqual(a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	var x, y any
	if decodeWorkflowJSON(a, &x) != nil || decodeWorkflowJSON(b, &y) != nil {
		return false
	}
	ra, err1 := json.Marshal(x)
	rb, err2 := json.Marshal(y)
	return err1 == nil && err2 == nil && string(ra) == string(rb)
}

// WorkflowStepInput is the private canonical receipt body. It is derived at the
// existing SQL owner, including inputs omitted by INOFY's public JSON tags.
// Epoch and wall-clock event times are writer/receipt metadata, not intent.
func WorkflowStepInput(c WorkflowStepCommit) ([]byte, error) {
	events := append([]WorkflowStepEvent(nil), c.Events...)
	for i := range events {
		events[i].CreatedAt = 0
	}
	raw, err := json.Marshal(struct {
		ProgramID     string
		HostBindingID string
		Expected      WorkflowStepStatus
		Target        WorkflowStepStatus
		Events        []WorkflowStepEvent
		Results       []WorkflowStepResult
		Checkpoint    *WorkflowStepCheckpoint
		AdmittedInput json.RawMessage
		ResumeKey     string
		ResumeAnswers json.RawMessage
	}{c.ProgramID, c.HostBindingID, c.Expected, c.Target, events, c.Results, c.Checkpoint, c.AdmittedInput, c.ResumeKey, c.ResumeAnswersJSON})
	if err != nil {
		return nil, err
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func decodeWorkflowJSON(raw []byte, value *any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(value)
}
