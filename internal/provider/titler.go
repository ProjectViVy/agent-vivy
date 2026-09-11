package provider

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/modelhost"
)

const (
	// titleMaxTokens matches the Crush-aligned 40-token title budget.
	titleMaxTokens = 40
	// titleCallTimeout bounds one model attempt inside the chain.
	titleCallTimeout = 20 * time.Second
	// titlePromptClip bounds how much of the exchange enters the prompt.
	titlePromptClip = 2000
	// titleFallbackRunes bounds the truncation fallback label.
	titleFallbackRunes = 60
)

// ChainTitler generates session titles over an ordered model chain: the
// configured small model first, then the main model (VC-2 small→large
// chain), with a plain truncation of the first user message as the
// never-failing tail so a session is still named when every model is
// unusable.
type ChainTitler struct {
	candidates []model.ToolCallingChatModel
}

func NewChainTitler(candidates ...model.ToolCallingChatModel) *ChainTitler {
	return &ChainTitler{candidates: candidates}
}

func (t *ChainTitler) GenerateTitle(ctx context.Context, userText, assistantText string) (string, error) {
	messages := titlePrompt(userText, assistantText)
	var lastErr error
	for _, candidate := range t.candidates {
		if candidate == nil {
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, titleCallTimeout)
		resp, err := candidate.Generate(callCtx, messages, model.WithMaxTokens(titleMaxTokens))
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if title := strings.TrimSpace(resp.Content); title != "" {
			return title, nil
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return truncateTitleFallback(userText), nil
}

// titlePrompt asks for a bare short label; the strict "title only" wording
// keeps reasoning-model chatter and trailing punctuation out of the label.
func titlePrompt(userText, assistantText string) []*schema.Message {
	system := "Generate a short title (3-6 words) for this conversation. " +
		"Reply with the title only — no quotes, no trailing punctuation, no explanation."
	var b strings.Builder
	b.WriteString("First user message:\n" + clipTitleText(userText))
	if strings.TrimSpace(assistantText) != "" {
		b.WriteString("\n\nAssistant reply:\n" + clipTitleText(assistantText))
	}
	return []*schema.Message{
		{Role: schema.System, Content: system},
		{Role: schema.User, Content: b.String()},
	}
}

func clipTitleText(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > titlePromptClip {
		return string(runes[:titlePromptClip])
	}
	return string(runes)
}

func truncateTitleFallback(userText string) string {
	words := strings.Fields(userText)
	if len(words) == 0 {
		return ""
	}
	title := strings.Join(words, " ")
	if runes := []rune(title); len(runes) > titleFallbackRunes {
		title = string(runes[:titleFallbackRunes])
	}
	return title
}

// modelOverrideSource copies the live provider selection but pins the model
// id: the small model rides the active provider's base URL and key, so
// provider/model management stays one data source (D-9).
type modelOverrideSource struct {
	base  SpecSource
	model string
}

func (s modelOverrideSource) Live() LiveSpec {
	live := s.base.Live()
	live.Model = s.model
	return live
}

// NewOverrideModel pins an auxiliary model id (session titles, compaction
// summaries, …) over the active provider's live spec, so provider/model
// management stays one data source (D-9). The id resolves at call time.
func NewOverrideModel(host *modelhost.Host, catalog *Catalog, resolver SpecSource, modelID string) model.ToolCallingChatModel {
	return NewResolvingChatModel(host, catalog, modelOverrideSource{base: resolver, model: strings.TrimSpace(modelID)})
}

// TitleCandidates builds the ordered title chain over the app composition:
// the configured small model (same provider, pinned id) first, then the
// main chat model.
func TitleCandidates(host *modelhost.Host, catalog *Catalog, resolver SpecSource, main model.ToolCallingChatModel, smallModel string) []model.ToolCallingChatModel {
	candidates := make([]model.ToolCallingChatModel, 0, 2)
	if small := strings.TrimSpace(smallModel); small != "" && catalog != nil {
		candidates = append(candidates, NewOverrideModel(host, catalog, resolver, small))
	}
	return append(candidates, main)
}
