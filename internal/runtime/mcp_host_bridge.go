package runtime

import (
	"agent-vivy/internal/mcphost"
	"agent-vivy/sdk/port/toolworld"
)

// MCPToolWorldProvider exposes the configured runtime MCP backend only as a
// standard dynamic ToolWorld Provider. The application still hands that
// Provider to the sole ToolHost before it can become model-visible.
func (backend *MCPBackend) MCPToolWorldProvider() toolworld.Provider {
	if backend == nil {
		return nil
	}
	configs := backend.ConfiguredServers()
	instances := make([]mcphost.InstanceConfig, 0, len(configs))
	for _, config := range configs {
		instances = append(instances, mcphost.InstanceConfig{
			ID:       config.Name,
			Endpoint: config.Endpoint,
			Command:  config.Command,
			Args:     append([]string(nil), config.Args...),
			EnvFrom:  cloneStringMap(config.EnvFrom),
			Cwd:      config.Cwd,
			AuthEnv:  config.AuthEnv,
		})
	}
	host, err := mcphost.New(mcphost.Config{
		Instances:        instances,
		Factory:          existingMCPBackendSessionFactory{backend: backend},
		OperationTimeout: backend.operationTimeout,
		MaxSafeRetries:   backend.maxSafeRetries,
	})
	if err != nil {
		return nil
	}
	return mcphost.NewToolWorld(host)
}

var _ interface{ MCPToolWorldProvider() toolworld.Provider } = (*MCPBackend)(nil)
