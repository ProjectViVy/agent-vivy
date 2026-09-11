package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"unicode"
	"unicode/utf8"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/contextsource"

	"github.com/cloudwego/eino/schema"
)

// contextCandidatesToUserParts is the only generic ContextHost -> Eino
// projection. Context Sources and ContextHost themselves never import Eino.
func contextCandidatesToUserParts(candidates []contexthost.Candidate) []schema.MessageInputPart {
	parts := make([]schema.MessageInputPart, 0, len(candidates))
	for _, candidate := range candidates {
		if !validContextProjectionCandidate(candidate) {
			continue
		}
		label := fmt.Sprintf("\n\n[context: %s/%s provenance=%s]\n", candidate.SourceID, candidate.ContentID, candidate.ProvenanceID)
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: label + candidate.Content,
		})
	}
	return parts
}

func contextCandidateBytes(candidate contexthost.Candidate) int {
	return len(fmt.Sprintf("\n\n[context: %s/%s provenance=%s]\n", candidate.SourceID, candidate.ContentID, candidate.ProvenanceID)) + len(candidate.Content)
}

func validContextProjectionCandidate(candidate contexthost.Candidate) bool {
	if candidate.SourceID == "" || candidate.ContentID == "" || candidate.ProvenanceID == "" || !utf8.ValidString(candidate.Content) {
		return false
	}
	for _, value := range []string{candidate.SourceID, candidate.ContentID, candidate.ProvenanceID} {
		if len(value) > 1024 || !utf8.ValidString(value) {
			return false
		}
		for _, r := range value {
			if unicode.IsControl(r) || (unicode.IsSpace(r) && (r == '\n' || r == '\r' || r == '\t')) {
				return false
			}
		}
	}
	return true
}

// hostedFileContextParts preserves the current project-file presentation while
// forcing first-party snapshots through ContextSource and ContextHost before
// they become Eino message parts. The source is memory-only and cannot perform
// filesystem or network work at this stage.
func hostedFileContextParts(files []domain.FileContext) []schema.MessageInputPart {
	parts, _ := hostedFileContextPartsWithContext(context.Background(), nil, files)
	return parts
}

func hostedFileContextPartsWithContext(ctx context.Context, host *contexthost.Host, files []domain.FileContext) ([]schema.MessageInputPart, error) {
	return hostedFileContextPartsWithBudget(ctx, host, files, 0, 0)
}

func hostedFileContextPartsWithBudget(ctx context.Context, host *contexthost.Host, files []domain.FileContext, byteBudget, tokenBudget int) ([]schema.MessageInputPart, error) {
	if len(files) == 0 {
		return nil, nil
	}
	validate := func(ctx context.Context, snapshots []domain.FileContext) ([]domain.FileContext, error) {
		return normalizeFileContextsContext(ctx, snapshots)
	}
	source := contexthost.NewFileSnapshotSourceWithValidator("vivy.project-files", files, validate)
	if host == nil {
		var err error
		host, err = contexthost.New(contexthost.Config{Sources: []contextsource.Provider{source}})
		if err != nil {
			return nil, err
		}
	}
	result, err := host.QuerySources(ctx, contexthost.Request{
		PerSourceLimit: len(files),
		ByteBudget:     byteBudget,
		TokenBudget:    tokenBudget,
		CandidateBytes: fileContextCandidateBytes,
	}, source)
	if err != nil {
		return nil, err
	}
	if len(result.Failures) > 0 {
		return nil, result.Failures[0].Cause
	}
	parts := make([]schema.MessageInputPart, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "\n\n[project file: " + candidate.ContentID + "]\n" + candidate.Content,
		})
	}
	return parts, nil
}

func fileContextCandidateBytes(candidate contexthost.Candidate) int {
	return len("\n\n[project file: "+candidate.ContentID+"]\n") + len(candidate.Content)
}

func userFeedMessageWithContext(ctx context.Context, host *contexthost.Host, msg domain.Message) (*schema.Message, error) {
	return userFeedMessageWithContextBudget(ctx, host, msg, nil)
}

func userFeedMessageWithContextBudget(ctx context.Context, host *contexthost.Host, msg domain.Message, budget *contextProjectionBudget) (*schema.Message, error) {
	if len(msg.Attachments) == 0 && len(msg.FileContexts) == 0 {
		projected := schema.UserMessage(msg.Content)
		budget.add(projected)
		return projected, nil
	}
	textParts := make([]schema.MessageInputPart, 0, 1)
	if msg.Content != "" {
		textParts = append(textParts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: msg.Content})
	}
	attachmentParts := make([]schema.MessageInputPart, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		data := base64.StdEncoding.EncodeToString(attachment.Data)
		attachmentParts = append(attachmentParts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{Base64Data: &data, MIMEType: attachment.MimeType},
			},
		})
	}
	fileParts := make([]schema.MessageInputPart, 0, len(msg.FileContexts))
	if len(msg.FileContexts) > 0 {
		byteBudget, tokenBudget := 0, 0
		if budget != nil && budget.limit > 0 {
			baseParts := append(append([]schema.MessageInputPart(nil), textParts...), attachmentParts...)
			base := userMessageFromParts(msg.Content, baseParts)
			remaining := budget.remaining()
			baseBytes := projectedContextBytes([]*schema.Message{base})
			if remaining > baseBytes {
				byteBudget = remaining - baseBytes
				tokenBudget = byteBudget
			}
		}
		if budget == nil || budget.limit <= 0 || byteBudget > 0 {
			var err error
			fileParts, err = hostedFileContextPartsWithBudget(ctx, host, msg.FileContexts, byteBudget, tokenBudget)
			if err != nil {
				return nil, err
			}
		}
	}
	parts := make([]schema.MessageInputPart, 0, len(textParts)+len(fileParts)+len(attachmentParts))
	parts = append(parts, textParts...)
	parts = append(parts, fileParts...)
	parts = append(parts, attachmentParts...)
	if len(parts) == 0 {
		projected := schema.UserMessage(msg.Content)
		budget.add(projected)
		return projected, nil
	}
	projected := &schema.Message{Role: schema.User, UserInputMultiContent: parts}
	budget.add(projected)
	return projected, nil
}

func userMessageFromParts(content string, parts []schema.MessageInputPart) *schema.Message {
	if len(parts) == 0 {
		return schema.UserMessage(content)
	}
	return &schema.Message{Role: schema.User, UserInputMultiContent: parts}
}

func userFeedMessage(msg domain.Message) *schema.Message {
	projected, _ := userFeedMessageWithContext(context.Background(), nil, msg)
	return projected
}
