package domain

import (
	"encoding/json"
	"testing"
)

func TestGenerationSettingsJSON(t *testing.T) {
	recipe := AssemblyRecipe{Settings: GenerationSettings{Locale: "zh"}}
	raw, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raw), `{"settings":{"locale":"zh"}}`; got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}

	var old AssemblyRecipe
	if err := json.Unmarshal([]byte(`{"loop":"eino"}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Loop != "eino" || old.Settings.Locale != "" {
		t.Fatalf("old recipe = %+v", old)
	}
}

func TestGenerationPhaseVocabulary(t *testing.T) {
	for _, p := range []GenerationPhase{GenerationBuilt, GenerationEvalPending, GenerationEvaluated, GenerationPromoted, GenerationReleased, GenerationRejected} {
		if !p.Valid() {
			t.Errorf("%q: Valid = false", p)
		}
	}
	if GenerationPhase("shipping").Valid() {
		t.Error("unknown generation phase must be invalid")
	}
}

func TestReleaseAndInstallPhases(t *testing.T) {
	if !ReleaseAccepted.Valid() {
		t.Error("ReleaseAccepted must be valid")
	}
	if ReleasePhase("shipped").Valid() {
		t.Error("unknown release phase must be invalid")
	}
	for _, p := range []InstallPhase{InstallCurrent, InstallRolledBack} {
		if !p.Valid() {
			t.Errorf("%q: Valid = false", p)
		}
	}
	if InstallPhase("partial").Valid() {
		t.Error("unknown install phase must be invalid")
	}
}

func TestEvalVerdictVocabulary(t *testing.T) {
	for _, v := range []EvalVerdict{EvalBetter, EvalWorse, EvalMixed, EvalFailedToRun} {
		if !v.Valid() {
			t.Errorf("%q: Valid = false", v)
		}
	}
	if EvalVerdict("win").Valid() {
		t.Error("unknown eval verdict must be invalid")
	}
}

func TestStudioEventTypeVocabulary(t *testing.T) {
	for _, et := range []StudioEventType{
		StudioGenerationCreated, StudioEvalRunRecorded, StudioPromotionAccepted, StudioGenerationRejected,
		StudioReleaseAccepted, StudioInstallRecorded, StudioInstallRolledBack, StudioWorktreePinned,
	} {
		if !et.Valid() {
			t.Errorf("%q: Valid = false", et)
		}
	}
	if StudioEventType("run.started").Valid() {
		t.Error("run events must not be studio events")
	}
}
