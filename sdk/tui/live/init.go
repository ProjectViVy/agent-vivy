package live

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/sdk/tui/surface"
)

//go:embed prompts/init.md
var initInstructions string

const initCreate = "\nThe project root has no AGENTS.md. Create it after inspecting the repository. Recheck that the file is still absent before writing; if it appeared, stop and suggest changes instead. Do not change other files.\n"
const initSuggest = "\nThe project root already has AGENTS.md. Inspect it and suggest a concise, concrete update or diff for the user to approve. Do not write or edit any files in this turn.\n"

// initProject uses the fixed code root owned by the control plane. The
// preflight decides between a normal creation turn and a tool-enforced plan
// turn; neither the packed TUI nor the slash parser touches the filesystem.
func (l *Live) initProject() tea.Cmd {
	if !l.SupportsCapability("project.init.status") {
		return commandResultCmd("init", "", errors.New(l.translator.T("vivy.tui.live.initUnavailable", nil)))
	}
	l.mu.Lock()
	sessionID := l.activeID
	if sessionID == "" || l.busy || l.gate != nil || l.loadPending || l.commandInFlight || len(l.queue) > 0 {
		l.mu.Unlock()
		return commandResultCmd("init", "", errors.New(l.translator.T("vivy.tui.live.sessionBusy", nil)))
	}
	l.commandInFlight = true
	thinking := l.thinkingMode
	l.mu.Unlock()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
		defer cancel()
		raw, err := l.client.Call(ctx, "project/init/status", nil)
		var status struct {
			Exists *bool `json:"exists"`
		}
		if err == nil {
			err = json.Unmarshal(raw, &status)
		}
		l.mu.Lock()
		l.commandInFlight = false
		blocked := l.activeID != sessionID || l.busy || l.gate != nil || l.loadPending || len(l.queue) > 0
		l.mu.Unlock()
		if err != nil || status.Exists == nil {
			if err == nil {
				err = errors.New("missing status")
			}
			return surface.CommandResultMsg{Name: "init", Err: fmt.Errorf("%s: %w", l.translator.T("vivy.tui.live.initInspectFailed", nil), err)}
		}
		if blocked {
			return surface.CommandResultMsg{Name: "init", Err: errors.New(l.translator.T("vivy.tui.live.sessionBusy", nil))}
		}
		mode, prompt := "normal", initInstructions+initCreate
		if *status.Exists {
			mode, prompt = "plan", initInstructions+initSuggest
		}
		cmd := l.sendWithAttachments(prompt, thinking, mode, nil, false)
		if cmd == nil {
			return surface.CommandResultMsg{Name: "init", Err: errors.New(l.translator.T("vivy.tui.live.sessionBusy", nil))}
		}
		return cmd()
	}
}
