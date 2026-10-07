package argumentguard

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/sdk/port/pretool"
)

func TestExplicitGuardRetainsOldHeuristics(t *testing.T) {
	for _, tc := range []struct {
		id, args string
		denied   bool
	}{
		{"execute", `{"command":"a && b"}`, true},
		{"execute", `{"command":"powershell -Command Get-ChildItem"}`, true},
		{"read_file", `{"path":"../secret.txt"}`, true},
		{"read_file", `{"path":"..\\secret.txt"}`, true},
		{"read_file", `{"path":"a\u0000b"}`, true},
		{"bash", `{"command":"a && b | c"}`, false},
		{"echo_info", `{"text":"../; | cmd.exe"}`, false},
		{"read_file", `{"path":"README.md"}`, false},
	} {
		decision, err := NewProvider().Evaluate(context.Background(), pretool.NewRequest(tc.id, json.RawMessage(tc.args)))
		if err != nil || !decision.Valid() || (decision.Kind == pretool.Deny) != tc.denied {
			t.Fatalf("%s %s decision=%+v err=%v", tc.id, tc.args, decision, err)
		}
	}
}
