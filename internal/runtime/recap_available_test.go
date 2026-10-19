package runtime

import (
	"testing"

	"agent-vivy/internal/cognitivecontract"
)

type availableResolver struct {
	cognitivecontract.MainConversationResolver
}
type availableRecap struct{ cognitivecontract.RecapPort }

func TestRecapAvailableNeedsResolverAndOwner(t *testing.T) {
	for name, tc := range map[string]struct {
		deps ServiceDeps
		want bool
	}{
		"none":          {ServiceDeps{}, false},
		"owner only":    {ServiceDeps{Recap: availableRecap{}}, false},
		"resolver only": {ServiceDeps{MainConversation: availableResolver{}}, false},
		"both":          {ServiceDeps{Recap: availableRecap{}, MainConversation: availableResolver{}}, true},
	} {
		if got := (&Service{deps: tc.deps}).RecapAvailable(); got != tc.want {
			t.Errorf("%s: RecapAvailable = %v, want %v", name, got, tc.want)
		}
	}
	if (*Service)(nil).RecapAvailable() {
		t.Error("nil service reported Recap available")
	}
}
