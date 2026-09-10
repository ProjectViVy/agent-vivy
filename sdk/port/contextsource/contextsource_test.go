package contextsource

import (
	"reflect"
	"testing"
)

func TestSourceCannotInjectSystemMessage(t *testing.T) {
	typeOfCandidate := reflect.TypeOf(Candidate{})
	for _, forbidden := range []string{"Role", "System", "Message", "Prompt", "Tool", "Model"} {
		if _, ok := typeOfCandidate.FieldByName(forbidden); ok {
			t.Fatalf("Candidate exposes forbidden execution/message field %s", forbidden)
		}
	}
}

func TestCandidateCopiesMetadata(t *testing.T) {
	candidate := NewCandidate(Candidate{
		SourceID:  "acme.docs",
		ContentID: "guide/intro",
		MediaType: "text/markdown",
		Content:   "hello",
		Metadata:  map[string]string{"path": "guide/intro.md"},
	})
	candidate.Metadata["path"] = "mutated"
	cloned := candidate.Clone()
	candidate.Metadata["path"] = "mutated-again"
	if cloned.Metadata["path"] != "mutated" {
		t.Fatalf("Clone metadata shares caller map: %#v", cloned.Metadata)
	}
}

func TestPageCopiesCandidates(t *testing.T) {
	items := []Candidate{{SourceID: "acme.docs", ContentID: "one", Content: "one"}}
	page := NewPage(items, "next")
	items[0].Content = "changed"
	if page.Candidates[0].Content != "one" || page.NextCursor != "next" {
		t.Fatalf("page mutated through caller slice: %#v", page)
	}
}
