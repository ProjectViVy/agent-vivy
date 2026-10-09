package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

var ErrWorkflowRevisionConflict = errors.New("storage: workflow operation key conflicts with an existing revision")

// ErrWorkflowAdmissionBusy marks a distinct operation key arriving while a
// non-terminal Run already owns the same admission target.
var ErrWorkflowAdmissionBusy = errors.New("storage: workflow admission target is busy")

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
	// Busy reports that the admission was declined because a non-terminal Run
	// already owns the target; Revision/Run then carry that existing pair.
	Busy bool
}

// WorkflowRevisionStore commits a validated immutable descriptor and its
// graph Run/start event atomically under the parent Session admission lock.
type WorkflowRevisionStore interface {
	CommitWorkflowAdmission(context.Context, WorkflowAdmission) (WorkflowAdmissionResult, error)
	GetWorkflowRevision(context.Context, domain.RunID) (domain.WorkflowRevision, error)
	GetWorkflowRevisionByOperation(context.Context, domain.RunID, string) (domain.WorkflowRevision, error)
	// GetWorkflowRevisionByOperationInNamespace reads one revision inside its
	// admission namespace: the parent Run id for child workflows, the hidden
	// control Session id for trusted report roots.
	GetWorkflowRevisionByOperationInNamespace(context.Context, string, string) (domain.WorkflowRevision, error)
	ListWorkflowRevisions(context.Context, domain.RunID) ([]domain.WorkflowRevision, error)
}

func ValidateWorkflowAdmission(in WorkflowAdmission) error {
	r := in.Revision
	if r.RunID == "" || r.ParentSessionID == "" || r.RootRunID == "" ||
		r.OperationKey == "" || len(r.OperationKey) > 128 || r.DescriptorDigest == "" ||
		r.AuthorityDigest == "" || len(r.DescriptorJSON) == 0 || len(r.AuthorityJSON) == 0 || r.SchemaVersion <= 0 || r.CreatedAt <= 0 {
		return errors.New("storage: workflow revision identity is incomplete")
	}
	if r.RootPurpose == "" {
		// Child workflow: parent lineage columns stay authoritative and the
		// admission namespace is the parent Run.
		if r.ParentRunID == "" {
			return errors.New("storage: workflow revision requires a parent run")
		}
		if r.AdmissionNamespace != "" && r.AdmissionNamespace != string(r.ParentRunID) {
			return errors.New("storage: child workflow admission namespace must be its parent run")
		}
	} else {
		// Trusted root: no parent Run; the namespace is the control Session,
		// the request digest binds the caller's semantic request, and the
		// target key scopes non-terminal exclusivity.
		if r.ParentRunID != "" {
			return errors.New("storage: trusted root revision must not carry a parent run")
		}
		if r.AdmissionNamespace == "" {
			return errors.New("storage: trusted root revision requires an admission namespace")
		}
		if !validSHA256Hex(r.RequestDigest) {
			return errors.New("storage: trusted root request digest must be lowercase SHA-256 hex")
		}
		if r.TargetKey == "" {
			return errors.New("storage: trusted root revision requires a target key")
		}
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
	// Storage discriminator: 1 = legacy descriptor path, 2 = INOFY. A
	// discriminator-2 admission must carry the complete INOFY identity so a
	// later recovery can rebind to the same immutable program; discriminator-1
	// rows must not carry it.
	if r.SchemaVersion == 2 {
		if !validINOFYDigest(r.ProgramDigest) || !validINOFYDigest(r.CatalogDigest) ||
			!validINOFYDigest(r.InputDigest) || r.CompilerVersion == "" ||
			len(r.EffectiveLimits) == 0 || !json.Valid(r.EffectiveLimits) || r.HostBindingID == "" ||
			len(r.InputJSON) == 0 || !json.Valid(r.InputJSON) {
			return errors.New("storage: inofy workflow revision identity is incomplete")
		}
		if len(r.DefinitionID) > 256 {
			return errors.New("storage: workflow definition id is oversized")
		}
		if r.DefinitionID == "" && r.DefinitionRevision != 0 {
			return errors.New("storage: workflow definition revision requires a definition id")
		}
	} else if r.ProgramDigest != "" || r.CatalogDigest != "" || r.CompilerVersion != "" ||
		r.EinoBuild != "" || r.InputDigest != "" || len(r.InputJSON) != 0 ||
		len(r.EffectiveLimits) != 0 || r.HostBindingID != "" ||
		r.DefinitionID != "" || r.DefinitionRevision != 0 {
		return errors.New("storage: legacy workflow revision must not carry inofy identity")
	}
	if in.Run.ID != r.RunID || in.Run.SessionID != r.ParentSessionID || in.Run.Kind != domain.RunKindWorkflow ||
		in.Run.ParentID != r.ParentRunID || in.Run.RootID != r.RootRunID || in.Run.Status != domain.RunAccepted ||
		in.Run.CreatedAt != r.CreatedAt {
		return errors.New("storage: workflow Run does not match its revision lineage")
	}
	if r.RootPurpose != "" && (in.Run.Purpose == "" || r.RootRunID != r.RunID) {
		return errors.New("storage: trusted root workflow must carry its run purpose and self root")
	}
	if r.RootPurpose == "" && in.Run.Purpose != "" {
		return errors.New("storage: child workflow run must not carry a root purpose")
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

// validINOFYDigest accepts the named digest envelope INOFY emits
// ("inofy-normal-v1:sha256:<64 hex>") as well as bare SHA-256 hex.
func validINOFYDigest(value string) bool {
	if validSHA256Hex(value) {
		return true
	}
	const prefix = "inofy-normal-v1:sha256:"
	return strings.HasPrefix(value, prefix) && validSHA256Hex(strings.TrimPrefix(value, prefix))
}

func WorkflowAdmissionConflict() error { return fmt.Errorf("%w", ErrWorkflowRevisionConflict) }

func WorkflowAdmissionBusy() error { return fmt.Errorf("%w", ErrWorkflowAdmissionBusy) }
