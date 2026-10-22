package storage

import (
	"context"
	"encoding/json"
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
		r.OperationKey == "" || len(r.OperationKey) > 128 || len(r.DescriptorJSON) == 0 || len(r.AuthorityJSON) == 0 || r.SchemaVersion <= 0 || r.CreatedAt <= 0 {
		return errors.New("storage: workflow revision identity is incomplete")
	}
	// Schema 3 owns exact inputs and opaque ProgramID. Earlier rows are
	// historical records and never accepted by the execution step owner.
	if r.SchemaVersion == 3 {
		if r.ProgramID == "" || len(r.ProgramID) > 1024 || r.CompilerVersion == "" ||
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
	} else if r.ProgramID != "" || r.CompilerVersion != "" ||
		r.EinoBuild != "" || len(r.InputJSON) != 0 ||
		len(r.EffectiveLimits) != 0 || r.HostBindingID != "" ||
		r.DefinitionID != "" || r.DefinitionRevision != 0 {
		return errors.New("storage: legacy workflow revision must not carry inofy identity")
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

func WorkflowAdmissionConflict() error { return fmt.Errorf("%w", ErrWorkflowRevisionConflict) }

// EqualWorkflowAdmissionInput compares the actual immutable owner inputs. Run
// allocation and timestamps are outputs; the operation key owns the receipt.
func EqualWorkflowAdmissionInput(a, b domain.WorkflowRevision) bool {
	return a.SchemaVersion == b.SchemaVersion &&
		a.ParentRunID == b.ParentRunID && a.ParentSessionID == b.ParentSessionID && a.RootRunID == b.RootRunID &&
		a.OperationKey == b.OperationKey && a.DefinitionID == b.DefinitionID && a.DefinitionRevision == b.DefinitionRevision &&
		a.CompilerVersion == b.CompilerVersion && a.EinoBuild == b.EinoBuild &&
		jsonBytesEqual(a.DescriptorJSON, b.DescriptorJSON) && jsonBytesEqual(a.AuthorityJSON, b.AuthorityJSON) &&
		jsonBytesEqual(a.InputJSON, b.InputJSON) && jsonBytesEqual(a.EffectiveLimits, b.EffectiveLimits)
}
