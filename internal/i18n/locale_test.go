package i18n

import "testing"

func TestParseAcceptsOnlySupportedLocales(t *testing.T) {
	tests := []struct {
		value string
		want  Locale
		ok    bool
	}{
		{value: "en", want: English, ok: true},
		{value: "zh", want: Chinese, ok: true},
		{value: "", ok: false},
		{value: "EN", ok: false},
		{value: "zh-CN", ok: false},
		{value: " en", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := Parse(tt.value)
			if tt.ok {
				if err != nil || got != tt.want {
					t.Fatalf("Parse(%q) = %q, %v; want %q, nil", tt.value, got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("Parse(%q) = %q, nil; want error", tt.value, got)
			}
		})
	}
}

func TestResolveLocalePrecedence(t *testing.T) {
	got, err := Resolve("zh", English, "en", true)
	if err != nil || got != Chinese {
		t.Fatalf("workspace override: got %q, %v", got, err)
	}
	got, err = Resolve("", Chinese, "en", true)
	if err != nil || got != Chinese {
		t.Fatalf("sealed generation: got %q, %v", got, err)
	}
	got, err = Resolve("", English, "zh", false)
	if err != nil || got != Chinese {
		t.Fatalf("unsealed developer default: got %q, %v", got, err)
	}
	got, err = Resolve("", English, "", false)
	if err != nil || got != English {
		t.Fatalf("compiled default: got %q, %v", got, err)
	}
	got, err = Resolve("zh", "invalid-generation", "invalid-developer", false)
	if err != nil || got != Chinese {
		t.Fatalf("workspace ignores lower priorities: got %q, %v", got, err)
	}
	got, err = Resolve("", "invalid-generation", Chinese, false)
	if err != nil || got != Chinese {
		t.Fatalf("developer ignores lower-priority generation: got %q, %v", got, err)
	}
}

func TestResolveRejectsSelectedInvalidLocale(t *testing.T) {
	tests := []struct {
		name       string
		workspace  string
		generation Locale
		developer  Locale
		sealed     bool
	}{
		{name: "workspace", workspace: "fr", generation: English, developer: Chinese, sealed: true},
		{name: "sealed generation", generation: "fr", developer: Chinese, sealed: true},
		{name: "unsealed developer", generation: English, developer: "fr", sealed: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := Resolve(tt.workspace, tt.generation, tt.developer, tt.sealed); err == nil {
				t.Fatalf("Resolve(...) = %q, nil; want error", got)
			}
		})
	}
}

func TestResolveSealedGenerationIgnoresDeveloperLocale(t *testing.T) {
	got, err := Resolve("", English, "fr", true)
	if err != nil || got != English {
		t.Fatalf("got %q, %v; want sealed English", got, err)
	}
}
