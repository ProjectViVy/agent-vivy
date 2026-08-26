package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsZero(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if s != (Settings{}) {
		t.Fatalf("expected zero settings, got %+v", s)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-home", FileName)
	saved, err := Save(path, Settings{Provider: ProviderOpenAI, DefaultModel: "gpt-4o", BaseURL: "https://gw.example.com/v1"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != saved {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
}

func TestValidateRejectsBadProvider(t *testing.T) {
	if err := (Settings{Provider: "banana"}).Validate(); err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestValidateRejectsModelWithoutProvider(t *testing.T) {
	if err := (Settings{DefaultModel: "gpt-4o"}).Validate(); err == nil {
		t.Fatal("expected error for default_model without provider")
	}
}

func TestValidateRejectsBadBaseURL(t *testing.T) {
	if err := (Settings{Provider: ProviderOpenAI, BaseURL: "ftp://example.com"}).Validate(); err == nil {
		t.Fatal("expected error for non-http base url")
	}
	if err := (Settings{Provider: ProviderOpenAI, BaseURL: "https://gw.example.com"}).Validate(); err != nil {
		t.Fatalf("https base url should be allowed: %v", err)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if _, err := Save(path, Settings{Provider: "bad"}); err == nil {
		t.Fatal("expected save to reject invalid settings")
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("invalid settings must not be written")
	}
}

func TestEmptyProviderAllowed(t *testing.T) {
	if err := (Settings{}).Validate(); err != nil {
		t.Fatalf("empty settings should validate: %v", err)
	}
}

func TestValidateExecuteMaxTimeoutBounds(t *testing.T) {
	if err := (Settings{ExecuteMaxTimeoutSeconds: 0}).Validate(); err != nil {
		t.Fatalf("0 (config default) should validate: %v", err)
	}
	if err := (Settings{ExecuteMaxTimeoutSeconds: 1}).Validate(); err != nil {
		t.Fatalf("1 should validate: %v", err)
	}
	if err := (Settings{ExecuteMaxTimeoutSeconds: 600}).Validate(); err != nil {
		t.Fatalf("600 should validate: %v", err)
	}
	for _, value := range []int{-1, 601, 3600} {
		if err := (Settings{ExecuteMaxTimeoutSeconds: value}).Validate(); err == nil {
			t.Fatalf("expected error for execute_max_timeout_seconds=%d", value)
		}
	}
}

func TestSaveAndLoadExecuteMaxTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	saved, err := Save(path, Settings{ExecuteMaxTimeoutSeconds: 300})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ExecuteMaxTimeoutSeconds != 300 || loaded != saved {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
	if _, err := Save(path, Settings{ExecuteMaxTimeoutSeconds: 601}); err == nil {
		t.Fatal("expected save to reject execute_max_timeout_seconds above hard cap")
	}
}
