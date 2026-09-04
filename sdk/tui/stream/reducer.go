package stream

import "agent-vivy/sdk/tui/surface"

// Projection is the protocol-independent chat projection. The face owns
// session/run lifecycle state; this reducer owns only messages and gates.
type Projection struct {
	Messages      []surface.Message
	Gate          *surface.Gate
	ProtocolError string
}

// Apply reduces one normalized notice. It returns true for terminal notices.
func (p *Projection) Apply(notice Notice, nextID func(prefix string) string) (done bool) {
	switch notice.Kind {
	case "model_request":
		p.FinishStreaming()
	case "delta":
		// Empty deltas are not an answer boundary. In particular, providers
		// may interleave empty answer chunks while reasoning is streaming.
		if notice.Delta == "" {
			return false
		}
		for i := range p.Messages {
			if p.Messages[i].Reasoning {
				p.Messages[i].Streaming = false
			}
		}
		p.EnsureAssistantDraft(nextID)
		for i := len(p.Messages) - 1; i >= 0; i-- {
			if p.Messages[i].Role == surface.RoleAssistant && p.Messages[i].Streaming && !p.Messages[i].Reasoning {
				p.Messages[i].Content += notice.Delta
				return false
			}
		}
	case "reasoning":
		// Reasoning starts a new answer phase. Close any stale answer draft so
		// a later completion cannot overwrite text from an earlier model round.
		for i := range p.Messages {
			if p.Messages[i].Streaming && !p.Messages[i].Reasoning {
				p.Messages[i].Streaming = false
			}
		}
		for i := len(p.Messages) - 1; i >= 0; i-- {
			if p.Messages[i].Reasoning && p.Messages[i].Streaming {
				p.Messages[i].Content += notice.Delta
				return false
			}
			break
		}
		p.Messages = append(p.Messages, surface.Message{
			ID: nextID("thinking"), Role: surface.RoleAssistant, Content: notice.Delta,
			Streaming: true, Reasoning: true,
		})
	case "model_completed":
		if !notice.HasCompleted {
			return false
		}
		if notice.ProtocolError != "" {
			return p.failProtocol(notice.ProtocolError, nextID)
		}
		if !notice.CompletedAuthoritative {
			if notice.Completion == nil {
				return p.failProtocol("model.completed v2: missing validated metadata", nextID)
			}
			if err := VerifyModelCompletedV2Metadata(*notice.Completion, p.streamingAnswerContent()); err != nil {
				return p.failProtocol(err.Error(), nextID)
			}
			p.FinishStreaming()
			return false
		}
		found := false
		for i := len(p.Messages) - 1; i >= 0; i-- {
			if p.Messages[i].Role == surface.RoleAssistant && p.Messages[i].Streaming && !p.Messages[i].Reasoning {
				// completed.content is authoritative for this model round,
				// including an explicitly empty completion.
				p.Messages[i].Content = notice.Completed
				found = true
				break
			}
		}
		if !found && notice.Completed != "" {
			p.Messages = append(p.Messages, surface.Message{
				ID: nextID("asst"), Role: surface.RoleAssistant,
				Content: notice.Completed, Streaming: true,
			})
		}
		p.FinishStreaming()
	case "tool_requested":
		p.FinishStreaming()
		p.Messages = append(p.Messages, surface.Message{
			ID: nextID("tool"), Role: surface.RoleTool,
			Tool: &surface.ToolCard{ToolName: notice.Message, ToolCallID: notice.ToolCallID, Status: "pending", Preview: notice.Line},
		})
	case "tool_finished":
		p.FinishStreaming()
		for i := len(p.Messages) - 1; i >= 0; i-- {
			if p.Messages[i].Tool != nil && ((notice.ToolCallID != "" && p.Messages[i].Tool.ToolCallID == notice.ToolCallID) || (notice.ToolCallID == "" && p.Messages[i].Tool.ToolName == notice.Message)) {
				if notice.Failed {
					p.Messages[i].Tool.Status = "failed"
					p.Messages[i].Tool.Result = notice.Line
				} else {
					p.Messages[i].Tool.Status = "done"
					p.Messages[i].Tool.Result = notice.Line
					if p.Messages[i].Tool.Result == "" {
						p.Messages[i].Tool.Result = "done"
					}
				}
				return false
			}
		}
	case "gate":
		p.FinishStreaming()
		if notice.Gate == nil {
			return false
		}
		p.Gate = &surface.Gate{
			Kind: notice.Gate.Kind, ID: notice.Gate.ID,
			Title: notice.Gate.Title, Body: notice.Gate.Body,
			Action: notice.Gate.Action, Target: notice.Gate.Target,
			PreconditionHash: notice.Gate.PreconditionHash,
			Preview:          notice.Gate.Preview, Risks: append([]string(nil), notice.Gate.Risks...),
		}
		if notice.Gate.Kind == "approval" {
			found := false
			for i := range p.Messages {
				if p.Messages[i].Tool != nil && (p.Messages[i].Tool.ApprovalID == notice.Gate.ID || (notice.Gate.ToolCallID != "" && p.Messages[i].Tool.ToolCallID == notice.Gate.ToolCallID)) {
					p.Messages[i].Tool.Status = "pending"
					p.Messages[i].Tool.ApprovalID = notice.Gate.ID
					p.Messages[i].Tool.Preview = approvalToolPreview(notice.Gate)
					found = true
					break
				}
			}
			if !found {
				p.Messages = append(p.Messages, surface.Message{
					ID: nextID("tool"), Role: surface.RoleTool,
					Tool: &surface.ToolCard{
						ToolName: notice.Gate.Title, ToolCallID: notice.Gate.ToolCallID, Status: "pending",
						Preview: approvalToolPreview(notice.Gate), ApprovalID: notice.Gate.ID,
					},
				})
			}
		}
	case "done":
		p.FinishStreaming()
		p.Gate = nil
		if notice.ProtocolError != "" {
			p.ProtocolError = notice.ProtocolError
		}
		if notice.Failed && notice.Message != "" {
			p.Messages = append(p.Messages, surface.Message{
				ID: nextID("end"), Role: surface.RoleAssistant,
				Content: "[" + notice.Message + "]",
			})
		}
		return true
	}
	return false
}

func approvalToolPreview(gate *GatePrompt) string {
	if gate == nil {
		return ""
	}
	if gate.Preview != "" {
		return gate.Preview
	}
	return gate.Body
}

func (p *Projection) streamingAnswerContent() string {
	for i := len(p.Messages) - 1; i >= 0; i-- {
		message := p.Messages[i]
		if message.Role == surface.RoleAssistant && message.Streaming && !message.Reasoning {
			return message.Content
		}
	}
	return ""
}

func (p *Projection) failProtocol(message string, nextID func(prefix string) string) bool {
	if message == "" {
		message = "invalid stream protocol"
	}
	p.ProtocolError = message
	p.FinishStreaming()
	p.Gate = nil
	p.Messages = append(p.Messages, surface.Message{
		ID: nextID("end"), Role: surface.RoleAssistant,
		Content: "[" + message + "]",
	})
	return true
}

// EnsureAssistantDraft starts the answer bubble after reasoning or a prior
// completed assistant message.
func (p *Projection) EnsureAssistantDraft(nextID func(prefix string) string) {
	for i := len(p.Messages) - 1; i >= 0; i-- {
		if p.Messages[i].Role == surface.RoleAssistant && p.Messages[i].Streaming && !p.Messages[i].Reasoning {
			return
		}
		break
	}
	p.Messages = append(p.Messages, surface.Message{
		ID: nextID("asst"), Role: surface.RoleAssistant, Streaming: true,
	})
}

// FinishStreaming closes all in-flight assistant bubbles before a tool, gate,
// or terminal notice is rendered.
func (p *Projection) FinishStreaming() {
	for i := range p.Messages {
		if p.Messages[i].Streaming {
			p.Messages[i].Streaming = false
		}
	}
}
