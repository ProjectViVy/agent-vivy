package runtime

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// AuthorizeRecap is the private control facade's current whole-domain check.
// Tool calls additionally require the actual persisted resident run and its
// process-owned selected-tool/policy snapshot; transport equality is insufficient.
func (s *Service) AuthorizeRecap(ctx context.Context, sid domain.SessionID, rid domain.RunID, name string) (cognitivecontract.RecapBinding, error) {
	deny := func() (cognitivecontract.RecapBinding, error) {
		return cognitivecontract.RecapBinding{}, ErrPolicyDenied
	}
	if s == nil || sid == "" {
		return deny()
	}
	current, err := s.resolveSubjectAdmission(ctx, sid)
	if err != nil || current == nil || current.ConversationID == "" || !current.Grant.WholeDomainRead {
		return deny()
	}
	binding := cognitivecontract.RecapBinding{SubjectID: current.SubjectID, DomainID: current.DomainID}
	if name == "" {
		if rid != "" {
			return deny()
		}
		return binding, nil
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok || s.deps.Runs == nil || s.deps.Journal == nil || rid == "" {
		return deny()
	}
	pin, err := sources.GetRunSubjectSource(ctx, rid)
	if err != nil || !storage.SameSubjectSourceBinding(pin, *current) {
		return deny()
	}
	run, err := s.deps.Runs.GetRun(ctx, rid)
	if err != nil || run.SessionID != sid || run.Status != domain.RunActive || (run.Kind != "" && run.Kind != domain.RunKindPrimary) || run.ParentID != "" || run.Depth != 0 || run.Purpose != "" {
		return deny()
	}
	s.mu.Lock()
	snapshot := s.snapshots[rid]
	_, selected := s.runTools[rid][name]
	active := s.runSessions[rid] == sid && s.active[rid] != nil
	_, pending := s.pending[rid]
	eng := s.engine
	s.mu.Unlock()
	if !active || pending || !selected || snapshot.Profile == "" || snapshot.Hash == "" || eng == nil || eng.cfg.Policy == nil {
		return deny()
	}
	if tool := eng.toolByName[name]; tool == nil {
		return deny()
	}
	actual, err := eng.cfg.Policy.Snapshot(snapshot.Profile)
	if err != nil || actual.Hash != snapshot.Hash {
		return deny()
	}
	// The Journal is the durable policy pin, including after approval recovery.
	it, err := s.deps.Journal.Replay(ctx, rid, 0)
	if err != nil {
		return deny()
	}
	defer it.Close()
	pinned := false
	for it.Next() {
		ev := it.Value().Event
		if ev.Type == domain.EventRunStarted {
			var p payloadRunStarted
			if json.Unmarshal(ev.Payload, &p) != nil || p.PolicyProfile != string(snapshot.Profile) || p.PolicyHash != snapshot.Hash {
				return deny()
			}
			pinned = true
			break
		}
	}
	if !pinned || it.Err() != nil {
		return deny()
	}
	return binding, nil
}
