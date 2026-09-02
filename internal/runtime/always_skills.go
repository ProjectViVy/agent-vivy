package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk"
	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/schema"
)

// SKILL-MKT-2 ratifies the DIVA `always` frontmatter capability: an enabled
// skill whose SKILL.md declares `always: true` has its body injected into
// every model call as transient context. The injection follows the agentsmd
// posture — a user-role message before the first user turn, tagged with an
// extra key for idempotency, never persisted, so compaction needs no
// carve-out. Budget semantics follow the DIVA reference (agent-diva
// skills.rs): a body over alwaysFileMaxChars runes is skipped entirely
// (never truncated mid-instruction), and the combined injection is capped at
// alwaysTotalMaxChars runes, filled in slug order.
const (
	alwaysFileMaxChars   = 4000
	alwaysTotalMaxChars  = 2000
	alwaysSkillsExtraKey = "__vivy_always_skills__"
)

// AlwaysSkillsSource is the engine seam for the always-injection capability.
// *EinoSkillBackend implements it; other einoskill.Backend implementations
// without the method simply get no injection.
type AlwaysSkillsSource interface {
	AlwaysSkills(ctx context.Context) (string, error)
}

var _ AlwaysSkillsSource = (*EinoSkillBackend)(nil)

// AlwaysSkills renders the always-on context for every enabled skill that
// declares `always: true`, under the file and total budgets. Empty when no
// skill qualifies.
func (b *EinoSkillBackend) AlwaysSkills(ctx context.Context) (string, error) {
	items, err := b.loadSkills(ctx)
	if err != nil {
		return "", err
	}
	var selected []loadedSkill
	for _, item := range items {
		if item.enabled && item.always {
			selected = append(selected, item)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].name < selected[j].name })
	remaining := alwaysTotalMaxChars
	sections := make([]string, 0, len(selected))
	for _, item := range selected {
		if utf8.RuneCountInString(item.content) > alwaysFileMaxChars || remaining == 0 {
			continue
		}
		included := truncateRunes(item.content, remaining)
		if strings.TrimSpace(included) == "" {
			continue
		}
		remaining -= utf8.RuneCountInString(included)
		sections = append(sections, fmt.Sprintf("### Skill: %s\n\n%s", item.name, included))
	}
	return strings.Join(sections, "\n\n---\n\n"), nil
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// alwaysSkillsMiddleware injects the always-on skill context per model call.
type alwaysSkillsMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	source AlwaysSkillsSource
}

func (m *alwaysSkillsMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	if state == nil {
		return ctx, state, nil
	}
	for _, msg := range state.Messages {
		if msg == nil || msg.Extra == nil {
			continue
		}
		if _, ok := msg.Extra[alwaysSkillsExtraKey]; ok {
			return ctx, state, nil
		}
	}
	content, err := m.source.AlwaysSkills(ctx)
	if err != nil {
		return ctx, nil, err
	}
	if strings.TrimSpace(content) == "" {
		return ctx, state, nil
	}
	injected := schema.UserMessage("## Active Skills\n\n" + content)
	injected.Extra = map[string]any{alwaysSkillsExtraKey: true}
	nState := *state
	nState.Messages = insertMessageBeforeFirstUser(state.Messages, injected)
	return ctx, &nState, nil
}

func insertMessageBeforeFirstUser(msgs []*schema.Message, injected *schema.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs)+1)
	for i, msg := range msgs {
		if msg != nil && msg.Role == schema.User {
			out = append(out, msgs[:i]...)
			out = append(out, injected)
			return append(out, msgs[i:]...)
		}
		out = append(out, msg)
	}
	return append(out, injected)
}

// buildAlwaysSkillsHandler is the NewEngine seam: a SkillBackend without the
// AlwaysSkills capability disables injection.
func buildAlwaysSkillsHandler(backend einoskill.Backend) adk.ChatModelAgentMiddleware {
	source, ok := backend.(AlwaysSkillsSource)
	if !ok {
		return nil
	}
	return &alwaysSkillsMiddleware{source: source}
}
