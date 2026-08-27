package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingFileReturnsZero(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if !reflect.DeepEqual(s, Settings{}) {
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
	if !reflect.DeepEqual(loaded, saved) {
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
	if !reflect.DeepEqual(loaded, saved) || loaded.ApiKey != "sk-test-overlay" {
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
	if !reflect.DeepEqual(loaded, saved) || loaded.NetworkSearch.Provider != "searxng" {
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
	if loaded.ExecuteMaxTimeoutSeconds != 300 || !reflect.DeepEqual(loaded, saved) {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
	if _, err := Save(path, Settings{ExecuteMaxTimeoutSeconds: 601}); err == nil {
		t.Fatal("expected save to reject execute_max_timeout_seconds above hard cap")
	}
}

func TestSaveAndLoadProviderRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	s := Settings{
		Provider:     ProviderOpenAI,
		DefaultModel: "gpt-4o",
		Providers: []ProviderEntry{
			{ID: "custom-1", DisplayName: "My Gateway", Bundle: ProviderOpenAI, BaseURL: "https://gateway.example.com/v1", DefaultModel: "deepseek-chat", Models: []string{"deepseek-chat"}, ApiKey: "sk-entry"},
		},
	}
	saved, err := Save(path, s)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Providers) != 1 {
		t.Fatalf("registry not persisted: %+v", loaded)
	}
	if loaded.Providers[0].ApiKey != "sk-entry" || loaded.Providers[0].BaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("entry not round-tripped: %+v", loaded.Providers[0])
	}
	if !reflect.DeepEqual(loaded, saved) {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
}

func TestProviderRegistryValidate(t *testing.T) {
	valid := Settings{Provider: ProviderOpenAI, Providers: []ProviderEntry{
		{ID: "custom-1", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1", Models: []string{"m1"}},
	}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid registry rejected: %v", err)
	}
	for name, bad := range map[string]Settings{
		"empty id":      {Providers: []ProviderEntry{{ID: "", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1"}}},
		"empty display": {Providers: []ProviderEntry{{ID: "c", DisplayName: " ", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1"}}},
		"bad bundle":    {Providers: []ProviderEntry{{ID: "c", DisplayName: "A", Bundle: "banana", BaseURL: "https://a.example.com/v1"}}},
		"bad base url":  {Providers: []ProviderEntry{{ID: "c", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "ftp://a.example.com"}}},
		"empty model":   {Providers: []ProviderEntry{{ID: "c", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1", Models: []string{"", "m2"}}}},
		"key newline":   {Providers: []ProviderEntry{{ID: "c", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1", ApiKey: "sk-a\nsk-b"}}},
		"mock bundle":   {Providers: []ProviderEntry{{ID: "c", DisplayName: "A", Bundle: ProviderMock, BaseURL: "https://a.example.com/v1"}}},
		"dup (bundle,url)": {Providers: []ProviderEntry{
			{ID: "c1", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1"},
			{ID: "c2", DisplayName: "B", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1"},
		}},
	} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("expected error for %s", name)
		}
	}
}

func TestProviderRegistryAllowsSameBaseURLAcrossBundles(t *testing.T) {
	s := Settings{Provider: ProviderOpenAI, Providers: []ProviderEntry{
		{ID: "c1", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1"},
		{ID: "c2", DisplayName: "B", Bundle: ProviderAnthropic, BaseURL: "https://a.example.com/v1"},
	}}
	if err := s.Validate(); err != nil {
		t.Fatalf("same base url on different bundles should be allowed: %v", err)
	}
}

func TestFindProviderMatchesBundleAndBaseURL(t *testing.T) {
	s := Settings{Providers: []ProviderEntry{
		{ID: "c1", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1", ApiKey: "sk-a"},
	}}
	if _, ok := s.FindProvider(ProviderOpenAI, "https://a.example.com/v1"); !ok {
		t.Fatal("expected exact (bundle, base_url) match")
	}
	if _, ok := s.FindProvider(ProviderOpenAI, "https://b.example.com/v1"); ok {
		t.Fatal("different base url must not match")
	}
	if _, ok := s.FindProvider(ProviderAnthropic, "https://a.example.com/v1"); ok {
		t.Fatal("different bundle must not match")
	}
}

func TestActiveKeyPrefersRegistryEntry(t *testing.T) {
	key := ActiveKey(Settings{
		ApiKey:   "sk-legacy",
		Provider: ProviderOpenAI,
		BaseURL:  "https://a.example.com/v1",
		Providers: []ProviderEntry{
			{ID: "c1", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1", ApiKey: "sk-entry"},
		},
	}, ProviderOpenAI, "https://a.example.com/v1")
	if key != "sk-entry" {
		t.Fatalf("registry entry should win, got %q", key)
	}
}

func TestActiveKeyFallsBackToLegacyOverlay(t *testing.T) {
	key := ActiveKey(Settings{ApiKey: "sk-legacy", Provider: ProviderOpenAI, BaseURL: ""}, ProviderOpenAI, "")
	if key != "sk-legacy" {
		t.Fatalf("legacy overlay should stand when no entry matches, got %q", key)
	}
}

func TestActiveKeyEmptyWhenNothingSet(t *testing.T) {
	if key := ActiveKey(Settings{Provider: ProviderOpenAI, BaseURL: ""}, ProviderOpenAI, ""); key != "" {
		t.Fatalf("no overlay should be empty, got %q", key)
	}
}

func TestUpsertProviderAppendsAndReplacesByID(t *testing.T) {
	s := Settings{}
	first := s.UpsertProvider(ProviderEntry{ID: "c1", DisplayName: "A", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1"})
	if len(first.Providers) != 1 {
		t.Fatalf("append failed: %+v", first.Providers)
	}
	second := first.UpsertProvider(ProviderEntry{ID: "c1", DisplayName: "A2", Bundle: ProviderOpenAI, BaseURL: "https://a.example.com/v1", Models: []string{"m2"}})
	if len(second.Providers) != 1 {
		t.Fatalf("replace must not grow the registry: %+v", second.Providers)
	}
	if second.Providers[0].DisplayName != "A2" || len(second.Providers[0].Models) != 1 {
		t.Fatalf("entry not replaced: %+v", second.Providers[0])
	}
}
