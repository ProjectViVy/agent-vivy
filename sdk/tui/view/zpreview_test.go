package view

import (
	"fmt"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

func TestDumpComposerPreview(t *testing.T) {
	if os.Getenv("TUI_PREVIEW") == "" {
		t.Skip("set TUI_PREVIEW=1")
	}
	driver := &testDriver{
		sessions: []surface.Session{{ID: "s1", Title: "overnight notes", PermissionPreset: "smart"}},
		active:   "s1",
		thinking: "auto",
		sidebar: surface.Sidebar{
			Session:            surface.Session{ID: "s1", Title: "overnight notes", PermissionPreset: "smart"},
			CWD:                "agent-vivy",
			Model:              "gpt-4.1",
			Provider:           "openai",
			ModifiedFilesKnown: true,
		},
		messages: map[string][]surface.Message{
			"s1": {
				{Role: surface.RoleUser, Content: "把输入框做成圆角"},
				{Role: surface.RoleAssistant, Content: "已经改成 Grok 风格的 composer：圆角盒子，左上角是模型、权限和思考模式。"},
			},
		},
	}
	m := New(driver)
	m.input = "hello from the composer"
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	m = updated.(Model)
	fmt.Print(m.View())
	fmt.Print("\n")

	// Busy + queued: chrome should show `<frame> run <elapsed>` on the left and
	// `queued 2` right-aligned.
	busy := &testDriver{
		sessions: driver.sessions,
		active:   "s1",
		thinking: "auto",
		sidebar:  driver.sidebar,
		messages: driver.messages,
		meta: surface.Meta{
			Busy:      true,
			Queued:    2,
			BusySince: time.Now().Add(-75 * time.Second),
		},
	}
	bm := New(busy)
	bm.input = "再跑一轮"
	updated, _ = bm.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	bm = updated.(Model)
	fmt.Print(bm.View())
	fmt.Print("\nchrome[busy+queued]: " + bm.renderInputChrome(bm.layout().mainW(), bm.palette) + "\n\n")

	// Scrolled up: chrome should show `↓ <pct>% · end 回底` right-aligned.
	scroll := &testDriver{
		sessions: driver.sessions,
		active:   "s1",
		thinking: "auto",
		sidebar:  driver.sidebar,
		messages: map[string][]surface.Message{"s1": {}},
	}
	for i := 0; i < 60; i++ {
		scroll.messages["s1"] = append(scroll.messages["s1"], surface.Message{Role: surface.RoleUser, Content: "被顶出视口的第 " + fmt.Sprint(i) + " 行"})
	}
	sm := New(scroll)
	updated, _ = sm.Update(tea.WindowSizeMsg{Width: 88, Height: 24})
	sm = updated.(Model)
	sm.scrollChat(-10)
	fmt.Print(sm.View())
	fmt.Print("\n")
	fmt.Print("chrome[scrolled]: " + sm.renderInputChrome(sm.layout().mainW(), sm.palette) + "\n")
}
