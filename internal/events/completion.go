package events

import (
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

// NormalizeModelCompletion projects supported historical metadata onto the
// current wire shape. It never validates an old digest or mutates Journal bytes.
func NormalizeModelCompletion(event domain.RunEvent) (domain.RunEvent, error) {
	if event.Type != domain.EventModelCompleted {
		return event, nil
	}
	if event.PayloadVersion != 2 && event.PayloadVersion != 3 {
		return event, fmt.Errorf("unsupported model.completed payload version %d", event.PayloadVersion)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(event.Payload, &fields); err != nil {
		return event, fmt.Errorf("decode model.completed metadata: %w", err)
	}
	if fields == nil {
		return event, fmt.Errorf("model.completed: invalid metadata object")
	}
	for key := range fields {
		if key != "byte_len" && !(event.PayloadVersion == 2 && key == "content_sha256") {
			return event, fmt.Errorf("model.completed: unknown field %q", key)
		}
	}
	raw, ok := fields["byte_len"]
	var n int
	if !ok || string(raw) == "null" {
		return event, fmt.Errorf("model.completed: byte_len must be a non-negative integer")
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return event, fmt.Errorf("decode model.completed byte_len: %w", err)
	}
	if n < 0 {
		return event, fmt.Errorf("model.completed: byte_len must be a non-negative integer")
	}
	payload, err := json.Marshal(struct {
		ByteLen int `json:"byte_len"`
	}{n})
	if err != nil {
		return event, fmt.Errorf("encode model.completed: %w", err)
	}
	event.PayloadVersion = 3
	event.Payload = payload
	return event, nil
}
