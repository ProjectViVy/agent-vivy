package turnlimit

import "testing"

func TestFormerTurnPolicy(t *testing.T) {
	if Default().MaxToolTurns != 8 {
		t.Fatal("former default must remain eight in EXP only")
	}
	for _, tc := range []struct {
		limit int
		child bool
		want  int
	}{
		{8, false, 8}, {2, false, 2}, {10000, false, 10000},
		{0, false, 0}, {8, true, 8}, {2, true, 2}, {10000, true, 8}, {0, true, 8},
	} {
		got, err := (Policy{MaxToolTurns: tc.limit}).MaxIterations(tc.child)
		if err != nil || got != tc.want {
			t.Fatalf("limit=%d child=%v: got %d, %v; want %d", tc.limit, tc.child, got, err, tc.want)
		}
	}

	for _, child := range []bool{false, true} {
		if _, err := (Policy{MaxToolTurns: -1}).MaxIterations(child); err == nil {
			t.Fatal("negative limit must remain invalid")
		}
	}
}
