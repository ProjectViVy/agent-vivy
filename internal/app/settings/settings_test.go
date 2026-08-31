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

func TestSaveAndLoadCompactionOverlay(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	enabled := true
	saved, err := Save(path, Settings{Compaction: &CompactionSettings{
		Enabled: &enabled, MaxTokens: 200000, TriggerPercent: 70, KeepRecent: 8,
	}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(loaded, saved) || loaded.Compaction == nil ||
		!*loaded.Compaction.Enabled || loaded.Compaction.MaxTokens != 200000 ||
		loaded.Compaction.TriggerPercent != 70 || loaded.Compaction.KeepRecent != 8 {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
	if loaded.IsZero() {
		t.Fatal("non-empty compaction overlay must not look zero")
	}
	// Zero fields inside a present overlay normalize to nil on load
	// (config default stands).
	if _, err := Save(path, Settings{Compaction: &CompactionSettings{}}); err != nil {
		t.Fatalf("save empty overlay: %v", err)
	}
	zeroLoaded, err := Load(path)
	if err != nil {
		t.Fatalf("load empty overlay: %v", err)
	}
	if zeroLoaded.Compaction != nil {
		t.Fatalf("empty compaction overlay should normalize to nil, got %+v", zeroLoaded.Compaction)
	}
}

func TestValidateCompactionOverlayBounds(t *testing.T) {
	enabled := true
	valid := Settings{Compaction: &CompactionSettings{Enabled: &enabled, MaxTokens: 0, TriggerPercent: 0, KeepRecent: 0}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("zeros (config default) should validate: %v", err)
	}
	for _, bad := range []CompactionSettings{
		{MaxTokens: -1},
		{TriggerPercent: -1},
		{TriggerPercent: 101},
		{KeepRecent: -1},
	} {
		if err := (Settings{Compaction: &bad}).Validate(); err == nil {
			t.Fatalf("expected error for %+v", bad)
		}
	}
}

func TestSaveAndLoadSandboxPreset(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	deny := false
	saved, err := Save(path, Settings{Sandbox: SandboxSettings{
		DefaultPreset: "cautious",
		Network:       SandboxNetworkSettings{DenyPrivateIPs: &deny, AllowedDomains: []string{"example.com"}},
	}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Sandbox.DefaultPreset != "cautious" || loaded.Sandbox.Network.DenyPrivateIPs == nil || *loaded.Sandbox.Network.DenyPrivateIPs {
		t.Fatalf("sandbox round trip = %+v saved %+v", loaded.Sandbox, saved.Sandbox)
	}
	if err := (Settings{Sandbox: SandboxSettings{DefaultPreset: "custom"}}).Validate(); err == nil {
		t.Fatal("custom preset must be rejected")
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

// The tools_enabled overlay is nil until written (config default stands),
// then round-trips as a whole-list replacement. An explicit empty list is
// the legal chat-only mode, so it must survive the round trip as non-nil.
func TestSaveAndLoadToolsEnabledOverlay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-home", FileName)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if loaded.ToolsEnabled != nil {
		t.Fatalf("missing document tools_enabled = %+v, want nil", loaded.ToolsEnabled)
	}

	active := []string{"list_dir", "read_file"}
	saved, err := Save(path, Settings{ToolsEnabled: &active})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err = Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(loaded, saved) {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved, loaded)
	}
	if loaded.ToolsEnabled == nil || !reflect.DeepEqual(*loaded.ToolsEnabled, active) {
		t.Fatalf("tools_enabled = %+v, want %v", loaded.ToolsEnabled, active)
	}

	empty := []string{}
	saved, err = Save(path, Settings{ToolsEnabled: &empty})
	if err != nil {
		t.Fatalf("save chat-only: %v", err)
	}
	loaded, err = Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ToolsEnabled == nil || len(*loaded.ToolsEnabled) != 0 {
		t.Fatalf("chat-only tools_enabled = %+v, want explicit empty", loaded.ToolsEnabled)
	}
}

func TestValidateToolsEnabledOverlay(t *testing.T) {
	dup := []string{"list_dir", "list_dir"}
	if err := (Settings{ToolsEnabled: &dup}).Validate(); err == nil {
		t.Fatal("duplicate names must be rejected")
	}
	blank := []string{"list_dir", "  "}
	if err := (Settings{ToolsEnabled: &blank}).Validate(); err == nil {
		t.Fatal("blank names must be rejected")
	}
	ok := []string{"list_dir", "read_file"}
	if err := (Settings{ToolsEnabled: &ok}).Validate(); err != nil {
		t.Fatalf("valid overlay rejected: %v", err)
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

func TestSaveAndLoadMCPServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	enabled := BoolPtr(false)
	list := []MCPServer{
		{Name: "docs", Endpoint: "https://docs.example.com/mcp", AuthEnv: "MCP_DOCS_TOKEN"},
		{Name: "idle", Endpoint: "http://127.0.0.1:9123/mcp", Enabled: enabled},
	}
	saved, err := Save(path, Settings{MCPServers: &list})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.MCPServers == nil || len(*loaded.MCPServers) != 2 {
		t.Fatalf("mcp overlay not persisted: %+v", loaded)
	}
	if !MCPServerEnabled((*loaded.MCPServers)[0]) || MCPServerEnabled((*loaded.MCPServers)[1]) {
		t.Fatalf("enabled defaults not round-tripped: %+v", *loaded.MCPServers)
	}
	if !reflect.DeepEqual(loaded.MCPServers, saved.MCPServers) {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved.MCPServers, loaded.MCPServers)
	}
}

func TestEmptyMCPOverlayIsNotZero(t *testing.T) {
	empty := []MCPServer{}
	s := Settings{MCPServers: &empty}
	if s.IsZero() {
		t.Fatal("explicit empty MCP overlay must not look like an unconfigured document")
	}
	if (Settings{}).IsZero() != true {
		t.Fatal("zero settings must remain zero")
	}
}

func TestEmptyMCPOverlayRoundTripStaysExplicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	empty := []MCPServer{}
	if _, err := Save(path, Settings{MCPServers: &empty}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MCPServers == nil {
		t.Fatal("empty overlay must not decode as unset (config default would return)")
	}
	if len(*loaded.MCPServers) != 0 {
		t.Fatalf("empty overlay decoded as %+v", *loaded.MCPServers)
	}
}

func TestValidateMCPServers(t *testing.T) {
	ok := []MCPServer{{Name: "docs", Endpoint: "https://docs.example.com/mcp", AuthEnv: "MCP_DOCS_TOKEN"}}
	if err := (Settings{MCPServers: &ok}).Validate(); err != nil {
		t.Fatalf("valid mcp overlay rejected: %v", err)
	}
	for name, bad := range map[string][]MCPServer{
		"missing name":     {{Endpoint: "https://docs.example.com/mcp"}},
		"missing endpoint": {{Name: "docs"}},
		"bad url":          {{Name: "docs", Endpoint: "ftp://docs.example.com/mcp"}},
		"bad auth env":     {{Name: "docs", Endpoint: "https://docs.example.com/mcp", AuthEnv: "not-an-env"}},
		"duplicate name": {
			{Name: "docs", Endpoint: "https://a.example.com/mcp"},
			{Name: "Docs", Endpoint: "https://b.example.com/mcp"},
		},
	} {
		if err := (Settings{MCPServers: &bad}).Validate(); err == nil {
			t.Fatalf("expected error for %s", name)
		}
	}
}

func TestUpsertAndDeleteMCPServer(t *testing.T) {
	s := Settings{}
	first := s.UpsertMCPServer(MCPServer{Name: "docs", Endpoint: "https://a.example.com/mcp"})
	if first.MCPServers == nil || len(*first.MCPServers) != 1 {
		t.Fatalf("append failed: %+v", first.MCPServers)
	}
	second := first.UpsertMCPServer(MCPServer{Name: "Docs", Endpoint: "https://b.example.com/mcp", AuthEnv: "MCP_DOCS_TOKEN", Enabled: BoolPtr(false)})
	if len(*second.MCPServers) != 1 {
		t.Fatalf("replace must not grow the list: %+v", *second.MCPServers)
	}
	if (*second.MCPServers)[0].Endpoint != "https://b.example.com/mcp" || MCPServerEnabled((*second.MCPServers)[0]) {
		t.Fatalf("entry not replaced: %+v", (*second.MCPServers)[0])
	}
	deleted, ok := second.DeleteMCPServer("docs")
	if !ok || deleted.MCPServers == nil || len(*deleted.MCPServers) != 0 {
		t.Fatalf("delete failed: ok=%v list=%+v", ok, deleted.MCPServers)
	}
	if _, ok := deleted.DeleteMCPServer("missing"); ok {
		t.Fatal("missing delete must report not found")
	}
}

func TestSaveAndLoadChannelOverlay(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	disabled := false
	emptyAllow := []string{}
	senders := []string{"alice", "bob"}
	saved, err := Save(path, Settings{Channels: []ChannelOverlay{
		// Fully pinned: explicit disabled, explicit empty allow_from (the
		// fail-closed deny-start state), explicit token_env name.
		{Name: "telegram", Enabled: &disabled, AllowFrom: &emptyAllow, TokenEnv: StringPtr("TELEGRAM_BOT_TOKEN")},
		// Partial: only allow_from replaced; enabled/token_env stay unset.
		{Name: "discord", AllowFrom: &senders},
	}})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(loaded.Channels, saved.Channels) {
		t.Fatalf("round trip mismatch: saved %+v loaded %+v", saved.Channels, loaded.Channels)
	}
	first := loaded.Channels[0]
	if first.Enabled == nil || *first.Enabled {
		t.Fatalf("explicit disabled not round-tripped: %+v", first)
	}
	if first.AllowFrom == nil || len(*first.AllowFrom) != 0 {
		t.Fatalf("explicit empty allow_from must stay an empty (deny-start) list, not unset: %+v", first.AllowFrom)
	}
	if first.TokenEnv == nil || *first.TokenEnv != "TELEGRAM_BOT_TOKEN" {
		t.Fatalf("token_env not round-tripped: %+v", first)
	}
	second := loaded.Channels[1]
	if second.Enabled != nil || second.TokenEnv != nil {
		t.Fatalf("unset knobs must decode as nil (config default stands): %+v", second)
	}
	if second.AllowFrom == nil || len(*second.AllowFrom) != 2 {
		t.Fatalf("allow_from list not round-tripped: %+v", second.AllowFrom)
	}
}

func TestEmptyChannelOverlayEntryIsDropped(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("channels:\n  - name: telegram\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Channels) != 0 {
		t.Fatalf("all-empty overlay entry must normalize away: %+v", loaded.Channels)
	}
	if !loaded.IsZero() {
		t.Fatal("a document whose only overlay entry carries no knob must stay zero")
	}
}

func TestChannelsOverlayIsNotZero(t *testing.T) {
	enabled := true
	s := Settings{Channels: []ChannelOverlay{{Name: "telegram", Enabled: &enabled}}}
	if s.IsZero() {
		t.Fatal("a channel overlay entry must not look like an unconfigured document")
	}
}

func TestValidateChannels(t *testing.T) {
	enabled := BoolPtr(true)
	senders := []string{"alice"}
	ok := []ChannelOverlay{
		{Name: "telegram", Enabled: enabled, AllowFrom: &senders, TokenEnv: StringPtr("TELEGRAM_BOT_TOKEN")},
		{Name: "my-bot-2", AllowFrom: &[]string{}},
	}
	if err := (Settings{Channels: ok}).Validate(); err != nil {
		t.Fatalf("valid channels overlay rejected: %v", err)
	}
	emptyAllow := []string{}
	for name, bad := range map[string][]ChannelOverlay{
		"bad name":         {{Name: "Telegram", Enabled: enabled}},
		"empty name":       {{Name: "", Enabled: enabled}},
		"duplicate name":   {{Name: "telegram", Enabled: enabled}, {Name: "telegram", AllowFrom: &senders}},
		"bad token env":    {{Name: "telegram", TokenEnv: StringPtr("not-an-env")}},
		"wildcard sender":  {{Name: "telegram", AllowFrom: &[]string{"*"}}},
		"empty sender":     {{Name: "telegram", AllowFrom: &[]string{" "}}},
		"empty list is ok": {{Name: "telegram", AllowFrom: &emptyAllow}},
	} {
		err := (Settings{Channels: bad}).Validate()
		if name == "empty list is ok" {
			if err != nil {
				t.Fatalf("explicit empty allow_from is the valid deny-start state: %v", err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("expected error for %s", name)
		}
	}
}

func TestUpsertChannelOverlay(t *testing.T) {
	enabled := BoolPtr(true)
	s := Settings{}.UpsertChannelOverlay(ChannelOverlay{Name: "telegram", Enabled: enabled})
	if len(s.Channels) != 1 || s.Channels[0].Name != "telegram" {
		t.Fatalf("append failed: %+v", s.Channels)
	}
	senders := []string{"alice"}
	s = s.UpsertChannelOverlay(ChannelOverlay{Name: "telegram", AllowFrom: &senders})
	if len(s.Channels) != 1 {
		t.Fatalf("replace must not grow the list: %+v", s.Channels)
	}
	if s.Channels[0].Enabled != nil {
		t.Fatal("replacement entry must carry exactly the fields it is given")
	}
	if s.Channels[0].AllowFrom == nil || len(*s.Channels[0].AllowFrom) != 1 {
		t.Fatalf("allow_from not replaced: %+v", s.Channels[0])
	}
}
