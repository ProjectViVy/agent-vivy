package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type titleFakeModel struct {
	response string
	err      error
	calls    int
}

func (m *titleFakeModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &schema.Message{Role: schema.Assistant, Content: m.response}, nil
}

func (m *titleFakeModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		defer w.Close()
		w.Send(msg, nil)
	}()
	return r, nil
}

func (m *titleFakeModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestChainTitlerFallsDownTheChain(t *testing.T) {
	broken := &titleFakeModel{err: errors.New("unknown small model")}
	main := &titleFakeModel{response: " main model title "}
	titler := NewChainTitler(broken, main)
	title, err := titler.GenerateTitle(context.Background(), "user asks something", "assistant answers")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if title != "main model title" {
		t.Fatalf("title = %q", title)
	}
	if broken.calls != 1 || main.calls != 1 {
		t.Fatalf("calls: broken=%d main=%d, want each candidate tried once", broken.calls, main.calls)
	}
}

func TestChainTitlerFailsOnlyWhenEveryModelFails(t *testing.T) {
	titler := NewChainTitler(&titleFakeModel{err: errors.New("a")}, &titleFakeModel{err: errors.New("b")})
	if _, err := titler.GenerateTitle(context.Background(), "user", "assistant"); err == nil {
		t.Fatal("all-model failure must surface the error so the session stays untitled")
	}
}

func TestChainTitlerTruncationFallbackWithoutModels(t *testing.T) {
	titler := NewChainTitler()
	title, err := titler.GenerateTitle(context.Background(), "  fix the   flaky test on windows  ", "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if title != "fix the flaky test on windows" {
		t.Fatalf("title = %q, want truncated first user message", title)
	}
	long := strings.Repeat("word ", titleFallbackRunes)
	title, err = titler.GenerateTitle(context.Background(), long, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len([]rune(title)) != titleFallbackRunes {
		t.Fatalf("fallback title = %d runes, want %d", len([]rune(title)), titleFallbackRunes)
	}
}

func TestChainTitlerSkipsEmptyModelReplies(t *testing.T) {
	empty := &titleFakeModel{response: "   "}
	main := &titleFakeModel{response: "real title"}
	titler := NewChainTitler(empty, main)
	title, err := titler.GenerateTitle(context.Background(), "user", "assistant")
	if err != nil || title != "real title" {
		t.Fatalf("title = %q err = %v", title, err)
	}
}

func TestModelOverrideSourcePinsOnlyTheModel(t *testing.T) {
	base := fixedSpecSource{live: LiveSpec{Provider: "openai", Model: "gpt-4o", BaseURL: "https://api", APIKey: "k", Ready: true}}
	over := modelOverrideSource{base: base, model: "gpt-4o-mini"}
	live := over.Live()
	if live.Model != "gpt-4o-mini" || live.Provider != "openai" || live.BaseURL != "https://api" || live.APIKey != "k" || !live.Ready {
		t.Fatalf("live = %+v", live)
	}
}

type fixedSpecSource struct{ live LiveSpec }

func (s fixedSpecSource) Live() LiveSpec { return s.live }

func TestTitleCandidatesOrder(t *testing.T) {
	main := &titleFakeModel{}
	onlyMain := TitleCandidates(nil, nil, fixedSpecSource{}, main, "ignored-without-catalog")
	if len(onlyMain) != 1 || onlyMain[0] != model.ToolCallingChatModel(main) {
		t.Fatalf("only-main chain = %d candidates", len(onlyMain))
	}
	withSmall := TitleCandidates(routedHost(t), NewCatalog(), fixedSpecSource{}, main, " gpt-4o-mini ")
	if len(withSmall) != 2 {
		t.Fatalf("small chain = %d candidates, want small then main", len(withSmall))
	}
	if withSmall[1] != model.ToolCallingChatModel(main) {
		t.Fatal("main model must be the chain tail")
	}
	if withSmall[0] == model.ToolCallingChatModel(main) {
		t.Fatal("small model must be a distinct resolving instance")
	}
}
