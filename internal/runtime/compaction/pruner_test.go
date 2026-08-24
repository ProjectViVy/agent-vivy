package compaction

import (
	"strings"
	"testing"
	"unicode/utf8"

	"agent-vivy/internal/domain"
)

func TestNewToolResultPrunerValidatesConfig(t *testing.T) {
	// Valid config should work
	cfg := DefaultPrunerConfig()
	pruner, err := NewToolResultPruner(cfg)
	if err != nil {
		t.Fatalf("DefaultPrunerConfig should be valid: %v", err)
	}
	if pruner.thresholdBytes != 8192 {
		t.Errorf("thresholdBytes = %d, want 8192", pruner.thresholdBytes)
	}

	// Invalid config where head+marker+tail exceeds threshold
	badCfg := PrunerConfig{
		ThresholdBytes: 100,
		HeadBytes:      60,
		TailBytes:      50,
		Marker:         "[...]",
	}
	_, err = NewToolResultPruner(badCfg)
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
}

func TestPruneSmallContent(t *testing.T) {
	cfg := DefaultPrunerConfig()
	pruner, _ := NewToolResultPruner(cfg)

	msg := domain.Message{
		Role:    domain.RoleTool,
		Content: "small result",
	}
	result := pruner.Prune(msg)
	if result.Content != msg.Content {
		t.Errorf("Small content should not be pruned: got %q", result.Content)
	}
}

func TestPruneLargeContent(t *testing.T) {
	cfg := PrunerConfig{
		ThresholdBytes: 100,
		HeadBytes:      40,
		TailBytes:      20,
		Marker:         "[...pruned...]",
	}
	pruner, _ := NewToolResultPruner(cfg)

	largeContent := strings.Repeat("x", 200)
	msg := domain.Message{
		Role:    domain.RoleTool,
		Content: largeContent,
	}
	result := pruner.Prune(msg)

	if len(result.Content) >= len(largeContent) {
		t.Errorf("Pruned content (%d bytes) should be smaller than original (%d bytes)",
			len(result.Content), len(largeContent))
	}
	if len(result.Content) > cfg.ThresholdBytes {
		t.Errorf("Pruned content (%d bytes) should not exceed threshold (%d bytes)",
			len(result.Content), cfg.ThresholdBytes)
	}
	if !strings.Contains(result.Content, cfg.Marker) {
		t.Errorf("Pruned content should contain marker %q", cfg.Marker)
	}
}

func TestPruneNonToolMessage(t *testing.T) {
	cfg := DefaultPrunerConfig()
	pruner, _ := NewToolResultPruner(cfg)

	msg := domain.Message{
		Role:    domain.RoleUser,
		Content: strings.Repeat("x", 10000),
	}
	result := pruner.Prune(msg)
	if result.Content != msg.Content {
		t.Error("Non-tool messages should not be pruned")
	}
}

func TestPruneSession(t *testing.T) {
	cfg := PrunerConfig{
		ThresholdBytes: 100,
		HeadBytes:      40,
		TailBytes:      20,
		Marker:         "[...]",
	}
	pruner, _ := NewToolResultPruner(cfg)

	messages := []domain.Message{
		{Role: domain.RoleUser, Content: "query"},
		{Role: domain.RoleAssistant, Content: "response"},
		{Role: domain.RoleTool, ToolCallID: "call-1", Content: strings.Repeat("x", 200)},
		{Role: domain.RoleTool, ToolCallID: "call-2", Content: "small"},
	}

	result := pruner.PruneSession(messages)

	// User and assistant messages should pass through unchanged
	if result[0].Content != messages[0].Content {
		t.Error("User message should be unchanged")
	}
	if result[1].Content != messages[1].Content {
		t.Error("Assistant message should be unchanged")
	}

	// Large tool result should be pruned
	if len(result[2].Content) >= len(messages[2].Content) {
		t.Error("Large tool result should be pruned")
	}
	if !strings.Contains(result[2].Content, cfg.Marker) {
		t.Error("Pruned tool result should contain marker")
	}

	// Small tool result should be unchanged
	if result[3].Content != messages[3].Content {
		t.Error("Small tool result should be unchanged")
	}
}

func TestNeedsPruning(t *testing.T) {
	cfg := DefaultPrunerConfig()
	pruner, _ := NewToolResultPruner(cfg)

	if pruner.NeedsPruning(strings.Repeat("x", 100)) {
		t.Error("Short content should not need pruning")
	}
	if !pruner.NeedsPruning(strings.Repeat("x", 10000)) {
		t.Error("Long content should need pruning")
	}
}

func TestCompactToolResultKeepsUnderBudget(t *testing.T) {
	result := strings.Repeat("x", 200)
	got := CompactToolResult(result, 96)
	if len(got) > 96 {
		t.Errorf("compacted result bytes = %d, want <= 96: %q", len(got), got)
	}
	// Check that head and tail contain some x's (not exactly "xx")
	if !strings.HasPrefix(got, "x") || !strings.HasSuffix(got, "[TRUNCATED]") {
		t.Errorf("compacted result lost structure: %q", got)
	}
	if !strings.Contains(got, "[TRUNCATED]") {
		t.Errorf("compacted result missing tombstone: %q", got)
	}
	if !strings.Contains(got, "[... truncated ...]") {
		t.Errorf("compacted result missing marker: %q", got)
	}
}

func TestCompactToolResultLeavesSmallResultsUntouched(t *testing.T) {
	for _, budget := range []int{100, 1000} {
		if got := CompactToolResult("small", budget); got != "small" {
			t.Errorf("small result was modified with budget %d: got %q", budget, got)
		}
	}
}

func TestCompactToolResultUTF8Safety(t *testing.T) {
	// Create content with multi-byte UTF-8 characters (Chinese characters)
	content := strings.Repeat("你好世界", 50) // Each character is 3 bytes
	budget := 100

	result := CompactToolResult(content, budget)

	// Verify the result is valid UTF-8
	if !utf8.ValidString(result) {
		t.Errorf("Compacted result is not valid UTF-8: %q", result)
	}

	// Should still be under budget
	if len(result) > budget {
		t.Errorf("Result exceeds budget: %d > %d", len(result), budget)
	}
}

func TestCompactToolResultEmptyAndTiny(t *testing.T) {
	if got := CompactToolResult("", 100); got != "" {
		t.Errorf("empty result should stay empty: got %q", got)
	}
	if got := CompactToolResult("ab", 100); got != "ab" {
		t.Errorf("tiny result should stay unchanged: got %q", got)
	}
}
