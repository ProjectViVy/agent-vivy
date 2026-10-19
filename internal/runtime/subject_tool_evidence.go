package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type originalToolOutcomeKey struct{}
type originalToolOutcome struct {
	result   string
	failure  error
	captured bool
}

func (s *Service) toolEvidenceBlobs() storage.BlobStore {
	if s.deps.ToolEvidence != nil {
		return s.deps.ToolEvidence
	}
	for _, candidate := range []any{s.deps.Journal, s.deps.Sessions} {
		if owner, ok := candidate.(interface{ Blobs() storage.BlobStore }); ok {
			return owner.Blobs()
		}
	}
	return nil
}

// retained rechecks current Host authorization; durable membership is not a grant.
func (c serviceToolOperationCoordinator) retained(ctx context.Context) (bool, error) {
	sources, ok := c.service.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return false, nil
	}
	pin, err := sources.GetRunSubjectSource(ctx, c.runID)
	if err == storage.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	current, err := c.service.resolveSubjectAdmission(ctx, c.sessionID)
	if err != nil {
		return false, err
	}
	if current == nil || !storage.SameSubjectSourceBinding(pin, *current) {
		return false, storage.ErrSubjectBinding
	}
	if c.service.toolEvidenceBlobs() == nil {
		return false, ErrToolOperationUnavailable
	}
	return true, nil
}
func (c serviceToolOperationCoordinator) RetainsEvidence(ctx context.Context) (bool, error) {
	return c.retained(ctx)
}

func (c serviceToolOperationCoordinator) retainArguments(ctx context.Context, op domain.ToolOperation, request []byte) error {
	if !utf8.Valid(request) || !json.Valid(request) {
		return storage.ErrSubjectEvidenceIncomplete
	}
	id := storage.ToolEvidenceArgumentsID(op)
	blobs := c.service.toolEvidenceBlobs()
	prior, found, err := blobs.Get(ctx, id)
	if err != nil {
		return err
	}
	if found {
		return storage.VerifyToolEvidenceArguments(op, prior)
	}
	if err := blobs.Put(ctx, id, request); err != nil {
		return fmt.Errorf("runtime: persist exact tool request: %w", err)
	}
	return nil
}
func (c serviceToolOperationCoordinator) retainResult(ctx context.Context, op domain.ToolOperation, result string, invokeErr error, original originalToolOutcome) (string, string, error) {
	evidence := storage.ToolEvidence{Result: strings.TrimPrefix(result, untrustedToolResultHeader), Outcome: &storage.ToolEvidenceOutcome{}}
	if state := nudgeStateFromContext(ctx); state != nil {
		if failure, ok := state.Failure(op.OperationID); ok {
			evidence.Outcome = &storage.ToolEvidenceOutcome{Outcome: failure.Status, Reason: failure.Reason, Effects: failure.Effects, Diagnostic: failure.Diagnostic}
		}
	}
	if err := evidence.Outcome.Validate(); err != nil {
		return "", "", err
	}
	if invokeErr != nil {
		evidence.Failure = invokeErr.Error()
	}
	if original.captured {
		evidence.Result = original.result
		evidence.Failure = ""
		if original.failure != nil {
			evidence.Failure = original.failure.Error()
		}
	}
	if !utf8.ValidString(evidence.Result) || !utf8.ValidString(evidence.Failure) {
		return "", "", storage.ErrSubjectEvidenceIncomplete
	}
	budget := c.service.engine.cfg.MaxToolResultBytes
	// Bound the receipt as well as the model projection. Full bytes already have
	// an independent durable home, so event fitting must never shorten the ref.
	if max := c.service.engine.cfg.MaxEventPayloadBytes; max > 0 && (budget <= 0 || budget > max/4) {
		budget = max / 4
	}
	projection := compactToolResult(result, budget)
	failure := ""
	if invokeErr != nil {
		failure = boundToolOperationFailure(invokeErr.Error())
	}
	evidence.ModelProjection = projection
	evidence.ModelFailure = failure
	raw, err := json.Marshal(evidence)
	if err != nil {
		return "", "", err
	}
	ref := storage.NewToolEvidenceReference(op, raw, "")
	if invokeErr == nil {
		projection += "\n[exact tool result: " + ref.ID + " sha256=" + ref.SHA256 + "]"
	}
	ref.Projection = projection
	blobs := c.service.toolEvidenceBlobs()
	prior, found, err := blobs.Get(ctx, ref.ID)
	if err != nil {
		return "", "", err
	}
	if found {
		if _, err := storage.VerifyToolEvidence(ref, prior); err != nil {
			return "", "", err
		}
	} else if err := blobs.Put(ctx, ref.ID, raw); err != nil {
		return "", "", fmt.Errorf("runtime: persist exact tool result: %w", err)
	}
	encoded, err := json.Marshal(ref)
	if err != nil {
		return "", "", err
	}
	completed := op
	completed.State = domain.ToolOperationCompleted
	completed.Result = string(encoded)
	completed.Failure = failure
	if err := c.checkOperationEventSize(completed); err != nil {
		return "", "", err
	}
	return string(encoded), failure, nil
}
func (c serviceToolOperationCoordinator) resolve(ctx context.Context, op domain.ToolOperation) (string, error) {
	if op.RunID != c.runID || op.State != domain.ToolOperationCompleted {
		return "", storage.ErrToolOperationConflict
	}
	retained, err := c.retained(ctx)
	if err != nil {
		return "", err
	}
	if !retained {
		return toolOperationResolution(op)
	}
	ref, err := storage.DecodeToolEvidenceReference(op)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrToolOperationUnknown, err)
	}
	raw, found, err := c.service.toolEvidenceBlobs().Get(ctx, ref.ID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: missing exact result %s", ErrToolOperationUnknown, ref.ID)
	}
	evidence, err := storage.VerifyToolEvidence(ref, raw)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrToolOperationUnknown, err)
	}
	arguments, found, err := c.service.toolEvidenceBlobs().Get(ctx, storage.ToolEvidenceArgumentsID(op))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrToolOperationUnknown, err)
	}
	if !found {
		return "", fmt.Errorf("%w: missing exact arguments", ErrToolOperationUnknown)
	}
	if err := storage.VerifyToolEvidenceArguments(op, arguments); err != nil {
		return "", fmt.Errorf("%w: %w", ErrToolOperationUnknown, err)
	}
	// Selected LIFE keeps the complete authorized output in its live run.
	// The compact reference remains the durable public projection and ordinary
	// Coding behavior; original bytes were verified before this branch.
	pin, err := c.service.deps.Sessions.(storage.SubjectSourceStore).GetRunSubjectSource(ctx, c.runID)
	if err != nil {
		return "", err
	}
	outcome := evidence.Outcome
	if outcome == nil && pin.ConversationID != "" {
		outcome, err = c.priorToolOutcome(ctx, op)
		if err != nil {
			return "", err
		}
		if outcome == nil {
			return "", fmt.Errorf("%w: legacy tool outcome unavailable", ErrRecapUnavailable)
		}
	}
	if outcome != nil && outcome.Outcome != "" {
		if state := nudgeStateFromContext(ctx); state != nil {
			if err := state.MarkFailure(op.OperationID, toolFailure{Status: outcome.Outcome, Reason: outcome.Reason, Effects: outcome.Effects, Diagnostic: outcome.Diagnostic}); err != nil {
				return "", err
			}
		}
	}
	if pin.ConversationID != "" {
		if op.Failure != "" {
			// Hard failure: the exact original error stays the run's error.
			return "", fmt.Errorf("%s", evidence.Failure)
		}
		if evidence.Failure != "" {
			// A recoverable failure or refusal was converted into a model-visible
			// result; evidence keeps the original error, the model sees the
			// conversion, exactly as in an ordinary run.
			return ref.Projection, nil
		}
		return untrustedToolResultHeader + evidence.Result, nil
	}
	if op.Failure != "" {
		return "", fmt.Errorf("%s", op.Failure)
	}
	if isToolPartsEnvelope(evidence.Result) {
		var envelope rawToolEnvelope
		if err := json.Unmarshal([]byte(evidence.Result), &envelope); err != nil {
			return "", err
		}
		envelope.ExactReference = "[exact tool result: " + ref.ID + " sha256=" + ref.SHA256 + "]"
		encoded, err := json.Marshal(envelope)
		if err != nil {
			return "", err
		}
		return untrustedToolResultHeader + string(encoded), nil
	}
	return ref.Projection, nil
}

// Only an exact prior host tool.finished can qualify a legacy artifact's
// outcome. Missing proof pauses; result text and today's classifier never do.
func (c serviceToolOperationCoordinator) priorToolOutcome(ctx context.Context, op domain.ToolOperation) (*storage.ToolEvidenceOutcome, error) {
	replay, err := c.service.deps.Journal.Replay(ctx, c.runID, 0)
	if err != nil {
		return nil, err
	}
	defer replay.Close()
	for replay.Next() {
		event := replay.Value().Event
		if event.Type != domain.EventToolFinished {
			continue
		}
		var p payloadToolFinished
		if json.Unmarshal(event.Payload, &p) != nil {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		if p.ToolCallID != op.OperationID || p.ToolName != op.ToolName {
			continue
		}
		if event.PayloadVersion != 1 {
			return nil, ErrRecapUnavailable
		}
		// Legacy success omitted the entire failure unit. A present unit must
		// have all three explicit string fields; null is never an attestation.
		var fields map[string]json.RawMessage
		if json.Unmarshal(event.Payload, &fields) != nil {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		for _, name := range []string{"tool_call_id", "tool_name", "result"} {
			if !legacyOutcomeString(fields, name) {
				return nil, storage.ErrSubjectEvidenceIncomplete
			}
		}
		present := 0
		for _, name := range []string{"outcome", "reason", "effects"} {
			if _, ok := fields[name]; ok {
				present++
				if !legacyOutcomeString(fields, name) {
					return nil, storage.ErrSubjectEvidenceIncomplete
				}
			}
		}
		if present != 0 && present != 3 {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		if _, ok := fields["error"]; ok && !legacyOutcomeString(fields, "error") {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		outcome := &storage.ToolEvidenceOutcome{Outcome: p.Outcome, Reason: p.Reason, Effects: p.Effects}
		if p.Outcome != "" {
			outcome.Diagnostic = p.Error
		} else if p.Error != "" {
			return nil, ErrRecapUnavailable
		}
		if err := outcome.Validate(); err != nil {
			return nil, err
		}
		return outcome, nil
	}
	return nil, replay.Err()
}

func legacyOutcomeString(fields map[string]json.RawMessage, name string) bool {
	raw, ok := fields[name]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	var value string
	return json.Unmarshal(raw, &value) == nil
}
