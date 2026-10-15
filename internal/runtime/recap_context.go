package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"github.com/cloudwego/eino/schema"
)

// RecapContext is one disposable admission cut. No coverage or acknowledgement
// is written to core storage; the bound C1 head remains the sole text owner.
type RecapContext struct {
	Head         cognitivecontract.RecapHead
	Snapshot     storage.SubjectContextSnapshot
	Interactions []storage.SubjectInteraction
	MessageIDs   []string
	Messages     []domain.Message
	Notices      []string
	Usage        recapRequestUsage
}

func (s *Service) prepareRecapContext(ctx context.Context, binding storage.SubjectHistoryBinding, sid domain.SessionID, current []domain.Message) (RecapContext, error) {
	var out RecapContext
	if s.deps.Recap == nil {
		return out, ErrRecapUnavailable
	}
	captured, err := s.CaptureSubjectContext(ctx, sid)
	if err != nil {
		return out, err
	}
	if authorized, err := s.AuthorizeSubjectHistory(ctx, sid); err != nil || authorized != binding {
		return out, storage.ErrSubjectBinding
	}
	head, err := s.deps.Recap.ReadRecap(ctx)
	if err != nil {
		return out, err
	}
	if head.Binding.SubjectID != binding.SubjectID || head.Binding.DomainID != binding.DomainID || head.CoveredThrough > captured.Through {
		return out, ErrRecapUnavailable
	}
	if err = validateRecapHeadCertificate(head, captured, binding); err != nil {
		return out, err
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return out, storage.ErrSubjectBinding
	}
	out.Head, out.Snapshot = head, captured
	seen := map[string]bool{}
	after := head.CoveredThrough
	for after < captured.Through {
		page, err := sources.ListSubjectInteractions(ctx, binding, after, captured.Through, 128)
		if err != nil {
			return RecapContext{}, err
		}
		if len(page) == 0 {
			return RecapContext{}, storage.ErrSubjectEvidenceIncomplete
		}
		for _, row := range page {
			if row.Sequence != after+1 || row.Sequence > captured.Through {
				return RecapContext{}, storage.ErrSubjectEvidenceIncomplete
			}
			if err = s.authorizeSubjectRun(ctx, sources, binding, row.SessionID, row.RunID); err != nil {
				return RecapContext{}, err
			}
			originals, err := sources.ReadSubjectInteraction(ctx, binding, row.ID)
			if err != nil {
				return RecapContext{}, err
			}
			digest, err := storage.SubjectEvidenceDigest(originals)
			if err != nil || digest != row.SourceDigest {
				return RecapContext{}, storage.ErrSubjectEvidenceIncomplete
			}
			projected, notices, err := projectAutomaticRecapOriginals(originals)
			if err != nil {
				return RecapContext{}, err
			}
			out.Notices = append(out.Notices, notices...)
			out.Notices = append(out.Notices, recapDispositionNotices(captured, row.Sequence)...)
			for _, msg := range projected {
				if msg.ID == "" || seen[msg.ID] {
					return RecapContext{}, storage.ErrSubjectEvidenceIncomplete
				}
				seen[msg.ID] = true
				out.MessageIDs = append(out.MessageIDs, msg.ID)
				out.Messages = append(out.Messages, msg)
			}
			out.Interactions = append(out.Interactions, row)
			after = row.Sequence
		}
	}
	// The active interaction is never folded or filtered. Notebook results from
	// this authorized live run remain exact; automatic history excludes them later.
	for _, msg := range current {
		if msg.ID == "" {
			return RecapContext{}, storage.ErrSubjectEvidenceIncomplete
		}
		if seen[msg.ID] {
			continue
		}
		seen[msg.ID] = true
		out.MessageIDs = append(out.MessageIDs, msg.ID)
		out.Messages = append(out.Messages, msg)
	}
	return out, nil
}

func validateRecapHeadCertificate(head cognitivecontract.RecapHead, snapshot storage.SubjectContextSnapshot, binding storage.SubjectHistoryBinding) error {
	if strings.TrimSpace(head.Text) == "" {
		return nil
	}
	var receipt *recapReceipt
	switch head.Cause {
	case "summary", "reconcile":
		decoded, err := decodeRecapReceipt(head.Sources.Digest)
		if err != nil {
			return err
		}
		if decoded.Binding != binding || decoded.Range.After != head.Sources.After || decoded.Range.Through != head.Sources.Through || decoded.Range.Through != head.CoveredThrough {
			return ErrRecapUnavailable
		}
		receipt = &decoded
	case "edit", "restore":
		if head.Sources != (cognitivecontract.SourceRange{}) {
			return ErrRecapUnavailable
		}
	default:
		return ErrRecapUnavailable
	}
	for _, d := range snapshot.Dispositions {
		for _, target := range d.Targets {
			if target.Sequence <= head.CoveredThrough && (receipt == nil || receipt.Dispositions.Digest != snapshot.DispositionDigest) {
				return ErrRecapReconciliationRequired
			}
		}
	}
	return nil
}

const recapAdmissionExtra = "vivy_recap_admission_v1"

type recapAdmission struct {
	Binding                  storage.SubjectHistoryBinding
	SessionID                domain.SessionID
	Revision, Epoch, Through uint64
	Sources                  []storage.SubjectSourceBinding
}

func (s *Service) runRecapMessages(ctx context.Context, sid domain.SessionID, userText, preamble string, eng *Engine) ([]*schema.Message, ContextStats, error) {
	binding, err := s.AuthorizeSubjectHistory(ctx, sid)
	if err != nil {
		return nil, ContextStats{}, err
	}
	stored, err := s.deps.Messages.ListMessages(ctx, sid)
	if err != nil {
		return nil, ContextStats{}, err
	}
	rid := contextRunID(ctx)
	var current []domain.Message
	for _, row := range stored {
		if row.RunID == rid && rid != "" {
			current = append(current, row)
		}
	}
	if len(current) == 0 {
		return nil, ContextStats{}, fmt.Errorf("%w: admitted current interaction missing", storage.ErrSubjectEvidenceIncomplete)
	}
	cut, err := s.prepareRecapContext(ctx, binding, sid, current)
	if err != nil {
		return nil, ContextStats{}, err
	}
	admission := recapAdmission{Binding: binding, SessionID: sid, Revision: cut.Head.Revision, Epoch: cut.Head.InvalidationEpoch, Through: cut.Snapshot.Through, Sources: cut.Snapshot.Sources}
	raw, err := json.Marshal(admission)
	if err != nil {
		return nil, ContextStats{}, err
	}
	block := schema.SystemMessage(fmt.Sprintf("[Non-authoritative Recap data; revision=%d]\n%s\n[End Recap data]", cut.Head.Revision, cut.Head.Text))
	block.Extra = map[string]any{recapAdmissionExtra: string(raw)}
	messages := []*schema.Message{schema.SystemMessage(preamble), block}
	for _, notice := range cut.Notices {
		messages = append(messages, schema.SystemMessage(notice))
	}
	references, err := s.sessionAttachedReferences(ctx, sid)
	if err != nil {
		return nil, ContextStats{}, err
	}
	feed, err := projectFeedWithContextBudget(ctx, eng.cfg.ContextHost, cut.Messages, nil, references)
	if err != nil {
		return nil, ContextStats{}, err
	}
	messages = append(messages, feed...)
	// Exact original expansion and mandatory Recap precede optional ContextHost
	// retrieval. Required source failures remain visible through the same Host.
	stats := ContextStats{OriginalHistoryMessages: len(cut.Messages) - len(current), IncludedHistoryMessages: len(cut.Messages) - len(current), Bytes: projectedContextBytes(messages)}
	return messages, stats, nil
}

func (s *Service) validateRecapModelLeg(ctx context.Context, sid domain.SessionID, rid domain.RunID, input modelCallInput) error {
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return nil
	}
	pin, err := sources.GetRunSubjectSource(ctx, rid)
	if err == storage.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if pin.ConversationID == "" {
		return nil
	}
	if err = s.authorizeSubjectRun(ctx, sources, pin.SubjectHistoryBinding, sid, rid); err != nil {
		return err
	}
	var admission *recapAdmission
	for _, msg := range input.Messages {
		if msg == nil {
			continue
		}
		encoded, found := msg.Extra[recapAdmissionExtra]
		if !found {
			continue
		}
		if admission != nil {
			return ErrRecapUnavailable
		}
		text, ok := encoded.(string)
		if !ok {
			return ErrRecapUnavailable
		}
		var a recapAdmission
		if json.Unmarshal([]byte(text), &a) != nil {
			return ErrRecapUnavailable
		}
		admission = &a
	}
	if admission == nil || admission.Binding != pin.SubjectHistoryBinding || admission.SessionID != sid {
		return ErrRecapUnavailable
	}
	for _, source := range admission.Sources {
		current, err := s.resolveSubjectAdmission(ctx, source.SessionID)
		if err != nil {
			return err
		}
		if current == nil || !storage.SameSubjectSourceBinding(source, *current) {
			return storage.ErrSubjectBinding
		}
	}
	if s.deps.Recap == nil {
		return ErrRecapUnavailable
	}
	head, err := s.deps.Recap.ReadRecap(ctx)
	if err != nil {
		return err
	}
	if head.Binding.SubjectID != pin.SubjectID || head.Binding.DomainID != pin.DomainID || head.InvalidationEpoch != admission.Epoch {
		return fmt.Errorf("%w: recap clear invalidated admission", ErrRecapUnavailable)
	}
	_, err = s.measureRecapRequest(ctx, input.Messages, input.Tools)
	return err
}
