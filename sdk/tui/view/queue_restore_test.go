package view

import (
	"strings"
	"testing"

	"agent-vivy/sdk/tui/surface"
	tea "github.com/charmbracelet/bubbletea"
)

type recallDriver struct {
	*testDriver
	rejected bool
}

func (d *recallDriver) Handle(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(surface.RecallRejectedMsg); ok {
		d.rejected = true
		return nil
	}
	return d.testDriver.Handle(msg)
}
func (d *recallDriver) PendingFileContexts() []surface.FileContext {
	return []surface.FileContext{{Name: "captured.go", Path: "captured.go", Size: 8}}
}

func TestQueuedRecallDoesNotConsumeWhileEditorHasUserText(t *testing.T) {
	d := &testDriver{active: "s1", dequeueText: "queued"}
	m := New(d)
	m.input = "my draft"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp, Alt: true})
	if cmd != nil || d.dequeueText != "queued" || updated.(Model).input != "my draft" {
		t.Fatal("recall consumed or merged an existing draft")
	}
}

func TestInFlightCompleteRecallReturnsToDriverInsteadOfMerging(t *testing.T) {
	d := &recallDriver{testDriver: &testDriver{active: "s1"}}
	m := New(d)
	m.input = "new draft"
	updated, _ := m.Update(surface.RestoreInputMsg{Text: "queued", Recall: true})
	if !d.rejected || updated.(Model).input != "new draft" {
		t.Fatal("complete recall merged incompatible editor contents")
	}
}

func TestCapturedContextMetadataAppearsInComposer(t *testing.T) {
	d := &recallDriver{testDriver: &testDriver{active: "s1"}}
	m := New(d)
	m.width = 100
	m.height = 30
	if !strings.Contains(m.View(), "captured.go") {
		t.Fatal("captured file context invisible to editor")
	}
}
