package view

import (
	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/tui/surface"
)

type noDriver struct{}

func (noDriver) Sessions() []surface.Session       { return nil }
func (noDriver) Active() surface.Session           { return surface.Session{} }
func (noDriver) ActiveMessages() []surface.Message { return nil }
func (noDriver) PendingGate() *surface.Gate        { return nil }
func (noDriver) Meta() surface.Meta                { return surface.Meta{Mode: "empty", Error: "not connected"} }
func (noDriver) Init() tea.Cmd                     { return nil }
func (noDriver) Handle(tea.Msg) tea.Cmd            { return nil }
func (noDriver) MoveSession(int) tea.Cmd           { return nil }
func (noDriver) NewSession(string) tea.Cmd         { return nil }
func (noDriver) Send(string) tea.Cmd               { return nil }
func (noDriver) DecideApproval(string) tea.Cmd     { return nil }
func (noDriver) AnswerQuestion(string) tea.Cmd     { return nil }
func (noDriver) SetPermission(string) tea.Cmd      { return nil }
func (noDriver) ClearQueue() bool                  { return false }
func (noDriver) Cancel() tea.Cmd                   { return nil }
