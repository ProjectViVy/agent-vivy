package domain

import "testing"

func TestFaceValid(t *testing.T) {
	cases := map[Face]bool{
		FaceWeb:  true,
		FaceTui:  true,
		FaceCode: true,
		"":       false,
		"shell":  false,
		"WEB":    false,
	}
	for face, want := range cases {
		if got := face.Valid(); got != want {
			t.Errorf("Face(%q).Valid() = %v, want %v", face, got, want)
		}
	}
}
