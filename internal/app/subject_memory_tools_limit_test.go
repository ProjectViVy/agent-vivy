package app

import "testing"

func TestSubjectSearchLimitFollowsTheSharedSchema(t *testing.T) {
	for requested, want := range map[int]int{0: 20, 1: 1, 100: 100, 101: 100, 4294967295: 100} {
		got, ok := subjectSearchLimit(requested)
		if !ok || got != want {
			t.Errorf("limit %d = %d,%v want %d,true", requested, got, ok, want)
		}
	}
	if _, ok := subjectSearchLimit(-1); ok {
		t.Error("negative limit accepted")
	}
}
