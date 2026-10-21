package skillhost

import (
	"context"
	"testing"

	"agent-vivy/sdk/port/skillsource"
)

func TestSkillSourceUsesExplicitVersionAndActualContent(t *testing.T) {
	skill := skillsource.Skill{ID: "review", Name: "review", Version: "v1", Available: true, Content: "read only instructions"}
	source := &fixtureSkillSource{id: "source", summaries: []skillsource.Summary{skill.Summary()}, skills: map[string]skillsource.Skill{skill.ID: skill}}
	host, err := New(Config{Sources: []skillsource.Provider{source}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := host.List(context.Background(), Request{})
	if err != nil || len(items) != 1 {
		t.Fatalf("list without digest: %v / %#v", err, items)
	}
	got, err := host.Get(context.Background(), Request{}, skill.ID)
	if err != nil || got.Content != skill.Content || got.Version != "v1" || got.SourceID != "source" {
		t.Fatalf("get without digest: %v / %#v", err, got)
	}
}
