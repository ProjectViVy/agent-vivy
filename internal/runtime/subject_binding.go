package runtime

import (
	"context"
	"errors"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// RecapAvailable reports whether Recap management can ever be authorized: it
// needs a trusted main-conversation resolver and the Recap owner port. Hosts
// publish it as a capability so faces never offer a surface that always denies.
func (s *Service) RecapAvailable() bool {
	return s != nil && s.deps.Recap != nil && s.deps.MainConversation != nil
}

// resolveSubjectAdmission consumes trusted Host decisions on the existing Run
// path. No RunOptions or browser scope field can select resident LIFE policy.
func (s *Service) resolveSubjectAdmission(ctx context.Context, sid domain.SessionID) (*storage.SubjectSourceBinding, error) {
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		if s.deps.MainConversation != nil || s.deps.SubjectSources != nil {
			return nil, storage.ErrSubjectBinding
		}
		return nil, nil
	}
	var resolved *storage.SubjectSourceBinding
	if s.deps.MainConversation != nil {
		main, err := s.deps.MainConversation.ResolveMainConversation(ctx)
		if err != nil {
			return nil, err
		}
		candidate := storage.SubjectSourceBinding{SubjectHistoryBinding: storage.SubjectHistoryBinding{SubjectID: main.SubjectID, DomainID: main.DomainID}, SessionID: main.ActiveSessionID, SchemaVersion: main.SchemaVersion, ConversationID: main.ConversationID, PolicyID: main.PolicyID, ToolPolicyRef: main.ToolPolicyRef, Scope: "personal", Grant: main.Grant}
		if candidate.Validate() != nil || candidate.ConversationID == "" {
			return nil, storage.ErrSubjectBinding
		}
		if main.ActiveSessionID == sid {
			candidate.Grant.Snapshot = append([]byte(nil), candidate.Grant.Snapshot...)
			resolved = &candidate
		}
	}
	if resolved == nil && s.deps.SubjectSources != nil {
		source, err := s.deps.SubjectSources(ctx, sid)
		if err != nil {
			return nil, err
		}
		if source != nil {
			if source.Validate() != nil || source.SessionID != sid || source.ConversationID != "" || source.Grant.WholeDomainRead {
				return nil, storage.ErrSubjectBinding
			}
			copy := *source
			copy.Grant.Snapshot = append([]byte(nil), source.Grant.Snapshot...)
			resolved = &copy
		}
	}
	existing, err := sources.GetSubjectSource(ctx, sid)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if err == nil && (resolved == nil || !storage.SameSubjectSourceBinding(existing, *resolved)) {
		return nil, storage.ErrSubjectBinding
	}
	return resolved, nil
}

// AuthorizeSubjectHistory rechecks the current trusted whole-domain grant for
// resident history readers. Contribution-only ordinary sources always fail.
// Storage bindings remain internal coordinates, never authorization tokens.
func (s *Service) AuthorizeSubjectHistory(ctx context.Context, sid domain.SessionID) (storage.SubjectHistoryBinding, error) {
	source, err := s.resolveSubjectAdmission(ctx, sid)
	if err != nil {
		return storage.SubjectHistoryBinding{}, err
	}
	if source == nil || source.ConversationID == "" || !source.Grant.WholeDomainRead {
		return storage.SubjectHistoryBinding{}, storage.ErrSubjectBinding
	}
	return source.SubjectHistoryBinding, nil
}

// retainedSubjectSource checks durable protection even when a restarted Host
// has no resolver. Callers holding projectionMu close the race with admission.
func (s *Service) retainedSubjectSource(ctx context.Context, sid domain.SessionID) error {
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return nil
	}
	_, err := sources.GetSubjectSource(ctx, sid)
	if err == nil {
		return storage.ErrRetainedSession
	}
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	return err
}
