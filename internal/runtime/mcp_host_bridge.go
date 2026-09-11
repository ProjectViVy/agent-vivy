package runtime

import (
	"fmt"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/toolworld"
)

func mcpHostInstances(configs []MCPServerConfig) []mcphost.InstanceConfig {
	instances := make([]mcphost.InstanceConfig, 0, len(configs))
	for _, config := range configs {
		instances = append(instances, mcphost.InstanceConfig{
			ID:             config.Name,
			Endpoint:       config.Endpoint,
			Command:        config.Command,
			Args:           append([]string(nil), config.Args...),
			EnvFrom:        cloneStringMap(config.EnvFrom),
			Cwd:            config.Cwd,
			AuthEnv:        config.AuthEnv,
			ResourceBridge: config.ResourceBridge,
			DeferredReason: config.DeferredReason,
			Enabled:        config.Enabled,
		})
	}
	return instances
}

// ensureMCPBridge constructs the one live MCPHost authority shared by the
// tool, resource, and prompt projections. Every transport session is opened
// by the quarantined runtime factory and owned/closed by this Host.
func (backend *MCPBackend) ensureMCPBridge() (*mcphost.Host, error) {
	if backend == nil {
		return nil, nil
	}
	if err := backend.ensureOpen(); err != nil {
		return nil, err
	}
	backend.bridgeMu.Lock()
	defer backend.bridgeMu.Unlock()
	if backend.bridgeHost != nil {
		return backend.bridgeHost, nil
	}
	host, err := mcphost.New(mcphost.Config{
		Instances:         mcpHostInstances(backend.ConfiguredServers()),
		Factory:           newMCPHostSessionFactory(backend),
		ProtectedIDs:      tools.AssemblyControlledToolNames(),
		MaxSafeRetries:    defaultMCPHostRetries,
		MaxSafeRetriesSet: true,
	})
	if err != nil {
		return nil, fmt.Errorf("runtime: build MCP host: %w", err)
	}
	backend.bridgeHost = host
	backend.bridgeWorld = mcphost.NewToolWorld(host)
	return host, nil
}

// replaceMCPBridge publishes the backend's current configuration into the
// existing MCPHost. A bridge is created lazily by the composition root;
// settings changes therefore update it only after it has been bound to the app.
func (backend *MCPBackend) replaceMCPBridge() {
	if backend == nil {
		return
	}
	backend.bridgeMu.Lock()
	defer backend.bridgeMu.Unlock()
	if backend.bridgeHost == nil {
		return
	}
	_ = backend.bridgeHost.ReplaceInstances(mcpHostInstances(backend.ConfiguredServers()))
	backend.bridgeContextHost = nil
}

func (backend *MCPBackend) closeMCPBridge() error {
	if backend == nil {
		return nil
	}
	backend.bridgeMu.Lock()
	host := backend.bridgeHost
	backend.bridgeContextHost = nil
	backend.bridgeMu.Unlock()
	if host == nil {
		return nil
	}
	return host.Close()
}

// MCPToolWorldProvider exposes the configured runtime MCP backend only as a
// standard dynamic ToolWorld Provider. The application still hands that
// Provider to the sole ToolHost before it can become model-visible.
func (backend *MCPBackend) MCPToolWorldProvider() toolworld.Provider {
	if backend == nil {
		return nil
	}
	if _, err := backend.ensureMCPBridge(); err != nil {
		return nil
	}
	backend.bridgeMu.Lock()
	defer backend.bridgeMu.Unlock()
	return backend.bridgeWorld
}

// MCPResourceContextHost builds the explicit Resource -> ContextHost bridge
// only when the caller proves that the sealed Generation compiled
// vivy/context-host. The optional form keeps old internal callers source
// compatible while making the zero-argument form fail closed; production
// composition uses contextHostForAssembly, which supplies the same manifest
// gate before requesting a Resource Source.
func (backend *MCPBackend) MCPResourceContextHost(compiled ...bool) (*contexthost.Host, error) {
	if len(compiled) == 0 || !compiled[0] {
		return nil, nil
	}
	if backend == nil {
		return nil, nil
	}
	resourceSource, err := backend.MCPResourceProvider()
	if err != nil || resourceSource == nil {
		return nil, err
	}
	backend.bridgeMu.Lock()
	defer backend.bridgeMu.Unlock()
	if backend.bridgeContextHost != nil {
		return backend.bridgeContextHost, nil
	}
	contextHost, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{resourceSource}})
	if err != nil {
		return nil, err
	}
	backend.bridgeContextHost = contextHost
	return contextHost, nil
}

// MCPResourceProvider returns the explicit Resource -> ContextSource bridge
// without creating a second ContextHost. It performs only an in-process
// status snapshot; the ResourceSource remains lazy and opens no MCP session
// until ContextHost actually queries it.
func (backend *MCPBackend) MCPResourceProvider() (contextsource.Provider, error) {
	if backend == nil {
		return nil, nil
	}
	host, err := backend.ensureMCPBridge()
	if err != nil {
		return nil, err
	}
	backend.bridgeMu.Lock()
	defer backend.bridgeMu.Unlock()
	for _, status := range host.Status() {
		if status.ResourceBridge {
			return mcphost.NewResourceSource(host), nil
		}
	}
	backend.bridgeContextHost = nil
	return nil, nil
}

var _ interface{ MCPToolWorldProvider() toolworld.Provider } = (*MCPBackend)(nil)
