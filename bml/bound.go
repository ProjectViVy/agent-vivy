package bml

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

// OwnershipBinding is assigned by a trusted host, independently of legacy
// Record.Scope. Destination is part of ownership, not a routing hint.
type OwnershipBinding struct {
	SubjectID     string `json:"subject_id"`
	DomainID      string `json:"domain_id"`
	ScopeKind     string `json:"scope_kind"`
	WorkspaceID   string `json:"workspace_id"`
	DestinationID string `json:"destination_id"`
}

// BoundMutation preserves optional CAS shape. Nil record revision means absent;
// nil store revision chooses the actual store revision in the same transaction.
// A tombstone is a native marker with its own ID and an exact target revision.
type BoundMutation struct {
	Operation              string `json:"operation"`
	OperationID            string `json:"operation_id"`
	PayloadDigest          string `json:"payload_digest"`
	Record                 Record `json:"record"`
	ExpectedStoreRevision  *int64 `json:"expected_store_revision"`
	ExpectedRecordRevision *int64 `json:"expected_record_revision"`
	ExpectedTargetRevision *int64 `json:"expected_target_revision"`
}

// MutationOutcome is the immutable native commit result, not a second record
// body or a reconstruction from the record's current state.
type MutationOutcome struct {
	Binding         OwnershipBinding `json:"binding"`
	OperationID     string           `json:"operation_id"`
	PayloadDigest   string           `json:"payload_digest"`
	Operation       string           `json:"operation"`
	RecordID        string           `json:"record_id"`
	TargetID        string           `json:"target_id"`
	RecordRevision  int64            `json:"record_revision"`
	StoreRevision   int64            `json:"store_revision"`
	CanonicalStatus string           `json:"canonical_status"`
	IndexStatus     string           `json:"index_status"`
}

func boundUnavailable() error {
	return &StoreError{Code: "bound_capability_unavailable", Message: "bound native capability unavailable"}
}

func boundError(code, message string) error {
	return &StoreError{Code: StoreErrorCode(code), Message: message}
}

func (b OwnershipBinding) validate() error {
	if !validRawUTF8(reflect.ValueOf(b)) || strings.TrimSpace(b.SubjectID) == "" || strings.TrimSpace(b.DomainID) == "" || strings.TrimSpace(b.DestinationID) == "" {
		return boundError("invalid_binding", "complete UTF-8 ownership binding required")
	}
	if (b.ScopeKind == "personal" && b.WorkspaceID == "") || (b.ScopeKind == "workspace" && strings.TrimSpace(b.WorkspaceID) != "") {
		return nil
	}
	return boundError("invalid_binding", "invalid ownership scope shape")
}

// Check Go values before JSON can replace invalid UTF-8 with U+FFFD. All
// request strings, including nested provenance/evidence and optional values,
// participate; no caller digest can substitute for this check/fingerprint.
func validRawUTF8(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return utf8.ValidString(v.String())
	case reflect.Pointer:
		return v.IsNil() || validRawUTF8(v.Elem())
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !validRawUTF8(v.Index(i)) {
				return false
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() && !validRawUTF8(v.Field(i)) {
				return false
			}
		}
	}
	return true
}

type boundWrite struct {
	binding     OwnershipBinding
	bindingJSON string
	mutation    BoundMutation
	fingerprint string
	outcome     MutationOutcome
}

// MutateBound uses the existing canonical record transaction. Receipt replay
// is checked inside it before current row/store CAS or capacity preconditions.
func (s *Store) MutateBound(ctx context.Context, binding OwnershipBinding, m BoundMutation) (MutationOutcome, error) {
	if err := binding.validate(); err != nil {
		return MutationOutcome{}, err
	}
	if !validRawUTF8(reflect.ValueOf(m)) || strings.TrimSpace(m.OperationID) == "" || strings.TrimSpace(m.PayloadDigest) == "" {
		return MutationOutcome{}, boundError("invalid_mutation", "complete UTF-8 native operation identity required")
	}
	if s.schema < 2 || s.readOnly {
		return MutationOutcome{}, boundUnavailable()
	}
	if m.ExpectedStoreRevision != nil && *m.ExpectedStoreRevision < 0 {
		return MutationOutcome{}, boundError("invalid_mutation", "negative store revision")
	}
	var target *targetRevisionGuard
	switch m.Operation {
	case "create":
		if m.ExpectedRecordRevision != nil || m.ExpectedTargetRevision != nil || m.Record.Tombstone != nil {
			return MutationOutcome{}, boundError("invalid_mutation", "create requires absent native record")
		}
	case "update":
		if m.ExpectedRecordRevision == nil || *m.ExpectedRecordRevision <= 0 || m.ExpectedTargetRevision != nil || m.Record.Tombstone != nil {
			return MutationOutcome{}, boundError("invalid_mutation", "update requires exact positive record revision")
		}
	case "tombstone":
		if m.ExpectedRecordRevision != nil || m.ExpectedTargetRevision == nil || *m.ExpectedTargetRevision <= 0 || m.Record.Tombstone == nil {
			return MutationOutcome{}, boundError("invalid_mutation", "tombstone requires native marker and exact target revision")
		}
		if len(m.Record.Supersedes) != 1 || m.Record.Supersedes[0] != m.Record.Tombstone.TargetRecordID {
			return MutationOutcome{}, boundError("invalid_mutation", "bound tombstone requires only its guarded target")
		}
		target = &targetRevisionGuard{id: m.Record.Tombstone.TargetRecordID, revision: *m.ExpectedTargetRevision}
	default:
		return MutationOutcome{}, boundError("invalid_mutation", "unknown native mutation operation")
	}
	if err := m.Record.ValidateAt(time.Now().UTC(), validationClockSkew); err != nil {
		return MutationOutcome{}, invalidRecordErr(err)
	}
	if !canonicalNativeEnums(m.Record) {
		return MutationOutcome{}, boundError("invalid_mutation", "native record enum is not canonical")
	}
	for _, evidence := range m.Record.EvidenceRefs {
		if source, ok := evidenceSourceByName[string(evidence.Source)]; !ok || source != evidence.Source {
			return MutationOutcome{}, boundError("invalid_mutation", "native evidence source is not canonical")
		}
	}
	if m.Record.ValidateWorkspace(s.workspaceID) != nil {
		return MutationOutcome{}, workspaceMismatchErr(s.workspaceID, m.Record.Scope.WorkspaceID)
	}
	raw, err := marshalCanonical(struct {
		Binding  OwnershipBinding `json:"binding"`
		Mutation BoundMutation    `json:"mutation"`
	}{binding, m})
	if err != nil {
		return MutationOutcome{}, boundError("invalid_mutation", "native request cannot be encoded")
	}
	fingerprint := sha256.Sum256(raw)
	bindingRaw, err := marshalCanonical(binding)
	if err != nil {
		return MutationOutcome{}, err
	}
	op := &boundWrite{binding: binding, bindingJSON: string(bindingRaw), mutation: m, fingerprint: hex.EncodeToString(fingerprint[:])}
	var expectedStore int64
	if m.ExpectedStoreRevision != nil {
		expectedStore = *m.ExpectedStoreRevision
	}
	if _, err := s.putInner(ctx, m.Record, expectedStore, m.ExpectedRecordRevision, target, op); err != nil {
		return MutationOutcome{}, err
	}
	return op.outcome, nil
}

func canonicalNativeEnums(r Record) bool {
	kind, validKind := kindByName[string(r.Kind)]
	source, validSource := provenanceSourceByName[string(r.Provenance.Source)]
	trust, validTrust := trustByName[string(r.Trust)]
	sensitivity, validSensitivity := sensitivityByName[string(r.Sensitivity)]
	return validKind && kind == r.Kind && validSource && source == r.Provenance.Source && validTrust && trust == r.Trust && validSensitivity && sensitivity == r.Sensitivity
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) lookupBound(ctx context.Context, q rowQuerier, bindingJSON, operationID, digest, fingerprint string) (*MutationOutcome, error) {
	var owner, savedDigest, savedFingerprint, raw string
	err := q.QueryRowContext(ctx, `SELECT binding_json,payload_digest,request_fingerprint,outcome_json FROM memory_operations WHERE operation_id=?`, operationID).Scan(&owner, &savedDigest, &savedFingerprint, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, persistenceErr(s.path, err)
	}
	if owner != bindingJSON {
		return nil, boundError("ownership_denied", "native operation belongs to another binding")
	}
	if savedDigest != digest || (fingerprint != "" && savedFingerprint != fingerprint) {
		return nil, boundError("operation_conflict", "native operation request differs")
	}
	var out MutationOutcome
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, corruptRecordErr()
	}
	encoded, err := marshalCanonical(out.Binding)
	if err != nil || string(encoded) != owner || out.OperationID != operationID || out.PayloadDigest != digest || out.RecordRevision <= 0 || out.StoreRevision <= 0 || out.CanonicalStatus != "completed" || out.IndexStatus != "ready" {
		return nil, corruptRecordErr()
	}
	return &out, nil
}

// LookupBoundMutation reads only the immutable operation result. A missing
// result stays absent, including for old unbound records and after deletion.
func (s *Store) LookupBoundMutation(ctx context.Context, binding OwnershipBinding, operationID, digest string) (*MutationOutcome, error) {
	if err := binding.validate(); err != nil {
		return nil, err
	}
	if !utf8.ValidString(operationID) || !utf8.ValidString(digest) || strings.TrimSpace(operationID) == "" || strings.TrimSpace(digest) == "" {
		return nil, boundError("invalid_mutation", "native operation identity required")
	}
	if s.schema < 2 {
		return nil, boundUnavailable()
	}
	raw, err := marshalCanonical(binding)
	if err != nil {
		return nil, err
	}
	return s.lookupBound(ctx, s.db, string(raw), operationID, digest, "")
}

func (s *Store) validateWriteOwnership(ctx context.Context, tx *sql.Tx, record Record, op *boundWrite) error {
	if s.schema < 2 {
		return nil
	}
	check := func(id string, target bool) error {
		if op != nil {
			var foreign int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM memory_supersedes edge JOIN memory_records marker ON marker.memory_id=edge.memory_id LEFT JOIN memory_ownership own ON own.memory_id=marker.memory_id WHERE edge.superseded_id=? AND marker.tombstone=1 AND (own.binding_json IS NULL OR own.binding_json<>?)`, id, op.bindingJSON).Scan(&foreign); err != nil {
				return persistenceErr(s.path, err)
			}
			if foreign > 0 {
				return boundError("ownership_denied", "native record has foreign suppression evidence")
			}
		}
		var owner string
		err := tx.QueryRowContext(ctx, `SELECT binding_json FROM memory_ownership WHERE memory_id=?`, id).Scan(&owner)
		if errors.Is(err, sql.ErrNoRows) {
			if op == nil {
				return nil
			}
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM memory_records WHERE memory_id=?`, id).Scan(&exists); err != nil {
				return persistenceErr(s.path, err)
			}
			if !target && exists == 0 {
				return nil
			}
			return boundError("ownership_denied", "native record has no matching explicit ownership")
		}
		if err != nil {
			return persistenceErr(s.path, err)
		}
		if op == nil || owner != op.bindingJSON {
			return boundError("ownership_denied", "native record belongs to another binding")
		}
		if target || op.mutation.Operation == "update" {
			var deleted int
			if err := tx.QueryRowContext(ctx, `SELECT tombstone+(SELECT count(*) FROM memory_supersedes edge JOIN memory_records marker ON marker.memory_id=edge.memory_id WHERE edge.superseded_id=? AND marker.tombstone=1) FROM memory_records WHERE memory_id=?`, id, id).Scan(&deleted); err != nil {
				return persistenceErr(s.path, err)
			}
			if deleted > 0 {
				return boundError("source_unavailable", "native mutation target is no longer visible")
			}
		}
		return nil
	}
	if err := check(record.ID, false); err != nil {
		return err
	}
	for _, id := range record.Supersedes {
		if err := check(id, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) saveBoundOutcome(ctx context.Context, tx *sql.Tx, op *boundWrite, record Record, recordRevision, storeRevision int64) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_ownership(memory_id,binding_json) VALUES(?,?) ON CONFLICT(memory_id) DO NOTHING`, record.ID, op.bindingJSON); err != nil {
		return persistenceErr(s.path, err)
	}
	op.outcome = MutationOutcome{Binding: op.binding, OperationID: op.mutation.OperationID, PayloadDigest: op.mutation.PayloadDigest, Operation: op.mutation.Operation, RecordID: record.ID, RecordRevision: recordRevision, StoreRevision: storeRevision, CanonicalStatus: "completed", IndexStatus: "ready"}
	if record.Tombstone != nil {
		op.outcome.TargetID = record.Tombstone.TargetRecordID
	}
	raw, err := marshalCanonical(op.outcome)
	if err != nil {
		return corruptRecordErr()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_operations(operation_id,binding_json,payload_digest,request_fingerprint,outcome_json) VALUES(?,?,?,?,?)`, op.mutation.OperationID, op.bindingJSON, op.mutation.PayloadDigest, op.fingerprint, string(raw)); err != nil {
		return persistenceErr(s.path, err)
	}
	return nil
}
