package storage

import (
	"agent-vivy/internal/domain"
	"errors"
	"testing"
)

func TestIssue58EvidenceDigestRejectsInvalidText(t *testing.T) {
	for _, content := range []string{string([]byte{0xff}), string([]byte{0xfe})} {
		if _, err := SubjectEvidenceDigest([]domain.Message{{ID: "u", Content: content}}); !errors.Is(err, ErrSubjectEvidenceIncomplete) {
			t.Fatalf("invalid UTF-8 was silently normalized by digest: %v", err)
		}
	}
}
