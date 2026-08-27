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

func TestSaveAndLoadRoundTripWithAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-home", FileName)
	saved, err := Save(path, Settings{Provider: ProviderOpenAI, DefaultModel: "gpt-4o", BaseURL: "https://gw.example.com/v1", ApiKey: "sk-test-overlay"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != saved || loaded.ApiKey != "sk-test-overlay" {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
}

func TestSaveAndLoadRoundTripWithNetworkSearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-home", FileName)
	saved, err := Save(path, Settings{Provider: ProviderOpenAI, DefaultModel: "gpt-4o", NetworkSearch: NetworkSearchSettings{Provider: "searxng"}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != saved || loaded.NetworkSearch.Provider != "searxng" {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
}

func TestValidateRejectsBadNetworkSearchProvider(t *testing.T) {
	if err := (Settings{Provider: ProviderOpenAI, NetworkSearch: NetworkSearchSettings{Provider: "yandex"}}).Validate(); err == nil {
		t.Fatal("expected error for unsupported network_search provider")
	}
	for _, valid := range []string{"bing", "google", "duckduckgo", "searxng", "wikipedia"} {
		if err := (Settings{Provider: ProviderOpenAI, NetworkSearch: NetworkSearchSettings{Provider: valid}}).Validate(); err != nil {
			t.Fatalf("%s should be allowed: %v", valid, err)
		}
	}
}

func TestValidateRejectsNewlineInAPIKey(t *testing.T) {
	if err := (Settings{Provider: ProviderOpenAI, ApiKey: "sk-a\nsk-b"}).Validate(); err == nil {
		t.Fatal("expected error for newline in api_key")
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

func TestNetworkSearchPreference(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	saved, err := Save(path, Settings{NetworkSearch: NetworkSearchSettings{Provider: "searxng"}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.NetworkSearch.Provider != "searxng" {
		t.Fatalf("network_search round trip = %+v", loaded)
	}
	if saved.NetworkSearch.Provider != "searxng" {
		t.Fatalf("saved echo = %+v", saved)
	}

	if err := (Settings{NetworkSearch: NetworkSearchSettings{Provider: "alta vista"}}).Validate(); err == nil {
		t.Fatal("expected error for unsupported network_search provider")
	}
	if err := (Settings{NetworkSearch: NetworkSearchSettings{Provider: "wikipedia"}}).Validate(); err != nil {
		t.Fatalf("valid network_search provider rejected: %v", err)
	}
}
