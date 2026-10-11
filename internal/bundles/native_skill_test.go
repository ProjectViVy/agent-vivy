package bundles

import (
	"agent-vivy/internal/bundlecontract"
	"errors"
	"strings"
	"testing"
)

func TestNativeSkillRejectsAuthorityFields(t *testing.T) {
	s, err := DecodeNativeSkill([]byte("---\nname: Plan\ndescription: Create a plan\n---\nLiteral <&> body\n"))
	if err != nil || s.Name != "Plan" || s.Description != "Create a plan" || s.Body != "Literal <&> body\n" {
		t.Fatal(s, err)
	}
	for _, key := range []string{"tools", "agent", "model", "context", "always", "enabled"} {
		_, err := DecodeNativeSkill([]byte("---\ndescription: Good\n" + key + ": true\n---\nbody"))
		var e *bundlecontract.Error
		if !errors.As(err, &e) || e.Code != "invalid_manifest" {
			t.Fatal(key, err)
		}
	}
	for _, raw := range []string{"body", "---\nname: Plan\n---\nbody", "---\ndescription: first\ndescription: last\n---\nbody", "---\ndescription: &x Good\nname: *x\n---\nbody", "---\ndescription: null\n---\nbody"} {
		if _, err := DecodeNativeSkill([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
	if _, err := DecodeNativeSkill([]byte("---\ndescription: Good\n---\n" + strings.Repeat("a", 512<<10))); err == nil {
		t.Fatal("oversized skill")
	}
}
