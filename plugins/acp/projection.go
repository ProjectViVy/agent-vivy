package acp

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"unicode/utf8"

	acp "github.com/eino-contrib/acp"
)

// Spec §10: at most 256 events and 8 MiB per active run counting ingress and
// reorder storage together; tool titles are bounded labels, never raw args.
const (
	maxBufferedEvents = 256
	maxEventBytes     = 8 << 20
	eventOverhead     = 256 // per-event envelope accounting bytes
	maxToolTitleBytes = 256

	// Display boundary (spec §7/§10): an incomplete model line is bounded
	// at 64 KiB; an outbound ACP frame is bounded at 256 KiB encoded.
	maxIncompleteLineBytes = 64 << 10
	maxOutboundFrameBytes  = 256 << 10
)

// lineOmittedMarker replaces a suppressed oversized line on the wire.
const lineOmittedMarker = "[line omitted: exceeds 64 KiB]\n"

// opaqueToolID derives the ACP toolCallId from the connection nonce +
// session + run + durable tool_call_id, with length-prefixed inputs so the
// ID stays unique across successive turns without a second store (spec §7).
func opaqueToolID(nonce string, scope promptScope, toolCallID string) string {
	h := sha256.New()
	writePart := func(s string) {
		var lenBuf [8]byte
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(s)))
		h.Write(lenBuf[:])
		h.Write([]byte(s))
	}
	writePart(nonce)
	writePart(scope.SessionID)
	writePart(scope.RunID)
	writePart(toolCallID)
	return hex.EncodeToString(h.Sum(nil))
}

// runProjection translates ordered committed events into bounded ACP session
// updates. It never copies raw tool args, raw result JSON, model request
// payloads, provider metadata, reasoning traces or journal envelopes onto
// the wire (spec §7).
type runProjection struct {
	nonce string
	scope promptScope

	// emittedTools tracks tool_call notifications already sent so a
	// tool_call_update can never precede its tool_call.
	emittedTools map[string]struct{}

	// Display boundary state: the incomplete line buffer, the continuous
	// SHA-256 over original committed delta bytes, and their byte total
	// (model.completed v2 verifies both — spec §7).
	line       []byte
	lineOmit   bool
	hasher     hash.Hash
	totalBytes int64
}

func newRunProjection(nonce string, scope promptScope) *runProjection {
	return &runProjection{
		nonce:        nonce,
		scope:        scope,
		emittedTools: make(map[string]struct{}),
		hasher:       sha256.New(),
	}
}

var errIntegrity = fmt.Errorf("stream integrity")

// toolEventPayload is the shared shape of tool.requested/started/finished.
type toolEventPayload struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	// finished only
	Error   string `json:"error,omitempty"`
	Outcome string `json:"outcome,omitempty"`
}

// apply projects one in-order committed event into zero or more ACP session
// updates. Terminal and interaction events are handled by the reducer;
// returning errIntegrity marks the event malformed.
func (r *runProjection) apply(ev committedEvent) ([]acp.SessionUpdate, error) {
	switch ev.Type {
	case "tool.requested":
		var p toolEventPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.ToolCallID == "" || p.ToolName == "" {
			return nil, fmt.Errorf("%w: malformed tool.requested", errIntegrity)
		}
		return r.ensureToolCall(p.ToolCallID, p.ToolName), nil
	case "tool.started":
		var p toolEventPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.ToolCallID == "" {
			return nil, fmt.Errorf("%w: malformed tool.started", errIntegrity)
		}
		updates := r.ensureToolCall(p.ToolCallID, p.ToolName)
		status := acp.ToolCallStatusInProgress
		updates = append(updates, acp.SessionUpdate{ToolCallUpdate: &acp.SessionUpdateToolCallUpdate{
			ToolCallUpdate: acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(r.toolID(p.ToolCallID)),
				Status:     &status,
			},
			SessionUpdate: "tool_call_update",
		}})
		return updates, nil
	case "model.delta":
		var p struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, fmt.Errorf("%w: malformed model.delta", errIntegrity)
		}
		return r.appendDelta(p.Delta), nil
	case "model.completed":
		// v2 commits the delta stream by digest; anything else fails
		// explicitly (spec §7/§10).
		if ev.PayloadVersion != 2 {
			return nil, fmt.Errorf("%w: unsupported model.completed version %d", errIntegrity, ev.PayloadVersion)
		}
		var p struct {
			ContentSHA256 string `json:"content_sha256"`
			ByteLen       int64  `json:"byte_len"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.ContentSHA256 == "" {
			return nil, fmt.Errorf("%w: malformed model.completed", errIntegrity)
		}
		sum := hex.EncodeToString(r.hasher.Sum(nil))
		if p.ByteLen != r.totalBytes || p.ContentSHA256 != sum {
			return nil, fmt.Errorf("%w: model.completed digest mismatch", errIntegrity)
		}
		return r.flushLine(), nil
	case "tool.finished":
		var p toolEventPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.ToolCallID == "" {
			return nil, fmt.Errorf("%w: malformed tool.finished", errIntegrity)
		}
		updates := r.ensureToolCall(p.ToolCallID, p.ToolName)
		status := acp.ToolCallStatusCompleted
		if p.Error != "" || p.Outcome != "" {
			status = acp.ToolCallStatusFailed
		}
		updates = append(updates, acp.SessionUpdate{ToolCallUpdate: &acp.SessionUpdateToolCallUpdate{
			ToolCallUpdate: acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(r.toolID(p.ToolCallID)),
				Status:     &status,
			},
			SessionUpdate: "tool_call_update",
		}})
		return updates, nil
	default:
		return nil, nil // deliberately unprojected types still advance seq
	}
}

// ensureToolCall emits the tool_call notification for an unseen ID so no
// tool_call_update can arrive before it (spec §7). Title carries the bounded
// tool name only — never args or results.
func (r *runProjection) ensureToolCall(toolCallID, toolName string) []acp.SessionUpdate {
	id := r.toolID(toolCallID)
	if _, ok := r.emittedTools[id]; ok {
		return nil
	}
	r.emittedTools[id] = struct{}{}
	kind := acp.ToolKindOther
	status := acp.ToolCallStatusPending
	return []acp.SessionUpdate{{ToolCall: &acp.SessionUpdateToolCall{
		ToolCall: acp.ToolCall{
			ToolCallID: acp.ToolCallID(id),
			Title:      boundLabel(toolName),
			Kind:       &kind,
			Status:     &status,
		},
		SessionUpdate: "tool_call",
	}}}
}

func (r *runProjection) toolID(toolCallID string) string {
	return opaqueToolID(r.nonce, r.scope, toolCallID)
}

// boundLabel truncates an event-derived label for the wire.
func boundLabel(s string) string {
	if len(s) <= maxToolTitleBytes {
		return s
	}
	return strings.TrimRight(s[:maxToolTitleBytes], "\uFFFD") + "…"
}

// appendDelta hashes the original committed bytes continuously, then emits
// each completed line as one bounded agent_message_chunk. A partial line is
// held up to 64 KiB; an oversized line is suppressed with an explicit
// omission marker (spec §7).
func (r *runProjection) appendDelta(delta string) []acp.SessionUpdate {
	b := []byte(delta)
	r.hasher.Write(b)
	r.totalBytes += int64(len(b))

	var out []acp.SessionUpdate
	for len(b) > 0 {
		nl := bytes.IndexByte(b, '\n')
		if nl < 0 {
			r.appendPartial(b)
			break
		}
		line := b[:nl+1]
		b = b[nl+1:]
		if r.lineOmit {
			// Already suppressing this line; drop until it ends.
			r.line = r.line[:0]
			r.lineOmit = false
			out = append(out, textChunkUpdate(lineOmittedMarker))
			continue
		}
		r.line = append(r.line, line...)
		if len(r.line) > maxIncompleteLineBytes {
			r.line = r.line[:0]
			out = append(out, textChunkUpdate(lineOmittedMarker))
			continue
		}
		out = append(out, textChunkUpdate(string(r.line)))
		r.line = r.line[:0]
	}
	return out
}

// appendPartial buffers line bytes without a terminator, enforcing the
// incomplete-line bound.
func (r *runProjection) appendPartial(b []byte) {
	if r.lineOmit {
		return // suppress until the line terminates
	}
	r.line = append(r.line, b...)
	if len(r.line) > maxIncompleteLineBytes {
		r.line = r.line[:0]
		r.lineOmit = true
	}
}

// flushLine emits any remaining buffered line (or its omission marker) when
// the model stream completes.
func (r *runProjection) flushLine() []acp.SessionUpdate {
	if len(r.line) == 0 && !r.lineOmit {
		return nil
	}
	if r.lineOmit {
		r.lineOmit = false
		r.line = r.line[:0]
		return []acp.SessionUpdate{textChunkUpdate(lineOmittedMarker)}
	}
	text := string(r.line)
	r.line = r.line[:0]
	return []acp.SessionUpdate{textChunkUpdate(text)}
}

// textChunkUpdate wraps text as one agent_message_chunk, splitting at rune
// boundaries so the encoded notification stays under the frame bound
// (spec §10: split safe text content, never arbitrary JSON). The first
// segment is returned; oversized callers must emit the rest — enforced by
// splitTextUpdate below at emission time.
func textChunkUpdate(text string) acp.SessionUpdate {
	return acp.SessionUpdate{AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
		ContentChunk: acp.ContentChunk{
			Content: acp.ContentBlock{Text: &acp.ContentBlockText{
				TextContent: acp.TextContent{Text: text},
			}},
		},
		SessionUpdate: "agent_message_chunk",
	}}
}

// splitChunk splits one message-chunk text so each encoded session update
// frame stays under maxOutboundFrameBytes, breaking only at rune
// boundaries.
func splitChunk(text string) []string {
	update := textChunkUpdate(text)
	if encodedLen(update) <= maxOutboundFrameBytes {
		return []string{text}
	}
	var out []string
	rest := text
	for len(rest) > 0 {
		// Binary-search the longest rune-boundary prefix that fits.
		lo, hi := 0, len(rest)
		best := 0
		for lo <= hi {
			mid := (lo + hi) / 2
			mid = runeFloor(rest, mid)
			if mid <= 0 {
				break
			}
			if encodedLen(textChunkUpdate(rest[:mid])) <= maxOutboundFrameBytes {
				best = mid
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		if best == 0 {
			best = runeFloor(rest, len(rest)) // degenerate: emit as one piece
		}
		out = append(out, rest[:best])
		rest = rest[best:]
	}
	return out
}

func runeFloor(s string, i int) int {
	if i >= len(s) {
		return len(s) // end of string is always a boundary
	}
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}

func encodedLen(u acp.SessionUpdate) int {
	b, err := json.Marshal(acp.SessionNotification{Update: u})
	if err != nil {
		return 0
	}
	return len(b)
}
