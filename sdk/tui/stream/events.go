// Package stream contains the protocol-independent stream core used by both
// Vivy TUI faces. Transport adapters convert their wire event enum to Event;
// projection and sequence handling stay here so the faces cannot drift.
package stream

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"unicode/utf8"
)

// Event is the normalized run/event envelope. Type deliberately remains a
// string: the core does not import the kernel domain package or a plugin face.
type Event struct {
	SubscriptionID string
	RunID          string
	Seq            int
	Type           string
	PayloadVersion int
	Payload        json.RawMessage
}

// Notice is the normalized, render-oriented event consumed by Projection.
// Unknown events retain RunID and Seq so the sequence cursor can advance even
// when a face does not render that event yet.
type Notice struct {
	SubscriptionID string
	RunID          string
	Seq            int
	PayloadVersion int
	Kind           string
	ToolCallID     string
	Line           string
	Delta          string
	Completed      string
	HasCompleted   bool
	// Completion contains the structurally validated v2 metadata. It is nil
	// for v1 (which carries authoritative content) and for malformed v2
	// notices. The stream consumer still verifies it against the deltas it has
	// applied before accepting the completion boundary.
	Completion *ModelCompletionMetadata
	// CompletedAuthoritative is true only for legacy completions carrying a
	// whole content field. V2 completion is a boundary whose body is already
	// present in model.delta events.
	CompletedAuthoritative bool
	// ProtocolError is a fail-closed event decoding/verification error. It is
	// kept on the notice so each face can make the error visible and close its
	// local run state instead of silently treating a bad completion as done.
	ProtocolError string
	Gate          *GatePrompt
	Done          bool
	Failed        bool
	Message       string
}

// ModelCompletionMetadata is the metadata-only model.completed v2 payload.
// The body itself is reconstructed from the preceding model.delta events.
type ModelCompletionMetadata struct {
	ContentSHA256 string
	ByteLen       int
}

// ParseModelCompletedV2 validates the metadata-only completion shape. V2 is
// deliberately strict: both fields are required, the digest is lower-case
// hexadecimal SHA-256, byte_len is a non-negative integer, and no body or
// unknown fields are accepted.
func ParseModelCompletedV2(raw json.RawMessage) (ModelCompletionMetadata, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err == nil {
			err = fmt.Errorf("payload must be an object")
		}
		return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: invalid payload: %w", err)
	}
	for key := range fields {
		if key != "content_sha256" && key != "byte_len" {
			return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: unknown field %q", key)
		}
	}
	rawDigest, ok := fields["content_sha256"]
	if !ok {
		return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: missing content_sha256")
	}
	var digest string
	if err := json.Unmarshal(rawDigest, &digest); err != nil {
		return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: content_sha256 must be a string")
	}
	if len(digest) != sha256.Size*2 || strings.ToLower(digest) != digest {
		return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: content_sha256 must be 64 lower-case hexadecimal characters")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: content_sha256 must be 64 lower-case hexadecimal characters")
	}
	rawByteLen, ok := fields["byte_len"]
	if !ok {
		return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: missing byte_len")
	}
	var byteLen int
	if err := json.Unmarshal(rawByteLen, &byteLen); err != nil || byteLen < 0 {
		return ModelCompletionMetadata{}, fmt.Errorf("model.completed v2: byte_len must be a non-negative integer")
	}
	return ModelCompletionMetadata{ContentSHA256: digest, ByteLen: byteLen}, nil
}

// VerifyModelCompletedV2 verifies a v2 payload against the exact UTF-8 bytes
// reconstructed by a consumer. The error does not include assistant text.
func VerifyModelCompletedV2(raw json.RawMessage, content string) error {
	metadata, err := ParseModelCompletedV2(raw)
	if err != nil {
		return err
	}
	return VerifyModelCompletedV2Metadata(metadata, content)
}

// VerifyModelCompletedV2Metadata verifies already parsed completion metadata
// against content. len([]byte(content)) is intentional: the contract counts
// UTF-8 bytes, not runes or display cells.
func VerifyModelCompletedV2Metadata(metadata ModelCompletionMetadata, content string) error {
	if !utf8.ValidString(content) {
		return fmt.Errorf("model.completed v2: reconstructed content is not valid UTF-8")
	}
	actualBytes := []byte(content)
	if metadata.ByteLen != len(actualBytes) {
		return fmt.Errorf("model.completed v2: byte_len mismatch")
	}
	sum := sha256.Sum256(actualBytes)
	if metadata.ContentSHA256 != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("model.completed v2: content_sha256 mismatch")
	}
	return nil
}

// ParseModelDelta validates a model.delta payload and returns its text. It
// mirrors the v1 schema so a malformed delta cannot disappear as an empty
// chunk and accidentally make an empty v2 completion appear valid.
func ParseModelDelta(raw json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err == nil {
			err = fmt.Errorf("payload must be an object")
		}
		return "", fmt.Errorf("model.delta: invalid payload: %w", err)
	}
	for key := range fields {
		if key != "delta" {
			return "", fmt.Errorf("model.delta: unknown field %q", key)
		}
	}
	rawDelta, ok := fields["delta"]
	if !ok {
		return "", fmt.Errorf("model.delta: missing delta")
	}
	var delta string
	if err := json.Unmarshal(rawDelta, &delta); err != nil {
		return "", fmt.Errorf("model.delta: delta must be a string")
	}
	if !utf8.ValidString(delta) {
		return "", fmt.Errorf("model.delta: delta is not valid UTF-8")
	}
	return delta, nil
}

// ModelCompletionTracker accumulates one model round's model.delta bytes and
// verifies the following model.completed v2 boundary. A model.request starts
// a fresh round; a successful completion also clears the accumulator.
// Consumers must call it only for sequence-accepted events.
type ModelCompletionTracker struct {
	digest  hash.Hash
	byteLen int
}

// Reset starts an empty model round. The zero value is ready for use too.
func (t *ModelCompletionTracker) Reset() {
	t.digest = sha256.New()
	t.byteLen = 0
}

// BeginRound is an explicit alias used by event consumers at model.request.
func (t *ModelCompletionTracker) BeginRound() { t.Reset() }

func (t *ModelCompletionTracker) ensure() {
	if t.digest == nil {
		t.Reset()
	}
}

// AddDelta appends one exact assistant text chunk to the current round.
func (t *ModelCompletionTracker) AddDelta(delta string) error {
	if !utf8.ValidString(delta) {
		return fmt.Errorf("model.delta: delta is not valid UTF-8")
	}
	t.ensure()
	n, _ := t.digest.Write([]byte(delta))
	t.byteLen += n
	return nil
}

// VerifyMetadata checks the current round without changing it.
func (t *ModelCompletionTracker) VerifyMetadata(metadata ModelCompletionMetadata) error {
	t.ensure()
	if metadata.ByteLen != t.byteLen {
		return fmt.Errorf("model.completed v2: byte_len mismatch")
	}
	if metadata.ContentSHA256 != hex.EncodeToString(t.digest.Sum(nil)) {
		return fmt.Errorf("model.completed v2: content_sha256 mismatch")
	}
	return nil
}

// Complete consumes a model.completed event. V1 (and an unversioned legacy
// event that carries content) returns its authoritative content; v2 returns
// an empty string because its body is the already streamed delta sequence.
func (t *ModelCompletionTracker) Complete(payloadVersion int, raw json.RawMessage) (string, error) {
	version := payloadVersion
	if version == 0 && modelCompletedHasContent(raw) {
		version = 1
	}
	switch version {
	case 1:
		content, err := parseLegacyCompletedContent(raw)
		if err != nil {
			return "", err
		}
		t.Reset()
		return content, nil
	case 2:
		metadata, err := ParseModelCompletedV2(raw)
		if err != nil {
			return "", err
		}
		if err := t.VerifyMetadata(metadata); err != nil {
			return "", err
		}
		t.Reset()
		return "", nil
	default:
		return "", fmt.Errorf("unsupported model.completed payload version %d", version)
	}
}

// ObserveEvent applies the model stream portions of a normalized event. It
// is useful for non-rendering consumers such as headless faces; model.delta
// text is still parsed separately when the caller needs to print it.
func (t *ModelCompletionTracker) ObserveEvent(event Event) error {
	switch event.Type {
	case "model.request":
		t.BeginRound()
		return nil
	case "model.delta":
		delta, err := ParseModelDelta(event.Payload)
		if err != nil {
			return err
		}
		return t.AddDelta(delta)
	case "model.completed":
		_, err := t.Complete(event.PayloadVersion, event.Payload)
		return err
	default:
		return nil
	}
}

// ObserveNotice applies the model stream portions of a render notice. It is
// the adapter-friendly equivalent of ObserveEvent for queues that already
// normalized payloads.
func (t *ModelCompletionTracker) ObserveNotice(notice Notice) error {
	if notice.ProtocolError != "" {
		return fmt.Errorf("%s", notice.ProtocolError)
	}
	switch notice.Kind {
	case "model_request":
		t.BeginRound()
	case "delta":
		return t.AddDelta(notice.Delta)
	case "model_completed":
		if notice.CompletedAuthoritative {
			t.Reset()
			return nil
		}
		if notice.Completion == nil {
			return fmt.Errorf("model.completed v2: missing validated metadata")
		}
		if err := t.VerifyMetadata(*notice.Completion); err != nil {
			return err
		}
		t.Reset()
	}
	return nil
}

func modelCompletedHasContent(raw json.RawMessage) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return false
	}
	_, ok := fields["content"]
	return ok
}

func parseLegacyCompletedContent(raw json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err == nil {
			err = fmt.Errorf("payload must be an object")
		}
		return "", fmt.Errorf("model.completed v1: invalid payload: %w", err)
	}
	rawContent, ok := fields["content"]
	if !ok {
		return "", fmt.Errorf("model.completed v1: missing content")
	}
	var content string
	if err := json.Unmarshal(rawContent, &content); err != nil {
		return "", fmt.Errorf("model.completed v1: content must be a string")
	}
	return content, nil
}

// GatePrompt is the normalized interaction overlay attached to a notice.
type GatePrompt struct {
	Kind       string // approval | question
	ID         string
	ToolCallID string
	Title      string
	Body       string
}

// StreamError is the control message emitted when durable replay itself
// fails. It is keyed by subscription so stale failures cannot poison a newer
// recovery stream.
type StreamError struct {
	SubscriptionID string
	Message        string
}

// DecodeStreamError validates a run/stream_error notification.
func DecodeStreamError(params json.RawMessage) (StreamError, bool) {
	var envelope struct {
		SubscriptionID string `json:"subscription_id"`
		Message        string `json:"message"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil || envelope.SubscriptionID == "" {
		return StreamError{}, false
	}
	return StreamError{SubscriptionID: envelope.SubscriptionID, Message: envelope.Message}, true
}

// Decode validates and decodes the control-plane run/event envelope.
func Decode(params json.RawMessage) (Event, bool) {
	var envelope struct {
		SubscriptionID string `json:"subscription_id"`
		Event          struct {
			RunID          string          `json:"run_id"`
			Seq            int             `json:"seq"`
			Type           string          `json:"type"`
			PayloadVersion int             `json:"payload_version"`
			Payload        json.RawMessage `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return Event{}, false
	}
	// Durable RunEvent sequence numbers are strictly positive by schema.
	// Rejecting legacy/unsequenced wire events keeps overflow replay sound:
	// every accepted notification can be reconstructed from the Journal.
	if envelope.Event.Type == "" || envelope.Event.Seq <= 0 {
		return Event{}, false
	}
	return Event{
		SubscriptionID: envelope.SubscriptionID,
		RunID:          envelope.Event.RunID,
		Seq:            envelope.Event.Seq,
		Type:           envelope.Event.Type,
		PayloadVersion: envelope.Event.PayloadVersion,
		Payload:        envelope.Event.Payload,
	}, true
}

// Interpret converts a normalized event to a render-oriented notice.
func Interpret(event Event) Notice {
	base := Notice{SubscriptionID: event.SubscriptionID, RunID: event.RunID, Seq: event.Seq, PayloadVersion: event.PayloadVersion}
	switch event.Type {
	case "model.request":
		base.Kind = "model_request"
		return base
	case "model.delta":
		delta, err := ParseModelDelta(event.Payload)
		if err != nil {
			return protocolFailure(base, err)
		}
		base.Kind = "delta"
		base.Delta = delta
		return base
	case "model.reasoning_delta":
		text := PayloadString(event.Payload, "delta")
		if text == "" {
			return base
		}
		base.Kind = "reasoning"
		base.Delta = text
		return base
	case "model.completed":
		base.Kind = "model_completed"
		base.HasCompleted = true
		var fields map[string]json.RawMessage
		if json.Unmarshal(event.Payload, &fields) != nil {
			base.Kind, base.Done, base.Failed, base.Message = "done", true, true, "invalid model.completed payload"
			return base
		}
		raw, hasContent := fields["content"]
		version := event.PayloadVersion
		if version == 0 && hasContent { // direct/legacy adapters predating the envelope version
			version = 1
		}
		switch version {
		case 1:
			if !hasContent || json.Unmarshal(raw, &base.Completed) != nil {
				return protocolFailure(base, fmt.Errorf("model.completed v1: missing or invalid content"))
			}
			base.CompletedAuthoritative = true
		case 2:
			metadata, err := ParseModelCompletedV2(event.Payload)
			if err != nil {
				return protocolFailure(base, err)
			}
			base.Completion = &metadata
		default:
			return protocolFailure(base, fmt.Errorf("unsupported model.completed payload version %d", version))
		}
		return base
	case "tool.requested":
		name := PayloadString(event.Payload, "tool_name")
		base.Kind = "tool_requested"
		base.ToolCallID = PayloadString(event.Payload, "tool_call_id")
		base.Line = PayloadObject(event.Payload, "args")
		base.Message = name
		return base
	case "tool.finished":
		name := PayloadString(event.Payload, "tool_name")
		errText := PayloadString(event.Payload, "error")
		base.Kind = "tool_finished"
		base.ToolCallID = PayloadString(event.Payload, "tool_call_id")
		base.Message = name
		base.Line = DisplayToolResult(PayloadString(event.Payload, "result"))
		if errText != "" {
			base.Failed = true
			base.Line = fmt.Sprintf("tool %s failed: %s", name, errText)
			return base
		}
		if base.Line == "" {
			base.Line = "tool " + name + " done"
		}
		return base
	case "tool.approval_required":
		id := PayloadString(event.Payload, "approval_id")
		name := PayloadString(event.Payload, "tool_name")
		preview := PayloadObject(event.Payload, "args")
		body := name
		if preview != "" {
			body = name + "\n" + preview
		}
		base.Kind = "gate"
		base.Line = "approval required: " + name + "  (y/n)"
		base.Gate = &GatePrompt{Kind: "approval", ID: id, ToolCallID: PayloadString(event.Payload, "tool_call_id"), Title: name, Body: body}
		return base
	case "user.question_required":
		id := PayloadString(event.Payload, "question_id")
		prompt := PayloadString(event.Payload, "prompt")
		base.Kind = "gate"
		base.Line = "question: " + prompt
		base.Gate = &GatePrompt{Kind: "question", ID: id, ToolCallID: PayloadString(event.Payload, "tool_call_id"), Title: "question", Body: prompt}
		return base
	case "run.completed":
		base.Kind = "done"
		base.Done = true
		return base
	case "run.failed":
		base.Kind = "done"
		base.Done = true
		base.Failed = true
		base.Message = PayloadString(event.Payload, "message")
		return base
	case "run.cancelled":
		base.Kind = "done"
		base.Done = true
		base.Failed = true
		base.Message = "cancelled"
		return base
	default:
		return base
	}
}

func protocolFailure(base Notice, err error) Notice {
	message := "invalid stream event"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	base.Kind = "done"
	base.Done = true
	base.Failed = true
	base.Message = message
	base.ProtocolError = message
	return base
}

// DisplayToolResult renders the structured mutation result without coupling
// the stream core to a tool implementation package.
func DisplayToolResult(result string) string {
	var mutation struct {
		Path        string `json:"path"`
		Diff        string `json:"diff"`
		Diagnostics string `json:"diagnostics"`
	}
	if json.Unmarshal([]byte(result), &mutation) == nil && mutation.Diff != "" {
		out := mutation.Path
		if out != "" {
			out += "\n"
		}
		out += mutation.Diff
		if mutation.Diagnostics != "" {
			out += "\n\nDiagnostics:\n" + mutation.Diagnostics
		}
		return out
	}
	return result
}

// PayloadObject returns a compact JSON object member for tool previews.
func PayloadObject(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || len(obj[key]) == 0 {
		return ""
	}
	var compact bytes.Buffer
	if json.Compact(&compact, obj[key]) != nil {
		return ""
	}
	return compact.String()
}

// PayloadString returns a string member from a JSON payload.
func PayloadString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	value, _ := obj[key].(string)
	return value
}
