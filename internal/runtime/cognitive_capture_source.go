package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// captureMessageReader reuses the existing durable MessageStore read surface.
// The admitted user row already carries its RunID; no timestamp heuristic or
// session-history copy is allowed to invent a run's source.
type captureMessageReader interface {
	ListMessages(context.Context, domain.SessionID) ([]domain.Message, error)
}

type captureSourceMessage struct {
	ID       string `json:"id,omitempty"`
	Role     string `json:"role"`
	Content  string `json:"content"`
	Complete bool   `json:"complete"`
}

type captureConversationSource struct {
	Schema    string                 `json:"schema"`
	RunID     domain.RunID           `json:"run_id"`
	SessionID domain.SessionID       `json:"session_id"`
	Messages  []captureSourceMessage `json:"messages"`
}

func cognitiveConversationSource(ctx context.Context, store captureMessageReader, run domain.Run, summary string) (string, error) {
	content, _, err := cognitiveConversationCapture(ctx, store, run, summary)
	return content, err
}

func cognitiveConversationCapture(ctx context.Context, store captureMessageReader, run domain.Run, summary string) (string, string, error) {
	rows, err := store.ListMessages(ctx, run.SessionID)
	if err != nil {
		return "", "", fmt.Errorf("cognitive capture: read admitted user source: %w", err)
	}
	var userContents []string
	source := captureConversationSource{Schema: "vivy.conversation-source/v1", RunID: run.ID, SessionID: run.SessionID}
	for _, row := range rows {
		if row.RunID != run.ID || row.SessionID != run.SessionID || row.Role != domain.RoleUser {
			continue
		}
		content := tools.RedactSensitive(row.Content)
		userContents = append(userContents, content)
		source.Messages = append(source.Messages, captureSourceMessage{ID: row.ID, Role: string(row.Role), Content: content, Complete: content == row.Content})
	}
	if len(source.Messages) == 0 {
		return "", "", fmt.Errorf("cognitive capture: no admitted user source for run %s", run.ID)
	}
	if summary != "" {
		// Terminal summary is only a bounded assistant projection. Never label
		// it user evidence or claim it is the complete assistant transcript.
		source.Messages = append(source.Messages, captureSourceMessage{Role: "assistant", Content: tools.RedactSensitive(summary), Complete: false})
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return "", "", fmt.Errorf("cognitive capture: encode conversation source: %w", err)
	}
	return string(raw), strings.Join(userContents, "\n"), nil
}
