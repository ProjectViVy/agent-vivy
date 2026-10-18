package runtime

import (
	"context"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// CaptureSubjectContext reuses the existing projection lock and recovery
// owner. It is a producer only; no provider invocation or acknowledgement lives
// here. Source membership is never substituted for the current Host grant.
func (s *Service) CaptureSubjectContext(ctx context.Context, reader domain.SessionID) (storage.SubjectContextSnapshot, error) {
	binding, err := s.AuthorizeSubjectHistory(ctx, reader)
	if err != nil {
		return storage.SubjectContextSnapshot{}, err
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return storage.SubjectContextSnapshot{}, storage.ErrSubjectBinding
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	pending, err := sources.ListSubjectPendingRuns(ctx, binding)
	if err != nil {
		return storage.SubjectContextSnapshot{}, err
	}
	// First authorize and repair ALL selected projections. Sealing a repaired A
	// before projecting B would trip the existing terminal-prefix evidence guard.
	for _, p := range pending {
		if err = s.authorizeSubjectRun(ctx, sources, binding, p.SessionID, p.RunID); err != nil {
			return storage.SubjectContextSnapshot{}, err
		}
		if err = s.projectRunMessagesLocked(ctx, p.SessionID, p.RunID); err != nil && !errors.Is(err, storage.ErrSubjectEvidenceUnresolvable) {
			return storage.SubjectContextSnapshot{}, fmt.Errorf("repair subject run %s: %w", p.RunID, err)
		}
	}
	for _, p := range pending {
		if err = s.authorizeSubjectRun(ctx, sources, binding, p.SessionID, p.RunID); err != nil {
			return storage.SubjectContextSnapshot{}, err
		}
		_, interaction, err := sources.ReadSubjectRunEvidence(ctx, binding, p.RunID)
		unresolvable := errors.Is(err, storage.ErrSubjectEvidenceUnresolvable)
		if err != nil && !unresolvable {
			return storage.SubjectContextSnapshot{}, err
		}
		if interaction.TerminalSeq != p.TerminalSeq || interaction.SessionID != p.SessionID {
			return storage.SubjectContextSnapshot{}, storage.ErrSubjectEvidenceIncomplete
		}
		if err = sealOrExclude(ctx, sources, binding, interaction, unresolvable); err != nil {
			return storage.SubjectContextSnapshot{}, err
		}
	}
	snapshot, err := sources.CaptureSubjectContext(ctx, binding)
	if err != nil {
		return storage.SubjectContextSnapshot{}, err
	}
	// Immutable storage pins permit fresh per-participant authorization after the
	// SQL cut, including targets admitted since preflight. Never return a partial
	// snapshot when any current grant or an exact intermediate copy link fails.

	for _, pin := range snapshot.Sources {
		current, err := s.resolveSubjectAdmission(ctx, pin.SessionID)
		if err != nil {
			return storage.SubjectContextSnapshot{}, err
		}
		if pin.SubjectHistoryBinding != binding || current == nil || !storage.SameSubjectSourceBinding(pin, *current) {
			return storage.SubjectContextSnapshot{}, storage.ErrSubjectBinding
		}
	}
	for _, d := range snapshot.Dispositions {
		if err = s.authorizeSubjectSource(ctx, sources, binding, d.SessionID); err != nil {
			return storage.SubjectContextSnapshot{}, err
		}
		if d.Marker.RunID != "" {
			if err = s.authorizeSubjectRun(ctx, sources, binding, d.SessionID, d.Marker.RunID); err != nil {
				return storage.SubjectContextSnapshot{}, err
			}
		}
		for _, target := range d.Targets {
			if err = s.authorizeSubjectRun(ctx, sources, binding, target.SourceSessionID, target.RunID); err != nil {
				return storage.SubjectContextSnapshot{}, err
			}
			if len(target.MessageIDs) != len(target.ViewMessageIDs) {
				return storage.SubjectContextSnapshot{}, storage.ErrSubjectEvidenceIncomplete
			}
			for i, id := range target.ViewMessageIDs {
				if err = s.authorizeDispositionCopy(ctx, sources, binding, d.SessionID, id, target.SourceSessionID, target.MessageIDs[i]); err != nil {
					return storage.SubjectContextSnapshot{}, err
				}
			}
		}
	}
	if current, err := s.AuthorizeSubjectHistory(ctx, reader); err != nil {
		return storage.SubjectContextSnapshot{}, err
	} else if current != binding {
		return storage.SubjectContextSnapshot{}, storage.ErrSubjectBinding
	}
	return snapshot, nil
}
func (s *Service) authorizeSubjectSource(ctx context.Context, sources storage.SubjectSourceStore, b storage.SubjectHistoryBinding, sid domain.SessionID) error {
	pin, err := sources.GetSubjectSource(ctx, sid)
	if err != nil {
		return err
	}
	current, err := s.resolveSubjectAdmission(ctx, sid)
	if err != nil {
		return err
	}
	if pin.SubjectHistoryBinding != b || current == nil || !storage.SameSubjectSourceBinding(pin, *current) {
		return storage.ErrSubjectBinding
	}
	return nil
}
func (s *Service) authorizeSubjectRun(ctx context.Context, sources storage.SubjectSourceStore, b storage.SubjectHistoryBinding, sid domain.SessionID, rid domain.RunID) error {
	if err := s.authorizeSubjectSource(ctx, sources, b, sid); err != nil {
		return err
	}
	pin, err := sources.GetRunSubjectSource(ctx, rid)
	if err != nil {
		return err
	}
	source, err := sources.GetSubjectSource(ctx, sid)
	if err != nil {
		return err
	}
	if pin.SubjectHistoryBinding != b || pin.SessionID != sid || !storage.SameSubjectSourceBinding(pin, source) {
		return storage.ErrSubjectBinding
	}
	return nil
}
func (s *Service) authorizeDispositionCopy(ctx context.Context, sources storage.SubjectSourceStore, b storage.SubjectHistoryBinding, sid domain.SessionID, id string, wantSID domain.SessionID, wantID string) error {
	copies, ok := s.deps.Messages.(storage.MessageCopySourceStore)
	if !ok {
		return storage.ErrSubjectEvidenceIncomplete
	}
	seen := map[string]bool{}
	for depth := 0; depth < 128; depth++ {
		key := string(sid) + "\x00" + id
		if seen[key] {
			return storage.ErrSubjectEvidenceIncomplete
		}
		seen[key] = true
		if err := s.authorizeSubjectSource(ctx, sources, b, sid); err != nil {
			return err
		}
		link, err := copies.GetMessageCopySource(ctx, sid, id)
		if errors.Is(err, storage.ErrNotFound) {
			if sid != wantSID || id != wantID {
				return storage.ErrSubjectEvidenceIncomplete
			}
			return nil
		}
		if err != nil {
			return err
		}
		sid, id = link.SessionID, link.MessageID
	}
	return storage.ErrSubjectEvidenceIncomplete
}
