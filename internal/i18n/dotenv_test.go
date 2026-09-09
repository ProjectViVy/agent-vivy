package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeveloperDefaultEnvironmentWinsOverDotEnv(t *testing.T) {
	t.Setenv(DefaultLocaleEnv, "zh")
	path := writeDotEnv(t, "VIVY_DEFAULT_LOCALE=en\n")
	got, err := DeveloperDefault(path)
	if err != nil || got != Chinese {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDeveloperDefaultReadsOnlySupportedDotEnvSetting(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "unquoted", value: "zh"},
		{name: "single quoted", value: "'zh'"},
		{name: "double quoted", value: `"zh"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnv(t, DefaultLocaleEnv)
			unsetEnv(t, "IGNORED_SECRET")
			path := writeDotEnv(t, "IGNORED_SECRET=do-not-parse-this-quote-'\nVIVY_DEFAULT_LOCALE="+tt.value+"\n")
			got, err := DeveloperDefault(path)
			if err != nil || got != Chinese {
				t.Fatalf("got %q, %v", got, err)
			}
			if _, ok := os.LookupEnv(DefaultLocaleEnv); ok {
				t.Fatal("DeveloperDefault exported the dotenv value into the process")
			}
			if _, ok := os.LookupEnv("IGNORED_SECRET"); ok {
				t.Fatal("DeveloperDefault exported an unrelated dotenv entry")
			}
		})
	}
}

func TestDeveloperDefaultUsesEnglishWhenDotEnvIsMissing(t *testing.T) {
	unsetEnv(t, DefaultLocaleEnv)
	got, err := DeveloperDefault(filepath.Join(t.TempDir(), ".env"))
	if err != nil || got != English {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDeveloperDefaultUsesEnglishWhenDotEnvHasNoLocale(t *testing.T) {
	unsetEnv(t, DefaultLocaleEnv)
	unsetEnv(t, "IGNORED_SECRET")
	got, err := DeveloperDefault(writeDotEnv(t, "IGNORED_SECRET=not-a-locale\n"))
	if err != nil || got != English {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, ok := os.LookupEnv("IGNORED_SECRET"); ok {
		t.Fatal("DeveloperDefault exported an unrelated dotenv entry")
	}
}

func TestDeveloperDefaultRejectsMalformedOrUnreadableDotEnv(t *testing.T) {
	tests := []struct {
		name string
		path func(*testing.T) string
	}{
		{name: "single quote", path: func(t *testing.T) string {
			return writeDotEnv(t, "VIVY_DEFAULT_LOCALE='zh\n")
		}},
		{name: "double quote", path: func(t *testing.T) string {
			return writeDotEnv(t, "VIVY_DEFAULT_LOCALE=\"zh\n")
		}},
		{name: "directory", path: func(t *testing.T) string {
			return t.TempDir()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnv(t, DefaultLocaleEnv)
			if got, err := DeveloperDefault(tt.path(t)); err == nil {
				t.Fatalf("got %q, nil; want error", got)
			}
		})
	}
}

func TestDeveloperDefaultRejectsInvalidSelectedValue(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		t.Setenv(DefaultLocaleEnv, "fr")
		if got, err := DeveloperDefault(writeDotEnv(t, "VIVY_DEFAULT_LOCALE=en\n")); err == nil {
			t.Fatalf("got %q, nil; want error", got)
		}
	})
	t.Run("dotenv", func(t *testing.T) {
		unsetEnv(t, DefaultLocaleEnv)
		if got, err := DeveloperDefault(writeDotEnv(t, "VIVY_DEFAULT_LOCALE='fr'\n")); err == nil {
			t.Fatalf("got %q, nil; want error", got)
		}
	})
}

func writeDotEnv(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
			return
		}
		_ = os.Unsetenv(key)
	})
}
