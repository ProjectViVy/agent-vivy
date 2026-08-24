package compaction

import (
	"agent-vivy/internal/domain"
)

// ToolResultPruner trims oversized tool result messages by retaining a
// bounded head, a fixed omission marker, and a bounded tail. This is a
// deterministic, model-free operation that reduces context pressure without
// semantic understanding.
type ToolResultPruner struct {
	thresholdBytes int
	headBytes      int
	tailBytes      int
	marker         string
}

// PrunerConfig holds configuration for the ToolResultPruner. All values
// are in bytes. The pruner only activates when content exceeds thresholdBytes.
type PrunerConfig struct {
	// ThresholdBytes is the maximum allowed size before pruning kicks in.
	// Default: 8192 bytes (8 KB).
	ThresholdBytes int
	// HeadBytes is the number of leading bytes to retain. Default: 4096.
	HeadBytes int
	// TailBytes is the number of trailing bytes to retain. Default: 1024.
	TailBytes int
	// Marker is the text inserted between head and tail to indicate
	// omitted content. Default: "\n\n[... tool result middle pruned ...]\n\n".
	Marker string
}

// DefaultPrunerConfig returns a safe default configuration suitable for
// most use cases.
func DefaultPrunerConfig() PrunerConfig {
	return PrunerConfig{
		ThresholdBytes: 8192,
		HeadBytes:      4096,
		TailBytes:      1024,
		Marker:         "\n\n[... tool result middle pruned ...]\n\n",
	}
}

// NewToolResultPruner creates a new pruner with the given configuration.
// It validates that the configuration is sane: head + marker + tail must
// fit within the threshold.
func NewToolResultPruner(cfg PrunerConfig) (*ToolResultPruner, error) {
	if cfg.ThresholdBytes <= 0 {
		cfg.ThresholdBytes = DefaultPrunerConfig().ThresholdBytes
	}
	if cfg.HeadBytes < 0 {
		cfg.HeadBytes = DefaultPrunerConfig().HeadBytes
	}
	if cfg.TailBytes < 0 {
		cfg.TailBytes = DefaultPrunerConfig().TailBytes
	}
	if cfg.Marker == "" {
		cfg.Marker = DefaultPrunerConfig().Marker
	}

	// Validate that the retention budget fits within the threshold
	totalRetention := cfg.HeadBytes + len(cfg.Marker) + cfg.TailBytes
	if totalRetention > cfg.ThresholdBytes {
		return nil, ErrInvalidPrunerConfig{
			HeadBytes:      cfg.HeadBytes,
			TailBytes:      cfg.TailBytes,
			MarkerBytes:    len(cfg.Marker),
			ThresholdBytes: cfg.ThresholdBytes,
		}
	}

	return &ToolResultPruner{
		thresholdBytes: cfg.ThresholdBytes,
		headBytes:      cfg.HeadBytes,
		tailBytes:      cfg.TailBytes,
		marker:         cfg.Marker,
	}, nil
}

// ErrInvalidPrunerConfig reports that the pruner's retention budget exceeds
// its activation threshold, which would make pruning ineffective or even
// counterproductive.
type ErrInvalidPrunerConfig struct {
	HeadBytes      int
	TailBytes      int
	MarkerBytes    int
	ThresholdBytes int
}

func (e ErrInvalidPrunerConfig) Error() string {
	return "compaction: pruner config invalid: head+marker+tail " +
		"(" + itoa(e.HeadBytes) + "+" + itoa(e.MarkerBytes) + "+" + itoa(e.TailBytes) +
		"=" + itoa(e.HeadBytes+e.MarkerBytes+e.TailBytes) +
		") exceeds threshold (" + itoa(e.ThresholdBytes) + ")"
}

// Simple integer-to-string conversion to avoid importing strconv in this
// focused utility.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// Prune trims one tool result message if it exceeds the threshold. It
// returns the original message unchanged if it is already within bounds,
// or a new message with pruned content otherwise. The pruned message is
// guaranteed to be smaller than the input and no larger than the threshold.
// UTF-8 safety: the function ensures valid UTF-8 output by rounding cut
// points to character boundaries.
func (p *ToolResultPruner) Prune(msg domain.Message) domain.Message {
	if msg.Role != domain.RoleTool || len(msg.Content) <= p.thresholdBytes {
		return msg
	}

	content := msg.Content
	runes := []rune(content) // Convert to runes for UTF-8 safety
	totalRunes := len(runes)

	// Calculate rune budgets from byte budgets approximately
	// Since we're working with runes now, we use a conservative estimate
	headRunes := p.headBytes / 4 // ~4 bytes per rune average
	tailRunes := p.tailBytes / 4

	// Ensure we don't exceed actual content length
	if headRunes+tailRunes >= totalRunes {
		// Content is small enough in runes; just return as-is
		// (this shouldn't happen since we checked bytes above, but guard anyway)
		return msg
	}

	// Build pruned content
	markerRunes := []rune(p.marker)
	pruned := make([]rune, 0, headRunes+len(markerRunes)+tailRunes)

	// Add head
	if headRunes > 0 && headRunes <= totalRunes {
		pruned = append(pruned, runes[:headRunes]...)
	}

	// Add marker
	pruned = append(pruned, markerRunes...)

	// Add tail
	if tailRunes > 0 {
		startIdx := totalRunes - tailRunes
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx < totalRunes {
			pruned = append(pruned, runes[startIdx:]...)
		}
	}

	prunedContent := string(pruned)

	// Verify the pruned result is actually smaller
	if len(prunedContent) >= len(content) {
		// Fallback: if pruning didn't reduce size, return original
		// This can happen with very short markers or unusual encodings
		return msg
	}

	// Return new message with pruned content, preserving all other fields
	msg.Content = prunedContent
	return msg
}

// PruneSession applies pruning to all tool result messages in a session's
// message list. It returns a new slice with pruned messages replaced;
// non-tool messages pass through unchanged. This is useful for bulk
// preprocessing before a model call.
func (p *ToolResultPruner) PruneSession(messages []domain.Message) []domain.Message {
	result := make([]domain.Message, len(messages))
	copy(result, messages)

	for i, msg := range messages {
		if msg.Role == domain.RoleTool {
			result[i] = p.Prune(msg)
		}
	}

	return result
}

// NeedsPruning reports whether the given content exceeds the pruner's
// threshold.
func (p *ToolResultPruner) NeedsPruning(content string) bool {
	return len(content) > p.thresholdBytes
}

// CompactToolResult is a convenience function that uses simple byte-level
// truncation with head/tail preservation. It is UTF-8 aware and will not
// split multi-byte characters. This function exists for compatibility with
// the existing runtime.tooladapter implementation.
func CompactToolResult(result string, budget int) string {
	if len(result) <= budget {
		return result
	}

	const (
		tombstone = "\n\n[TRUNCATED]"
		marker    = "\n\n[... truncated ...]\n\n"
	)
	overhead := len(tombstone) + len(marker)
	contentBudget := budget - overhead
	if contentBudget <= 0 {
		return marker + tombstone
	}

	half := contentBudget / 2
	headEnd := half
	tailStart := len(result) - (contentBudget - half)

	// Ensure we don't create an invalid range
	if tailStart < headEnd {
		tailStart = headEnd
	}

	// Round to UTF-8 character boundaries
	headEnd = utf8RuneBoundary(result, headEnd)
	tailStart = utf8RuneBoundary(result, tailStart)

	head := result[:headEnd]
	tail := result[tailStart:]

	return head + marker + tail + tombstone
}

// utf8RuneBoundary adjusts the given byte index to fall on a valid UTF-8
// rune boundary by moving backward until we find the start of a rune.
func utf8RuneBoundary(s string, idx int) int {
	if idx >= len(s) {
		return len(s)
	}
	// Move backward until we find a byte that is not a continuation byte
	// (continuation bytes have the pattern 10xxxxxx)
	for idx > 0 && (s[idx]&0xC0) == 0x80 {
		idx--
	}
	return idx
}
