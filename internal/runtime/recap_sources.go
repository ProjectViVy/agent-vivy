package runtime

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"github.com/cloudwego/eino/schema"
)

// ReadRecapSources expands only coordinates certified by this historical owner
// revision. It never asks a memory index or model to reconstruct an original.
func (s *Service) ReadRecapSources(ctx context.Context, sid domain.SessionID, rid domain.RunID, tool string, revision uint64) ([]cognitivecontract.RecapSource, error) {
	binding, err := s.AuthorizeRecap(ctx, sid, rid, tool)
	if err != nil {
		return nil, err
	}
	if s.deps.Recap == nil || revision == 0 {
		return nil, ErrRecapUnavailable
	}
	head, err := s.deps.Recap.ReadRecapRevision(ctx, revision)
	if err != nil {
		return nil, err
	}
	if head.Binding != binding || head.Revision != revision {
		return nil, storage.ErrSubjectBinding
	}
	// Restore carries a durable link to its text's historical revision, not a
	// new coverage certificate. Follow only that link; human edits remain unproven.
	for head.Cause == "restore" {
		if head.RestoredFrom == 0 || head.RestoredFrom >= head.Revision {
			return nil, ErrRecapUnavailable
		}
		target := head.RestoredFrom
		head, err = s.deps.Recap.ReadRecapRevision(ctx, target)
		if err != nil {
			return nil, err
		}
		if head.Binding != binding || head.Revision != target {
			return nil, storage.ErrSubjectBinding
		}
	}
	receipt, err := decodeRecapReceipt(head.Sources.Digest)
	if err != nil {
		return nil, err
	}
	b := storage.SubjectHistoryBinding{SubjectID: binding.SubjectID, DomainID: binding.DomainID}
	if receipt.Binding != b || receipt.Range.After != head.Sources.After || receipt.Range.Through != head.Sources.Through || receipt.Range.Through != head.CoveredThrough {
		return nil, ErrRecapUnavailable
	}
	snapshot, err := s.CaptureSubjectContext(ctx, sid)
	if err != nil {
		return nil, err
	}
	if receipt.Range.Through > snapshot.Through || validateRecapCorrectionReferences(receipt.CorrectionRefs, snapshot) != nil {
		return nil, ErrRecapUnavailable
	}
	rows := []storage.SubjectInteraction{}
	after := receipt.Range.After
	for after < receipt.Range.Through {
		page, err := s.SubjectRecent(ctx, sid, after, receipt.Range.Through, 128)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		for _, row := range page {
			if row.Sequence != after+1 || row.Sequence > receipt.Range.Through {
				return nil, storage.ErrSubjectEvidenceIncomplete
			}
			rows = append(rows, row)
			after = row.Sequence
		}
	}
	out := []cognitivecontract.RecapSource{}
	positions := map[string]int{}
	appendOriginal := func(row storage.SubjectInteraction, selected []string) ([]domain.Message, error) {
		original, err := s.ReadSubjectOriginal(ctx, sid, row.ID)
		if err != nil {
			return nil, err
		}
		if original.SourceDigest != row.SourceDigest || original.RunID != row.RunID || original.SessionID != row.SessionID {
			return nil, storage.ErrSubjectEvidenceIncomplete
		}
		messages := original.Messages
		if selected != nil {
			messages = []domain.Message{}
			for _, id := range selected {
				found := false
				for _, message := range original.Messages {
					if message.ID == id {
						messages = append(messages, message)
						found = true
						break
					}
				}
				if !found {
					return nil, storage.ErrSubjectEvidenceIncomplete
				}
			}
		}
		index, exists := positions[row.ID]
		if !exists {
			index = len(out)
			positions[row.ID] = index
			out = append(out, cognitivecontract.RecapSource{InteractionID: row.ID, SourceRevision: head.Revision, SessionID: row.SessionID, RunID: row.RunID, SourceDigest: row.SourceDigest, Disposition: original.Disposition, Outcome: original.Outcome})
		}
		if out[index].References == nil {
			for _, message := range messages {
				if message.Role == domain.RoleUser {
					refs, err := s.sessionAttachedReferences(ctx, row.SessionID)
					if err != nil {
						return nil, err
					}
					out[index].References = refs[row.RunID]
					break
				}
			}
		}
		seen := map[string]bool{}
		for _, message := range out[index].Messages {
			seen[message.ID] = true
		}
		for _, message := range messages {
			if !seen[message.ID] {
				out[index].Messages = append(out[index].Messages, message)
				seen[message.ID] = true
			}
		}
		return original.Messages, nil
	}
	var exact []domain.Message
	for _, row := range rows {
		originals, err := appendOriginal(row, nil)
		if err != nil {
			return nil, err
		}
		exact = append(exact, originals...)
	}
	if !recapRangeDigestMatches(rows, exact, receipt.Range.ExactOriginalsDigest) {
		return nil, storage.ErrSubjectEvidenceIncomplete
	}
	for _, correction := range receipt.CorrectionRefs {
		for _, target := range correction.Targets {
			row := storage.SubjectInteraction{ID: target.InteractionID, SessionID: target.SourceSessionID, RunID: target.RunID, SourceDigest: target.SourceDigest}
			if _, err := appendOriginal(row, target.MessageIDs); err != nil {
				return nil, err
			}
			index := positions[target.InteractionID]
			out[index].Corrections = append(out[index].Corrections, cognitivecontract.RecapSourceCorrection{ViewSessionID: correction.SessionID, Reason: string(correction.Marker.Reason), MessageIDs: append([]string(nil), target.MessageIDs...), ViewMessageIDs: append([]string(nil), target.ViewMessageIDs...)})
		}
	}
	correction, err := s.recapCorrectionInput(ctx, b, receipt.CorrectionRefs)
	if err != nil {
		return nil, err
	}
	correctionRaw, err := json.Marshal(struct {
		References []storage.SubjectDisposition
		Input      []*schema.Message
	}{receipt.CorrectionRefs, correction})
	if err != nil || sha256Hex(correctionRaw) != receipt.CorrectionEvidenceDigest {
		return nil, storage.ErrSubjectEvidenceIncomplete
	}
	// Recheck the requesting authority after the expansion, including tool/run
	// pins. Each source read above independently checked its current source grant.
	if current, err := s.AuthorizeRecap(ctx, sid, rid, tool); err != nil || current != binding {
		return nil, storage.ErrSubjectBinding
	}
	return out, nil
}

func recapRangeDigestMatches(rows []storage.SubjectInteraction, originals []domain.Message, expected string) bool {
	digest := func(rows []storage.SubjectInteraction) string {
		raw, err := json.Marshal(struct {
			Interactions []storage.SubjectInteraction
			Originals    []domain.Message
		}{rows, originals})
		if err != nil {
			return ""
		}
		return sha256Hex(raw)
	}
	if digest(rows) == expected {
		return true
	}
	// v1 folds retain a nil or a zero-length range depending on whether an
	// uncovered suffix existed. Both encode the same empty coordinate set.
	return len(rows) == 0 && digest(nil) == expected
}
