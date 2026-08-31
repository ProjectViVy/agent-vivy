package runtime

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
)

func TestNormalizeFace(t *testing.T) {
	t.Run("empty defaults to web", func(t *testing.T) {
		got, err := normalizeFace("")
		if err != nil || got != domain.FaceWeb {
			t.Fatalf("normalizeFace of empty = (%q, %v), want web, nil", got, err)
		}
	})
	for _, face := range []domain.Face{domain.FaceWeb, domain.FaceTui, domain.FaceCode} {
		t.Run("accepts "+string(face), func(t *testing.T) {
			got, err := normalizeFace(face)
			if err != nil || got != face {
				t.Fatalf("normalizeFace(%q) = (%q, %v), want (%q, nil)", face, got, err, face)
			}
		})
	}
	t.Run("rejects unknown", func(t *testing.T) {
		_, err := normalizeFace("execute")
		if !errors.Is(err, ErrInvalidFace) {
			t.Fatalf("error = %v, want ErrInvalidFace", err)
		}
	})
}

func TestRunFaceContextDefaultsAndOverrides(t *testing.T) {
	if got := runFace(context.Background()); got != domain.FaceWeb {
		t.Fatalf("unbound context face = %q, want web", got)
	}
	if got := runFace(withFace(context.Background(), domain.FaceCode)); got != domain.FaceCode {
		t.Fatalf("bound context face = %q, want code", got)
	}
}
