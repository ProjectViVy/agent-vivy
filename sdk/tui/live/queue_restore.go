package live

import (
	"encoding/base64"
	"encoding/json"

	"agent-vivy/sdk/tui/surface"
)

// Private editor snapshots are never admitted automatically or rendered as raw bodies.
func (l *Live) installEditorTurnLocked(sessionID string, turn *queuedTurnView) {
	if l.editorTurns == nil {
		l.editorTurns = make(map[string]*queuedTurnView)
	}
	if l.drafts == nil {
		l.drafts = make(map[string][]surface.Attachment)
	}
	l.editorTurns[sessionID] = turn
	l.drafts[sessionID] = nil
	for _, raw := range turn.Attachments {
		var attachment struct {
			Name     string `json:"name"`
			MimeType string `json:"mime_type"`
			Data     string `json:"data"`
		}
		if json.Unmarshal(raw, &attachment) == nil {
			l.drafts[sessionID] = append(l.drafts[sessionID], surface.Attachment{Name: attachment.Name, MimeType: attachment.MimeType, Size: int64(base64.StdEncoding.DecodedLen(len(attachment.Data)))})
		}
	}
	l.thinkingMode = turn.Thinking
	if l.thinkingMode == "" {
		l.thinkingMode = "auto"
	}
	l.thinkingEffective = ""
	l.runMode = turn.Mode
	if l.runMode == "" {
		l.runMode = "normal"
	}
}

func (l *Live) keepReturnedTurnLocked(sessionID string, turn queuedTurnView) {
	if l.returnedTurns == nil {
		l.returnedTurns = make(map[string][]queuedTurnView)
	}
	for _, old := range l.returnedTurns[sessionID] {
		if old.ID != "" && old.ID == turn.ID {
			return
		}
	}
	l.returnedTurns[sessionID] = append(l.returnedTurns[sessionID], turn)
	l.lastErr = l.translator.T("vivy.tui.live.returnedTurns", nil)
}

func (t *queuedTurnView) snapshot() *queuedTurnView {
	if t == nil {
		return nil
	}
	copy := *t
	copy.Attachments = append([]json.RawMessage(nil), t.Attachments...)
	copy.Payload = make(map[string]json.RawMessage, len(t.Payload))
	for key, value := range t.Payload {
		copy.Payload[key] = append(json.RawMessage(nil), value...)
	}
	return &copy
}

func (l *Live) PendingFileContexts() []surface.FileContext {
	l.mu.Lock()
	defer l.mu.Unlock()
	turn := l.editorTurns[l.activeID]
	if turn == nil {
		return nil
	}
	var files []surface.FileContext
	_ = json.Unmarshal(turn.Payload["file_contexts"], &files)
	return files
}
