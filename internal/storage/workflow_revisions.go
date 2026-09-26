package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
)

var ErrWorkflowRevisionConflict = errors.New("storage: workflow operation key conflicts with an existing revision")

type WorkflowAdmission struct {
	Revision domain.WorkflowRevision
	Run      domain.Run
	Started  domain.RunEvent
}

type WorkflowAdmissionResult struct {
	Revision domain.WorkflowRevision
	Run      domain.Run
	Started  domain.RunEvent
	Created  bool
}

// WorkflowRevisionStore commits a validated immutable descriptor and its
// graph Run/start event atomically under the parent Session admission lock.
type WorkflowRevisionStore interface {
	CommitWorkflowAdmission(context.Context, WorkflowAdmission) (WorkflowAdmissionResult, error)
	GetWorkflowRevision(context.Context, domain.RunID) (domain.WorkflowRevision, error)
	GetWorkflowRevisionByOperation(context.Context, domain.RunID, string) (domain.WorkflowRevision, error)
	ListWorkflowRevisions(context.Context, domain.RunID) ([]domain.WorkflowRevision, error)
}

func ValidateWorkflowAdmission(in WorkflowAdmission) error {
	r := in.Revision
	if r.RunID == "" || r.ParentRunID == "" || r.ParentSessionID == "" || r.RootRunID == "" ||
		r.OperationKey == "" || len(r.OperationKey) > 128 || r.DescriptorDigest == "" ||
		r.AuthorityDigest == "" || len(r.DescriptorJSON) == 0 || len(r.AuthorityJSON) == 0 || r.SchemaVersion <= 0 || r.CreatedAt <= 0 {
		return errors.New("storage: workflow revision identity is incomplete")
	}
	if !validSHA256Hex(r.DescriptorDigest) || !validSHA256Hex(r.AuthorityDigest) {
		return errors.New("storage: workflow revision digests must be lowercase SHA-256 hex")
	}
	digest := sha256.Sum256(r.DescriptorJSON)
	if hex.EncodeToString(digest[:]) != r.DescriptorDigest {
		return errors.New("storage: workflow descriptor digest does not match its bytes")
	}
	authorityDigest := sha256.Sum256(r.AuthorityJSON)
	if hex.EncodeToString(authorityDigest[:]) != r.AuthorityDigest {
		return errors.New("storage: workflow authority digest does not match its bytes")
	}
	if in.Run.ID != r.RunID || in.Run.SessionID != r.ParentSessionID || in.Run.Kind != domain.RunKindWorkflow ||
		in.Run.ParentID != r.ParentRunID || in.Run.RootID != r.RootRunID || in.Run.Status != domain.RunAccepted ||
		in.Run.CreatedAt != r.CreatedAt {
		return errors.New("storage: workflow Run does not match its revision lineage")
	}
	if in.Started.RunID != r.RunID || in.Started.Type != domain.EventRunStarted || in.Started.CreatedAt != r.CreatedAt ||
		in.Started.PayloadVersion <= 0 || len(in.Started.Payload) == 0 {
		return errors.New("storage: workflow admission requires its run.started event")
	}
	return nil
}

func validSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func WorkflowAdmissionConflict() error { return fmt.Errorf("%w", ErrWorkflowRevisionConflict) }
