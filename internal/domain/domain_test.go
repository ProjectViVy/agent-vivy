package domain

import "testing"

var allStatuses = []RunStatus{
	RunAccepted, RunQueued, RunActive, RunCompleted, RunFailed, RunCancelled,
}

func TestRunStateMachine(t *testing.T) {
	legal := map[RunStatus][]RunStatus{
		RunAccepted: {RunQueued, RunActive, RunCancelled},
		RunQueued:   {RunActive, RunCancelled},
		RunActive:   {RunCompleted, RunFailed, RunCancelled},
	}
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			want := false
			for _, ok := range legal[from] {
				if ok == to {
					want = true
				}
			}
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%q -> %q: CanTransitionTo = %v, want %v", from, to, got, want)
			}

			s := from
			err := s.TransitionTo(to)
			if want && err != nil {
				t.Errorf("%q -> %q: TransitionTo = %v, want nil", from, to, err)
			}
			if !want && err == nil {
				t.Errorf("%q -> %q: TransitionTo = nil, want error", from, to)
			}
			if want && s != to {
				t.Errorf("after legal transition: status = %q, want %q", s, to)
			}
			if !want && s != from {
				t.Errorf("after illegal transition: status = %q, want unchanged %q", s, from)
			}
		}
	}
}

func TestTerminalStates(t *testing.T) {
	for _, s := range []RunStatus{RunCompleted, RunFailed, RunCancelled} {
		if !s.Terminal() {
			t.Errorf("%q: Terminal = false, want true", s)
		}
		for _, to := range allStatuses {
			if s.CanTransitionTo(to) {
				t.Errorf("terminal %q allows transition to %q", s, to)
			}
		}
	}
	for _, s := range []RunStatus{RunAccepted, RunQueued, RunActive} {
		if s.Terminal() {
			t.Errorf("%q: Terminal = true, want false", s)
		}
	}
}

func TestStatusVocabulary(t *testing.T) {
	for _, s := range allStatuses {
		if !s.Valid() {
			t.Errorf("%q: Valid = false", s)
		}
	}
	if RunStatus("paused").Valid() {
		t.Error("unknown status must be invalid")
	}
}

func TestRunKindVocabulary(t *testing.T) {
	for _, kind := range []RunKind{RunKindPrimary, RunKindChild} {
		if !kind.Valid() {
			t.Errorf("%q: Valid = false", kind)
		}
	}
	if RunKind("other").Valid() {
		t.Error("unknown run kind must be invalid")
	}
}

func TestEventVocabulary(t *testing.T) {
	if len(EventTypes) != 36 {
		t.Fatalf("vocabulary size = %d, want 36", len(EventTypes))
	}
	seen := map[EventType]bool{}
	terminals := 0
	for _, et := range EventTypes {
		if !et.Valid() {
			t.Errorf("%q: Valid = false", et)
		}
		if seen[et] {
			t.Errorf("%q duplicated in vocabulary", et)
		}
		seen[et] = true
		if et.Terminal() {
			terminals++
			st, ok := et.RunStatus()
			if !ok || !st.Terminal() {
				t.Errorf("terminal event %q must map to a terminal status", et)
			}
		} else if _, ok := et.RunStatus(); ok {
			t.Errorf("non-terminal event %q must not map to a status", et)
		}
	}
	if terminals != 6 {
		t.Errorf("terminal events = %d, want 6", terminals)
	}
	if EventType("run.paused").Valid() {
		t.Error("unknown event type must be invalid")
	}
}

func TestTerminalEventStatusMapping(t *testing.T) {
	cases := map[EventType]RunStatus{
		EventRunCompleted:   RunCompleted,
		EventRunFailed:      RunFailed,
		EventRunCancelled:   RunCancelled,
		EventChildCompleted: RunCompleted,
		EventChildFailed:    RunFailed,
		EventChildCancelled: RunCancelled,
	}
	for et, want := range cases {
		if got, ok := et.RunStatus(); !ok || got != want {
			t.Errorf("%q: RunStatus = %q/%v, want %q/true", et, got, ok, want)
		}
	}
}

func TestRoleVocabulary(t *testing.T) {
	for _, r := range []Role{RoleUser, RoleAssistant, RoleTool} {
		if !r.Valid() {
			t.Errorf("%q: Valid = false", r)
		}
	}
	if Role("system").Valid() {
		t.Error("unknown role must be invalid")
	}
}

func TestMessageEffectiveSource(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"zero value reads as ui", "", "ui"},
		{"channel provenance passes through", "channel", "channel"},
		{"unknown provenance passes through", "widget", "widget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m Message
			m.Source = tc.source
			if got := m.EffectiveSource(); got != tc.want {
				t.Errorf("EffectiveSource = %q, want %q", got, tc.want)
			}
		})
	}
}
