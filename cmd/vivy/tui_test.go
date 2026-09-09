package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/live"
	"agent-vivy/sdk/tui/surface"
	"agent-vivy/sdk/tui/view"
)

type localeTransport struct {
	calls  []string
	locale string
}

func (t *localeTransport) Call(_ context.Context, method string, _ any) (json.RawMessage, error) {
	t.calls = append(t.calls, method)
	if method == "settings/get" {
		return json.Marshal(map[string]any{"locale": t.locale, "generation_locale": "en", "workspace_locale": t.locale, "locale_read_only": false})
	}
	return json.RawMessage(`{"capabilities":[]}`), nil
}

func (*localeTransport) OnNotify(func(string, json.RawMessage)) {}

func TestRemoteTUISettingsLocaleReachesView(t *testing.T) {
	for _, locale := range []string{"en", "zh"} {
		t.Run(locale, func(t *testing.T) {
			transport := &localeTransport{locale: locale}
			rendered := false
			err := runRemoteTUI(context.Background(), transport, live.Options{Host: "remote", Title: "TUI"}, true,
				func(driver surface.Driver, options ...view.Options) error {
					rendered = true
					if len(options) != 1 || string(options[0].Locale) != locale || !options[0].DebugToolOutput {
						t.Fatalf("options=%+v", options)
					}
					model := view.New(driver, options...)
					updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
					want := "Ask something"
					if locale == "zh" {
						want = "问点什么"
					}
					if !strings.Contains(updated.View(), want) {
						t.Fatalf("localized composer missing: %s", updated.View())
					}
					return nil
				})
			if err != nil || !rendered || strings.Join(transport.calls, ",") != "initialize,settings/get" {
				t.Fatalf("err=%v calls=%v rendered=%t", err, transport.calls, rendered)
			}
		})
	}
}
