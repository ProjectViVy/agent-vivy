// Package channelcontract defines the implementation-free composition seam
// for the canonical build-owned Channel Host Module.
package channelcontract

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/rpccontract"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/channel"
)

// PrepareRunCallback registers owner state that must exist before runtime
// persistence or events become visible.
type PrepareRunCallback func(domain.RunID) error

// RunCallback enters the organism's one Service.RunWithOptions path.
type RunCallback func(
	context.Context,
	domain.SessionID,
	string,
	[]domain.Attachment,
	*domain.Provenance,
	PrepareRunCallback,
) (domain.RunID, error)

// DecideApprovalCallback settles one approval through the organism's
// server-attributed approval path.
type DecideApprovalCallback func(context.Context, string, string, string) error

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

// ChannelOverlay is the persisted, optional per-field override for one
// Provider. Pointer fields preserve the distinction between absent and an
// explicit zero value, including an intentionally empty allowlist.
type ChannelOverlay struct {
	Name      string
	Enabled   *bool
	AllowFrom *[]string
	TokenEnv  *string
}

// Settings is the Channel-owned projection of the shared settings document.
// The adapter behind SettingsAccess preserves all non-Channel document data.
type Settings struct {
	Channels []ChannelOverlay
}

// SettingsAccess is the focused authority needed by Channel management
// methods. Writable and Frozen preserve the current deployment policy without
// exposing a settings path or the shared document implementation.
type SettingsAccess interface {
	Read(context.Context) (Settings, error)
	Update(context.Context, ChannelOverlay) (Settings, error)
	Writable() bool
	Frozen() bool
}

// ErrInvalidSettings classifies a rejected Channel settings update without
// coupling the canonical Module to the application's settings package.
var ErrInvalidSettings = errors.New("invalid channel settings")

type invalidSettingsError struct {
	cause error
}

func (e invalidSettingsError) Error() string { return e.cause.Error() }
func (e invalidSettingsError) Unwrap() error { return e.cause }
func (e invalidSettingsError) Is(target error) bool {
	return target == ErrInvalidSettings
}

// MarkInvalidSettings preserves the cause and its message while adding the
// implementation-free invalid-settings classification.
func MarkInvalidSettings(cause error) error {
	if cause == nil {
		return nil
	}
	return invalidSettingsError{cause: cause}
}

// Dependencies contains focused existing authorities. The Channel Module
// does not create or own these services.
type Dependencies struct {
	Journal        storage.Journal
	Messages       storage.MessageStore
	Sessions       storage.SessionStore
	Deliveries     storage.ChannelDeliveryStore
	Maintenance    storage.ChannelMaintenanceStore
	Approvals      storage.ApprovalStore
	Runs           storage.RunStore
	DecideApproval DecideApprovalCallback
	Credentials    CredentialResolver
	Settings       SettingsAccess
	Logger         *slog.Logger
	Run            RunCallback
	// OnSettingsChanged is called after a successful settings update so the
	// application can refresh its current settings projection.
	OnSettingsChanged func()
}

// Selection is the generated Provider/Grant set plus effective inert config.
// An empty Providers slice is valid.
type Selection struct {
	Providers        []channel.ChannelProvider
	Grants           map[string][]module.GrantBinding
	Config           Config
	ProcessAvailable bool
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
	rpccontract.Contribution
	Inspect() State
}
