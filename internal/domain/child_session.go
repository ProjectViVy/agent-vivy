package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// ChildMode distinguishes short-lived child activations from children that
// own a durable, addressable Session. Empty values on legacy child Runs read
// as one-shot.
type ChildMode string

const (
	ChildModeOneShot     ChildMode = "one-shot"
	ChildModeContinuable ChildMode = "continuable"
)

func (m ChildMode) Effective() ChildMode {
	if m == "" {
		return ChildModeOneShot
	}
	return m
}

func (m ChildMode) Valid() bool {
	switch m.Effective() {
	case ChildModeOneShot, ChildModeContinuable:
		return true
	default:
		return false
	}
}

type ChildSessionState string

const (
	ChildSessionOpen   ChildSessionState = "open"
	ChildSessionClosed ChildSessionState = "closed"
)

func (s ChildSessionState) Valid() bool {
	return s == ChildSessionOpen || s == ChildSessionClosed
}

// ChildSessionBinding is the host-owned durable identity and authority
// lineage for one continuable child. Origin fields and AuthorityCeilingDigest
// are immutable; AuthorizerRunID and ActivationRunID advance on continuation.
type ChildSessionBinding struct {
	ChildSessionID                SessionID
	OriginParentSessionID         SessionID
	OriginParentRunID             RunID
	AuthorizerRunID               RunID
	InitialActivationRunID        RunID
	ActivationRunID               RunID
	OperationKey                  string
	RequestDigest                 string
	AuthorityCeilingDigest        string
	AuthorityCeiling              ChildAuthorityCeiling
	ActivationOperationKey        string
	ActivationRequestDigest       string
	ActivationToolNames           []string
	State                         ChildSessionState
	NextMessageSequence           int64 // parent-to-child inbox sequence
	ConsumedMessageSequence       int64 // parent-to-child inbox cursor
	NextParentMessageSequence     int64 // child-to-parent inbox sequence
	ConsumedParentMessageSequence int64 // child-to-parent inbox cursor
	CreatedAt                     int64
	UpdatedAt                     int64
}

// ChildAuthorityCeiling is the immutable maximum policy and tool set granted
// to a continuable child. Each activation may select a strict subset.
type ChildAuthorityCeiling struct {
	PolicyProfile  PolicyProfile  `json:"policy_profile"`
	PolicyHash     string         `json:"policy_hash"`
	SandboxMode    SandboxMode    `json:"sandbox_mode"`
	ApprovalPolicy ApprovalPolicy `json:"approval_policy"`
	ToolNames      []string       `json:"tool_names"`
}

// CanonicalToolNames returns a sorted, duplicate-free tool list and rejects
// empty names. Sorting gives digests and persisted JSON one stable encoding.
func CanonicalToolNames(names []string) ([]string, error) {
	canonical := make([]string, len(names))
	copy(canonical, names)
	for i := range canonical {
		canonical[i] = strings.TrimSpace(canonical[i])
		if canonical[i] == "" {
			return nil, errors.New("domain: child tool name is empty")
		}
	}
	sort.Strings(canonical)
	for i := 1; i < len(canonical); i++ {
		if canonical[i] == canonical[i-1] {
			return nil, errors.New("domain: duplicate child tool name")
		}
	}
	return canonical, nil
}

func (c ChildAuthorityCeiling) CanonicalJSON() ([]byte, error) {
	if !c.PolicyProfile.Valid() || strings.TrimSpace(c.PolicyHash) == "" || !c.SandboxMode.Valid() || !c.ApprovalPolicy.Valid() {
		return nil, errors.New("domain: child authority ceiling is incomplete")
	}
	toolNames, err := CanonicalToolNames(c.ToolNames)
	if err != nil {
		return nil, err
	}
	c.ToolNames = toolNames
	return json.Marshal(c)
}

func (c ChildAuthorityCeiling) Digest() (string, error) {
	encoded, err := c.CanonicalJSON()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// ChildRequestDigest identifies the task and selected tools for one
// idempotent child admission or activation.
func ChildRequestDigest(operationKey, task string, toolNames []string) (string, error) {
	if operationKey == "" || strings.TrimSpace(operationKey) != operationKey || strings.TrimSpace(task) == "" {
		return "", errors.New("domain: child operation key and task are required")
	}
	canonical, err := CanonicalToolNames(toolNames)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(struct {
		OperationKey string   `json:"operation_key"`
		Task         string   `json:"task"`
		ToolNames    []string `json:"tool_names"`
	}{operationKey, task, canonical})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// ChildMailboxMessage is one durable direct parent-child message. Sequence is
// monotonic within the recipient's inbox and belongs to the stable ChildSession,
// never an activation Run.
type ChildMailboxMessage struct {
	ID                 string
	ChildSessionID     SessionID
	SenderSessionID    SessionID
	RecipientSessionID SessionID
	IdempotencyKey     string
	Sequence           int64
	Body               []byte
	Status             ChildMessageStatus
	CreatedAt          int64
	ConsumedAt         int64
	ConsumedByRunID    RunID
}

type ChildMessageStatus string

const (
	ChildMessagePending  ChildMessageStatus = "pending"
	ChildMessageConsumed ChildMessageStatus = "consumed"
	ChildMessageRejected ChildMessageStatus = "rejected"
	ChildMessageExpired  ChildMessageStatus = "expired"
)

func (s ChildMessageStatus) Valid() bool {
	switch s {
	case ChildMessagePending, ChildMessageConsumed, ChildMessageRejected, ChildMessageExpired:
		return true
	default:
		return false
	}
}

// ChildMessageReceipt records one participant Run's safe-point outcome.
// Failed or in-progress receipts leave the message pending for at-least-once retry.
type ChildMessageReceipt struct {
	ChildSessionID SessionID
	MessageID      string
	ConsumerRunID  RunID
	State          ChildMessageReceiptState
	CreatedAt      int64
	UpdatedAt      int64
}

type ChildMessageReceiptState string

const (
	ChildMessageReceiptInProgress ChildMessageReceiptState = "in-progress"
	ChildMessageReceiptConsumed   ChildMessageReceiptState = "consumed"
	ChildMessageReceiptFailed     ChildMessageReceiptState = "failed"
)

func (s ChildMessageReceiptState) Valid() bool {
	switch s {
	case ChildMessageReceiptInProgress, ChildMessageReceiptConsumed, ChildMessageReceiptFailed:
		return true
	default:
		return false
	}
}
