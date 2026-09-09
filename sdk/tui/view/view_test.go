package view

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	corei18n "agent-vivy/internal/i18n"
	"agent-vivy/sdk/tui/command"
	"agent-vivy/sdk/tui/surface"
)

func TestLocaleViewChromeAndDialogs(t *testing.T) {
	// Ignoring Options.Locale or falling back to fixed copy must fail these
	// independently authored expectations. Each case gets fresh model caches.
	cases := []struct {
		name   string
		render func(*Model, *testDriver) string
		en, zh []string
	}{
		{"composer", func(m *Model, d *testDriver) string {
			return m.renderEditor(120, m.palette)
		}, []string{"Smart", "Ask something", "provider-原样", "model-原样"}, []string{"智能", "问点什么", "provider-原样", "model-原样"}},
		{"mode cycle", func(m *Model, d *testDriver) string {
			var out []string
			for i := 0; i < 3; i++ {
				out = append(out, m.renderEditor(120, m.palette))
				*m = paletteKey(t, *m, tea.KeyMsg{Type: tea.KeyShiftTab})
			}
			return strings.Join(out, "\n")
		}, []string{"Smart", "Plan", "Read-only"}, []string{"智能", "计划", "只读"}},
		{"shortcuts", func(m *Model, d *testDriver) string {
			return m.renderShortcutsDialog(m.layout(), m.palette)
		}, []string{"Shortcuts", "Switch mode", "ctrl+s", "Sessions"}, []string{"快捷方式", "切换模式", "ctrl+s", "会话"}},
		{"help", func(m *Model, d *testDriver) string {
			*m, _ = m.dispatchCommand(&command.Invocation{Name: "help"})
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"Commands", "/help", "View commands", "Input prefixes"}, []string{"命令", "/help", "查看命令", "输入前缀"}},
		{"palette", func(m *Model, d *testDriver) string {
			m.commandPaletteFilter = "help"
			return m.renderCommandPalette(m.layout(), m.palette)
		}, []string{"Help", "Filter: help", "/help", "View commands", "enter fill"}, []string{"帮助", "筛选：help", "/help", "查看命令", "enter 填入"}},
		{"palette loading", func(m *Model, d *testDriver) string {
			m.commandCatalogLoading = true
			return m.renderCommandPalette(m.layout(), m.palette)
		}, []string{"Refreshing dynamic commands"}, []string{"正在刷新动态命令"}},
		{"palette empty", func(m *Model, d *testDriver) string {
			m.commandPaletteFilter = "zzzz-no-command"
			m.commandCatalogError = "server 原样 error"
			return m.renderCommandPalette(m.layout(), m.palette)
		}, []string{"No matching commands", "Dynamic refresh failed", "server 原样 error"}, []string{"没有匹配的命令", "动态刷新失败", "server 原样 error"}},
		{"sessions", func(m *Model, d *testDriver) string {
			m.sessionRows = d.sessions
			return m.renderSessionsDialog(m.layout(), m.palette)
		}, []string{"Sessions", "Filter by title", "session-原样", "^r rename"}, []string{"会话", "按标题筛选", "session-原样", "^r 重命名"}},
		{"sessions loading", func(m *Model, d *testDriver) string {
			m.sessionLoading = true
			return m.renderSessionsDialog(m.layout(), m.palette)
		}, []string{"Loading sessions"}, []string{"正在加载会话"}},
		{"sessions empty", func(m *Model, d *testDriver) string {
			m.sessionFilter = "no-match"
			return m.renderSessionsDialog(m.layout(), m.palette)
		}, []string{"Filter: no-match", "No matching sessions"}, []string{"筛选：no-match", "没有匹配的会话"}},
		{"rename", func(m *Model, d *testDriver) string {
			m.sessionRenaming = true
			m.sessionRenameInput = "rename-原样"
			return m.renderSessionsDialog(m.layout(), m.palette)
		}, []string{"Rename: rename-原样", "enter confirm"}, []string{"重命名：rename-原样", "enter 确认"}},
		{"delete", func(m *Model, d *testDriver) string {
			m.sessionRows = d.sessions
			m.sessionDeleteID = "s1"
			return m.renderSessionsDialog(m.layout(), m.palette)
		}, []string{"Delete session-原样?", "y delete"}, []string{"删除 session-原样？", "y 删除"}},
		{"fork confirmation", func(m *Model, d *testDriver) string {
			*m, _ = m.confirmCommand("fork", []string{"msg-原样"})
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"Confirm /fork", "Fork at message msg-原样?", "n/esc cancel"}, []string{"确认 /fork", "在消息 msg-原样 处分叉？", "n/esc 取消"}},
		{"rewind confirmation", func(m *Model, d *testDriver) string {
			*m, _ = m.confirmCommand("rewind", []string{"msg-原样"})
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"Rewind to message msg-原样?"}, []string{"回退到消息 msg-原样？"}},
		{"compact confirmation", func(m *Model, d *testDriver) string {
			*m, _ = m.confirmCommand("compact", nil)
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"Compact current session context?"}, []string{"压缩当前会话上下文？"}},
		{"history loading", func(m *Model, d *testDriver) string {
			d.meta.Loading = true
			return strings.Join(m.chatLines(120, m.palette), "\n")
		}, []string{"Loading session history"}, []string{"正在加载会话历史"}},
		{"empty hero", func(m *Model, d *testDriver) string {
			return strings.Join(m.chatLines(120, m.palette), "\n")
		}, []string{"A journey to find a true heart", "/ commands", "ctrl+s sessions"}, []string{"寻找真心之旅", "/ 命令", "ctrl+s 会话"}},
		{"scroll bottom", func(m *Model, d *testDriver) string {
			return m.renderChromeRow(200, m.palette, chatScrollInfo{offset: 2, maxScroll: 12, viewport: 10})
		}, []string{"↓ end", "Back to bottom"}, []string{"↓ end", "回到底部"}},
		{"scroll history", func(m *Model, d *testDriver) string {
			return m.renderChromeRow(200, m.palette, chatScrollInfo{maxScroll: 79, viewport: 10})
		}, []string{"History", "79 lines below"}, []string{"历史", "下方还有 79 行"}},
		{"paste guard", func(m *Model, d *testDriver) string {
			m.input = strings.Repeat("界", 2001)
			return m.renderEditor(160, m.palette)
		}, []string{"Large paste", "lines: 1 / chars: 2001", "enter"}, []string{"大段粘贴", "1 行 / 2001 字符", "enter"}},
		{"models empty", func(m *Model, d *testDriver) string {
			return m.renderModelDialog(m.layout(), m.palette)
		}, []string{"Switch global model", "No matching configured models"}, []string{"切换全局模型", "没有匹配的已配置模型"}},
		{"files empty", func(m *Model, d *testDriver) string {
			return m.renderFileCompletion(m.layout(), m.palette)
		}, []string{"Project files", "No matching project files"}, []string{"项目文件", "没有匹配的项目文件"}},
		{"files loading", func(m *Model, d *testDriver) string {
			m.fileCompletionQuery = "src/file.go"
			m.fileCompletionLoading = true
			return m.renderFileCompletion(m.layout(), m.palette)
		}, []string{"@src/file.go", "loading…"}, []string{"@src/file.go", "加载中…"}},
		{"files partial", func(m *Model, d *testDriver) string {
			m.fileCompletionTruncated = true
			return m.renderFileCompletion(m.layout(), m.palette)
		}, []string{"partial results"}, []string{"部分结果"}},
		{"arguments", func(m *Model, d *testDriver) string {
			m.dynamicArgumentCommand = &surface.DynamicCommand{Name: "review", Description: "description-原样", Arguments: []surface.DynamicCommandArgument{{Name: "focus", Required: true}, {Name: "hint"}}}
			m.dynamicArgumentValues = []string{"", ""}
			*m, _ = m.handleDynamicArgumentKey(tea.KeyMsg{Type: tea.KeyEnter})
			return m.renderDynamicArguments(m.layout(), m.palette)
		}, []string{"/review arguments", "description-原样", "focus (required)", "hint (optional)", "focus is required"}, []string{"/review 参数", "description-原样", "focus (必填)", "hint (可选)", "focus 为必填"}},
		{"busy and queued", func(m *Model, d *testDriver) string {
			d.busy = true
			d.meta.Queued = 3
			return m.renderInputChrome(200, m.palette)
		}, []string{"run", "3 queued"}, []string{"运行", "3 排队中"}},
		{"question", func(m *Model, d *testDriver) string {
			return m.renderGateDialog(&surface.Gate{Kind: "question", Title: "question-原样", Body: "body-原样"}, m.layout(), m.palette)
		}, []string{"question", "type answer · enter submit", "body-原样"}, []string{"问题", "输入回答 · enter 提交", "body-原样"}},
		{"submitting", func(m *Model, d *testDriver) string {
			return m.renderGateDialog(&surface.Gate{Kind: "approval", Submitting: true, Body: "body-原样"}, m.layout(), m.palette)
		}, []string{"submitting…"}, []string{"提交中…"}},
		{"gate", func(m *Model, d *testDriver) string {
			return m.renderGateDialog(&surface.Gate{Kind: "approval", Title: "gate-原样", Body: "body-原样", Action: "write_file", Target: "file-原样.go"}, m.layout(), m.palette)
		}, []string{"permission", "gate-原样", "body-原样", "action: write_file", "target: file-原样.go", "enter/y approve"}, []string{"权限", "gate-原样", "body-原样", "操作：write_file", "目标：file-原样.go", "enter/y 批准"}},
		{"shell unavailable", func(m *Model, d *testDriver) string {
			m.input = "!echo hello"
			*m, _ = m.submitInput()
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"! shell commands are unavailable"}, []string{"! shell 命令不可用"}},
		{"file prompt required", func(m *Model, d *testDriver) string {
			m.input = "@README.md"
			*m, _ = m.submitInput()
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"@file references require a prompt"}, []string{"@file 引用需要提示文本"}},
		{"redacted target", func(m *Model, d *testDriver) string {
			return m.renderGateDialog(&surface.Gate{Kind: "approval", Target: "/secret/path", Body: "preview"}, m.layout(), m.palette)
		}, []string{"[redacted target]"}, []string{"[目标已隐藏]"}},
		{"sidebar empty states", func(m *Model, d *testDriver) string {
			d.sidebar.ModifiedFilesKnown = true
			d.sidebar.LSPKnown = true
			d.sidebar.MCPKnown = true
			d.sidebar.SkillsKnown = true
			return strings.Join(m.sidebarLines(120, m.palette), "\n")
		}, []string{"Modified Files", "None initialized", "None configured", "Skills · enabled"}, []string{"已修改文件", "尚未初始化", "尚未配置", "技能 · 已启用"}},
		{"status", func(m *Model, d *testDriver) string {
			d.meta.Queued = 3
			d.meta.Error = "server-原样"
			return m.statusText()
		}, []string{"session: session-原样", "id: s1", "queue: 3", "permission: smart", "server-原样"}, []string{"会话：session-原样", "标识：s1", "队列：3", "权限档：smart", "server-原样"}},
		{"reasoning collapsed", func(m *Model, d *testDriver) string {
			d.messages = map[string][]surface.Message{"s1": {{ID: "r1", Role: surface.RoleAssistant, Reasoning: true, Content: "private thought"}}}
			m.reasoningCollapsed = true
			return strings.Join(m.chatLines(120, m.palette), "\n")
		}, []string{"reasoning", "ctrl+r expand"}, []string{"思考过程", "ctrl+r 展开"}},
		{"tool omission", func(m *Model, d *testDriver) string {
			d.messages = map[string][]surface.Message{"s1": {{ID: "t1", Tool: &surface.ToolCard{ToolName: "tool-原样", Status: "done", Result: strings.Repeat("data\n", 20)}}}}
			return strings.Join(m.chatLines(120, m.palette), "\n")
		}, []string{"tool-原样", "more lines", "ctrl+o expand"}, []string{"tool-原样", "还有", "ctrl+o 展开"}},
		{"server error", func(m *Model, d *testDriver) string {
			*m = m.showCommandError(errors.New("server 原样 error"))
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"Command error", "server 原样 error"}, []string{"命令错误", "server 原样 error"}},
		{"local error", func(m *Model, d *testDriver) string {
			*m, _ = m.dispatchCommand(&command.Invocation{Name: "unknown-原样"})
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"unknown command /unknown-原样"}, []string{"未知命令 /unknown-原样"}},
		{"content and attachments", func(m *Model, d *testDriver) string {
			d.messages = map[string][]surface.Message{"s1": {
				{ID: "u1", Role: surface.RoleUser, Content: "user-原样", Attachments: []surface.Attachment{{Name: "photo-原样.png"}}},
				{ID: "a1", Role: surface.RoleAssistant, Content: "model-output-原样", FileContexts: []surface.FileContext{{Name: "file-原样.go"}}},
				{ID: "t1", Tool: &surface.ToolCard{ToolName: "tool-原样", Status: "done", Result: "tool-output-原样"}},
			}}
			return strings.Join(m.chatLines(120, m.palette), "\n")
		}, []string{"user-原样", "model-output-原样", "tool-output-原样", "[image: photo-原样.png]", "[file: file-原样.go]"}, []string{"user-原样", "model-output-原样", "tool-output-原样", "[图片：photo-原样.png]", "[文件：file-原样.go]"}},
	}
	for _, locale := range []corei18n.Locale{corei18n.English, corei18n.Chinese} {
		for _, tc := range cases {
			t.Run(string(locale)+"/"+tc.name, func(t *testing.T) {
				d := &testDriver{
					sessions: []surface.Session{{ID: "s1", Title: "session-原样", PermissionPreset: "smart"}},
					active:   "s1",
					sidebar:  surface.Sidebar{Provider: "provider-原样", Model: "model-原样"},
				}
				m := New(d, Options{Locale: locale})
				got := ansi.Strip(tc.render(&m, d))
				wants := tc.en
				if locale == corei18n.Chinese {
					wants = tc.zh
				}
				for _, want := range wants {
					if !strings.Contains(got, want) {
						t.Errorf("missing %q:\n%s", want, got)
					}
				}
				if strings.Contains(got, "vivy.tui.") || strings.Contains(got, "{{") {
					t.Errorf("unresolved translation:\n%s", got)
				}
			})
		}
	}
}

func TestLocaleChromeWidths(t *testing.T) {
	for _, locale := range []corei18n.Locale{corei18n.English, corei18n.Chinese} {
		t.Run(string(locale), func(t *testing.T) {
			m := New(&testDriver{}, Options{Locale: locale})
			for _, width := range []int{8, 12, 20, 40, 80, 120} {
				for _, content := range []string{m.renderEditor(width, m.palette), m.renderInputChrome(width, m.palette)} {
					for _, line := range strings.Split(content, "\n") {
						if got := lipgloss.Width(line); got > width {
							t.Errorf("width %d overflowed to %d: %q", width, got, line)
						}
					}
				}
			}
			m.width, m.height = 32, 10
			m.commandPaletteOpen = true
			got := ansi.Strip(m.View())
			want := "Help"
			if locale == corei18n.Chinese {
				want = "帮助"
			}
			for _, text := range []string{want, "/help", "enter", "esc"} {
				if !strings.Contains(got, text) {
					t.Errorf("compact palette missing %q:\n%s", text, got)
				}
			}
			if len(strings.Split(got, "\n")) != 10 {
				t.Errorf("compact palette height changed:\n%s", got)
			}
		})
	}
}

func TestLocaleModelDialogNarrowAndWide(t *testing.T) {
	for _, locale := range []corei18n.Locale{corei18n.English, corei18n.Chinese} {
		for _, size := range []struct {
			width, height, outer, inner int
		}{{30, 10, 24, 16}, {120, 36, 74, 66}} {
			for _, mode := range []string{"writable", "read-only", "frozen"} {
				t.Run(fmt.Sprintf("%s/%d/%s", locale, size.width, mode), func(t *testing.T) {
					d := modelPickerFixture()
					d.catalog.Options = nil
					d.catalog.ReadOnly = mode == "read-only"
					d.catalog.Frozen = mode == "frozen"
					m := New(d, Options{Locale: locale})
					m.width, m.height = size.width, size.height
					m.modelPickerOpen = true
					raw := m.renderModelDialog(m.layout(), m.palette)
					wants := []string{"Switch global", "No matching", "↑↓ select", "esc"}
					if locale == corei18n.Chinese {
						wants = []string{"切换全局模型", "没有匹配", "↑↓ 选择", "esc"}
					}
					if mode != "writable" {
						wants[2] = "↑↓ browse"
						if locale == corei18n.Chinese {
							wants[2] = "↑↓ 浏览"
						}
					}
					if size.width == 120 {
						wants[0], wants[1] = "Switch global model", "No matching configured models"
						wants = append(wants, "esc close")
						if locale == corei18n.Chinese {
							wants[0], wants[1] = "切换全局模型", "没有匹配的已配置模型"
							wants[4] = "esc 关闭"
						}
						context := "Global · next idle turn"
						if mode != "writable" {
							context = "Read-only"
						}
						if locale == corei18n.Chinese {
							context = "全局 · 下一空闲回合生效"
							if mode != "writable" {
								context = "只读"
							}
						}
						wants = append(wants, context)
					}
					assertLocaleDialogVisible(t, m, raw, size.outer, wants)
					// Inspect pre-overlay rows: clipping must not make an oversized
					// title/empty state look like a correctly bounded dialog.
					rows := strings.Split(ansi.Strip(raw), "\n")
					if len(rows) != 10 {
						t.Errorf("empty dialog wrapped to %d rows, want 10:\n%s", len(rows), raw)
					}
					for _, row := range rows[1 : len(rows)-1] {
						body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(row, "│"), "│"))
						if got := lipgloss.Width(body); got > size.inner {
							t.Errorf("body width %d exceeds inner budget %d: %q", got, size.inner, body)
						}
					}
				})
			}
		}
	}
}

func TestLocaleDialogAndStateNarrowAndWide(t *testing.T) {
	cases := []struct {
		name   string
		dialog bool
		render func(*Model, *testDriver) string
		en, zh []string
	}{
		{"sessions", true, func(m *Model, d *testDriver) string {
			m.sessionsOpen, m.sessionRows = true, d.sessions
			return m.renderSessionsDialog(m.layout(), m.palette)
		}, []string{"Sessions", "Filter by title", "会话-A", "↑/↓ move", "enter/tab select", "^r rename", "^x", "delete"}, []string{"会话", "按标题筛选", "会话-A", "↑/↓ 移动", "enter/tab 选择", "^r 重命名", "^x", "删除"}},
		{"confirmation", true, func(m *Model, d *testDriver) string {
			*m, _ = m.confirmCommand("fork", []string{"msg-1"})
			return m.renderCommandDialog(m.layout(), m.palette)
		}, []string{"Confirm /fork", "Fork at message msg-1?", "y confirm", "n/esc cancel"}, []string{"确认 /fork", "在消息 msg-1 处分叉？", "y 确认", "n/esc 取消"}},
		{"loading dialog", true, func(m *Model, d *testDriver) string {
			m.sessionsOpen, m.sessionLoading = true, true
			return m.renderSessionsDialog(m.layout(), m.palette)
		}, []string{"Sessions", "Loading sessions", "↑/↓ move", "enter/tab select", "^r rename", "^x", "delete"}, []string{"会话", "正在加载会话", "↑/↓ 移动", "enter/tab 选择", "^r 重命名", "^x", "删除"}},
		{"loading state", false, func(m *Model, d *testDriver) string {
			d.meta.Loading = true
			return strings.Join(m.chatLines(m.layout().mainW(), m.palette), "\n")
		}, []string{"Loading session history"}, []string{"正在加载会话历史"}},
		{"paste warning", false, func(m *Model, d *testDriver) string {
			m.input = strings.Repeat("界", 2001)
			return m.renderEditor(m.layout().mainW(), m.palette)
		}, []string{"Large paste", "lines: 1 / chars: 2001", "check enter"}, []string{"大段粘贴", "1 行 / 2001 字符", "enter"}},
		{"parked scroll", false, func(m *Model, d *testDriver) string {
			d.messages = map[string][]surface.Message{"s1": {}}
			for i := 0; i < 60; i++ {
				d.messages["s1"] = append(d.messages["s1"], surface.Message{ID: fmt.Sprint(i), Role: surface.RoleUser, Content: fmt.Sprintf("history line %d", i)})
			}
			m.chatFollow, m.chatScroll = false, m.chatMaxScroll()-10
			l := m.layout()
			_, scroll := m.renderChat(l.mainW(), l.mainH(), m.palette)
			return m.renderChromeRow(l.mainW(), m.palette, scroll)
		}, []string{"↓ end", "Back to bottom", "shift+tab", "Switch mode"}, []string{"↓ end", "回到底部", "shift+tab", "切换模式"}},
	}
	for _, locale := range []corei18n.Locale{corei18n.English, corei18n.Chinese} {
		for _, size := range []struct{ width, height, dialogWidth, mainWidth int }{{60, 24, 54, 58}, {120, 36, 74, 85}} {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/%d/%s", locale, size.width, tc.name), func(t *testing.T) {
					d := &testDriver{active: "s1", sessions: []surface.Session{{ID: "s1", Title: "会话-A"}}}
					m := New(d, Options{Locale: locale})
					m.width, m.height = size.width, size.height
					raw := tc.render(&m, d)
					wants := tc.en
					if locale == corei18n.Chinese {
						wants = tc.zh
					}
					if tc.dialog {
						assertLocaleDialogVisible(t, m, raw, size.dialogWidth, wants)
						return
					}
					assertLocaleCellWidths(t, raw, size.mainWidth)
					assertLocaleCopy(t, raw, wants)
					frame := m.View()
					assertLocaleCellWidths(t, frame, size.width)
					assertLocaleCopy(t, frame, wants)
					assertLocaleCopy(t, frame, []string{"╭", "╮", "╰", "╯"})
					if tc.name != "parked scroll" {
						help := "Help"
						if locale == corei18n.Chinese {
							help = "帮助"
						}
						assertLocaleCopy(t, frame, []string{"shift+h", help})
					}
					if tc.name == "paste warning" {
						assertLocaleCopy(t, raw, []string{"╭", "╮", "╰", "╯"})
						if m.input != strings.Repeat("界", 2001) {
							t.Error("rendering changed the pasted draft")
						}
					}
				})
			}
		}
	}
}

func assertLocaleCopy(t *testing.T, content string, wants []string) {
	t.Helper()
	plain := ansi.Strip(content)
	for _, want := range wants {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q:\n%s", want, plain)
		}
	}
}

func assertLocaleCellWidths(t *testing.T, content string, width int) {
	t.Helper()
	for _, row := range strings.Split(content, "\n") {
		if got := lipgloss.Width(row); got > width {
			t.Errorf("row width %d exceeds %d cells: %q", got, width, row)
		}
	}
}

func assertLocaleDialogVisible(t *testing.T, m Model, raw string, outerWidth int, wants []string) {
	t.Helper()
	assertLocaleCellWidths(t, raw, outerWidth)
	assertLocaleCopy(t, raw, wants)
	assertLocaleCopy(t, raw, []string{"╭", "╮", "╰", "╯"})
	rows := strings.Split(ansi.Strip(raw), "\n")
	if len(rows) > m.height {
		t.Errorf("raw dialog height %d exceeds frame height %d", len(rows), m.height)
	}
	frame := m.View()
	assertLocaleCellWidths(t, frame, m.width)
	assertLocaleCopy(t, frame, wants)
	// Every raw border/body row must survive overlay placement unchanged.
	// A final width/height assertion alone would pass even if placeOverlay
	// clipped the footer or the bottom border out of the frame.
	for _, row := range rows {
		if got := lipgloss.Width(row); got != outerWidth {
			t.Errorf("dialog row width = %d, want %d: %q", got, outerWidth, row)
		}
		assertLocaleCopy(t, frame, []string{strings.TrimSpace(row)})
	}
}

func TestLocaleDefaultAndIndependentViews(t *testing.T) {
	d := &testDriver{}
	en := New(d)
	zh := New(d, Options{Locale: corei18n.Chinese})
	fallback := New(d, Options{Locale: corei18n.Locale("unsupported")})
	for _, m := range []Model{en, zh, fallback, en} {
		want := "Ask something"
		if m.translator.Locale() == corei18n.Chinese {
			want = "问点什么"
		}
		if got := ansi.Strip(m.renderEditor(100, m.palette)); !strings.Contains(got, want) {
			t.Fatalf("locale isolation/default missing %q: %s", want, got)
		}
	}
	if en.translator.Locale() != corei18n.English {
		t.Fatalf("default locale = %q, want en", en.translator.Locale())
	}
}

func TestWideLayoutKeepsSidebarWhenMarkdownIsWide(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		messages: map[string][]surface.Message{
			"active": {{
				Role:    surface.RoleAssistant,
				Content: "# Title\n\n" + strings.Repeat("汉字宽行内容与表格 ", 24) + "\n\n| a | b | c | d |\n| --- | --- | --- | --- |\n| 1 | 2 | 3 | 4 |\n\n```go\n" + strings.Repeat("fmt.Println(\"hello world from a long fenced block\")\n", 4) + "```\n",
			}},
		},
	}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = next.(Model)
	view := m.View()
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "VIVY CODE") || !strings.Contains(plain, "Current") {
		t.Fatalf("sidebar missing beside wide markdown:\n%s", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 120 {
			t.Fatalf("line %d width %d exceeds terminal:\n%q", i, got, line)
		}
	}
}

func TestWrapTextPreservesStreamingTextAndWrapsCJK(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{name: "continuous Chinese", text: "这是一句话", width: 20, want: []string{"这是一句话"}},
		{name: "CJK cell wrap", text: "这是一句话", width: 8, want: []string{"这是一句", "话"}},
		{name: "exact spaces", text: "a  b c ", width: 20, want: []string{"a  b c "}},
		{name: "safe controls", text: "a\tb\r\x1b[31mc", width: 20, want: []string{"a    bc"}},
		{name: "newlines", text: "甲\n\n乙", width: 8, want: []string{"甲", "", "乙"}},
		{name: "emoji grapheme", text: "AAAAAAA👨‍👩‍👧‍👦B", width: 8, want: []string{"AAAAAAA", "👨‍👩‍👧‍👦B"}},
		{name: "Latin words", text: "hello world", width: 8, want: []string{"hello ", "world"}},
		{name: "boundary space", text: "12345678 x", width: 8, want: []string{"12345678", " x"}},
		{name: "wide whitespace", text: "         ", width: 8, want: []string{"        ", " "}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := wrapText(test.text, test.width)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("wrapText(%q, %d) = %#v, want %#v", test.text, test.width, got, test.want)
			}
		})
	}
}

func TestRenderMessageTrimsTrailingMarkdownNewline(t *testing.T) {
	lines := (Model{}).renderMessage(surface.Message{Role: surface.RoleAssistant, Content: "line\n"}, 40, DefaultPalette())
	if len(lines) != 1 || !strings.Contains(ansi.Strip(lines[0]), "line") {
		t.Fatalf("rendered lines = %#v, want a single trimmed markdown line", lines)
	}
}

func TestTruncateAndBackspacePreserveGraphemeAndANSIIntegrity(t *testing.T) {
	styled := "\x1b[31m" + "e\u0301👨‍👩‍👧‍👦界" + "\x1b[0m"
	got := truncate(styled, 4)
	if lipgloss.Width(got) > 4 || !strings.Contains(got, "e\u0301") || strings.Contains(got, "界") {
		t.Fatalf("ANSI/grapheme truncate = %q width=%d", got, lipgloss.Width(got))
	}
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: "ae\u0301", want: "a"},
		{input: "a👨‍👩‍👧‍👦", want: "a"},
		{input: "a🇨🇳", want: "a"},
		{input: "a👍🏽", want: "a"},
	} {
		if got := removeLastRune(test.input); got != test.want {
			t.Fatalf("removeLastRune(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestRenderToolSanitizesAndFitsEveryViewport(t *testing.T) {
	tool := &surface.ToolCard{
		ToolName: "\x1b]2;PWN\aedit\rfile",
		Status:   "done",
		Result:   "@@ -1 +1 @@\n-old\tvalue\n+new 👨‍👩‍👧‍👦 value\x1b]2;BODY-PWN\a\u202evisual\u2066",
	}
	for _, width := range []int{1, 3, 4, 7, 8, 9, 20, 56, 80} {
		for _, line := range (Model{}).renderTool(tool, width, DefaultPalette()) {
			if got := lipgloss.Width(line); got > max(1, width) {
				t.Fatalf("width %d rendered line width %d: %q", width, got, line)
			}
			plain := ansi.Strip(line)
			if strings.Contains(plain, "PWN") || strings.ContainsAny(plain, "\r\a") {
				t.Fatalf("width %d retained terminal control payload: %q", width, plain)
			}
			if strings.ContainsAny(plain, "\u202e\u2066") {
				t.Fatalf("width %d retained bidi controls: %q", width, plain)
			}
		}
	}
}

func TestRenderToolCompactsResultUnlessDebugEnabled(t *testing.T) {
	tool := &surface.ToolCard{ToolName: "list_dir", Status: "done", Result: strings.Repeat("entry\n", 20)}

	compact := strings.Join((Model{}).renderToolWithOptions(tool, 80, DefaultPalette(), false), "\n")
	if !strings.Contains(compact, "more lines · ctrl+o expand") {
		t.Fatalf("compact tool result has no omission marker: %s", ansi.Strip(compact))
	}
	if strings.Count(ansi.Strip(compact), "entry") >= 20 {
		t.Fatalf("compact tool result rendered every line: %s", ansi.Strip(compact))
	}

	debug := strings.Join((Model{}).renderToolWithOptions(tool, 80, DefaultPalette(), true), "\n")
	if strings.Contains(debug, "more lines") {
		t.Fatalf("debug tool result was compacted: %s", ansi.Strip(debug))
	}
	if strings.Count(ansi.Strip(debug), "entry") != 20 {
		t.Fatalf("debug tool result did not render every line: %s", ansi.Strip(debug))
	}
}

func TestRenderMessageFitsNarrowViewportAndDropsBidiControls(t *testing.T) {
	message := surface.Message{Role: surface.RoleAssistant, Content: "你e\u0301👨‍👩‍👧‍👦\u202eabc\u2066", Streaming: true}
	for width := 1; width <= 12; width++ {
		lines := (Model{}).renderMessage(message, width, DefaultPalette())
		if len(lines) == 0 {
			t.Fatalf("width %d rendered no message lines", width)
		}
		visible := false
		for _, line := range lines {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d rendered line width %d: %q", width, got, line)
			}
			plain := ansi.Strip(line)
			visible = visible || plain != ""
			if strings.ContainsAny(plain, "\u202e\u2066") {
				t.Fatalf("width %d retained bidi controls: %q", width, plain)
			}
		}
		if !visible {
			t.Fatalf("width %d silently erased the message", width)
		}
	}
}

const approvalDiffFixture = "--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-old value\n+new value\n same"

func TestApprovalDiffUsesAuthoritativePreviewAndResponsiveModes(t *testing.T) {
	driver := &testDriver{gate: &surface.Gate{
		Kind: "approval", ID: "gate-1", Title: "write", Body: "args must-not-render-as-diff",
		Action: "write", Target: "a.go", Preview: approvalDiffFixture,
	}}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 36})
	m = next.(Model)
	wide := ansi.Strip(m.renderGateDialog(driver.gate, computeLayout(180, 36), DefaultPalette()))
	if !strings.Contains(wide, "split") || !strings.Contains(wide, "old value") || !strings.Contains(wide, "new value") || strings.Contains(wide, "args must-not-render-as-diff") {
		t.Fatalf("wide approval diff = %q", wide)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = next.(Model)
	if !m.gateUnified {
		t.Fatal("t did not switch approval diff to unified")
	}
	unified := ansi.Strip(m.renderGateDialog(driver.gate, computeLayout(180, 36), DefaultPalette()))
	if !strings.Contains(unified, "unified") || !strings.Contains(unified, "-old value") || !strings.Contains(unified, "+new value") {
		t.Fatalf("unified approval diff = %q", unified)
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = next.(Model)
	if m.gateUsesSplit(driver.gate, computeLayout(80, 30)) {
		t.Fatal("explicit unified selection was lost across resize")
	}

	narrowModel := New(driver)
	next, _ = narrowModel.Update(tea.WindowSizeMsg{Width: 70, Height: 18})
	narrowModel = next.(Model)
	narrow := narrowModel.View()
	if strings.Contains(ansi.Strip(narrow), "split ·") {
		t.Fatalf("narrow viewport offered split mode: %q", ansi.Strip(narrow))
	}
	for _, line := range strings.Split(narrow, "\n") {
		if lipgloss.Width(line) > 70 {
			t.Fatalf("narrow line overflow width=%d: %q", lipgloss.Width(line), line)
		}
	}
	next, _ = narrowModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	narrowModel = next.(Model)
	if !strings.Contains(ansi.Strip(narrowModel.renderGateDialog(driver.gate, computeLayout(70, 18), DefaultPalette())), "split ·") {
		t.Fatal("t was a no-op below the default split threshold")
	}
}

func TestApprovalDiffScrollsResetsAndKeepsDecisionRouting(t *testing.T) {
	var diff strings.Builder
	diff.WriteString("--- a/a.go\n+++ b/a.go\n@@ -1,40 +1,40 @@\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&diff, "-old %02d\n+new %02d\n", i, i)
	}
	driver := &testDriver{gate: &surface.Gate{Kind: "approval", ID: "gate-1", Title: "write", Action: "patch", Preview: diff.String()}}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = next.(Model)
	if m.gateScroll == 0 {
		t.Fatal("page down did not scroll approval diff")
	}
	beforeWheel := m.gateScroll
	next, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	m = next.(Model)
	if m.gateScroll <= beforeWheel {
		t.Fatal("mouse wheel did not stay with foreground approval diff")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = next.(Model)
	if !m.gateFullscreen {
		t.Fatal("f did not toggle fullscreen")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = next.(Model)
	if driver.decision != approvalApproved {
		t.Fatalf("approval decision = %q", driver.decision)
	}
	driver.gate = &surface.Gate{Kind: "approval", ID: "gate-2", Title: "write", Action: "patch", Preview: approvalDiffFixture}
	next, _ = m.Update(surface.ErrMsg{})
	m = next.(Model)
	if m.gateScroll != 0 || m.gateUnified || m.gateViewExplicit || m.gateFullscreen || m.gateID != "gate-2" {
		t.Fatalf("new gate retained old view state: %+v", m)
	}
}

func TestApprovalDiffSanitizesTerminalControlsAndFallsBackWithoutDiff(t *testing.T) {
	gate := &surface.Gate{
		Kind: "approval", ID: "safe", Title: "write\x1b]2;TITLE-PWN\a", Action: "write\r", Target: "C:\\secret\\a.go\u202e",
		Preview: "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\x1b]2;BODY-PWN\a\n+new\u2066", Risks: []string{"risk\x1b]2;RISK-PWN\a"},
	}
	m := New(&testDriver{gate: gate})
	plain := ansi.Strip(m.renderGateDialog(gate, computeLayout(120, 30), DefaultPalette()))
	if strings.Contains(plain, "PWN") || strings.ContainsAny(plain, "\r\a\u202e\u2066") {
		t.Fatalf("approval retained terminal control payload: %q", plain)
	}
	if strings.Contains(plain, "secret") || !strings.Contains(plain, "[redacted target]") {
		t.Fatalf("approval exposed an absolute target: %q", plain)
	}
	gate.Preview = "not a unified diff"
	gate.Body = "plain approval body"
	plain = ansi.Strip(m.renderGateDialog(gate, computeLayout(120, 30), DefaultPalette()))
	if !strings.Contains(plain, "not a unified diff") || strings.Contains(plain, "plain approval body") || strings.Contains(plain, "t view") {
		t.Fatalf("non-diff approval did not prefer the authoritative preview: %q", plain)
	}
}

func TestNonMutationCannotSpoofApprovalDiffControls(t *testing.T) {
	gate := &surface.Gate{Kind: "approval", ID: "remote", Title: "mcp", Action: "mcp_call", Body: "remote request", Preview: approvalDiffFixture}
	m := New(&testDriver{gate: gate})
	plain := ansi.Strip(m.renderGateDialog(gate, computeLayout(160, 36), DefaultPalette()))
	if strings.Contains(plain, "t view") || strings.Contains(plain, "split ·") || !strings.Contains(plain, "--- a/a.go") || strings.Contains(plain, "remote request") {
		t.Fatalf("non-mutation preview spoofed a file diff: %q", plain)
	}
}

func TestApprovalDiffCrushDecisionKeysAndBoundedHorizontalScroll(t *testing.T) {
	gate := &surface.Gate{Kind: "approval", ID: "keys", Title: "patch", Action: "patch", Preview: approvalDiffFixture + strings.Repeat("x", 300)}
	driver := &testDriver{gate: gate}
	m := New(driver)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	m = next.(Model)
	for i := 0; i < 200; i++ {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
		m = next.(Model)
	}
	if m.gateHorizontal <= 0 || m.gateHorizontal > m.gateMaxHorizontal() {
		t.Fatalf("horizontal offset was not bounded: %d max=%d", m.gateHorizontal, m.gateMaxHorizontal())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if driver.decision != approvalDenied {
		t.Fatalf("Esc decision = %q", driver.decision)
	}
	driver.decision = ""
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if driver.decision != approvalApproved {
		t.Fatalf("Enter decision = %q", driver.decision)
	}
}

func TestSplitDiffDistinguishesHeadersFromContentAndShortAxisForcesFullscreen(t *testing.T) {
	preview := "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n---literal old\n+++literal new"
	rows := parseSplitDiffRows(strings.Split(preview, "\n"))
	if len(rows) != 2 || rows[0].kind != '@' || rows[1].left != "---literal old" || rows[1].right != "+++literal new" || rows[1].oldNo != 1 || rows[1].newNo != 1 {
		t.Fatalf("split rows confused file headers with content: %+v", rows)
	}
	if adds, dels := visibleDiffStats(strings.Split(preview, "\n")); adds != 1 || dels != 1 {
		t.Fatalf("visible stats = +%d -%d", adds, dels)
	}
	gate := &surface.Gate{Kind: "approval", ID: "short", Action: "patch", Preview: preview}
	m := New(&testDriver{gate: gate})
	for _, size := range []struct{ w, h int }{{160, 20}, {77, 40}} {
		l := computeLayout(size.w, size.h)
		if got := m.gateDialogWidth(gate, l); got != size.w-2 {
			t.Fatalf("%dx%d dialog width = %d, want forced fullscreen %d", size.w, size.h, got, size.w-2)
		}
		if size.w == 160 && !m.gateUsesSplit(gate, l) {
			t.Fatal("forced fullscreen width did not drive the default split mode")
		}
	}
	gate.Target = "src/main.go"
	gate.PreconditionHash = strings.Repeat("a", 64)
	gate.Risks = []string{"one", "two", "three", "four", "five"}
	dialog := m.renderGateDialog(gate, computeLayout(160, 21), DefaultPalette())
	if lines := len(strings.Split(dialog, "\n")); lines > 21 {
		t.Fatalf("short dialog used %d terminal rows", lines)
	}
	if !strings.Contains(ansi.Strip(dialog), "approve") {
		t.Fatalf("short dialog cropped decision help: %q", ansi.Strip(dialog))
	}
}

func TestToolDiffDistinguishesHeadersFromContent(t *testing.T) {
	preview := "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n---literal old\n+++literal new"
	rendered := renderDiffBody(preview, DefaultPalette())
	if !strings.Contains(ansi.Strip(rendered), "+1 -1") {
		t.Fatalf("tool diff rendered wrong stat header: %q", ansi.Strip(rendered))
	}
	if adds, dels := visibleDiffStats(strings.Split(preview, "\n")); adds != 1 || dels != 1 {
		t.Fatalf("tool diff stats confused content with headers: +%d -%d", adds, dels)
	}
}

func TestApprovalTargetKeepsNestedWorkspaceRelativePath(t *testing.T) {
	if got := (Model{}).sanitizeApprovalTarget("src/nested/main.go"); got != "src/nested/main.go" {
		t.Fatalf("workspace-relative target = %q", got)
	}
}

type testDriver struct {
	sessions    []surface.Session
	active      string
	busy        bool
	meta        surface.Meta
	sidebar     surface.Sidebar
	selected    string
	rename      string
	deleted     string
	gate        *surface.Gate
	sendBlocked bool
	sent        string
	thinking    string
	runMode     string
	decision    string
	messages    map[string][]surface.Message
	attachments []surface.Attachment
}

func (d *testDriver) Sessions() []surface.Session {
	return append([]surface.Session(nil), d.sessions...)
}
func (d *testDriver) Active() surface.Session {
	for _, session := range d.sessions {
		if session.ID == d.active {
			return session
		}
	}
	return surface.Session{}
}
func (d *testDriver) ActiveMessages() []surface.Message {
	return append([]surface.Message(nil), d.messages[d.active]...)
}
func (d *testDriver) PendingGate() *surface.Gate { return d.gate }
func (d *testDriver) Meta() surface.Meta {
	meta := d.meta
	meta.Busy = meta.Busy || d.busy
	return meta
}
func (d *testDriver) Init() tea.Cmd { return nil }
func (d *testDriver) Handle(msg tea.Msg) tea.Cmd {
	if sessions, ok := msg.(surface.SessionsMsg); ok && sessions.Err == nil {
		if sessions.Action == "rename" {
			for i := range d.sessions {
				if d.sessions[i].ID == sessions.ID {
					d.sessions[i] = sessions.Session
				}
			}
		}
		if sessions.Action == "delete" {
			rows := d.sessions[:0]
			for _, session := range d.sessions {
				if session.ID != sessions.ID {
					rows = append(rows, session)
				}
			}
			d.sessions = rows
		}
	}
	return nil
}
func (d *testDriver) MoveSession(int) tea.Cmd   { return nil }
func (d *testDriver) NewSession(string) tea.Cmd { return nil }
func (d *testDriver) Send(text string) tea.Cmd {
	if d.sendBlocked {
		return nil
	}
	d.sent = text
	return func() tea.Msg { return surface.RefreshMsg{} }
}
func (d *testDriver) DecideApproval(decision string) tea.Cmd {
	d.decision = decision
	return nil
}
func (d *testDriver) AnswerQuestion(string) tea.Cmd { return nil }
func (d *testDriver) SetPermission(preset string) tea.Cmd {
	for i := range d.sessions {
		if d.sessions[i].ID == d.active {
			d.sessions[i].PermissionPreset = preset
		}
	}
	if d.sidebar.Session.ID == d.active || d.sidebar.Session.ID == "" {
		d.sidebar.Session.PermissionPreset = preset
	}
	return func() tea.Msg { return surface.RefreshMsg{} }
}
func (d *testDriver) ClearQueue() bool         { return false }
func (d *testDriver) Cancel() tea.Cmd          { return nil }
func (d *testDriver) Sidebar() surface.Sidebar { return d.sidebar }
func (d *testDriver) ThinkingMode() string {
	if d.thinking == "" {
		return "auto"
	}
	return d.thinking
}
func (d *testDriver) SetThinkingMode(mode string) error {
	if mode == "on" && (!d.sidebar.HasContext || !d.sidebar.Context.ThinkingSupported) {
		return fmt.Errorf("extended thinking is unavailable for the active model")
	}
	d.thinking = mode
	return nil
}
func (d *testDriver) RunMode() string {
	if d.runMode == "" {
		return "normal"
	}
	return d.runMode
}
func (d *testDriver) SetRunMode(mode string) error {
	if mode != "normal" && mode != "plan" {
		return fmt.Errorf("run mode must be normal or plan")
	}
	d.runMode = mode
	return nil
}
func (d *testDriver) RefreshSessions() tea.Cmd {
	return func() tea.Msg { return surface.SessionsMsg{Action: "list", Sessions: d.Sessions()} }
}
func (d *testDriver) SelectSession(id string) tea.Cmd {
	d.selected = id
	d.active = id
	return func() tea.Msg { return surface.RefreshMsg{} }
}
func (d *testDriver) RenameSession(id, title string) tea.Cmd {
	d.rename = id + ":" + title
	var session surface.Session
	for _, item := range d.sessions {
		if item.ID == id {
			session = item
		}
	}
	session.Title = title
	return func() tea.Msg { return surface.SessionsMsg{Action: "rename", ID: id, Session: session} }
}
func (d *testDriver) DeleteSession(id string) tea.Cmd {
	d.deleted = id
	return func() tea.Msg { return surface.SessionsMsg{Action: "delete", ID: id} }
}

func (d *testDriver) ExecuteCommand(name string, args []string) tea.Cmd {
	switch name {
	case "new":
		return d.NewSession(strings.TrimSpace(strings.Join(args, " ")))
	case "session":
		if len(args) > 0 {
			return d.SelectSession(args[0])
		}
	case "rename":
		if active := d.Active(); active.ID != "" {
			return d.RenameSession(active.ID, strings.TrimSpace(strings.Join(args, " ")))
		}
	case "cancel":
		return d.Cancel()
	case "queue":
		d.ClearQueue()
	case "permission":
		preset := ""
		if len(args) > 0 {
			preset = args[0]
		}
		return d.SetPermission(preset)
	}
	return nil
}
func (*testDriver) DynamicCommands() []surface.DynamicCommand { return nil }
func (*testDriver) RefreshDynamicCommands(uint64) tea.Cmd     { return nil }
func (*testDriver) CancelDynamicCommand(uint64)               {}
func (*testDriver) ExecuteDynamicCommand(uint64, string, string, []string) tea.Cmd {
	return nil
}
func (*testDriver) SupportsModelSelection() bool       { return false }
func (*testDriver) ModelCatalog() surface.ModelCatalog { return surface.ModelCatalog{} }
func (*testDriver) RefreshModels(uint64) tea.Cmd       { return nil }
func (*testDriver) SelectModel(uint64, surface.ModelOption) tea.Cmd {
	return nil
}
func (d *testDriver) PendingAttachments() []surface.Attachment {
	return append([]surface.Attachment(nil), d.attachments...)
}
func (*testDriver) SendWithContext(string, []string) tea.Cmd { return nil }
func (*testDriver) CompleteProjectFiles(uint64, string) tea.Cmd {
	return nil
}
func (*testDriver) ExecuteShell(string) tea.Cmd    { return nil }
func (*testDriver) SupportsCapability(string) bool { return false }

func TestComputeLayoutUsesBothCrushBreakpoints(t *testing.T) {
	for _, tc := range []struct {
		width, height int
		wide          bool
	}{
		{120, 36, true},
		{100, 30, true},
		{99, 30, false},
		{100, 29, false},
		{120, 29, false},
		{120, 30, true},
	} {
		got := computeLayout(tc.width, tc.height)
		if got.showSidebar != tc.wide {
			t.Fatalf("layout(%d,%d).showSidebar = %v, want %v", tc.width, tc.height, got.showSidebar, tc.wide)
		}
		if tc.wide && got.sidebarW != 32 {
			t.Fatalf("wide sidebar width = %d, want 32", got.sidebarW)
		}
	}
}

func TestSidebarDoesNotRenderSessionCollection(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current", PermissionPreset: "smart"}, {ID: "other", Title: "Other"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:    surface.Session{ID: "active", Title: "Current", PermissionPreset: "smart"},
			HasContext: true,
			Context: surface.Context{
				FeedTokens: 1200, ModelLimitTokens: 8000, TriggerTokens: 6400,
				ModelLimitKnown: true,
				TotalMessages:   5, FeedMessages: 3, HasCompactionSummary: true,
			},
		},
	}
	driver.meta = surface.Meta{Busy: true, RunID: "run_active", Queued: 2}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := updated.(Model).View()
	for _, want := range []string{"Current", "1.2k / 8.0k tokens", "run · run_active", "queue · 2", "3 / 5 feed messages", "compact at 6.4k", "summary available"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sidebar omitted %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Other") {
		t.Fatalf("inactive session leaked into sidebar:\n%s", view)
	}
}

func TestSidebarLSPSectionRequiresAuthoritativeOwner(t *testing.T) {
	driver := &testDriver{sessions: []surface.Session{{ID: "active", Title: "Current"}}, active: "active", sidebar: surface.Sidebar{Session: surface.Session{ID: "active", Title: "Current"}}}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	if strings.Contains(m.View(), "LSP · live") {
		t.Fatalf("unknown LSP owner rendered a status section:\n%s", m.View())
	}
	driver.sidebar.LSPKnown = true
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if !strings.Contains(m.View(), "LSP · live") || !strings.Contains(m.View(), "None initialized") {
		t.Fatalf("known idle LSP owner was hidden:\n%s", m.View())
	}
	driver.sidebar.LSP = []surface.LanguageServer{{Language: "go", State: "guessed"}}
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if strings.Contains(m.View(), "guessed") || strings.Contains(m.View(), "go · starting") || !strings.Contains(m.View(), "None initialized") {
		t.Fatalf("unknown LSP state failed open:\n%s", m.View())
	}
}

func TestSidebarRendersTruthAndScrollsIndependently(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		meta:     surface.Meta{Host: "127.0.0.1:8787"},
		sidebar: surface.Sidebar{
			Session: surface.Session{ID: "active", Title: "Current", UpdatedAt: 1725552000000},
			CWD:     "C:/code/project", Model: "reasoning-model", Provider: "provider-a",
			ReasoningKnown: true, ReasoningSupported: true,
			HasContext: true, Context: surface.Context{FeedTokens: 1200, ModelLimitTokens: 8000, ModelLimitKnown: true, TokenCountsEstimated: true},
			HasUsage: true, Usage: surface.SidebarUsage{
				PromptTokens: 900, CompletionTokens: 600, TotalTokens: 1500,
				ReasoningTokens: 120, CachedTokens: 300, RequestCount: 3, CostKnown: false,
			},
			ModifiedFilesKnown: true,
			MCPKnown:           true, MCP: []surface.MCPServer{{Name: "docs", State: "initialized", ToolCount: -1}, {Name: "local", State: "configured", ToolCount: -1}},
			SkillsKnown: true, Skills: []surface.SidebarSkill{{Name: "review", Origin: "user"}},
			LSPKnown: true, LSP: []surface.LanguageServer{{Language: "go", State: "initialized"}, {Language: "typescript", State: "starting"}},
		},
	}
	for i := 0; i < 20; i++ {
		// No UpdatedAt: a full-width timestamp would legitimately starve the
		// path below the file name, which this scroll test depends on.
		driver.sidebar.ModifiedFiles = append(driver.sidebar.ModifiedFiles, surface.ModifiedFile{
			Path: fmt.Sprintf("pkg/file-%02d.go", i),
			Diff: surface.SidebarDiff{Additions: 1, Deletions: 1},
		})
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	view := m.View()
	for _, want := range []string{
		"C:/code/project", "host · 127.0.0.1:8787", "reasoning-model", "provider-a", "reasoning · supported",
		"~15% · ~1.2k / 8.0k tokens", "total · 1.5k tokens", "input · 900", "output · 600",
		"Session Usage", "reasoning · 120", "cached · 300", "requests · 3", "est. cost · unknown", "pkg/file-00.go",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("sidebar omitted %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "0.0000") {
		t.Fatalf("unknown cost rendered as a free value:\n%s", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlRight})
	m = updated.(Model)
	if !m.sidebarFocused {
		t.Fatal("ctrl+right did not focus a scrollable sidebar")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(Model)
	scrolled := m.View()
	if m.sidebarScroll == 0 || !strings.Contains(scrolled, "pkg/file-19.go") {
		t.Fatalf("end did not scroll to the newest modified file: offset=%d\n%s", m.sidebarScroll, m.View())
	}
	for _, want := range []string{"VIVY CODE", "go · initialized", "typescript · starting", "docs · initialized", "local · configured", "Skills · enabled", "review"} {
		if !strings.Contains(scrolled, want) {
			t.Fatalf("scrolled sidebar omitted %q:\n%s", want, scrolled)
		}
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if m.sidebarFocused {
		t.Fatal("left arrow did not exit sidebar focus")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlRight})
	m = updated.(Model)
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 99, Height: 30})
	m = updated.(Model)
	if m.sidebarFocused || m.sidebarScroll != 0 {
		t.Fatalf("compact resize retained hidden sidebar state: focused=%v scroll=%d", m.sidebarFocused, m.sidebarScroll)
	}
}

func TestSidebarRendersMCPDetailsAndSkillOriginsSafely(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:  surface.Session{ID: "active", Title: "Current"},
			MCPKnown: true,
			MCP: []surface.MCPServer{
				{Name: "docs", Transport: "http", State: "initialized", ToolCount: 2},
				{Name: "broken", Transport: "stdio", State: "error", Error: "connection\x1b]2;PWN\a\r\nrefused\u202e", AuthMissing: true, EnvMissing: []string{"MCP_TOKEN"}, ToolCount: 0},
				{Name: "catalog", State: "configured", ToolCount: -1},
				{Name: "skip-me", State: "unknown", ToolCount: 9},
			},
			SkillsKnown: true,
			Skills: []surface.SidebarSkill{
				{Name: "review", Origin: "project"},
				{Name: "safe", Origin: "\x1b]2;PWN\auser"},
			},
		},
	}
	model := New(driver)
	width := 48
	lines := model.sidebarLines(width, DefaultPalette())
	plainLines := make([]string, 0, len(lines))
	for i, line := range lines {
		plain := ansi.Strip(line)
		plainLines = append(plainLines, plain)
		if got := lipgloss.Width(plain); got > width-1 {
			t.Fatalf("sidebar line %d width=%d, want <=%d: %q", i, got, width-1, plain)
		}
	}
	plain := strings.Join(plainLines, "\n")
	for _, want := range []string{
		"docs \u00b7 initialized \u00b7 http", "   2 tools",
		"broken \u00b7 error", "   ! connection refused", "   auth missing", "   0 tools",
		"broken \u00b7 error \u00b7 stdio", "   env missing: MCP_TOKEN",
		"catalog \u00b7 configured", "review \u00b7 project", "safe \u00b7 user",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("sidebar omitted %q:\n%s", want, plain)
		}
	}
	for _, absent := range []string{"skip-me", "   9 tools", "PWN", "\u202e"} {
		if strings.Contains(plain, absent) {
			t.Fatalf("sidebar retained/skipped unsafe or unknown value %q:\n%s", absent, plain)
		}
	}
}

func TestSidebarContextWarnsAboveEightyPercent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		feedTokens int
		warn       bool
	}{
		{name: "exactly eighty", feedTokens: 8000},
		{name: "just above eighty", feedTokens: 8001, warn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver := &testDriver{
				sessions: []surface.Session{{ID: "active", Title: "Current"}},
				active:   "active",
				sidebar: surface.Sidebar{
					Session:    surface.Session{ID: "active", Title: "Current"},
					HasContext: true,
					Context:    surface.Context{FeedTokens: tc.feedTokens, ModelLimitTokens: 10000, ModelLimitKnown: true, TokenCountsEstimated: true},
				},
			}
			m := New(driver)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
			view := updated.(Model).View()
			warned := strings.Contains(view, "! ~80%")
			if warned != tc.warn {
				t.Fatalf("warning=%v, want %v:\n%s", warned, tc.warn, view)
			}
		})
	}
}

func TestSidebarContextHidesFallbackLimitWhenModelLimitIsUnknown(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:    surface.Session{ID: "active", Title: "Current"},
			HasContext: true,
			Context:    surface.Context{FeedTokens: 1200, ModelLimitTokens: 128000, TokenCountsEstimated: true},
		},
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := updated.(Model).View()
	if !strings.Contains(view, "~1.2k tokens · limit unknown") || strings.Contains(view, "128.0k") || strings.Contains(view, "%") {
		t.Fatalf("fallback model limit was presented as authoritative:\n%s", view)
	}
}

func TestSidebarUsageShowsKnownZeroCostAndAuthoritativeZeroBreakdown(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:  surface.Session{ID: "active", Title: "Current"},
			HasUsage: true,
			Usage:    surface.SidebarUsage{RequestCount: 1, CostKnown: true},
		},
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := updated.(Model).View()
	for _, want := range []string{"total · 0 tokens", "input · 0", "output · 0", "requests · 1", "est. cost · $0.0000"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sidebar omitted authoritative zero %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "reasoning · 0") || strings.Contains(view, "cached · 0") {
		t.Fatalf("optional zero-only usage dimensions added noise:\n%s", view)
	}
}

func TestSidebarShortcutDoesNotStealEditorInput(t *testing.T) {
	driver := &testDriver{
		sidebar: surface.Sidebar{
			ModifiedFilesKnown: true,
			ModifiedFiles:      make([]surface.ModifiedFile, 20),
		},
	}
	for i := range driver.sidebar.ModifiedFiles {
		driver.sidebar.ModifiedFiles[i].Path = fmt.Sprintf("pkg/file-%02d.go", i)
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = updated.(Model)
	if m.sidebarFocused || m.input != "l" {
		t.Fatalf("typing l was stolen by sidebar: focused=%v input=%q", m.sidebarFocused, m.input)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.sidebarFocused {
		t.Fatal("plain right arrow was stolen by sidebar")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlRight})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.sidebarFocused || m.input != "lx" {
		t.Fatalf("editor input did not resume from sidebar focus: focused=%v input=%q", m.sidebarFocused, m.input)
	}
}

func TestSidebarMouseWheelIsRegionBoundedAndOverlaySafe(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		messages: map[string][]surface.Message{"active": {}},
		sidebar:  surface.Sidebar{ModifiedFilesKnown: true, ModifiedFiles: make([]surface.ModifiedFile, 50)},
	}
	for i := 0; i < 40; i++ {
		driver.messages["active"] = append(driver.messages["active"], surface.Message{Role: surface.RoleAssistant, Content: fmt.Sprintf("history-%02d", i)})
	}
	for i := range driver.sidebar.ModifiedFiles {
		driver.sidebar.ModifiedFiles[i].Path = fmt.Sprintf("pkg/file-%02d.go", i)
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	l := computeLayout(m.width, m.height)
	sidebarX := l.marginX + l.mainW() + 1

	// Hover routing works without first clicking the sidebar and does not
	// steal keyboard focus from the editor.
	updated, _ = m.Update(tea.MouseMsg{X: sidebarX, Y: l.marginY + 1, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.sidebarScroll != sidebarWheelStep || m.sidebarFocused {
		t.Fatalf("hovered sidebar wheel = offset %d focused %v, want %d/false", m.sidebarScroll, m.sidebarFocused, sidebarWheelStep)
	}

	updated, _ = m.Update(tea.MouseMsg{X: sidebarX, Y: l.marginY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(Model)
	if !m.sidebarFocused {
		t.Fatal("sidebar click did not focus scroll owner")
	}
	updated, _ = m.Update(tea.MouseMsg{X: sidebarX, Y: l.marginY + 1, Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.sidebarScroll != 2*sidebarWheelStep || !m.sidebarFocused {
		t.Fatalf("sidebar wheel = offset %d focused %v, want %d/true", m.sidebarScroll, m.sidebarFocused, 2*sidebarWheelStep)
	}

	before := m.sidebarScroll
	chatBefore := m.chatScroll
	updated, _ = m.Update(tea.MouseMsg{X: l.marginX, Y: l.marginY + l.headerH + 1, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.sidebarScroll != before {
		t.Fatalf("chat-region wheel moved focused sidebar from %d to %d", before, m.sidebarScroll)
	}
	if !m.sidebarFocused {
		t.Fatal("pointer-region wheel unexpectedly cleared explicit sidebar focus")
	}
	if m.chatCanScroll() && m.chatScroll == chatBefore {
		t.Fatal("chat-region wheel did not route to chat while sidebar retained keyboard focus")
	}

	m.sessionsOpen = true
	updated, _ = m.Update(tea.MouseMsg{X: sidebarX, Y: l.marginY + 1, Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.sidebarScroll != before {
		t.Fatalf("dialog wheel leaked into sidebar: %d -> %d", before, m.sidebarScroll)
	}

	m.sessionsOpen = false
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 99, Height: 30})
	m = updated.(Model)
	updated, _ = m.Update(tea.MouseMsg{X: 98, Y: 2, Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.sidebarScroll != 0 || m.sidebarFocused {
		t.Fatalf("compact mouse retained hidden sidebar state: offset=%d focused=%v", m.sidebarScroll, m.sidebarFocused)
	}
}

func TestSidebarRendersKnownEmptyModifiedFiles(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		sidebar: surface.Sidebar{
			Session:            surface.Session{ID: "active", Title: "Current"},
			ModifiedFilesKnown: true,
		},
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := updated.(Model).View()
	if !strings.Contains(view, "Modified Files") || !strings.Contains(view, "None") {
		t.Fatalf("known empty modified-files section missing:\n%s", view)
	}
}

func TestChatViewportPreservesHistoryAndFollowState(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "one", Title: "One"}, {ID: "two", Title: "Two"}},
		active:   "one",
		messages: map[string][]surface.Message{},
	}
	for i := 0; i < 40; i++ {
		driver.messages["one"] = append(driver.messages["one"], surface.Message{Role: surface.RoleAssistant, Content: fmt.Sprintf("history-%02d", i)})
	}
	driver.messages["two"] = []surface.Message{{Role: surface.RoleAssistant, Content: "second-session-latest"}}

	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	m = updated.(Model)
	if !m.chatFollow || m.chatScroll != m.chatMaxScroll() || !strings.Contains(m.View(), "history-39") {
		t.Fatalf("initial viewport did not follow latest: offset=%d max=%d follow=%v\n%s", m.chatScroll, m.chatMaxScroll(), m.chatFollow, m.View())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(Model)
	pausedOffset := m.chatScroll
	pausedView := m.View()
	if m.chatFollow || pausedOffset >= m.chatMaxScroll() || strings.Contains(pausedView, "history-39") {
		t.Fatalf("page-up did not expose older history: offset=%d max=%d follow=%v\n%s", pausedOffset, m.chatMaxScroll(), m.chatFollow, pausedView)
	}
	driver.messages["one"] = append(driver.messages["one"], surface.Message{Role: surface.RoleAssistant, Content: "new-while-paused"})
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.chatScroll != pausedOffset || m.chatFollow || strings.Contains(m.View(), "new-while-paused") {
		t.Fatalf("new content stole paused viewport: offset=%d want=%d follow=%v\n%s", m.chatScroll, pausedOffset, m.chatFollow, m.View())
	}
	m.input = "resume at latest"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.chatFollow || m.chatScroll != m.chatMaxScroll() || m.input != "" {
		t.Fatalf("sending did not resume follow: offset=%d max=%d follow=%v input=%q", m.chatScroll, m.chatMaxScroll(), m.chatFollow, m.input)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(Model)
	if !m.chatFollow || m.chatScroll != m.chatMaxScroll() || !strings.Contains(m.View(), "new-while-paused") {
		t.Fatalf("end did not resume latest follow: offset=%d max=%d follow=%v\n%s", m.chatScroll, m.chatMaxScroll(), m.chatFollow, m.View())
	}

	driver.active = "two"
	updated, _ = m.Update(surface.RefreshMsg{})
	m = updated.(Model)
	if m.chatSessionID != "two" || !m.chatFollow || m.chatScroll != 0 || !strings.Contains(m.View(), "second-session-latest") {
		t.Fatalf("session switch retained old viewport: session=%q offset=%d follow=%v\n%s", m.chatSessionID, m.chatScroll, m.chatFollow, m.View())
	}
}

func TestChatMouseWheelIsFocusedAndOverlaySafe(t *testing.T) {
	driver := &testDriver{sessions: []surface.Session{{ID: "one", Title: "One"}}, active: "one", messages: map[string][]surface.Message{}}
	for i := 0; i < 40; i++ {
		driver.messages["one"] = append(driver.messages["one"], surface.Message{Role: surface.RoleAssistant, Content: fmt.Sprintf("mouse-history-%02d", i)})
	}
	m := New(driver)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	m = updated.(Model)
	l := computeLayout(m.width, m.height)
	bottom := m.chatScroll

	updated, _ = m.Update(tea.MouseMsg{X: l.marginX + 1, Y: l.marginY + l.headerH + 2, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.chatScroll != bottom-sidebarWheelStep || m.chatFollow {
		t.Fatalf("chat wheel = offset %d follow %v, want %d/false", m.chatScroll, m.chatFollow, bottom-sidebarWheelStep)
	}

	paused := m.chatScroll
	m.sessionsOpen = true
	updated, _ = m.Update(tea.MouseMsg{X: l.marginX + 1, Y: l.marginY + l.headerH + 2, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.chatScroll != paused {
		t.Fatalf("dialog wheel leaked into chat: %d -> %d", paused, m.chatScroll)
	}

	m.sessionsOpen = false
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 100})
	m = updated.(Model)
	if m.chatScroll != 0 || !m.chatFollow {
		t.Fatalf("non-scrollable resize kept stale chat state: offset=%d follow=%v", m.chatScroll, m.chatFollow)
	}
}

func TestImageHistoryRenderingUsesMetadataOnlyChips(t *testing.T) {
	lines := (Model{}).renderMessage(surface.Message{
		Role:        surface.RoleUser,
		Content:     "look",
		Attachments: []surface.Attachment{{Name: "photo.png", MimeType: "image/png", Size: 42}},
	}, 80, DefaultPalette())
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "look") || !strings.Contains(joined, "[image: photo.png]") {
		t.Fatalf("attachment chip missing: %s", joined)
	}
	if strings.Contains(joined, "data:") || strings.Contains(joined, "base64") {
		t.Fatalf("history rendered raw attachment payload: %s", joined)
	}
}

func TestFileContextHistoryRenderingUsesMetadataOnlyChips(t *testing.T) {
	lines := (Model{}).renderMessage(surface.Message{
		Role:         surface.RoleUser,
		Content:      "inspect",
		FileContexts: []surface.FileContext{{Path: "README.md", Name: "README.md", Size: 42}},
	}, 80, DefaultPalette())
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "inspect") || !strings.Contains(joined, "[file: README.md]") {
		t.Fatalf("file context chip missing: %s", joined)
	}
	if strings.Contains(joined, "content:") || strings.Contains(joined, "base64") {
		t.Fatalf("file context body leaked into history: %s", joined)
	}
}

func TestSessionsDialogRefreshesFiltersAndSelects(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other task"}},
		active:   "active",
		sidebar:  surface.Sidebar{Session: surface.Session{ID: "active", Title: "Current"}},
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	if !m.sessionsOpen || cmd == nil {
		t.Fatalf("ctrl+s did not open dialog: open=%v cmd=%v", m.sessionsOpen, cmd != nil)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !strings.Contains(m.View(), "▸ Current") {
		t.Fatalf("current session was not preselected:\n%s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ohr tk")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "Other task") {
		t.Fatalf("title filter hid matching session:\n%s", m.View())
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.sessionsOpen || cmd == nil || driver.selected != "other" {
		t.Fatalf("filtered selection failed: open=%v cmd=%v selected=%q", m.sessionsOpen, cmd != nil, driver.selected)
	}
}

func TestFuzzyContainsIsOrderedAndUnicodeSafe(t *testing.T) {
	if !fuzzyContains("另一个会话 🚀", "另会🚀") {
		t.Fatal("ordered Unicode subsequence should match")
	}
	if fuzzyContains("other task", "task other") {
		t.Fatal("out-of-order query should not match")
	}
}

func TestPendingGateCannotBeHiddenBySessionsDialog(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
		gate:     &surface.Gate{Kind: "approval", ID: "approval_1", Title: "write file"},
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	if m.sessionsOpen || cmd != nil {
		t.Fatalf("ctrl+s hid a pending gate: open=%v cmd=%v", m.sessionsOpen, cmd != nil)
	}
}

func TestGateArrivingWhileSessionsOpenTakesPriority(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
	}
	m := New(driver)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	driver.gate = &surface.Gate{Kind: "approval", ID: "approval_1", Title: "write file", Body: "confirm"}
	if view := m.View(); !strings.Contains(view, "permission") || strings.Contains(view, "filter title") {
		t.Fatalf("gate did not replace sessions overlay:\n%s", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.sessionsOpen {
		t.Fatal("session dialog remained interactive behind an arriving gate")
	}
}

func TestRejectedSendKeepsDraft(t *testing.T) {
	driver := &testDriver{sendBlocked: true}
	m := New(driver)
	m.input = "keep this draft"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.input != "keep this draft" {
		t.Fatalf("rejected send lost draft: cmd=%v input=%q", cmd != nil, m.input)
	}
}

func TestSessionsDialogIgnoresOlderMutationResult(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
	}
	m := New(driver)
	m.sessionsOpen = true
	m.sessionRows = driver.Sessions()
	m.applySessionsMsg(surface.SessionsMsg{Action: "rename", Request: 2, ID: "active", Session: surface.Session{ID: "active", Title: "Newest"}})
	m.applySessionsMsg(surface.SessionsMsg{Action: "list", Request: 1, Sessions: []surface.Session{{ID: "active", Title: "Stale"}}})
	if got := m.sessionRows[0].Title; got != "Newest" {
		t.Fatalf("older session response overwrote newer mutation: %q", got)
	}
}

func TestSessionsDialogRenameAndDeleteUseSingleConfirm(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other"}},
		active:   "active",
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(Model)
	if !m.sessionRenaming {
		t.Fatal("ctrl+r did not enter rename mode")
	}
	m.sessionRenameInput = "Renamed"
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.sessionActionBusy || cmd == nil {
		t.Fatal("rename did not enter submitting state")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.sessionRenaming || !strings.Contains(m.View(), "Renamed") || driver.rename != "active:Renamed" {
		t.Fatalf("rename result not applied: renaming=%v rename=%q", m.sessionRenaming, driver.rename)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	if m.sessionDeleteID == "" {
		t.Fatal("ctrl+x did not open delete confirmation")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(Model)
	if m.sessionDeleteID != "" || cmd != nil {
		t.Fatal("n should cancel delete without an RPC")
	}
}

func TestSessionsDialogDeleteConfirmAndBusyRefusal(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other"}},
		active:   "active",
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if !m.sessionActionBusy || cmd == nil || driver.deleted != "active" {
		t.Fatalf("delete confirmation did not submit: busy=%v cmd=%v id=%q", m.sessionActionBusy, cmd != nil, driver.deleted)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.sessionDeleteID != "" || m.sessionActionBusy || strings.Contains(m.View(), "Current") {
		t.Fatalf("delete result not applied: id=%q busy=%v", m.sessionDeleteID, m.sessionActionBusy)
	}

	busyDriver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}, {ID: "other", Title: "Other"}},
		active:   "active",
		busy:     true,
	}
	m = New(busyDriver)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = updated.(Model)
	if m.sessionDeleteID != "" || !strings.Contains(m.sessionError, "run is in progress") {
		t.Fatalf("busy active delete was not refused: id=%q err=%q", m.sessionDeleteID, m.sessionError)
	}
}

func TestSessionsDialogCancelIsSafe(t *testing.T) {
	driver := &testDriver{
		sessions: []surface.Session{{ID: "active", Title: "Current"}},
		active:   "active",
	}
	m := New(driver)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.sessionRenaming || m.sessionRenameInput != "" {
		t.Fatal("escape did not cancel rename")
	}
}
