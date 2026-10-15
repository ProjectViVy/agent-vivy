package storage

import (
	"agent-vivy/internal/domain"
	"bytes"
	"context"
	"encoding/json"
	"io"
)

// InputProvenance records the trusted origin of inseparable model derivatives.
// Unknown is unavailable evidence, even when another input is excluded.
// The existing Journal is its authority; no input bodies are copied here.
type InputProvenance struct {
	ContentOrigin          string `json:"content_origin"`
	ExcludeAutomaticIngest bool   `json:"exclude_automatic_ingest"`
}

const ContentOriginOriginal = "original"
const ContentOriginUnknown = "unknown"

func UnknownInputProvenance() InputProvenance {
	return InputProvenance{ContentOrigin: ContentOriginUnknown}
}

func MergeInputProvenance(inputs ...InputProvenance) InputProvenance {
	result := InputProvenance{ContentOrigin: ContentOriginOriginal}
	for _, p := range inputs {
		excluded, available := ClassifyInputProvenance(p.ContentOrigin, p.ExcludeAutomaticIngest)
		if !available {
			return UnknownInputProvenance()
		}
		if excluded {
			result = InputProvenance{ContentOrigin: domain.ContentOriginNotebook, ExcludeAutomaticIngest: true}
		}
	}
	return result
}

// DecodeModelInputProvenance is shared by original validation and projection.
// v1–v3 remain readable but cannot certify derivative eligibility. In v4 an
// explicit nonnull boolean is mandatory: omission must never mean false.
func DecodeModelInputProvenance(event domain.RunEvent) (InputProvenance, error) {
	if event.PayloadVersion < 4 {
		return UnknownInputProvenance(), nil
	}
	if event.PayloadVersion != 4 {
		return InputProvenance{}, ErrSubjectEvidenceIncomplete
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(event.Payload, &envelope) != nil {
		return InputProvenance{}, ErrSubjectEvidenceIncomplete
	}
	raw, ok := envelope["input_provenance"]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return InputProvenance{}, ErrSubjectEvidenceIncomplete
	}
	var wire struct {
		ContentOrigin          *string `json:"content_origin"`
		ExcludeAutomaticIngest *bool   `json:"exclude_automatic_ingest"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || wire.ContentOrigin == nil || wire.ExcludeAutomaticIngest == nil {
		return InputProvenance{}, ErrSubjectEvidenceIncomplete
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return InputProvenance{}, ErrSubjectEvidenceIncomplete
	}
	p := InputProvenance{ContentOrigin: *wire.ContentOrigin, ExcludeAutomaticIngest: *wire.ExcludeAutomaticIngest}
	switch p.ContentOrigin {
	case ContentOriginOriginal, ContentOriginUnknown, domain.ContentOriginNotebook:
	default:
		return InputProvenance{}, ErrSubjectEvidenceIncomplete
	}
	if p.ContentOrigin == ContentOriginUnknown && p.ExcludeAutomaticIngest {
		return InputProvenance{}, ErrSubjectEvidenceIncomplete
	}
	return p, nil
}

// ClassifyInputProvenance distinguishes unavailable provenance from a known
// automatic-ingest exclusion. Unknown always dominates a claimed exclusion.
func ClassifyInputProvenance(origin string, excluded bool) (knownExcluded, available bool) {
	switch origin {
	case ContentOriginOriginal:
		return excluded, true
	case domain.ContentOriginNotebook:
		return true, true
	default:
		return false, false
	}
}

// RunInputProvenance folds existing request metadata without loading input bodies.
// It intentionally visits every request: unknown cannot be hidden by a later
// known exclusion or by the final assistant response.
func RunInputProvenance(ctx context.Context, journal Journal, runID domain.RunID) (InputProvenance, error) {
	it, err := journal.Replay(ctx, runID, 0)
	if err != nil {
		return InputProvenance{}, err
	}
	defer it.Close()
	result := InputProvenance{ContentOrigin: ContentOriginOriginal}
	for it.Next() {
		event := it.Value().Event
		if event.Type != domain.EventModelRequest {
			continue
		}
		var route struct {
			Source string `json:"source"`
		}
		if json.Unmarshal(event.Payload, &route) != nil {
			return InputProvenance{}, ErrSubjectEvidenceIncomplete
		}
		if route.Source == "maintenance" {
			continue
		}
		p, err := DecodeModelInputProvenance(event)
		if err != nil {
			return InputProvenance{}, err
		}
		result = MergeInputProvenance(result, p)
	}
	return result, it.Err()
}
