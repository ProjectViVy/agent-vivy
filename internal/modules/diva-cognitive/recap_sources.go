package divacognitive

import (
	"encoding/base64"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
)

type recapSourceView struct {
	InteractionID  string                                    `json:"interaction_id"`
	SourceRevision uint64                                    `json:"source_revision"`
	SessionID      domain.SessionID                          `json:"session_id"`
	RunID          domain.RunID                              `json:"run_id"`
	SourceDigest   string                                    `json:"source_digest"`
	Outcome        domain.RunStatus                          `json:"outcome"`
	Messages       []recapSourceMessage                      `json:"messages"`
	References     []domain.ContextReference                 `json:"references,omitempty"`
	Corrections    []cognitivecontract.RecapSourceCorrection `json:"corrections,omitempty"`
}
type recapSourceMessage struct {
	ID            string                  `json:"id"`
	Role          domain.Role             `json:"role"`
	Content       string                  `json:"content"`
	Disposition   string                  `json:"disposition,omitempty"`
	ToolStatus    string                  `json:"tool_status,omitempty"`
	ToolName      string                  `json:"tool_name,omitempty"`
	ToolCallID    string                  `json:"tool_call_id,omitempty"`
	ToolArguments string                  `json:"tool_arguments,omitempty"`
	ToolError     string                  `json:"tool_error,omitempty"`
	Attachments   []recapSourceAttachment `json:"attachments,omitempty"`
	FileContexts  []recapSourceFile       `json:"file_contexts,omitempty"`
}
type recapSourceAttachment struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	DataURL  string `json:"data_url"`
}
type recapSourceFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

func recapSourceViews(sources []cognitivecontract.RecapSource) []recapSourceView {
	out := make([]recapSourceView, 0, len(sources))
	for _, source := range sources {
		view := recapSourceView{InteractionID: source.InteractionID, SourceRevision: source.SourceRevision, SessionID: source.SessionID, RunID: source.RunID, SourceDigest: source.SourceDigest, Outcome: source.Outcome, References: source.References, Corrections: source.Corrections, Messages: []recapSourceMessage{}}
		for _, m := range source.Messages {
			item := recapSourceMessage{ID: m.ID, Role: m.Role, Content: m.Content, Disposition: source.Disposition[m.ID], ToolName: m.ToolName, ToolCallID: m.ToolCallID, ToolArguments: string(m.ToolArgs), ToolError: m.ToolError}
			if m.Role == domain.RoleTool {
				item.ToolStatus = "completed"
				if m.ToolError != "" {
					item.ToolStatus = "failed"
				}
			} else if m.ToolCallID != "" {
				item.ToolStatus = "requested"
			}
			for _, attachment := range m.Attachments {
				item.Attachments = append(item.Attachments, recapSourceAttachment{Name: attachment.Name, MimeType: attachment.MimeType, DataURL: "data:" + attachment.MimeType + ";base64," + base64.StdEncoding.EncodeToString(attachment.Data)})
			}
			for _, file := range m.FileContexts {
				item.FileContexts = append(item.FileContexts, recapSourceFile{Name: file.Name, Path: file.Path, Content: string(file.Content)})
			}
			view.Messages = append(view.Messages, item)
		}
		out = append(out, view)
	}
	return out
}
