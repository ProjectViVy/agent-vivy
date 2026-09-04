package compaction

import (
	"testing"

	"agent-vivy/internal/domain"
)

func TestMeasureEmptyMessages(t *testing.T) {
	breakdown := Measure(nil, "system prompt", 128000)
	if breakdown.SystemPrompt != len("system prompt") {
		t.Errorf("SystemPrompt = %d, want %d", breakdown.SystemPrompt, len("system prompt"))
	}
	// When messages is nil, Total should only include system prompt
	expectedTotal := len("system prompt")
	if breakdown.Total != expectedTotal {
		t.Errorf("Total = %d, want %d", breakdown.Total, expectedTotal)
	}
	if breakdown.ModelLimit != 128000 {
		t.Errorf("ModelLimit = %d, want 128000", breakdown.ModelLimit)
	}
	// UsagePercent will be very small but not exactly zero due to token estimation
	if breakdown.UsagePercent > 1.0 {
		t.Errorf("UsagePercent should be near zero for small system prompt, got %f", breakdown.UsagePercent)
	}
}

func TestMeasureWithHistoryAndCurrent(t *testing.T) {
	messages := []*domain.Message{
		{Role: domain.RoleUser, Content: "hello"},
		{Role: domain.RoleAssistant, Content: "hi there"},
		{Role: domain.RoleUser, Content: "current request"},
	}
	breakdown := Measure(messages, "sys", 128000)

	if breakdown.HistoryMessages <= 0 {
		t.Error("HistoryMessages should be positive")
	}
	if breakdown.CurrentRequest <= 0 {
		t.Error("CurrentRequest should be positive")
	}
	if breakdown.Total <= breakdown.HistoryMessages {
		t.Error("Total should exceed HistoryMessages")
	}
}

func TestMeasureIncludesFileContextSnapshots(t *testing.T) {
	body := []byte("package main")
	messages := []*domain.Message{
		{Role: domain.RoleUser, Content: "inspect", FileContexts: []domain.FileContext{{Path: "main.go", Name: "main.go", Size: int64(len(body)), Content: body}}},
		{Role: domain.RoleUser, Content: "current"},
	}
	withFile := Measure(messages, "", 128000)
	messages[0].FileContexts = nil
	withoutFile := Measure(messages, "", 128000)
	if withFile.HistoryMessages <= withoutFile.HistoryMessages+len(body) {
		t.Fatalf("file snapshot missing from meter: with=%d without=%d", withFile.HistoryMessages, withoutFile.HistoryMessages)
	}
}

func TestMeasureToolResults(t *testing.T) {
	messages := []*domain.Message{
		{Role: domain.RoleUser, Content: "read file"},
		{Role: domain.RoleAssistant, ToolCallID: "call-1", ToolName: "read_file"},
		{Role: domain.RoleTool, ToolCallID: "call-1", Content: "very large tool output here with lots of text"},
		{Role: domain.RoleUser, Content: "current"},
	}
	breakdown := Measure(messages, "", 128000)

	if breakdown.ToolResults <= 0 {
		t.Error("ToolResults should be positive for tool role messages")
	}
	if breakdown.ToolResults > breakdown.HistoryMessages {
		t.Error("ToolResults should not exceed total history")
	}
}

func TestIsOverBudget(t *testing.T) {
	tests := []struct {
		name            string
		estimatedTokens int
		modelLimit      int
		thresholdRatio  float64
		want            bool
	}{
		{"under budget", 50000, 128000, 0.8, false},
		{"over budget", 110000, 128000, 0.8, true},
		{"exactly at threshold", 102400, 128000, 0.8, false}, // 102400 == 128000*0.8
		{"unknown limit", 50000, 0, 0.8, false},
		{"zero threshold", 50000, 128000, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := ContextBreakdown{
				EstimatedTokens: tt.estimatedTokens,
				ModelLimit:      tt.modelLimit,
			}
			got := b.IsOverBudget(tt.thresholdRatio)
			if got != tt.want {
				t.Errorf("IsOverBudget(%f) = %v, want %v", tt.thresholdRatio, got, tt.want)
			}
		})
	}
}

func TestHeadroom(t *testing.T) {
	tests := []struct {
		name            string
		estimatedTokens int
		modelLimit      int
		want            int
	}{
		{"positive headroom", 50000, 128000, 78000},
		{"no headroom", 128000, 128000, 0},
		{"negative headroom", 150000, 128000, -22000},
		{"unknown limit", 50000, 0, -50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := ContextBreakdown{
				EstimatedTokens: tt.estimatedTokens,
				ModelLimit:      tt.modelLimit,
			}
			got := b.Headroom()
			if got != tt.want {
				t.Errorf("Headroom() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		bytes int
		want  int
	}{
		{0, 0},
		{4, 1},
		{100, 25},
		{1000, 250},
		{-10, 0},
	}

	for _, tt := range tests {
		got := estimateTokens(tt.bytes)
		if got != tt.want {
			t.Errorf("estimateTokens(%d) = %d, want %d", tt.bytes, got, tt.want)
		}
	}
}
