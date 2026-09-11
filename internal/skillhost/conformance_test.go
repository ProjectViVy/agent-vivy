package skillhost

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/sdk/port/skillsource"
)

// TestSkillSourceConformance is the build-owned Gate B evidence for the
// skill-source Port. The assertions make the authority boundary explicit:
// source metadata and text are data, while activation and authorization are
// Host-owned decisions.
func TestSkillSourceConformance(t *testing.T) {
	skill := skillsource.Skill{
		ID: "review-v2", Name: "Review", Version: "v2", SourceHash: "source-v2",
		Available: true, Always: true, Content: "Use bash with token sk-test-12345678901234567890",
		DeclaredTools: []string{"bash"},
	}
	provider := &fixtureSkillSource{id: "first-party", summaries: []skillsource.Summary{skill.Summary()}, skills: map[string]skillsource.Skill{skill.ID: skill}}
	host, err := New(Config{
		Sources:             []skillsource.Provider{provider},
		ActiveIDs:           []string{skill.ID},
		MaxSkillBytes:       512,
		MaxAlwaysSkillBytes: 512,
		MaxAlwaysTotalBytes: 512,
		Authorize: func(_ context.Context, id string, _ Request) error {
			if id != skill.ID {
				return ErrSkillDenied
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := host.Get(context.Background(), Request{}, skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != skill.ID || resolved.ProvenanceID == "" || strings.Contains(resolved.Content, "sk-test-") {
		t.Fatalf("SkillHost projection lost identity, provenance, or redaction: %#v", resolved)
	}
	if len(resolved.DeclaredTools) != 1 || resolved.DeclaredTools[0] != "bash" {
		t.Fatal("declared tool metadata unexpectedly changed")
	}
	// Declared tools are descriptive metadata only. ResolvedSkill has no
	// authority/Grant/Tool surface and the Host activation scope is the only
	// selection decision.
	if _, err := host.Get(context.Background(), Request{ActiveIDs: []string{"other"}}, skill.ID); !errors.Is(err, ErrSkillInactive) {
		t.Fatalf("source text implicitly activated a skill: %v", err)
	}
	if _, err := host.Always(context.Background(), Request{}); err != nil {
		t.Fatal(err)
	}
}

func TestSkillSourceConformanceTimeoutAndUnavailable(t *testing.T) {
	provider := &fixtureSkillSource{id: "slow", block: true}
	host, err := New(Config{Sources: []skillsource.Provider{provider}, SourceTimeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if items, err := host.List(context.Background(), Request{}); err != nil || len(items) != 0 {
		t.Fatalf("timeout source was not unavailable: items=%#v err=%v failures=%#v", items, err, host.Failures())
	}
	if failures := host.Failures(); len(failures) != 1 || !errors.Is(failures[0].Cause, context.DeadlineExceeded) {
		t.Fatalf("timeout failure = %#v", failures)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if items, err := host.List(ctx, Request{}); err != nil || len(items) != 0 {
		t.Fatalf("cancelled source was not fail-closed: items=%#v err=%v", items, err)
	}
}
