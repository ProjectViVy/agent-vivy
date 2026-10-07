package view

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// External editor (VCP-G3): the "external_editor" action writes the composer
// draft to a temp file, opens $EDITOR (falling back to VISUAL, then the first
// of vi/nano/notepad found on PATH), and loads the file back when it exits.

// externalEditorResultMsg delivers the edited draft after the editor exits.
type externalEditorResultMsg struct {
	content string
	err     error
}

// resolveEditor picks the editor command line. A configured EDITOR/VISUAL is
// honored verbatim (fields split so "code --wait" works); otherwise the first
// known terminal editor on PATH wins. The env/lookPath seams keep the
// resolution testable without mutating the process environment.
func resolveEditor(env func(string) string, lookPath func(string) (string, error)) []string {
	for _, variable := range []string{"EDITOR", "VISUAL"} {
		if value := strings.TrimSpace(env(variable)); value != "" {
			return strings.Fields(value)
		}
	}
	candidates := []string{"vi", "nano"}
	if runtime.GOOS == "windows" {
		candidates = []string{"notepad"}
	}
	for _, name := range candidates {
		if _, err := lookPath(name); err == nil {
			return []string{name}
		}
	}
	return nil
}

// runExternalEditor is the synchronous composition used by tests: seed a temp
// file, run the editor to completion, read the result back. The interactive
// path performs the same steps around tea.ExecProcess.
func runExternalEditor(editor []string, initial string) (string, error) {
	if len(editor) == 0 {
		return "", errNoEditor
	}
	path, err := prepareEditorFile(initial)
	if err != nil {
		return "", err
	}
	if err := editorCommand(editor, path).Run(); err != nil {
		os.Remove(path)
		return "", err
	}
	return readEditorFile(path)
}

var errNoEditor = errors.New("no editor found: set EDITOR")

// prepareEditorFile seeds a temp file with the draft and returns its path.
func prepareEditorFile(initial string) (string, error) {
	file, err := os.CreateTemp("", "vivy-compose-*.md")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(initial); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// editorCommand builds the process that opens path in the resolved editor.
func editorCommand(editor []string, path string) *exec.Cmd {
	args := append(append([]string(nil), editor[1:]...), path)
	return exec.Command(editor[0], args...)
}

// readEditorFile returns the saved draft and removes the temp file.
func readEditorFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	os.Remove(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func (m Model) openExternalEditor() (Model, tea.Cmd) {
	editor := resolveEditor(os.Getenv, exec.LookPath)
	if len(editor) == 0 {
		return m.showCommandResult(
			m.translator.T("vivy.tui.dialog.editor", nil),
			m.translator.T("vivy.tui.editor.none", nil)), nil
	}
	path, err := prepareEditorFile(m.input)
	if err != nil {
		return m.showCommandResult(
			m.translator.T("vivy.tui.dialog.editor", nil),
			m.translator.T("vivy.tui.editor.error", map[string]any{"error": err.Error()})), nil
	}
	return m, tea.ExecProcess(editorCommand(editor, path), func(runErr error) tea.Msg {
		if runErr != nil {
			os.Remove(path)
			return externalEditorResultMsg{err: runErr}
		}
		content, readErr := readEditorFile(path)
		if readErr != nil {
			return externalEditorResultMsg{err: readErr}
		}
		return externalEditorResultMsg{content: content}
	})
}
