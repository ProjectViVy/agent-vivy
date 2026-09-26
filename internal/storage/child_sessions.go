package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"agent-vivy/internal/domain"
)

// ChildSessionAdmission atomically creates the continuable ChildSession and
// its first activation. The nested RunAdmission is otherwise identical to a
// normal immutable run admission.
type ChildSessionAdmission struct {
	Session   domain.Session
	Binding   domain.ChildSessionBinding
	Admission RunAdmission
}

type ChildSessionAdmissionResult struct {
	Binding domain.ChildSessionBinding
	Run     domain.Run
	Started domain.RunEvent
	Created bool
}

// ChildSessionActivation reauthorizes a stable ChildSession under a current
// active parent Run and atomically appends one new child activation.
type ChildSessionActivation struct {
	ChildSessionID  domain.SessionID
	AuthorizerRunID domain.RunID
	OperationKey    string
	RequestDigest   string
	ToolNames       []string
	Admission       RunAdmission
}

type ChildSessionStore interface {
	CommitChildSessionAdmission(context.Context, ChildSessionAdmission) (ChildSessionAdmissionResult, error)
	CommitChildSessionActivation(context.Context, ChildSessionActivation) (ChildSessionAdmissionResult, error)
	GetChildSessionBinding(context.Context, domain.SessionID) (domain.ChildSessionBinding, error)
	ListChildSessions(context.Context, domain.SessionID) ([]domain.ChildSessionBinding, error)
}

type ChildMailboxStore interface {
	EnqueueChildMessage(context.Context, domain.ChildMailboxMessage) (domain.ChildMailboxMessage, bool, error)
	ListPendingChildMessages(context.Context, domain.SessionID, domain.SessionID, int64, int) ([]domain.ChildMailboxMessage, error)
	RecordChildMessageReceipt(context.Context, domain.ChildMessageReceipt) (domain.ChildMessageReceipt, bool, error)
	GetChildMessageReceipt(context.Context, domain.SessionID, string, domain.RunID) (domain.ChildMessageReceipt, error)
	CloseChildSession(context.Context, domain.SessionID, int64) error
}

// ValidDirectChildMailboxRoute permits only the two directions across one
// immutable parent-child relationship.
func ValidDirectChildMailboxRoute(binding domain.ChildSessionBinding, sender, recipient domain.SessionID) bool {
	return (sender == binding.OriginParentSessionID && recipient == binding.ChildSessionID) ||
		(sender == binding.ChildSessionID && recipient == binding.OriginParentSessionID)
}

var (
	ErrChildAdmissionConflict = errors.New("storage: child admission conflict")
	ErrChildSessionClosed     = errors.New("storage: child session closed")
	ErrChildMessageConflict   = errors.New("storage: child message conflict")
	ErrChildMailboxFull       = errors.New("storage: child mailbox reached its message limit")
	ErrChildConcurrencyLimit  = errors.New("storage: active child run concurrency limit reached")
)

const (
	MaxChildMessageBytes                = 32 << 10
	MaxChildMailboxMessagesPerRecipient = 128
	MaxActiveChildrenPerRun             = 4
)

func EncodeChildAuthorityCeiling(ceiling domain.ChildAuthorityCeiling) ([]byte, string, error) {
	encoded, err := ceiling.CanonicalJSON()
	if err != nil {
		return nil, "", err
	}
	digest, err := ceiling.Digest()
	if err != nil {
		return nil, "", err
	}
	return encoded, digest, nil
}

func DecodeChildAuthorityCeiling(encoded []byte, expectedDigest string) (domain.ChildAuthorityCeiling, error) {
	var ceiling domain.ChildAuthorityCeiling
	if err := json.Unmarshal(encoded, &ceiling); err != nil {
		return domain.ChildAuthorityCeiling{}, errors.New("storage: invalid child authority ceiling JSON")
	}
	canonical, digest, err := EncodeChildAuthorityCeiling(ceiling)
	if err != nil || string(canonical) != string(encoded) || digest != expectedDigest {
		return domain.ChildAuthorityCeiling{}, errors.New("storage: child authority ceiling digest does not match its contents")
	}
	return ceiling, nil
}

func EncodeChildToolNames(names []string) ([]byte, error) {
	canonical, err := domain.CanonicalToolNames(names)
	if err != nil {
		return nil, err
	}
	if canonical == nil {
		canonical = []string{}
	}
	return json.Marshal(canonical)
}

func DecodeChildToolNames(encoded []byte) ([]string, error) {
	var names []string
	if err := json.Unmarshal(encoded, &names); err != nil || names == nil {
		return nil, errors.New("storage: invalid child tool list JSON")
	}
	canonical, err := domain.CanonicalToolNames(names)
	if err != nil || len(canonical) != len(names) {
		return nil, errors.New("storage: non-canonical child tool list")
	}
	for i := range canonical {
		if canonical[i] != names[i] {
			return nil, errors.New("storage: non-canonical child tool list")
		}
	}
	return names, nil
}

func ValidateChildSessionAdmission(in ChildSessionAdmission) error {
	if in.Session.ID == "" || in.Binding.ChildSessionID == "" || in.Session.ID != in.Binding.ChildSessionID ||
		in.Binding.OriginParentSessionID == "" || in.Binding.OriginParentRunID == "" ||
		in.Binding.AuthorizerRunID == "" || in.Binding.InitialActivationRunID == "" || in.Binding.ActivationRunID == "" ||
		in.Binding.OperationKey == "" || in.Binding.RequestDigest == "" || in.Binding.AuthorityCeilingDigest == "" {
		return errors.New("storage: child admission identity is incomplete")
	}
	if in.Binding.ChildSessionID == in.Binding.OriginParentSessionID || in.Binding.AuthorizerRunID != in.Binding.OriginParentRunID {
		return errors.New("storage: invalid initial child lineage")
	}
	if in.Binding.State == "" {
		in.Binding.State = domain.ChildSessionOpen
	}
	if !in.Binding.State.Valid() || in.Binding.State != domain.ChildSessionOpen {
		return errors.New("storage: new child session must be open")
	}
	_, authorityDigest, err := EncodeChildAuthorityCeiling(in.Binding.AuthorityCeiling)
	if err != nil || in.Binding.AuthorityCeilingDigest != authorityDigest {
		return errors.New("storage: child authority ceiling digest does not match its contents")
	}
	activationTools, err := domain.CanonicalToolNames(in.Binding.ActivationToolNames)
	if err != nil || !childToolsSubset(activationTools, in.Binding.AuthorityCeiling.ToolNames) {
		return errors.New("storage: initial activation widens child authority")
	}
	expectedDigest, err := domain.ChildRequestDigest(in.Binding.ActivationOperationKey, in.Admission.Message.Content, activationTools)
	if err != nil || in.Binding.ActivationRequestDigest != expectedDigest {
		return errors.New("storage: child activation request digest does not match its contents")
	}
	if in.Admission.Run.ID != in.Binding.InitialActivationRunID || in.Admission.Run.ID != in.Binding.ActivationRunID || in.Admission.Run.SessionID != in.Binding.ChildSessionID ||
		in.Admission.Run.Kind != domain.RunKindChild || in.Admission.Run.ChildMode.Effective() != domain.ChildModeContinuable ||
		in.Admission.Run.ParentID != in.Binding.AuthorizerRunID || in.Admission.Run.Status != domain.RunAccepted {
		return errors.New("storage: child activation does not match its binding")
	}
	if err := ValidateRunAdmissionInput(in.Admission); err != nil {
		return err
	}
	return nil
}

func ValidateChildSessionActivation(in ChildSessionActivation) error {
	if in.ChildSessionID == "" || in.AuthorizerRunID == "" || in.Admission.Run.ID == "" ||
		in.Admission.Run.SessionID != in.ChildSessionID || in.Admission.Run.Kind != domain.RunKindChild ||
		in.Admission.Run.ChildMode.Effective() != domain.ChildModeContinuable ||
		in.Admission.Run.ParentID != in.AuthorizerRunID || in.Admission.Run.Status != domain.RunAccepted {
		return errors.New("storage: child activation identity is incomplete")
	}
	if strings.TrimSpace(in.OperationKey) == "" || strings.TrimSpace(in.RequestDigest) == "" {
		return errors.New("storage: child activation operation identity is incomplete")
	}
	toolNames, err := domain.CanonicalToolNames(in.ToolNames)
	if err != nil || len(toolNames) != len(in.ToolNames) {
		return errors.New("storage: child activation tool list is invalid")
	}
	expectedDigest, err := domain.ChildRequestDigest(in.OperationKey, in.Admission.Message.Content, toolNames)
	if err != nil || in.RequestDigest != expectedDigest {
		return errors.New("storage: child activation request digest does not match its contents")
	}
	return ValidateRunAdmissionInput(in.Admission)
}

func childToolsSubset(requested, ceiling []string) bool {
	allowed := make(map[string]struct{}, len(ceiling))
	for _, name := range ceiling {
		allowed[name] = struct{}{}
	}
	for _, name := range requested {
		if _, ok := allowed[name]; !ok {
			return false
		}
	}
	return true
}
