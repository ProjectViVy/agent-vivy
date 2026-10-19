package runtime

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Called only after validation and durable model.request persistence. A summary
// model call does not mean the conversation model consumed that newer summary.
func (s *Service) noteLoadedRecap(rid domain.RunID, input modelCallInput) {
	for _, message := range input.Messages {
		if message == nil {
			continue
		}
		encoded, ok := message.Extra[recapAdmissionExtra].(string)
		if !ok {
			continue
		}
		var admission recapAdmission
		if json.Unmarshal([]byte(encoded), &admission) != nil {
			return
		}
		s.mu.Lock()
		if s.loadedRecaps == nil {
			s.loadedRecaps = make(map[domain.RunID]recapAdmission)
		}
		s.loadedRecaps[rid] = admission
		s.mu.Unlock()
		return
	}
}

func (s *Service) ReadRecapLoaded(ctx context.Context, sid domain.SessionID) (cognitivecontract.RecapLoaded, error) {
	unavailable := cognitivecontract.RecapLoaded{Status: "unavailable"}
	binding, err := s.AuthorizeRecap(ctx, sid, "", "")
	if err != nil {
		return unavailable, err
	}
	s.mu.Lock()
	var runID domain.RunID
	var pin recapAdmission
	for rid, loaded := range s.loadedRecaps {
		if loaded.SessionID == sid && loaded.Binding.SubjectID == binding.SubjectID && loaded.Binding.DomainID == binding.DomainID {
			if runID != "" {
				s.mu.Unlock()
				return unavailable, nil
			}
			runID, pin = rid, loaded
		}
	}
	s.mu.Unlock()
	if runID == "" || s.deps.Runs == nil {
		return unavailable, nil
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return unavailable, ErrRecapUnavailable
	}
	if err = s.authorizeSubjectRun(ctx, sources, pin.Binding, sid, runID); err != nil {
		return unavailable, err
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return unavailable, err
	}
	if run.Status != domain.RunActive {
		return unavailable, nil
	}
	if _, err = s.AuthorizeRecap(ctx, sid, "", ""); err != nil {
		return unavailable, err
	}
	return cognitivecontract.RecapLoaded{Status: "loaded", RunID: runID, Revision: pin.Revision, InvalidationEpoch: pin.Epoch}, nil
}
