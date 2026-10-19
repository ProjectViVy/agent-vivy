package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// sealSubjectRunLocked repairs the existing derived projection before asking
// core storage to validate/hash canonical evidence. Missing evidence leaves the
// terminal source protected and unsealed; a later authorized read retries it.
func (s *Service) sealSubjectRunLocked(ctx context.Context, sid domain.SessionID, rid domain.RunID) error {
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
	current, err := s.resolveSubjectAdmission(ctx, sid)
	if err != nil {
		return err
	}
	if current == nil || !storage.SameSubjectSourceBinding(pin, *current) {
		return storage.ErrSubjectBinding
	}
	if err := s.projectRunMessagesLocked(ctx, sid, rid); err != nil && !errors.Is(err, storage.ErrSubjectEvidenceUnresolvable) {
		return err
	}
	_, interaction, err := sources.ReadSubjectRunEvidence(ctx, pin.SubjectHistoryBinding, rid)
	unresolvable := errors.Is(err, storage.ErrSubjectEvidenceUnresolvable)
	if err != nil && !unresolvable {
		return err
	}
	return sealOrExclude(ctx, sources, pin.SubjectHistoryBinding, interaction, unresolvable)
}

// sealOrExclude seals the run's exact originals, or, when its evidence can never
// resolve (unfinished tool operation, invalid tool-argument bytes), seals it as
// an excluded interaction so the subject's history keeps advancing. Storage
// re-derives the condition; the flag is only the caller's reading of it.
func sealOrExclude(ctx context.Context, sources storage.SubjectSourceStore, binding storage.SubjectHistoryBinding, interaction storage.SubjectInteraction, unresolvable bool) error {
	var err error
	if unresolvable {
		_, err = sources.SealSubjectExclusion(ctx, binding, interaction)
	} else {
		_, err = sources.SealSubjectInteraction(ctx, binding, interaction)
	}
	return err
}

// RepairSubjectHistory retries retained terminal runs after projection/seal
// interruption. Current Host grants are required for every source independently.
func (s *Service) RepairSubjectHistory(ctx context.Context, sid domain.SessionID) error {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if _, err := s.resolveSubjectAdmission(ctx, sid); err != nil {
		return err
	}
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sid)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if !run.Status.Terminal() {
			continue
		}
		if err := s.sealSubjectRunLocked(ctx, sid, run.ID); err != nil {
			return fmt.Errorf("repair subject run %s: %w", run.ID, err)
		}
	}
	return nil
}

// SubjectRecent returns metadata in immutable index order across authorized
// sources. Bodies remain in the source stores, expanded only by exact reads.
func (s *Service) SubjectRecent(ctx context.Context, reader domain.SessionID, after, through uint64, limit int) ([]storage.SubjectInteraction, error) {
	binding, err := s.AuthorizeSubjectHistory(ctx, reader)
	if err != nil {
		return nil, err
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return nil, storage.ErrSubjectBinding
	}
	rows, err := sources.ListSubjectInteractions(ctx, binding, after, through, limit)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, err := s.resolveSubjectAdmission(ctx, row.SessionID); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// SubjectOriginalView keeps immutable originals/digest separate from mutable
// disposition derived from the existing rewind/edit markers.
type SubjectOriginalView struct {
	InteractionID string
	RunID         domain.RunID
	SessionID     domain.SessionID
	SourceDigest  string
	Outcome       domain.RunStatus
	Messages      []domain.Message
	Disposition   map[string]string
	Supersession  []storage.SessionTruncation
}

func (s *Service) ReadSubjectOriginal(ctx context.Context, reader domain.SessionID, id string) (SubjectOriginalView, error) {
	var view SubjectOriginalView
	binding, err := s.AuthorizeSubjectHistory(ctx, reader)
	if err != nil {
		return view, err
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return view, storage.ErrSubjectBinding
	}
	split := strings.LastIndex(id, ":")
	if split <= 0 {
		return view, storage.ErrSubjectBinding
	}
	rid := domain.RunID(id[:split])
	pin, err := sources.GetRunSubjectSource(ctx, rid)
	if err != nil {
		return view, err
	}
	if pin.SubjectHistoryBinding != binding {
		return view, storage.ErrSubjectBinding
	}
	current, err := s.resolveSubjectAdmission(ctx, pin.SessionID)
	if err != nil {
		return view, err
	}
	if current == nil || !storage.SameSubjectSourceBinding(pin, *current) {
		return view, storage.ErrSubjectBinding
	}
	rows, err := sources.ReadSubjectInteraction(ctx, binding, id)
	if err != nil {
		return view, err
	}
	run, err := s.deps.Runs.GetRun(ctx, rid)
	if err != nil {
		return view, err
	}
	digest, err := storage.SubjectEvidenceDigest(rows)
	if err != nil {
		return view, err
	}
	view = SubjectOriginalView{InteractionID: id, RunID: rid, SessionID: pin.SessionID, SourceDigest: digest, Outcome: run.Status, Messages: rows, Disposition: map[string]string{}}
	if s.deps.Truncations == nil {
		return view, ErrRewindNotWired
	}
	stored, err := s.deps.Messages.ListMessages(ctx, pin.SessionID)
	if err != nil {
		return view, err
	}
	markers, err := s.deps.Truncations.ListViewTruncations(ctx, pin.SessionID)
	if err != nil {
		return view, err
	}
	visible := map[string]bool{}
	for _, m := range storage.ApplySessionTruncations(stored, markers) {
		visible[m.ID] = true
	}
	for _, m := range rows {
		view.Disposition[m.ID] = "included"
		if !visible[m.ID] {
			view.Disposition[m.ID] = "superseded"
		}
	}
	view.Supersession = markers
	return view, nil
}

// ArchiveSubjectSource is an internal source-management operation, independent
// of user-facing session deletion and resident conversation creation.
func (s *Service) ArchiveSubjectSource(ctx context.Context, sid domain.SessionID) error {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	source, err := s.resolveSubjectAdmission(ctx, sid)
	if err != nil {
		return err
	}
	if source == nil {
		return storage.ErrSubjectBinding
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return storage.ErrSubjectBinding
	}
	pin, err := sources.GetSubjectSource(ctx, sid)
	if err != nil {
		return err
	}
	if !storage.SameSubjectSourceBinding(pin, *source) {
		return storage.ErrSubjectBinding
	}
	return sources.ArchiveRetainedSession(ctx, sid)
}
