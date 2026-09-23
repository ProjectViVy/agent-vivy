package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
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
		label := contextCandidateLabel(candidate)
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: label + candidate.Content,
		})
	}
	return parts
}

func contextCandidateBytes(candidate contexthost.Candidate) int {
	return len(contextCandidateLabel(candidate)) + len(candidate.Content)
}

func contextCandidateLabel(candidate contexthost.Candidate) string {
	if candidate.Version == "" {
		return fmt.Sprintf("\n\n[context: %s/%s provenance=%s]\n", candidate.SourceID, candidate.ContentID, candidate.ProvenanceID)
	}
	return fmt.Sprintf("\n\n[context: %s/%s version=%s provenance=%s]\n", candidate.SourceID, candidate.ContentID, candidate.Version, candidate.ProvenanceID)
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
	return userFeedMessageWithContextBudget(ctx, host, msg, nil, nil)
}

func userFeedMessageWithContextBudget(ctx context.Context, host *contexthost.Host, msg domain.Message, budget *contextProjectionBudget, references []domain.ContextReference) (*schema.Message, error) {
	if len(msg.Attachments) == 0 && len(msg.FileContexts) == 0 && len(references) == 0 {
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
	referenceParts := make([]schema.MessageInputPart, 0, len(references))
	if len(references) > 0 {
		// Imported blocks take whatever the rest of this message leaves;
		// their status markers are reserved inside the same allowance.
		remaining := -1
		if budget != nil && budget.limit > 0 {
			baseParts := make([]schema.MessageInputPart, 0, len(textParts)+len(fileParts)+len(attachmentParts))
			baseParts = append(append(append(baseParts, textParts...), fileParts...), attachmentParts...)
			used := projectedContextBytes([]*schema.Message{userMessageFromParts("", baseParts)})
			if rem := budget.remaining() - used; rem > 0 {
				remaining = rem
			} else {
				remaining = 0
			}
		}
		var err error
		referenceParts, _, err = ProjectReferenceContext(ctx, host, references, remaining)
		if err != nil {
			return nil, err
		}
	}
	parts := make([]schema.MessageInputPart, 0, len(textParts)+len(fileParts)+len(attachmentParts)+len(referenceParts))
	parts = append(parts, textParts...)
	parts = append(parts, fileParts...)
	parts = append(parts, attachmentParts...)
	parts = append(parts, referenceParts...)
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

// ReferenceInclusion records one reference's projection decision for a feed
// build. The states are the wire vocabulary for imported-data outcomes.
type ReferenceInclusion struct {
	ReferenceID string `json:"reference_id"`
	State       string `json:"state"`
	Digest      string `json:"digest,omitempty"`
}

const (
	ReferenceInclusionIncluded     = "included"
	ReferenceInclusionElidedBudget = "elided_budget"
	ReferenceInclusionUnavailable  = "unavailable"
)

// referenceSnapshotSourceID names the memory-only snapshot source used for
// imported reference projection. It performs no history or filesystem work.
const referenceSnapshotSourceID = "vivy.context-references"

// ProjectReferenceContext renders attached reference snapshots as user-data
// text parts. Imported content never becomes its own message, a system role,
// or a tool call: every snapshot is exactly one quoted block, and every
// non-included snapshot keeps an explicit status marker. Snapshot blocks pass
// through ContextHost budgeting via a memory-only resolved source; the
// per-reference marker lines are reserved before the query so statuses stay
// honest even under a tight budget. remainingBytes < 0 means unbounded.
func ProjectReferenceContext(ctx context.Context, host *contexthost.Host, refs []domain.ContextReference, remainingBytes int) ([]schema.MessageInputPart, []ReferenceInclusion, error) {
	inclusions := make([]ReferenceInclusion, 0, len(refs))
	if len(refs) == 0 {
		return nil, inclusions, nil
	}
	snapshots := make([]contexthost.ResolvedSnapshot, 0, len(refs))
	rendered := make(map[string]string, len(refs))
	projectable := make(map[string]bool, len(refs))
	markerBytes := 0
	for _, ref := range refs {
		if !referenceProjectable(ref) {
			markerBytes += len(referenceMarkerText(ref, ReferenceInclusionUnavailable))
			continue
		}
		projectable[ref.ID] = true
		markerBytes += len(referenceMarkerText(ref, ReferenceInclusionElidedBudget))
		rendered[ref.ID] = renderReferenceBlock(ref)
		snapshots = append(snapshots, contexthost.ResolvedSnapshot{
			ContentID: ref.ID,
			MediaType: "text/plain",
			Content:   rendered[ref.ID],
			Version:   ref.Digest,
		})
	}
	included := make(map[string]bool, len(snapshots))
	if len(snapshots) > 0 {
		source := contexthost.NewResolvedSnapshotSource(referenceSnapshotSourceID, snapshots)
		if host == nil {
			var err error
			host, err = contexthost.New(contexthost.Config{Sources: []contextsource.Provider{source}})
			if err != nil {
				return nil, inclusions, err
			}
		}
		request := contexthost.Request{
			PerSourceLimit: len(snapshots),
			CandidateBytes: referenceCandidateBytes,
		}
		if remainingBytes >= 0 {
			budget := remainingBytes - markerBytes
			if budget < 0 {
				budget = 0
			}
			request.EnforceByteBudget = true
			request.ByteBudget = budget
			request.TokenBudget = budget
		}
		result, err := host.QuerySources(ctx, request, source)
		if err != nil {
			return nil, inclusions, err
		}
		if len(result.Failures) > 0 {
			return nil, inclusions, result.Failures[0].Cause
		}
		for _, candidate := range result.Candidates {
			included[candidate.ContentID] = true
		}
	}
	parts := make([]schema.MessageInputPart, 0, len(refs))
	for _, ref := range refs {
		switch {
		case !projectable[ref.ID]:
			inclusions = append(inclusions, ReferenceInclusion{ReferenceID: ref.ID, State: ReferenceInclusionUnavailable})
			parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: referenceMarkerText(ref, ReferenceInclusionUnavailable)})
		case included[ref.ID]:
			inclusions = append(inclusions, ReferenceInclusion{ReferenceID: ref.ID, State: ReferenceInclusionIncluded, Digest: ref.Digest})
			parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: rendered[ref.ID]})
		default:
			inclusions = append(inclusions, ReferenceInclusion{ReferenceID: ref.ID, State: ReferenceInclusionElidedBudget, Digest: ref.Digest})
			parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: referenceMarkerText(ref, ReferenceInclusionElidedBudget)})
		}
	}
	return parts, inclusions, nil
}

// referenceProjectable reports whether the stored copy can be rendered. A
// snapshot missing identity, digest, or items is an explicit unavailable,
// never an empty substituted block.
func referenceProjectable(ref domain.ContextReference) bool {
	if ref.ID == "" || ref.Digest == "" || len(ref.Items) == 0 {
		return false
	}
	for _, field := range []string{ref.ID, ref.Digest, string(ref.SourceSessionID), ref.Origin} {
		if !utf8.ValidString(field) {
			return false
		}
	}
	for _, item := range ref.Items {
		if !utf8.ValidString(item.Text) || !utf8.ValidString(item.Author) {
			return false
		}
	}
	return true
}

// renderReferenceBlock emits the snapshot as one quoted text block. Item
// authors are labels inside the text, never message roles or tool calls.
func renderReferenceBlock(ref domain.ContextReference) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\n[imported reference: %s | origin=%s | source=%s | digest=%s | captured=%s]\n",
		ref.ID, ref.Origin, ref.SourceSessionID, ref.Digest,
		time.UnixMilli(ref.CapturedAt).UTC().Format("2006-01-02T15:04:05Z"))
	for _, item := range ref.Items {
		fmt.Fprintf(&b, "- [%s] %s\n", item.Author, item.Text)
	}
	b.WriteString("[end imported reference]")
	return b.String()
}

// referenceMarkerText is the explicit status line emitted when a snapshot is
// not rendered into the feed. Control bytes in a stored ID never reach the
// marker.
func referenceMarkerText(ref domain.ContextReference, state string) string {
	id := ref.ID
	if !utf8.ValidString(id) || len(id) > 128 {
		id = "<invalid>"
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			id = "<invalid>"
			break
		}
	}
	return fmt.Sprintf("\n\n[imported reference: %s | state=%s]", id, state)
}

func referenceCandidateBytes(candidate contexthost.Candidate) int {
	return len(candidate.Content)
}
