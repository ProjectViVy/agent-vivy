package skillsource

import (
	"reflect"
	"testing"
)

func TestSkillTextReceivesNoImplicitGrant(t *testing.T) {
	typeOfSkill := reflect.TypeOf(Skill{})
	for _, forbidden := range []string{"Host", "Tool", "Tools", "Filesystem", "Network", "Secret", "Grant", "ModelHandle", "AgentHandle"} {
		if _, ok := typeOfSkill.FieldByName(forbidden); ok {
			t.Fatalf("Skill exposes implicit authority field %s", forbidden)
		}
	}
}

func TestSkillCloneCopiesSlicesAndMetadata(t *testing.T) {
	skill := Skill{
		ID: "review", Name: "review", Description: "Review code", Version: "v1", SourceHash: "abc", Content: "body",
		Dependencies: []string{"base"}, Metadata: map[string]string{"origin": "test"},
	}
	clone := skill.Clone()
	skill.Dependencies[0] = "changed"
	skill.Metadata["origin"] = "changed"
	if clone.Dependencies[0] != "base" || clone.Metadata["origin"] != "test" {
		t.Fatalf("clone shares mutable fields: %#v", clone)
	}
}

func TestSummaryContainsNoContent(t *testing.T) {
	typeOfSummary := reflect.TypeOf(Summary{})
	if _, ok := typeOfSummary.FieldByName("Content"); ok {
		t.Fatal("skill Summary must not expose content")
	}
}

func TestSkillClonePreservesLiteralIdentityForHostValidation(t *testing.T) {
	skill := Skill{ID: " review ", Name: "review", Version: " v1 ", SourceHash: "hash"}
	clone := skill.Clone()
	if clone.ID != skill.ID || clone.Version != skill.Version {
		t.Fatalf("Clone normalized stable identity/version: %#v", clone)
	}
}
