package runtime

import (
	"fmt"
	"sort"

	"agent-vivy/internal/domain"
)

// ResolveHistoryScope is a pure task-admission helper. Storage/workspace
// expansion and policy derivation must happen before this function is called.
// Empty selection grants only the current session when policy allows it.
func ResolveHistoryScope(current domain.SessionID, selected []domain.SessionID, policyAllowed []domain.SessionID) ([]domain.SessionID, error) {
	limits := domain.DefaultContinuityLimits()
	if current == "" {
		return nil, fmt.Errorf("history scope current session is required")
	}
	if len(selected) > limits.ExplicitSessions {
		return nil, fmt.Errorf("history scope has %d explicit sessions; maximum is %d", len(selected), limits.ExplicitSessions)
	}
	if len(policyAllowed) > limits.ExpandedSessions {
		return nil, fmt.Errorf("history policy has %d expanded sessions; maximum is %d", len(policyAllowed), limits.ExpandedSessions)
	}

	allowed := make(map[domain.SessionID]struct{}, len(policyAllowed))
	for _, id := range policyAllowed {
		if id == "" {
			return nil, fmt.Errorf("history policy contains an invalid session ID")
		}
		allowed[id] = struct{}{}
	}

	if len(selected) == 0 {
		if _, ok := allowed[current]; !ok {
			return nil, fmt.Errorf("history scope current session is not allowed")
		}
		return []domain.SessionID{current}, nil
	}

	resolved := make(map[domain.SessionID]struct{}, len(selected)+1)
	for _, id := range selected {
		if id == "" {
			return nil, fmt.Errorf("history scope contains an invalid session ID")
		}
		if _, duplicate := resolved[id]; duplicate {
			return nil, fmt.Errorf("history scope contains a duplicate session ID")
		}
		if _, ok := allowed[id]; !ok {
			return nil, fmt.Errorf("history scope selected session is not allowed")
		}
		resolved[id] = struct{}{}
	}
	if _, ok := allowed[current]; ok {
		resolved[current] = struct{}{}
	}

	result := make([]domain.SessionID, 0, len(resolved))
	for id := range resolved {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	if len(result) > limits.ExpandedSessions {
		return nil, fmt.Errorf("history scope has %d resolved sessions; maximum is %d", len(result), limits.ExpandedSessions)
	}
	return result, nil
}
