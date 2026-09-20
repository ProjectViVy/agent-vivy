// Package channelcontract defines the implementation-free composition seam
// for the canonical build-owned Channel Host Module.
package channelcontract

import (
	"context"
	"encoding/json"
	"log/slog"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

// RunCallback enters the organism's one Service.RunWithOptions path.
type RunCallback func(context.Context, domain.SessionID, string, *domain.Provenance) (domain.RunID, error)

// CredentialResolver is the focused credential authority required by the
// existing Channel Host. The selected Module receives this facade; it never
// owns or enumerates the credential store.
type CredentialResolver interface {
	Resolve(moduleID, ref string) (string, error)
	IsSet(moduleID, ref string) bool
}

// ProviderConfig is inert effective configuration for one selected Provider.
// Platform validation and activation belong to the canonical implementation.
type ProviderConfig struct {
	Enabled   bool
	AllowFrom []string
	TokenEnv  string
	Settings  json.RawMessage
}

// Config maps a Provider name to its effective configuration.
type Config map[string]ProviderConfig

// ProviderState is process truth for one selected Provider.
type ProviderState struct {
	Name       string
	Compiled   bool
	Configured bool
	Running    bool
	Error      string
}

// State keeps immutable Generation presence separate from process
// availability, including the existing WithoutEars process override.
type State struct {
	Compiled         bool
	ProcessAvailable bool
	Providers        []ProviderState
}

// Dependencies contains focused existing authorities. The Channel Module
// does not create or own these services.
type Dependencies struct {
	Journal     storage.Journal
	Messages    storage.MessageStore
	Sessions    storage.SessionStore
	Credentials CredentialResolver
	Logger      *slog.Logger
	Run         RunCallback
}

// Selection is the generated Provider/Grant set plus effective inert config.
// An empty Providers slice is valid.
type Selection struct {
	Providers []channel.ChannelProvider
	Grants    map[string][]module.GrantBinding
	Config    Config
}

// Factory constructs the single owned Channel instance without starting
// networking. Generated Assembly wiring supplies the selected implementation.
type Factory interface {
	Construct(context.Context, Dependencies, Selection) (Owned, error)
}

// Owned is the complete internal surface exposed by the selected Channel
// Module. Lifecycle, validation, settings I/O, and Provider activation remain
// implementation responsibilities; app and RPC consume only these facets.
type Owned interface {
	module.Instance
	runtime.RunHook
	runtime.ChannelDeliverer
	rpc.Contribution
	Inspect() State
}
