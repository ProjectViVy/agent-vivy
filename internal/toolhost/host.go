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

	"agent-vivy/internal/tools"

	"agent-vivy/sdk/port/pretool"
	porttool "agent-vivy/sdk/port/tool"
	"agent-vivy/sdk/port/toolworld"
)

const (
	defaultDiscoveryTimeout  = 2 * time.Second
	defaultMiddlewareTimeout = 500 * time.Millisecond
)

var (
	ErrDuplicateToolID       = errors.New("duplicate tool id")
	ErrProtectedToolID       = errors.New("protected tool id collision")
	ErrDuplicateWorldID      = errors.New("duplicate tool world id")
	ErrDuplicateMiddlewareID = errors.New("duplicate middleware id")
	ErrUnknownToolID         = errors.New("unknown tool id")
	ErrInvalidTool           = errors.New("invalid tool binding")
	ErrInvalidWorld          = errors.New("invalid tool world binding")
	ErrInvalidMiddleware     = errors.New("invalid middleware binding")
	ErrStaleDiscovery        = errors.New("stale tool discovery")
)

type Trust uint8

const (
	TrustPublic Trust = iota
	TrustCore
)

type StaticBinding struct {
	OwnerID  string
	Provider porttool.ToolProvider
	Host     porttool.Host
	Trust    Trust
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
	Static            []StaticBinding
	Worlds            []WorldBinding
	ProtectedIDs      []string
	DiscoveryTimeout  time.Duration
	Middleware        []pretool.Provider
	MiddlewareTimeout time.Duration
}

type Request struct {
	ID   string
	Args json.RawMessage
}

type Entry struct {
	Definition porttool.Definition
	SchemaHash string
	Provenance toolworld.Provenance
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
	static            map[string]StaticBinding
	staticEntries     map[string]Entry
	protected         map[string]struct{}
	worlds            []WorldBinding
	discoveryTimeout  time.Duration
	middleware        []pretool.Provider
	middlewareTimeout time.Duration

	mu                  sync.RWMutex
	dynamic             map[string]dynamicBinding
	discoveryGeneration uint64
}

func New(cfg Config) (*Host, error) {
	discoveryTimeout := cfg.DiscoveryTimeout
	if discoveryTimeout <= 0 {
		discoveryTimeout = defaultDiscoveryTimeout
	}
	middlewareTimeout := cfg.MiddlewareTimeout
	if middlewareTimeout <= 0 {
		middlewareTimeout = defaultMiddlewareTimeout
	}
	h := &Host{
		static:            make(map[string]StaticBinding, len(cfg.Static)),
		staticEntries:     make(map[string]Entry, len(cfg.Static)),
		protected:         make(map[string]struct{}, len(cfg.ProtectedIDs)),
		discoveryTimeout:  discoveryTimeout,
		middlewareTimeout: middlewareTimeout,
		dynamic:           make(map[string]dynamicBinding),
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
		if h.IsProtected(definition.ID) && binding.Trust != TrustCore {
			return nil, fmt.Errorf("%w: %s", ErrProtectedToolID, definition.ID)
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

	middlewareIDs := make(map[string]struct{}, len(cfg.Middleware))
	for _, provider := range cfg.Middleware {
		if provider == nil {
			return nil, ErrInvalidMiddleware
		}
		id := strings.TrimSpace(provider.ID())
		if id == "" {
			return nil, ErrInvalidMiddleware
		}
		if _, exists := middlewareIDs[id]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateMiddlewareID, id)
		}
		middlewareIDs[id] = struct{}{}
		h.middleware = append(h.middleware, provider)
	}
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
		decoder := json.NewDecoder(bytes.NewReader(schema))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) == nil {
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

func identityHash(kind string, parts ...string) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(kind))
	for _, part := range parts {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(part))
	}
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
	h.mu.Lock()
	h.discoveryGeneration++
	generation := h.discoveryGeneration
	// A new projection generation invalidates every prior dynamic binding
	// before any remote response can arrive. This makes a failed discovery
	// fail closed and prevents an older concurrent response from resurrecting
	// a retired schema.
	h.dynamic = make(map[string]dynamicBinding)
	h.mu.Unlock()

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
			if !h.discoveryCurrent(generation) {
				return nil, ErrStaleDiscovery
			}
			return nil, fmt.Errorf("%w: %s has no host", ErrInvalidWorld, worldID)
		}
		definitions, err := world.Provider.Discover(discoveryCtx, worldHost)
		cancel()
		if err != nil {
			if !h.discoveryCurrent(generation) {
				return nil, ErrStaleDiscovery
			}
			return nil, fmt.Errorf("discover tool world %s: %w", worldID, err)
		}
		if !h.discoveryCurrent(generation) {
			return nil, ErrStaleDiscovery
		}
		for _, remote := range definitions {
			id := strings.TrimSpace(remote.ID)
			if id == "" {
				if !h.discoveryCurrent(generation) {
					return nil, ErrStaleDiscovery
				}
				return nil, fmt.Errorf("%w: %s returned an empty tool id", ErrInvalidWorld, worldID)
			}
			if h.IsProtected(id) {
				if !h.discoveryCurrent(generation) {
					return nil, ErrStaleDiscovery
				}
				return nil, fmt.Errorf("%w: %s", ErrProtectedToolID, id)
			}
			if _, exists := seen[id]; exists {
				if !h.discoveryCurrent(generation) {
					return nil, ErrStaleDiscovery
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
			if len(definition.Schema) > 0 {
				if err := tools.ValidateSchema(definition.Schema); err != nil {
					if !h.discoveryCurrent(generation) {
						return nil, ErrStaleDiscovery
					}
					return nil, fmt.Errorf("%w: %s schema: %v", ErrInvalidWorld, id, err)
				}
			}
			provenance := remote.Provenance
			if provenance.ServerInstanceID == "" {
				provenance.ServerInstanceID = remote.ServerInstanceID
			}
			if provenance.RemoteCapability == "" {
				provenance.RemoteCapability = remote.RemoteCapability
			}
			if provenance.SchemaHash == "" {
				provenance.SchemaHash = remote.SchemaHash
			}
			if provenance.InstanceHash == "" {
				provenance.InstanceHash = remote.InstanceHash
			}
			if provenance.RemoteHash == "" {
				provenance.RemoteHash = remote.RemoteHash
			}
			if provenance.SchemaHash == "" {
				provenance.SchemaHash = hashDefinition(definition)
			}
			provenance.ServerInstanceID = strings.TrimSpace(provenance.ServerInstanceID)
			provenance.RemoteCapability = strings.TrimSpace(provenance.RemoteCapability)
			provenance.SchemaHash = strings.TrimSpace(provenance.SchemaHash)
			if provenance.InstanceHash == "" && provenance.ServerInstanceID != "" {
				provenance.InstanceHash = identityHash("mcp-instance", provenance.ServerInstanceID)
			}
			if provenance.RemoteHash == "" && provenance.ServerInstanceID != "" && provenance.RemoteCapability != "" {
				provenance.RemoteHash = identityHash("mcp-remote", provenance.ServerInstanceID, provenance.RemoteCapability)
			}
			next[id] = dynamicBinding{
				entry: Entry{
					Definition: definition,
					SchemaHash: provenance.SchemaHash,
					Provenance: provenance,
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
	if h.discoveryGeneration != generation {
		h.mu.Unlock()
		return nil, ErrStaleDiscovery
	}
	h.dynamic = next
	h.mu.Unlock()
	return h.ListVisible(), nil
}

func (h *Host) discoveryCurrent(generation uint64) bool {
	h.mu.RLock()
	current := h.discoveryGeneration == generation
	h.mu.RUnlock()
	return current
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
