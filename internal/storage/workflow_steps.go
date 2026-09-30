package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

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
	Type           domain.EventType
	CreatedAt      int64
	PayloadVersion int
	Payload        []byte
}

// WorkflowStepResult is the transaction-local reference to a protected node
// output. Output bytes live in a content-addressed workflow-scoped blob that
// the runtime adapter stages before Commit.
type WorkflowStepResult struct {
	Path    string `json:"path"`
	Attempt int    `json:"attempt"`
	Digest  string `json:"digest"`
	BlobID  string `json:"blob_id"`
	Bytes   int64  `json:"bytes"`
}

// WorkflowStepCheckpoint references the latest committed checkpoint: the
// serialized envelope (payload elided) plus the staged payload blob.
type WorkflowStepCheckpoint struct {
	Envelope []byte `json:"envelope"`
	BlobID   string `json:"blob_id"`
	Digest   string `json:"digest"`
}

// WorkflowStepCommit is one atomic INOFY commit as host storage state.
// Expected "" means the projection row does not exist yet (the initial
// admit transition); every later commit must name the current state.
type WorkflowStepCommit struct {
	RunID         domain.RunID
	CommitID      string
	Digest        string
	Epoch         uint64
	ProgramDigest string
	HostBindingID string
	Expected      WorkflowStepStatus
	Target        WorkflowStepStatus
	Events        []WorkflowStepEvent
	Results       []WorkflowStepResult
	Checkpoint    *WorkflowStepCheckpoint
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
	Status              WorkflowStepStatus `json:"status"`
	Epoch               uint64             `json:"epoch"`
	Revision            uint64             `json:"revision"`
	UsageJSON           []byte             `json:"usage,omitempty"`
	CheckpointJSON      []byte             `json:"checkpoint,omitempty"`
	CheckpointBlobID    string             `json:"checkpoint_blob_id,omitempty"`
	CheckpointDigest    string             `json:"checkpoint_digest,omitempty"`
	WaitsJSON           []byte             `json:"waits,omitempty"`
	InterruptsJSON      []byte             `json:"interrupts,omitempty"`
	GatesJSON           []byte             `json:"gates,omitempty"`
	ResumeKey           string             `json:"resume_key,omitempty"`
	ResumeAnswersDigest string             `json:"resume_answers_digest,omitempty"`
	UnresolvedJSON      []byte             `json:"unresolved,omitempty"`
	UpdatedAt           int64              `json:"updated_at"`
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

var workflowStepDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ValidateWorkflowStepCommit performs cheap caller-side validation; the
// drivers re-verify everything inside the commit transaction.
func ValidateWorkflowStepCommit(c WorkflowStepCommit) error {
	if c.RunID == "" || c.CommitID == "" {
		return fmt.Errorf("storage: workflow step commit requires run id and commit id")
	}
	if !workflowStepDigestPattern.MatchString(c.Digest) {
		return fmt.Errorf("storage: workflow step commit digest must be 64 hex chars")
	}
	if !validINOFYDigest(c.ProgramDigest) {
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
		if r.Path == "" || r.BlobID == "" || !workflowStepDigestPattern.MatchString(r.Digest) {
			return fmt.Errorf("storage: workflow step result requires path, blob id and digest")
		}
	}
	if c.Checkpoint != nil {
		if len(c.Checkpoint.Envelope) == 0 || c.Checkpoint.BlobID == "" ||
			!workflowStepDigestPattern.MatchString(c.Checkpoint.Digest) {
			return fmt.Errorf("storage: workflow step checkpoint requires envelope, blob id and digest")
		}
	}
	return nil
}

// workflowStepAdmissionMeta is the identity data parsed from the
// workflow.admitted event payload.
type workflowStepAdmissionMeta struct {
	InputDigest string          `json:"input_digest"`
	Limits      json.RawMessage `json:"limits"`
}

// workflowStepWaitingMeta is parsed from the workflow.waiting event payload.
type workflowStepWaitingMeta struct {
	Waits      json.RawMessage            `json:"waits"`
	Interrupts map[string]json.RawMessage `json:"interrupts"`
	Gates      json.RawMessage            `json:"gates"`
}

// workflowStepResumedMeta is parsed from the workflow.resumed event payload.
type workflowStepResumedMeta struct {
	IdempotencyKey string `json:"idempotency_key"`
	AnswersDigest  string `json:"answers_digest"`
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
// interrupts, resume identity, unresolved operations and usage stay derived
// from the same event evidence everywhere.
func WorkflowStepDerive(proj *WorkflowStepProjection, c WorkflowStepCommit, revision domain.WorkflowRevision) (*WorkflowStepProjection, error) {
	next := WorkflowStepProjection{
		Status:              c.Target,
		Epoch:               c.Epoch,
		Revision:            proj.Revision + 1,
		UsageJSON:           proj.UsageJSON,
		CheckpointJSON:      proj.CheckpointJSON,
		CheckpointBlobID:    proj.CheckpointBlobID,
		CheckpointDigest:    proj.CheckpointDigest,
		WaitsJSON:           proj.WaitsJSON,
		InterruptsJSON:      proj.InterruptsJSON,
		GatesJSON:           proj.GatesJSON,
		ResumeKey:           proj.ResumeKey,
		ResumeAnswersDigest: proj.ResumeAnswersDigest,
		UnresolvedJSON:      proj.UnresolvedJSON,
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
			if meta.InputDigest != revision.InputDigest {
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
			var meta workflowStepResumedMeta
			if err := json.Unmarshal(e.Payload, &meta); err != nil {
				return nil, fmt.Errorf("storage: decode workflow.resumed payload: %w", err)
			}
			next.ResumeKey = meta.IdempotencyKey
			next.ResumeAnswersDigest = meta.AnswersDigest
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
		next.CheckpointBlobID = c.Checkpoint.BlobID
		next.CheckpointDigest = c.Checkpoint.Digest
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
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ra, err1 := json.Marshal(x)
	rb, err2 := json.Marshal(y)
	return err1 == nil && err2 == nil && string(ra) == string(rb)
}
