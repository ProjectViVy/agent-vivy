package toolhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	porttool "agent-vivy/sdk/port/tool"
	"agent-vivy/sdk/port/toolworld"
)

const defaultDiscoveryTimeout = 2 * time.Second

var (
	ErrDuplicateToolID  = errors.New("duplicate tool id")
	ErrProtectedToolID  = errors.New("protected tool id collision")
	ErrDuplicateWorldID = errors.New("duplicate tool world id")
	ErrUnknownToolID    = errors.New("unknown tool id")
	ErrInvalidTool      = errors.New("invalid tool binding")
	ErrInvalidWorld     = errors.New("invalid tool world binding")
)

type StaticBinding struct {
	OwnerID  string
	Provider porttool.ToolProvider
	Host     porttool.Host
}

type WorldBinding struct {
	OwnerID  string
	Provider toolworld.Provider
	Host     toolworld.Host
	HostFor  func(context.Context) toolworld.Host
}

func (binding WorldBinding) host(ctx context.Context) toolworld.Host {
	if binding.HostFor != nil {
		return binding.HostFor(ctx)
	}
	return binding.Host
}

type Config struct {
	Static           []StaticBinding
	Worlds           []WorldBinding
	ProtectedIDs     []string
	DiscoveryTimeout time.Duration
}

type Request struct {
	ID   string
	Args json.RawMessage
}

type Entry struct {
	Definition porttool.Definition
	SchemaHash string
	OwnerID    string
	WorldID    string
	Dynamic    bool
}

type dynamicBinding struct {
	entry    Entry
	provider toolworld.Provider
	hostFor  func(context.Context) toolworld.Host
	remoteID string
}

type Host struct {
	static           map[string]StaticBinding
	staticEntries    map[string]Entry
	protected        map[string]struct{}
	worlds           []WorldBinding
	discoveryTimeout time.Duration

	mu      sync.RWMutex
	dynamic map[string]dynamicBinding
}

func New(cfg Config) (*Host, error) {
	timeout := cfg.DiscoveryTimeout
	if timeout <= 0 {
		timeout = defaultDiscoveryTimeout
	}
	h := &Host{
		static:           make(map[string]StaticBinding, len(cfg.Static)),
		staticEntries:    make(map[string]Entry, len(cfg.Static)),
		protected:        make(map[string]struct{}, len(cfg.ProtectedIDs)),
		discoveryTimeout: timeout,
		dynamic:          make(map[string]dynamicBinding),
	}
	for _, id := range cfg.ProtectedIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			h.protected[id] = struct{}{}
		}
	}
	for _, binding := range cfg.Static {
		if binding.Provider == nil || binding.Host == nil {
			return nil, ErrInvalidTool
		}
		definition := normalizedToolDefinition(binding.Provider.Definition())
		if definition.ID == "" {
			return nil, ErrInvalidTool
		}
		if _, exists := h.static[definition.ID]; exists {
			if h.IsProtected(definition.ID) {
				return nil, fmt.Errorf("%w: %s", ErrProtectedToolID, definition.ID)
			}
			return nil, fmt.Errorf("%w: %s", ErrDuplicateToolID, definition.ID)
		}
		h.static[definition.ID] = binding
		h.staticEntries[definition.ID] = Entry{
			Definition: definition,
			SchemaHash: hashDefinition(definition),
			OwnerID:    strings.TrimSpace(binding.OwnerID),
		}
	}

	worldIDs := make(map[string]struct{}, len(cfg.Worlds))
	for _, binding := range cfg.Worlds {
		if binding.Provider == nil || (binding.Host == nil && binding.HostFor == nil) {
			return nil, ErrInvalidWorld
		}
		worldID := strings.TrimSpace(binding.Provider.Definition().ID)
		if worldID == "" {
			return nil, ErrInvalidWorld
		}
		if _, exists := worldIDs[worldID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateWorldID, worldID)
		}
		worldIDs[worldID] = struct{}{}
		h.worlds = append(h.worlds, binding)
	}
	sort.SliceStable(h.worlds, func(i, j int) bool {
		return strings.TrimSpace(h.worlds[i].Provider.Definition().ID) < strings.TrimSpace(h.worlds[j].Provider.Definition().ID)
	})
	return h, nil
}

func normalizedToolDefinition(definition porttool.Definition) porttool.Definition {
	definition.ID = strings.TrimSpace(definition.ID)
	definition.Description = strings.TrimSpace(definition.Description)
	definition.Schema = append(json.RawMessage(nil), definition.Schema...)
	return definition
}

func hashDefinition(definition porttool.Definition) string {
	schema := bytes.TrimSpace(definition.Schema)
	if len(schema) != 0 {
		var value any
		if json.Unmarshal(schema, &value) == nil {
			if canonical, err := json.Marshal(value); err == nil {
				schema = canonical
			}
		}
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(definition.ID))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(definition.Description))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(definition.Effect))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(schema)
	return hex.EncodeToString(digest.Sum(nil))
}

func (h *Host) IsProtected(id string) bool {
	_, ok := h.protected[strings.TrimSpace(id)]
	return ok
}

func (h *Host) Lookup(id string) (Entry, bool) {
	id = strings.TrimSpace(id)
	if entry, ok := h.staticEntries[id]; ok {
		return entry, true
	}
	h.mu.RLock()
	binding, ok := h.dynamic[id]
	h.mu.RUnlock()
	if !ok {
		return Entry{}, false
	}
	return binding.entry, true
}

func (h *Host) ListVisible() []porttool.Definition {
	entries := make([]Entry, 0, len(h.staticEntries))
	for _, entry := range h.staticEntries {
		entries = append(entries, entry)
	}
	h.mu.RLock()
	for _, binding := range h.dynamic {
		entries = append(entries, binding.entry)
	}
	h.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Definition.ID < entries[j].Definition.ID })
	out := make([]porttool.Definition, 0, len(entries))
	for _, entry := range entries {
		out = append(out, normalizedToolDefinition(entry.Definition))
	}
	return out
}

func (h *Host) Discover(ctx context.Context) ([]porttool.Definition, error) {
	next := make(map[string]dynamicBinding)
	seen := make(map[string]struct{}, len(h.staticEntries))
	for id := range h.staticEntries {
		seen[id] = struct{}{}
	}
	for _, world := range h.worlds {
		worldID := strings.TrimSpace(world.Provider.Definition().ID)
		discoveryCtx, cancel := context.WithTimeout(ctx, h.discoveryTimeout)
		worldHost := world.host(discoveryCtx)
		if worldHost == nil {
			cancel()
			return nil, fmt.Errorf("%w: %s has no host", ErrInvalidWorld, worldID)
		}
		definitions, err := world.Provider.Discover(discoveryCtx, worldHost)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("discover tool world %s: %w", worldID, err)
		}
		for _, remote := range definitions {
			id := strings.TrimSpace(remote.ID)
			if id == "" {
				return nil, fmt.Errorf("%w: %s returned an empty tool id", ErrInvalidWorld, worldID)
			}
			if _, exists := seen[id]; exists {
				if h.IsProtected(id) {
					return nil, fmt.Errorf("%w: %s", ErrProtectedToolID, id)
				}
				return nil, fmt.Errorf("%w: %s", ErrDuplicateToolID, id)
			}
			seen[id] = struct{}{}
			definition := normalizedToolDefinition(porttool.Definition{
				ID:          id,
				Description: remote.Description,
				Effect:      porttool.Effect(remote.Effect),
				Schema:      remote.Schema,
			})
			next[id] = dynamicBinding{
				entry: Entry{
					Definition: definition,
					SchemaHash: hashDefinition(definition),
					OwnerID:    strings.TrimSpace(world.OwnerID),
					WorldID:    worldID,
					Dynamic:    true,
				},
				provider: world.Provider,
				hostFor:  world.host,
				remoteID: id,
			}
		}
	}
	h.mu.Lock()
	h.dynamic = next
	h.mu.Unlock()
	return h.ListVisible(), nil
}

func (h *Host) Invoke(ctx context.Context, req Request) (porttool.Result, error) {
	id := strings.TrimSpace(req.ID)
	if binding, ok := h.static[id]; ok {
		return binding.Provider.Invoke(ctx, binding.Host, req.Args)
	}
	h.mu.RLock()
	binding, ok := h.dynamic[id]
	h.mu.RUnlock()
	if !ok {
		return porttool.Result{}, fmt.Errorf("%w: %s", ErrUnknownToolID, req.ID)
	}
	worldHost := binding.hostFor(ctx)
	if worldHost == nil {
		return porttool.Result{}, fmt.Errorf("%w: %s has no host", ErrInvalidWorld, binding.entry.WorldID)
	}
	result, err := binding.provider.Invoke(ctx, worldHost, binding.remoteID, req.Args)
	return porttool.Result{Text: result.Text}, err
}
