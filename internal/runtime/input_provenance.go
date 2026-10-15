package runtime

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"context"
	"encoding/json"
	"github.com/cloudwego/eino/schema"
)

const inputProvenanceExtra = "vivy_input_provenance"

func stampInputProvenance(msg *schema.Message, p storage.InputProvenance) {
	if msg.Extra == nil {
		msg.Extra = map[string]any{}
	}
	// Eino checkpoint serializers only accept registered or primitive values.
	// A JSON string survives native Eino and existing gob checkpoint copies.
	raw, _ := json.Marshal(p)
	msg.Extra[inputProvenanceExtra] = string(raw)
}

func projectedInputProvenance(rows []domain.Message) storage.InputProvenance {
	inputs := make([]storage.InputProvenance, 0, len(rows))
	for _, m := range rows {
		origin := m.ContentOrigin
		if origin == "" && m.Role == domain.RoleUser {
			origin = storage.ContentOriginOriginal
		}
		inputs = append(inputs, storage.InputProvenance{ContentOrigin: origin, ExcludeAutomaticIngest: m.ExcludeAutomaticIngest})
	}
	return storage.MergeInputProvenance(inputs...)
}

func (s *Service) modelInputProvenance(ctx context.Context, runID domain.RunID, messages []*schema.Message) (storage.InputProvenance, error) {
	inputs := make([]storage.InputProvenance, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if p, present := schemaInputProvenance(msg); present {
			inputs = append(inputs, p)
			continue
		}

		if msg.Role == schema.User || msg.Role == schema.System {
			inputs = append(inputs, storage.InputProvenance{ContentOrigin: storage.ContentOriginOriginal})
			continue
		}
		p := storage.UnknownInputProvenance()
		if msg.Role == schema.Tool && msg.ToolCallID != "" && s.deps.ToolOperations != nil {
			op, err := s.deps.ToolOperations.GetToolOperation(ctx, runID, msg.ToolCallID)
			if err != nil && err != storage.ErrNotFound {
				return p, err
			}
			if err == nil {
				p = storage.InputProvenance{ContentOrigin: op.ContentOrigin, ExcludeAutomaticIngest: op.ExcludeAutomaticIngest}
			}
		}
		inputs = append(inputs, p)
	}
	return storage.MergeInputProvenance(inputs...), nil
}

// Latest Journal request is the sole input-provenance authority for a tool.
// Shell-first execution has no model ancestor and is original evidence.
func (s *Service) latestModelInputProvenance(ctx context.Context, runID domain.RunID) (storage.InputProvenance, error) {
	p := storage.InputProvenance{ContentOrigin: storage.ContentOriginOriginal}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return p, err
	}
	defer it.Close()
	for it.Next() {
		e := it.Value().Event
		if e.Type != domain.EventModelRequest {
			continue
		}
		var route struct {
			Source string `json:"source"`
		}
		if json.Unmarshal(e.Payload, &route) != nil {
			return p, storage.ErrSubjectEvidenceIncomplete
		}
		if route.Source == modelCallSourceMaintenance || route.Source == modelCallSourceSummary {
			continue
		}
		p, err = storage.DecodeModelInputProvenance(e)
		if err != nil {
			return p, err
		}
	}
	return p, it.Err()
}

// AutomaticInputProvenance classifies an inseparable message range. Native
// user rows are original; legacy assistant/tool rows without provenance are
// unavailable. It never changes canonical messages or their existing digests.
func AutomaticInputProvenance(messages []domain.Message) (knownExcluded, available bool) {
	p := projectedInputProvenance(messages)
	return storage.ClassifyInputProvenance(p.ContentOrigin, p.ExcludeAutomaticIngest)
}

func schemaInputProvenance(msg *schema.Message) (storage.InputProvenance, bool) {
	extra, present := msg.Extra[inputProvenanceExtra]
	if !present {
		return storage.UnknownInputProvenance(), false
	}
	value := extra
	if encoded, ok := extra.(string); ok {
		value = json.RawMessage(encoded)
	}
	raw, err := json.Marshal(map[string]any{"input_provenance": value})
	if err != nil {
		return storage.UnknownInputProvenance(), true
	}
	p, err := storage.DecodeModelInputProvenance(domain.RunEvent{PayloadVersion: 4, Payload: raw})
	if err != nil {
		return storage.UnknownInputProvenance(), true
	}
	return p, true
}
