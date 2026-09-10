package runtime

import (
	"testing"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
)

func TestHostedFileContextPartsTraverseContextHost(t *testing.T) {
	parts := hostedFileContextParts([]domain.FileContext{{
		Path: "docs/readme.md", Name: "readme.md", Size: 5, Content: []byte("hello"),
	}})
	if len(parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(parts))
	}
	if parts[0].Text != "\n\n[project file: docs/readme.md]\nhello" {
		t.Fatalf("part text = %q", parts[0].Text)
	}
}

func TestHostedFileContextPartsRejectPathEscape(t *testing.T) {
	parts := hostedFileContextParts([]domain.FileContext{{
		Path: "../secret.txt", Name: "secret.txt", Size: 6, Content: []byte("secret"),
	}})
	if len(parts) != 0 {
		t.Fatalf("path escape produced Eino parts: %#v", parts)
	}
}

func TestContextCandidateAdapterUsesOnlyApprovedContent(t *testing.T) {
	candidate := contexthost.Candidate{}
	candidate.SourceID = "docs"
	candidate.ContentID = "one"
	candidate.Content = "approved"
	candidate.ProvenanceID = "proof"
	parts := contextCandidatesToUserParts([]contexthost.Candidate{candidate})
	if len(parts) != 1 || parts[0].Text != "\n\n[context: docs/one provenance=proof]\napproved" {
		t.Fatalf("candidate parts = %#v", parts)
	}
}
