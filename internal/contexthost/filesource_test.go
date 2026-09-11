package contexthost

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/contextsource"
)

func TestFileSnapshotSourceRejectsWorkspaceEscape(t *testing.T) {
	source := NewFileSnapshotSource("vivy.project-files", []domain.FileContext{{
		Path: "../secret.txt", Name: "secret.txt", Size: 6, Content: []byte("secret"),
	}})
	page, err := source.Query(context.Background(), contextsource.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Candidates) != 0 {
		t.Fatalf("workspace escape produced candidates: %#v", page.Candidates)
	}
}

func TestFileSnapshotSourceRejectsPathControlsBeforeNormalization(t *testing.T) {
	source := NewFileSnapshotSource("vivy.project-files", []domain.FileContext{{
		Path: " \nmain.go ", Name: "main.go", Size: 1, Content: []byte("x"),
	}})
	page, err := source.Query(context.Background(), contextsource.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Candidates) != 0 {
		t.Fatalf("path control was normalized into a candidate: %#v", page.Candidates)
	}
}

func TestFileSnapshotSourcePreservesSnapshotOrder(t *testing.T) {
	source := NewFileSnapshotSource("vivy.project-files", []domain.FileContext{
		{Path: "b.txt", Name: "b.txt", Size: 1, Content: []byte("b")},
		{Path: "a.txt", Name: "a.txt", Size: 1, Content: []byte("a")},
	})
	page, err := source.Query(context.Background(), contextsource.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Candidates) != 2 || page.Candidates[0].ContentID != "b.txt" || page.Candidates[1].ContentID != "a.txt" {
		t.Fatalf("snapshot order changed: %#v", page.Candidates)
	}
}
