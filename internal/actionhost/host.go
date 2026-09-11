// Package actionhost owns the kernel boundary for std/control-action@v1.
// Providers are supplied by generated Assembly code at construction time;
// there is deliberately no runtime route registry. This package is the only
// place that resolves action ownership, instances, effective Grants, schemas,
// deadlines, Secrets, and audit projections around a Provider call.
package actionhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/logging"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/module"
	action "agent-vivy/sdk/port/controlaction"
)

// Focused error aliases make Host failures discoverable without requiring
// callers to import both the internal Host and public Port packages.
var (
	ErrInvalidDefinition         = action.ErrInvalidDefinition
	ErrInvalidInput              = action.ErrInvalidInput
	ErrInvalidOutput             = action.ErrInvalidOutput
	ErrOwnerMismatch             = action.ErrOwnerMismatch
	ErrActionNotFound            = action.ErrActionNotFound
	ErrUnauthenticated           = action.ErrUnauthenticated
	ErrGenerationUnavailable     = action.ErrGenerationUnavailable
	ErrInstanceUnavailable       = action.ErrInstanceUnavailable
	ErrGrantDenied               = action.ErrGrantDenied
	ErrApprovalRequired          = action.ErrApprovalRequired
	ErrSecretLeak                = action.ErrSecretLeak
	ErrActionTimeout             = action.ErrActionTimeout
	ErrProviderPanic             = action.ErrProviderPanic
	ErrProviderFailed            = action.ErrProviderFailed
	ErrHostClosed                = action.ErrHostClosed
	ErrDuplicateAction           = action.ErrDuplicateAction
	ErrAuthenticationUnavailable = action.ErrAuthenticationUnavailable
	ErrAuthorizationUnavailable  = action.ErrAuthorizationUnavailable
	ErrAuditUnavailable          = action.ErrAuditUnavailable
	ErrSecretUnavailable         = action.ErrSecretUnavailable
	ErrRunDenied                 = action.ErrRunDenied
	ErrToolDenied                = action.ErrToolDenied
	ErrConcurrencyLimit          = action.ErrConcurrencyLimit
)

const (
	defaultTimeout       = 30 * time.Second
	defaultMaxPayload    = 1 << 20
	maxAuditError        = 512
	defaultMaxConcurrent = 32
	defaultCloseWait     = 2 * time.Second
	maxRunTextBytes      = 64 << 10
	maxBridgeIDBytes     = 256
	maxAuditPayload      = 16 << 20
)

// publicFailure deliberately discards the cause. A Provider, resolver, or
// bridge callback can return an error containing credentials or process
// internals; exposing an Unwrap chain would make those bytes reachable through
// errors.Is/As even when Error() itself is sanitized.
func publicFailure(message string) error { return errors.New(message) }

// Caller is an opaque transport handle. It carries no identity, approval,
// trust, grant, or session fields. The Host passes it to the server-owned
// Authenticate callback, which must validate the transport binding and return
// a trusted Identity. NewCaller is intentionally not an authentication
// decision; possession of an arbitrary value is insufficient without that
// callback.
type Caller struct{ opaque string }

// NewCaller wraps an opaque transport-bound credential for the Host. Callers
// must not use user/session/approval data as the opaque value; the configured
// Authenticate callback remains the sole authority.
func NewCaller(opaque string) Caller { return Caller{opaque: strings.TrimSpace(opaque)} }

// Opaque returns a copy of the transport handle for the server-owned
// authenticator. It is not an identity claim and must never be used directly
// as an authorization decision.
func (caller Caller) Opaque() string { return caller.opaque }

// Identity is produced only by Authenticate. Its fields are trusted server
// state used to bind actions to a local operator/session and to audit them;
// callers cannot construct an authoritative Identity through Invoke.
type Identity struct {
	ID        string
	SessionID string
	Face      string
	RunID     string
}

func (identity Identity) valid() bool {
	return strings.TrimSpace(identity.ID) != "" &&
		len(identity.ID) <= maxBridgeIDBytes && !containsControl(identity.ID) &&
		len(identity.SessionID) <= maxBridgeIDBytes && !containsControl(identity.SessionID) &&
		len(identity.Face) <= maxBridgeIDBytes && !containsControl(identity.Face) &&
		len(identity.RunID) <= maxBridgeIDBytes && !containsControl(identity.RunID)
}

// WithIdentity records a server-authenticated identity in a context for a
// transport adapter. It is intentionally separate from Caller: the adapter
// must obtain the value from its own authentication middleware, not from
// browser JSON. ActionHost itself still requires Authenticate and never
// accepts this value as a replacement for that dependency.
type identityContextKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	if ctx == nil {
		return Identity{}, false
	}
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok && identity.valid()
}

// AuthenticatedCaller is a descriptive alias used by callers that want the
type AuthenticatedCaller = Caller

// InstanceState is the process truth for one configured Module instance.
type InstanceState string

const (
	InstanceConfigured   InstanceState = "configured"
	InstanceReady        InstanceState = "ready"
	InstanceUnavailable  InstanceState = "unavailable"
	InstanceInactive     InstanceState = "inactive"
	InstanceUnconfigured InstanceState = "unconfigured"
)

// InstanceStatus is returned by the Host's instance resolver. A status is
// usable only when State is ready, or when Available is explicitly true for a
// resolver that predates the State field.
type InstanceStatus struct {
	ModuleID     string
	ID           string
	State        InstanceState
	Available    bool
	GenerationID string
}

// ProviderBinding carries compiler-owned identity and effective Grants for a
// Provider. Generated Assembly normally uses this envelope so a Provider's
// self-description cannot widen the binding supplied by the compiler.
type ProviderBinding struct {
	ModuleID string
	// Owner is a source-compatible alias for ModuleID.
	Owner string
	// ActionID is the compiler-owned provider identity. The Provider's
	// Definition ID must match it exactly; a Provider cannot select an action
	// by returning a different self-description.
	ActionID string
	Provider action.Provider
	// EffectiveGrants is the compiler's frozen intersection for this Module.
	EffectiveGrants []module.GrantBinding
	// Grants is a source-compatible alias for EffectiveGrants.
	Grants     []module.GrantBinding
	InstanceID string
}

// Registration is an alias for ProviderBinding used by explicit assembly
// builders.
type Registration = ProviderBinding

// InstanceResolver resolves one declared runtime instance. It must not probe,
// revive, or configure the instance; an unavailable result is terminal for
// this invocation.
type InstanceResolver func(context.Context, string, string) (InstanceStatus, error)

// SecretResolver resolves one compiler-authorized Secret reference. Secret
// values are retained only in the invocation-local redaction set.
type SecretResolver interface {
	Resolve(context.Context, string, string, string) (string, error)
}

type SecretFunc func(context.Context, string, string, string) (string, error)

func (fn SecretFunc) Resolve(ctx context.Context, moduleID, instanceID, name string) (string, error) {
	if fn == nil {
		return "", errors.New("actionhost: nil SecretResolver")
	}
	return fn(ctx, moduleID, instanceID, name)
}

// RunFunc and ToolFunc are the only bridges a Provider may use for Kernel
// execution. App wiring supplies Service.RunWithOptions and the governed
// ToolHost boundary respectively.
type RunFunc func(context.Context, action.RunRequest) (action.RunResult, error)
type ToolFunc func(context.Context, string, json.RawMessage) (json.RawMessage, error)

// SettingsFunc returns an opaque, Host-owned settings projection. It may
// return nil to mean an empty object; values remain subject to the action
// output/schema policy when a Provider returns them.
type SettingsFunc func(context.Context, Identity, string, string) (json.RawMessage, error)

// AuthenticateFunc is the server-owned transport/session resolver. It must
// validate Caller.Opaque against the transport that admitted the request and
// return identity/session state from the server. Returning an identity copied
// from request JSON is a security bug; the Host validates the projection and
// never accepts a Caller field as authority.
type AuthenticateFunc func(context.Context, Caller) (Identity, error)

// BridgeKind names the two kernel execution bridges. Each bridge has a
// separate authorization check because a provider can request it after the
// action-level decision has completed.
type BridgeKind string

const (
	BridgeRun  BridgeKind = "run"
	BridgeTool BridgeKind = "tool"
)

// BridgeRequest is a bounded, server-side authorization input. Secret values
// are rejected by the Host before this reaches an AuthorizeBridge callback.
type BridgeRequest struct {
	Kind   BridgeKind
	Action action.Definition
	ToolID string
	Input  json.RawMessage
	Run    action.RunRequest
}

// AuthorizeFunc performs the server-side policy/approval decision required by
// every action Definition. The Host requires this dependency at construction;
// there is no default-allow path.
type AuthorizeFunc func(context.Context, Identity, action.Definition, json.RawMessage) error

// AuthorizeBridgeFunc performs a second policy/approval check immediately
// before a Provider can re-enter Service.Run or ToolHost. A nil callback means
// both bridges are denied, even when the action itself was authorized.
type AuthorizeBridgeFunc func(context.Context, Identity, BridgeRequest) error

// AuditOutcome values are intentionally closed and suitable for structured
// dashboards and Inspect projections.
type AuditOutcome string

const (
	AuditOutcomeSucceeded AuditOutcome = "succeeded"
	AuditOutcomeDenied    AuditOutcome = "denied"
	AuditOutcomeFailed    AuditOutcome = "failed"
	AuditOutcomeTimedOut  AuditOutcome = "timed_out"
	AuditOutcomeCancelled AuditOutcome = "cancelled"
	AuditOutcomeStarted   AuditOutcome = "started"
)

// AuditRecord is a bounded, Secret-free projection of one action attempt.
// Event is effect-specific (`action.read`, `action.write`, or
// `action.external-effect`) so consumers never infer mutation semantics from
// a free-form message.
type AuditRecord struct {
	InvocationID string
	Phase        string
	Event        string
	Effect       action.Effect
	ModuleID     string
	ActionID     string
	InstanceID   string
	CallerID     string
	Outcome      AuditOutcome
	Error        string
	InputBytes   int
	OutputBytes  int
	InputSHA256  string
	OutputSHA256 string
	DurationMS   int64
}

// AuditSink receives best-effort, structured action metadata after each
// decision. Sink failure cannot turn a completed Provider side effect into a
// successful retry or expose the raw payload.
type AuditSink interface {
	Record(context.Context, AuditRecord) error
}

// AuditRecorder is a descriptive alias used by integration code.
type AuditRecorder = AuditSink

// SlogAuditSink is the default process audit projection. It deliberately
// emits only bounded identity, effect, outcome, and digest metadata; action
// input/output and provider error text never enter the log record.
type SlogAuditSink struct {
	Logger *slog.Logger
}

func (sink SlogAuditSink) Record(_ context.Context, record AuditRecord) error {
	logger := sink.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("control action audit",
		"invocation", record.InvocationID,
		"phase", record.Phase,
		"event", record.Event,
		"effect", record.Effect,
		"module", record.ModuleID,
		"action", record.ActionID,
		"instance", record.InstanceID,
		"caller", record.CallerID,
		"outcome", record.Outcome,
		"error", record.Error,
		"input_bytes", record.InputBytes,
		"output_bytes", record.OutputBytes,
		"input_sha256", record.InputSHA256,
		"output_sha256", record.OutputSHA256,
		"duration_ms", record.DurationMS,
	)
	return nil
}

// AuditSinkFunc adapts a function to the mandatory audit boundary.
type AuditSinkFunc func(context.Context, AuditRecord) error

func (fn AuditSinkFunc) Record(ctx context.Context, record AuditRecord) error {
	if fn == nil {
		return action.ErrAuditUnavailable
	}
	return fn(ctx, record)
}

// JournalAuditSink persists the bounded action projection before and after a
// Provider effect. The synthetic run namespace is deliberately separate from
// user runs; action records are non-terminal append-only audit entries and
// never become executable Run state.
type JournalAuditSink struct {
	Journal storage.Journal
	Logger  *slog.Logger
}

func (sink JournalAuditSink) Record(ctx context.Context, record AuditRecord) error {
	if sink.Journal == nil || strings.TrimSpace(record.InvocationID) == "" {
		return action.ErrAuditUnavailable
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return action.ErrAuditUnavailable
	}
	if len(payload) > defaultMaxPayload {
		return action.ErrAuditUnavailable
	}
	_, err = sink.Journal.Append(ctx, storage.Commit{
		RunID: domain.RunID("action-audit-" + record.InvocationID),
		Events: []domain.RunEvent{{
			RunID:          domain.RunID("action-audit-" + record.InvocationID),
			Type:           domain.EventType("action.audit"),
			CreatedAt:      time.Now().UnixMilli(),
			PayloadVersion: 1,
			Payload:        payload,
		}},
	})
	if err != nil {
		return action.ErrAuditUnavailable
	}
	if sink.Logger != nil {
		sink.Logger.Debug("control action audit persisted", "invocation", record.InvocationID, "phase", record.Phase)
	}
	return nil
}

// Deps wires one immutable ActionHost. Providers and Bindings are Assembly
// inputs; the remaining callbacks are already-authoritative Kernel seams.
type Deps struct {
	// ProviderSets are generated Assembly envelopes. Their owner, action
	// allow-list, and grants are compiler-owned; the provider only supplies
	// schema/effect metadata after its identity is checked against this set.
	ProviderSets []action.ProviderSet
	// Providers and Actions are retained as deprecated source aliases only;
	// New rejects them because an unbound provider has no compiler-owned
	// identity or effective Grant projection.
	Providers []action.Provider
	// Actions is an alias for Providers.
	Actions  []action.Provider
	Bindings []ProviderBinding

	// GenerationAvailable is the sealed Generation readiness attestation. A
	// zero value is unavailable (fail closed).
	GenerationAvailable bool
	GenerationID        string

	// EffectiveGrants is keyed by Module ID and carries compiler-approved
	// intersections. Grants is a source-compatible alias.
	EffectiveGrants map[string][]module.GrantBinding
	Grants          map[string][]module.GrantBinding

	// AllowedTools is the compiler/runtime active ToolHost allow-list exposed
	// to an action bridge. An absent entry means no Tool bridge is permitted.
	AllowedTools map[string]struct{}

	// Instances accepts keys `moduleID\x00instanceID`, `moduleID/instanceID`,
	// or the bare instance ID. ResolveInstance takes precedence.
	Instances       map[string]InstanceStatus
	InstanceStates  map[string]InstanceStatus
	ResolveInstance InstanceResolver

	ResolveSecret SecretFunc
	// SecretResolver is an alias for ResolveSecret.
	SecretResolver SecretResolver
	// ResolveSecretValue supports a small host-owned resolver that does not
	// need Module/instance arguments.
	ResolveSecretValue func(context.Context, string) (string, error)

	Authenticate    AuthenticateFunc
	Authorize       AuthorizeFunc
	AuthorizeBridge AuthorizeBridgeFunc

	StartRun   RunFunc
	InvokeTool ToolFunc
	Settings   SettingsFunc

	Audit     AuditSink
	AuditSink AuditSink
	Logger    *slog.Logger

	// Timeout is the default per-action deadline. MaxTimeout is an immutable
	// ceiling; a Provider cannot extend it through Definition.Timeout.
	Timeout        time.Duration
	MaxTimeout     time.Duration
	MaxInputBytes  int
	MaxOutputBytes int
	MaxConcurrent  int
}

type registeredAction struct {
	provider   action.Provider
	definition action.Definition
	input      *jsonschema.Schema
	result     *jsonschema.Schema
	grants     map[module.Grant]module.GrantBinding
}

// Host is safe for concurrent Invoke calls. Registration occurs only during
// New; there is no Register or route-mutation method after construction.
type Host struct {
	mu             sync.RWMutex
	closed         bool
	providers      map[string]registeredAction // owner + NUL + action ID
	byID           map[string]string           // exact action ID -> owner
	definitions    []action.Definition
	deps           Deps
	logger         *slog.Logger
	inflight       map[uint64]context.CancelFunc
	nextInvocation uint64
	nextAudit      uint64
	semaphore      chan struct{}
	providerWG     sync.WaitGroup
}

// ActionHost is the descriptive public name of the concrete Host. The alias
// keeps the short Host name convenient inside the app while making the Port's
// one-consumer boundary explicit to RPC and conformance callers.
type ActionHost = Host

// New constructs one immutable ActionHost and validates all Provider
// Definitions deterministically. Duplicate IDs, invalid schemas, alias
// conflicts, and invalid owners fail before any Provider runs.
func New(deps Deps) (*Host, error) {
	deps = cloneDeps(deps)
	if len(deps.Providers) > 0 || len(deps.Actions) > 0 {
		return nil, fmt.Errorf("%w: unbound Providers are not accepted; use compiler ProviderBinding", action.ErrInvalidDefinition)
	}
	if deps.Audit == nil {
		deps.Audit = deps.AuditSink
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.MaxInputBytes < 0 || deps.MaxOutputBytes < 0 {
		return nil, fmt.Errorf("%w: negative payload bound", action.ErrInvalidDefinition)
	}
	if deps.Timeout < 0 || deps.MaxTimeout < 0 {
		return nil, fmt.Errorf("%w: negative timeout", action.ErrInvalidDefinition)
	}
	if deps.MaxTimeout == 0 {
		deps.MaxTimeout = defaultTimeout
	}
	if deps.Timeout == 0 {
		deps.Timeout = deps.MaxTimeout
	}
	if deps.Timeout > deps.MaxTimeout {
		deps.Timeout = deps.MaxTimeout
	}
	if deps.MaxInputBytes == 0 {
		deps.MaxInputBytes = defaultMaxPayload
	}
	if deps.MaxOutputBytes == 0 {
		deps.MaxOutputBytes = defaultMaxPayload
	}
	if deps.MaxConcurrent <= 0 {
		deps.MaxConcurrent = defaultMaxConcurrent
	}
	if deps.MaxConcurrent > 1024 {
		return nil, fmt.Errorf("%w: concurrency limit exceeds 1024", action.ErrInvalidDefinition)
	}
	hasInventory := len(deps.Bindings) > 0 || len(deps.ProviderSets) > 0
	if !hasInventory {
		// An empty generated action inventory is a disabled capability. It is
		// useful for the baseline Assembly and must not force application
		// startup to invent authentication, policy, audit, or Generation
		// authority for a host that cannot execute anything.
		return &Host{
			providers: make(map[string]registeredAction),
			byID:      make(map[string]string),
			deps:      deps,
			logger:    deps.Logger,
			inflight:  make(map[uint64]context.CancelFunc),
			semaphore: make(chan struct{}, deps.MaxConcurrent),
		}, nil
	}
	if deps.Authenticate == nil {
		return nil, action.ErrAuthenticationUnavailable
	}
	if deps.Authorize == nil {
		return nil, action.ErrAuthorizationUnavailable
	}
	if deps.Audit == nil {
		return nil, action.ErrAuditUnavailable
	}
	if !deps.GenerationAvailable || strings.TrimSpace(deps.GenerationID) == "" {
		return nil, action.ErrGenerationUnavailable
	}
	if err := validateProviderSets(deps.ProviderSets); err != nil {
		return nil, err
	}

	host := &Host{
		providers: make(map[string]registeredAction),
		byID:      make(map[string]string),
		deps:      deps,
		logger:    deps.Logger,
		inflight:  make(map[uint64]context.CancelFunc),
		semaphore: make(chan struct{}, deps.MaxConcurrent),
	}
	// Keep the provider/binding association as a slice. A Provider may be a
	// function-backed value (and therefore an unhashable interface value); a
	// map keyed by Provider would panic before definitions can be validated.
	type providerEntry struct {
		provider   action.Provider
		binding    ProviderBinding
		bound      bool
		allowedIDs []string
	}
	entries := make([]providerEntry, 0, len(deps.Bindings))
	for _, binding := range deps.Bindings {
		entries = append(entries, providerEntry{provider: binding.Provider, binding: binding, bound: true})
	}
	for _, set := range deps.ProviderSets {
		for _, provider := range set.Providers {
			entries = append(entries, providerEntry{
				provider: provider,
				binding: ProviderBinding{
					Provider: provider, ModuleID: set.ModuleID, EffectiveGrants: set.EffectiveGrants,
					InstanceID: set.InstanceID,
				},
				bound: true, allowedIDs: append([]string(nil), set.AllowedIDs...),
			})
		}
	}
	for _, entry := range entries {
		provider := entry.provider
		if provider == nil {
			return nil, fmt.Errorf("%w: nil Provider", action.ErrInvalidDefinition)
		}
		definition, err := providerDefinition(provider)
		if err != nil {
			return nil, err
		}
		binding, bound := entry.binding, entry.bound
		if bound {
			owner := strings.TrimSpace(binding.ModuleID)
			bindingOwner := strings.TrimSpace(binding.Owner)
			if owner != "" && bindingOwner != "" && owner != bindingOwner {
				return nil, fmt.Errorf("%w: Provider binding owner aliases disagree", action.ErrInvalidDefinition)
			}
			if owner == "" {
				owner = bindingOwner
			}
			if owner == "" {
				return nil, fmt.Errorf("%w: Provider binding module ID is required", action.ErrInvalidDefinition)
			}
			actionID := strings.TrimSpace(binding.ActionID)
			if actionID == "" && len(entry.allowedIDs) > 0 {
				definitionID := strings.TrimSpace(definition.ID)
				if !contains(entry.allowedIDs, definitionID) {
					return nil, fmt.Errorf("%w: Provider action ID %s is outside compiler allow-list", action.ErrOwnerMismatch, definitionID)
				}
				actionID = definitionID
			}
			if actionID == "" || len(actionID) > maxBridgeIDBytes || containsControl(actionID) {
				return nil, fmt.Errorf("%w: Provider binding action ID is required", action.ErrInvalidDefinition)
			}
			if binding.Provider == nil {
				return nil, fmt.Errorf("%w: nil Provider binding", action.ErrInvalidDefinition)
			}
			declaredOwner := firstNonEmpty(definition.Owner, definition.ModuleID)
			if declaredOwner == "" || declaredOwner != owner || (definition.Owner != "" && definition.ModuleID != "" && definition.Owner != definition.ModuleID) {
				return nil, fmt.Errorf("%w: Provider %s is bound to %s, declares %s", action.ErrOwnerMismatch, definition.ID, owner, declaredOwner)
			}
			definition.Owner = owner
			if definition.ID != actionID {
				return nil, fmt.Errorf("%w: Provider action ID %s does not match compiler binding %s", action.ErrOwnerMismatch, definition.ID, actionID)
			}
			instanceID := strings.TrimSpace(binding.InstanceID)
			if instanceID != "" {
				if definition.InstanceID == "" {
					definition.InstanceID = instanceID
				} else if definition.InstanceID != instanceID {
					return nil, fmt.Errorf("%w: Provider %s is bound to instance %s, declares %s", action.ErrInvalidDefinition, definition.ID, instanceID, definition.InstanceID)
				}
			}
		}
		normalized, err := definition.Normalize()
		if err != nil {
			return nil, err
		}
		if priorOwner, exists := host.byID[normalized.ID]; exists {
			return nil, fmt.Errorf("%w: %s owned by both %s and %s", action.ErrDuplicateAction, normalized.ID, priorOwner, normalized.Owner)
		}
		key := actionKey(normalized.Owner, normalized.ID)
		if _, exists := host.providers[key]; exists {
			return nil, fmt.Errorf("%w: %s", action.ErrDuplicateAction, normalized.ID)
		}
		input, err := compileSchema(normalized.InputSchema, normalized.ID, "input")
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", action.ErrInvalidDefinition, normalized.ID, err)
		}
		result, err := compileSchema(normalized.ResultSchema, normalized.ID, "result")
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", action.ErrInvalidDefinition, normalized.ID, err)
		}
		grants, err := effectiveGrantsFor(normalized.Owner, binding, bound)
		if err != nil {
			return nil, err
		}
		host.providers[key] = registeredAction{provider: provider, definition: normalized, input: input, result: result, grants: grants}
		host.byID[normalized.ID] = normalized.Owner
		host.definitions = append(host.definitions, normalized)
	}
	sort.Slice(host.definitions, func(i, j int) bool {
		if host.definitions[i].Owner != host.definitions[j].Owner {
			return host.definitions[i].Owner < host.definitions[j].Owner
		}
		return host.definitions[i].ID < host.definitions[j].ID
	})
	return host, nil
}

func validateProviderSets(sets []action.ProviderSet) error {
	for _, set := range sets {
		moduleID := strings.TrimSpace(set.ModuleID)
		if moduleID == "" || len(moduleID) > maxBridgeIDBytes || containsControl(moduleID) {
			return fmt.Errorf("%w: generated action set module ID is invalid", action.ErrInvalidDefinition)
		}
		if len(set.Providers) == 0 || len(set.AllowedIDs) == 0 {
			return fmt.Errorf("%w: generated action set %s is incomplete", action.ErrInvalidDefinition, moduleID)
		}
		seen := make(map[string]struct{}, len(set.AllowedIDs))
		for _, id := range set.AllowedIDs {
			id = strings.TrimSpace(id)
			if id == "" || len(id) > maxBridgeIDBytes || containsControl(id) {
				return fmt.Errorf("%w: generated action set %s contains an invalid action ID", action.ErrInvalidDefinition, moduleID)
			}
			if _, exists := seen[id]; exists {
				return fmt.Errorf("%w: generated action set %s repeats action ID", action.ErrDuplicateAction, moduleID)
			}
			seen[id] = struct{}{}
		}
		if set.InstanceID != "" && (len(set.InstanceID) > maxBridgeIDBytes || containsControl(set.InstanceID) || set.InstanceID != strings.TrimSpace(set.InstanceID)) {
			return fmt.Errorf("%w: generated action set %s instance ID is invalid", action.ErrInvalidDefinition, moduleID)
		}
		seenGrants := make(map[module.Grant]struct{}, len(set.EffectiveGrants))
		for _, grant := range set.EffectiveGrants {
			if !grant.Name.Valid() {
				return fmt.Errorf("%w: generated action set %s contains an unknown Grant", action.ErrGrantDenied, moduleID)
			}
			if _, exists := seenGrants[grant.Name]; exists {
				return fmt.Errorf("%w: generated action set %s repeats an effective Grant", action.ErrInvalidDefinition, moduleID)
			}
			seenGrants[grant.Name] = struct{}{}
		}
	}
	return nil
}

func cloneDeps(deps Deps) Deps {
	if deps.ProviderSets != nil {
		sets := deps.ProviderSets
		deps.ProviderSets = make([]action.ProviderSet, len(sets))
		for index, set := range sets {
			deps.ProviderSets[index] = action.ProviderSet{
				ModuleID: set.ModuleID, AllowedIDs: append([]string(nil), set.AllowedIDs...),
				Providers:       append([]action.Provider(nil), set.Providers...),
				EffectiveGrants: cloneGrantBindings(set.EffectiveGrants), InstanceID: set.InstanceID,
			}
		}
	}
	deps.Providers = append([]action.Provider(nil), deps.Providers...)
	deps.Actions = append([]action.Provider(nil), deps.Actions...)
	if deps.Bindings != nil {
		bindings := deps.Bindings
		deps.Bindings = make([]ProviderBinding, len(deps.Bindings))
		for index, binding := range bindings {
			deps.Bindings[index] = binding
			deps.Bindings[index].EffectiveGrants = cloneGrantBindings(binding.EffectiveGrants)
			deps.Bindings[index].Grants = cloneGrantBindings(binding.Grants)
		}
	}
	deps.EffectiveGrants = cloneGrantBindingMap(deps.EffectiveGrants)
	deps.Grants = cloneGrantBindingMap(deps.Grants)
	if deps.AllowedTools != nil {
		deps.AllowedTools = cloneStringSet(deps.AllowedTools)
	}
	deps.Instances = cloneInstanceMap(deps.Instances)
	deps.InstanceStates = cloneInstanceMap(deps.InstanceStates)
	return deps
}

func cloneStringSet(values map[string]struct{}) map[string]struct{} {
	if values == nil {
		return nil
	}
	cloned := make(map[string]struct{}, len(values))
	for value := range values {
		cloned[value] = struct{}{}
	}
	return cloned
}

func cloneGrantBindings(values []module.GrantBinding) []module.GrantBinding {
	if values == nil {
		return nil
	}
	cloned := make([]module.GrantBinding, len(values))
	for index, value := range values {
		cloned[index] = cloneGrantBinding(value)
	}
	return cloned
}

func cloneGrantBindingMap(values map[string][]module.GrantBinding) map[string][]module.GrantBinding {
	if values == nil {
		return nil
	}
	cloned := make(map[string][]module.GrantBinding, len(values))
	for owner, grants := range values {
		cloned[owner] = cloneGrantBindings(grants)
	}
	return cloned
}

func cloneInstanceMap(values map[string]InstanceStatus) map[string]InstanceStatus {
	if values == nil {
		return nil
	}
	cloned := make(map[string]InstanceStatus, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

// providerDefinition contains a malformed Provider at the registration
// boundary. A descriptor callback is untrusted plugin code just like Invoke;
// a panic must fail construction rather than bring down the process.
func providerDefinition(provider action.Provider) (definition action.Definition, err error) {
	defer func() {
		if recover() != nil {
			definition = action.Definition{}
			err = fmt.Errorf("%w: %w", action.ErrInvalidDefinition, action.ErrProviderPanic)
		}
	}()
	return provider.Definition(), nil
}

// NewHost is an explicit spelling for integrations that reserve New for a
// dependency-free constructor.
func NewHost(deps Deps) (*Host, error) { return New(deps) }

// NewActionHost is an explicit spelling for integrations that name the
// concrete consumer after the Port rather than using New.
func NewActionHost(deps Deps) (*Host, error) { return New(deps) }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func containsControl(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}

func actionRegistration(registered *registeredAction, exact bool) *registeredAction {
	if !exact || registered == nil || registered.provider == nil {
		return nil
	}
	return registered
}

func effectiveGrantsFor(owner string, binding ProviderBinding, bound bool) (map[module.Grant]module.GrantBinding, error) {
	if len(binding.EffectiveGrants) > 0 && len(binding.Grants) > 0 && !reflect.DeepEqual(binding.EffectiveGrants, binding.Grants) {
		return nil, fmt.Errorf("%w: effective Grants aliases disagree for %s", action.ErrInvalidDefinition, owner)
	}
	if !bound {
		return nil, fmt.Errorf("%w: unbound provider %s", action.ErrInvalidDefinition, owner)
	}
	// The binding is the only Grant authority. In particular, nil means an
	// explicit empty compiler projection, never permission to consult a broad
	// owner map supplied by a caller.
	values := binding.EffectiveGrants
	if values == nil {
		values = binding.Grants
	}
	result := make(map[module.Grant]module.GrantBinding, len(values))
	for _, grant := range values {
		if !grant.Name.Valid() {
			return nil, fmt.Errorf("%w: unknown effective Grant %q for %s", action.ErrGrantDenied, grant.Name, owner)
		}
		if _, exists := result[grant.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate effective Grant %q for %s", action.ErrInvalidDefinition, grant.Name, owner)
		}
		result[grant.Name] = cloneGrantBinding(grant)
	}
	return result, nil
}

func cloneGrantBinding(binding module.GrantBinding) module.GrantBinding {
	copy := module.GrantBinding{Name: binding.Name}
	if binding.Constraints != nil {
		copy.Constraints = make(map[string][]string, len(binding.Constraints))
		for key, values := range binding.Constraints {
			copy.Constraints[key] = append([]string(nil), values...)
		}
	}
	return copy
}

func actionKey(owner, id string) string { return owner + "\x00" + id }

// Definitions returns canonical, deterministic definitions for Inspect and
// conformance callers. Provider code is never invoked by this method.
func (host *Host) Definitions() []action.Definition {
	if host == nil {
		return nil
	}
	host.mu.RLock()
	declarations := append([]action.Definition(nil), host.definitions...)
	host.mu.RUnlock()
	for i := range declarations {
		declarations[i] = cloneDefinition(declarations[i])
	}
	return declarations
}

func cloneDefinition(definition action.Definition) action.Definition {
	definition.InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
	definition.ResultSchema = append(json.RawMessage(nil), definition.ResultSchema...)
	definition.OutputSchema = append(json.RawMessage(nil), definition.OutputSchema...)
	definition.RequiredGrants = append([]module.Grant(nil), definition.RequiredGrants...)
	definition.Grants = append([]module.Grant(nil), definition.Grants...)
	return definition
}

// Close prevents new invocations and cancels active Provider contexts. It is
// idempotent; already-running same-process Provider code remains responsible
// for respecting its context before returning.
func (host *Host) Close() error {
	if host == nil {
		return nil
	}
	host.mu.Lock()
	if host.closed {
		host.mu.Unlock()
		return nil
	}
	host.closed = true
	cancels := make([]context.CancelFunc, 0, len(host.inflight))
	for _, cancel := range host.inflight {
		cancels = append(cancels, cancel)
	}
	host.inflight = make(map[uint64]context.CancelFunc)
	host.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		host.providerWG.Wait()
		close(done)
	}()
	timer := time.NewTimer(defaultCloseWait)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return action.ErrActionTimeout
	}
}

// CloseContext is the bounded shutdown form used by compositions that own a
// shutdown deadline. Close remains available for small callers and uses the
// same fixed safety ceiling.
func (host *Host) CloseContext(ctx context.Context) error {
	if host == nil {
		return nil
	}
	host.mu.Lock()
	if !host.closed {
		host.closed = true
	}
	cancels := make([]context.CancelFunc, 0, len(host.inflight))
	for _, cancel := range host.inflight {
		cancels = append(cancels, cancel)
	}
	host.inflight = make(map[uint64]context.CancelFunc)
	host.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		host.providerWG.Wait()
		close(done)
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	waitCtx, waitCancel := context.WithTimeout(ctx, defaultCloseWait)
	defer waitCancel()
	select {
	case <-done:
		return nil
	case <-waitCtx.Done():
		return action.ErrActionTimeout
	}
}

// Invoke executes one exact owner/action pair through the frozen Host. It
// returns a copy of the validated JSON result; all failures are bounded and
// Secret-redacted.
func (host *Host) Invoke(ctx context.Context, caller Caller, moduleID, actionID string, input json.RawMessage) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	ctx = host.withAuditInvocationID(ctx)
	if host == nil {
		return nil, action.ErrHostClosed
	}
	requestCtx, requestCancel := context.WithTimeout(ctx, host.deps.Timeout)
	defer requestCancel()
	host.mu.RLock()
	closed := host.closed
	ready := host.deps.GenerationAvailable
	registered, exact := host.providers[actionKey(strings.TrimSpace(moduleID), strings.TrimSpace(actionID))]
	owner, knownID := host.byID[strings.TrimSpace(actionID)]
	host.mu.RUnlock()
	if err := requestCtx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, action.ErrActionTimeout
		}
		return nil, context.Canceled
	}
	if closed {
		return nil, host.denyAudit(ctx, &registered, nil, actionID, len(input), action.ErrHostClosed, started)
	}
	if !ready {
		return nil, host.denyAudit(ctx, actionRegistration(&registered, exact), nil, actionID, len(input), action.ErrGenerationUnavailable, started)
	}
	if !exact {
		if knownID && owner != strings.TrimSpace(moduleID) {
			// Recover the known declaration so a wrong-owner attempt still gets
			// the action's effect-specific audit class. Never echo the untrusted
			// caller strings in the public error.
			known := registeredAction{}
			host.mu.RLock()
			known, _ = host.providers[actionKey(owner, strings.TrimSpace(actionID))]
			host.mu.RUnlock()
			err := action.ErrOwnerMismatch
			return nil, host.denyAudit(ctx, &known, nil, actionID, len(input), err, started)
		}
		err := action.ErrActionNotFound
		return nil, err
	}
	definition := registered.definition
	if definition.Owner != strings.TrimSpace(moduleID) {
		err := action.ErrOwnerMismatch
		return nil, host.denyAudit(ctx, &registered, nil, actionID, len(input), err, started)
	}
	instanceID := definition.InstanceID
	identity, err := authenticate(host.deps.Authenticate, requestCtx, caller)
	if err != nil {
		return nil, host.denyAudit(ctx, &registered, nil, actionID, len(input), action.ErrUnauthenticated, started)
	}
	if status, resolveErr := host.resolveInstance(requestCtx, definition.Owner, instanceID); resolveErr != nil {
		return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrInstanceUnavailable, started)
	} else if instanceID != "" && !instanceMatches(status, definition.Owner, instanceID, host.deps.GenerationID) {
		return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrInstanceUnavailable, started)
	}
	for _, required := range definition.RequiredGrants {
		if _, ok := registered.grants[required]; !ok {
			return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrGrantDenied, started)
		}
	}
	maxInputBytes := host.actionInputLimit(definition)
	if len(input) == 0 || len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage("null")
	}
	if len(input) > maxInputBytes {
		return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrInvalidInput, started)
	}
	decodedInput, err := decodeJSON(input)
	if err != nil {
		return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrInvalidInput, started)
	}
	if err := registered.input.Validate(decodedInput); err != nil {
		return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrInvalidInput, started)
	}
	if err := authorize(host.deps.Authorize, requestCtx, identity, cloneDefinition(definition), append(json.RawMessage(nil), input...)); err != nil {
		return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), authorizationPublicError(definition), started)
	}

	if err := host.acquire(requestCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrActionTimeout, started)
		}
		return nil, host.denyAudit(ctx, &registered, &identity, actionID, len(input), action.ErrConcurrencyLimit, started)
	}
	if err := host.recordAudit(ctx, &registered, &identity, actionID, input, nil, AuditOutcomeStarted, "pre", nil, started); err != nil {
		host.release()
		return nil, err
	}
	invocationCtx, cancel := context.WithCancel(requestCtx)
	timeout := host.actionTimeout(definition)
	if timeout > 0 {
		var timeoutCancel context.CancelFunc
		invocationCtx, timeoutCancel = context.WithTimeout(invocationCtx, timeout)
		defer timeoutCancel()
	}
	invocationHost := &providerHost{parent: host, definition: definition, identity: identity, instanceID: instanceID, generationID: host.deps.GenerationID, ctx: invocationCtx, secrets: make([]string, 0, 2)}
	token, accepted := host.trackInvocation(cancel)
	if !accepted {
		host.release()
		return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, nil, AuditOutcomeCancelled, action.ErrHostClosed, started)
	}
	resultCh := make(chan providerResult, 1)
	go func() {
		defer host.providerWG.Done()
		defer host.release()
		result, invokeErr := invokeProvider(invocationCtx, registered.provider, invocationHost, append(json.RawMessage(nil), input...))
		resultCh <- providerResult{result: result, err: invokeErr}
	}()
	var providerOut providerResult
	select {
	case providerOut = <-resultCh:
		// Provider completed before the deadline.
	case <-requestCtx.Done():
		host.untrackInvocation(token)
		outcome, public := contextPublicError(host, ctx, requestCtx, invocationCtx)
		return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, nil, outcome, public, started)
	case <-invocationCtx.Done():
		host.untrackInvocation(token)
		if host.isClosed() {
			return nil, host.completionAudit(context.WithoutCancel(ctx), &registered, &identity, actionID, input, nil, AuditOutcomeCancelled, action.ErrHostClosed, started)
		}
		outcome, public := contextPublicError(host, ctx, requestCtx, invocationCtx)
		return nil, host.completionAudit(context.WithoutCancel(ctx), &registered, &identity, actionID, input, nil, outcome, public, started)
	}
	host.untrackInvocation(token)
	if err := invocationCtx.Err(); err != nil {
		outcome, public := contextPublicError(host, ctx, requestCtx, invocationCtx)
		return nil, host.completionAudit(context.WithoutCancel(ctx), &registered, &identity, actionID, input, nil, outcome, public, started)
	}
	if host.isClosed() {
		return nil, host.completionAudit(context.WithoutCancel(ctx), &registered, &identity, actionID, input, nil, AuditOutcomeCancelled, action.ErrHostClosed, started)
	}
	if providerOut.err != nil {
		if errors.Is(providerOut.err, action.ErrProviderPanic) {
			return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, nil, AuditOutcomeFailed, action.ErrProviderPanic, started)
		}
		if errors.Is(providerOut.err, action.ErrRunDenied) {
			return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, nil, AuditOutcomeDenied, action.ErrRunDenied, started)
		}
		if errors.Is(providerOut.err, action.ErrToolDenied) {
			return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, nil, AuditOutcomeDenied, action.ErrToolDenied, started)
		}
		if errors.Is(providerOut.err, action.ErrGrantDenied) {
			return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, nil, AuditOutcomeDenied, action.ErrGrantDenied, started)
		}
		if (errors.Is(providerOut.err, context.Canceled) || errors.Is(providerOut.err, context.DeadlineExceeded)) && invocationCtx.Err() != nil {
			outcome, public := contextPublicError(host, ctx, requestCtx, invocationCtx)
			auditCtx := ctx
			if outcome == AuditOutcomeTimedOut {
				auditCtx = context.WithoutCancel(ctx)
			}
			return nil, host.completionAudit(auditCtx, &registered, &identity, actionID, input, nil, outcome, public, started)
		}
		return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, nil, AuditOutcomeFailed, action.ErrProviderFailed, started)
	}
	output := bytes.TrimSpace(providerOut.result)
	maxOutputBytes := host.actionOutputLimit(definition)
	if len(output) == 0 || len(output) > maxOutputBytes {
		return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, output, AuditOutcomeFailed, action.ErrInvalidOutput, started)
	}
	secrets := invocationHost.secretValues()
	decodedOutput, err := decodeJSON(output)
	if err != nil {
		return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, output, AuditOutcomeFailed, action.ErrInvalidOutput, started)
	}
	if leaked := findSecret(output, secrets) || containsSecretValue(decodedOutput, secrets) || containsCredential(output); leaked {
		return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, output, AuditOutcomeFailed, action.ErrSecretLeak, started)
	}
	if err := registered.result.Validate(decodedOutput); err != nil {
		return nil, host.completionAudit(ctx, &registered, &identity, actionID, input, output, AuditOutcomeFailed, action.ErrInvalidOutput, started)
	}
	if err := host.completionAudit(ctx, &registered, &identity, actionID, input, output, AuditOutcomeSucceeded, nil, started); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), output...), nil
}

type auditInvocationContextKey struct{}

func (host *Host) withAuditInvocationID(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if host == nil {
		return ctx
	}
	host.mu.Lock()
	host.nextAudit++
	sequence := host.nextAudit
	host.mu.Unlock()
	return context.WithValue(ctx, auditInvocationContextKey{}, fmt.Sprintf("%x-%x", time.Now().UnixNano(), sequence))
}

func (host *Host) auditInvocationID(ctx context.Context, started time.Time) string {
	if ctx != nil {
		if id, ok := ctx.Value(auditInvocationContextKey{}).(string); ok && id != "" {
			return id
		}
	}
	if host == nil {
		return fmt.Sprintf("%x", started.UnixNano())
	}
	host.mu.Lock()
	host.nextAudit++
	sequence := host.nextAudit
	host.mu.Unlock()
	return fmt.Sprintf("%x-%x", started.UnixNano(), sequence)
}

type providerResult struct {
	result json.RawMessage
	err    error
}

func invokeProvider(ctx context.Context, provider action.Provider, host action.Host, input json.RawMessage) (result json.RawMessage, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			// Do not include recovered values: a panic value can contain a
			// Secret or arbitrary process internals.
			result = nil
			err = action.ErrProviderPanic
		}
	}()
	return provider.Invoke(ctx, host, input)
}

func (host *Host) trackInvocation(cancel context.CancelFunc) (uint64, bool) {
	host.mu.Lock()
	if host.closed {
		host.mu.Unlock()
		cancel()
		return 0, false
	}
	host.nextInvocation++
	token := host.nextInvocation
	host.providerWG.Add(1)
	host.inflight[token] = cancel
	host.mu.Unlock()
	return token, true
}

func (host *Host) untrackInvocation(token uint64) {
	host.mu.Lock()
	delete(host.inflight, token)
	host.mu.Unlock()
}

func (host *Host) actionTimeout(definition action.Definition) time.Duration {
	timeout := definition.Timeout
	if timeout <= 0 {
		timeout = host.deps.Timeout
	}
	if timeout > host.deps.MaxTimeout {
		timeout = host.deps.MaxTimeout
	}
	return timeout
}

func (host *Host) actionInputLimit(definition action.Definition) int {
	limit := host.deps.MaxInputBytes
	if definition.MaxInputBytes > 0 && definition.MaxInputBytes < limit {
		return definition.MaxInputBytes
	}
	return limit
}

func (host *Host) actionOutputLimit(definition action.Definition) int {
	limit := host.deps.MaxOutputBytes
	if definition.MaxOutputBytes > 0 && definition.MaxOutputBytes < limit {
		return definition.MaxOutputBytes
	}
	return limit
}

func (host *Host) acquire(ctx context.Context) error {
	select {
	case host.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (host *Host) release() {
	select {
	case <-host.semaphore:
	default:
	}
}

func contextPublicError(host *Host, callerCtx, requestCtx, invocationCtx context.Context) (AuditOutcome, error) {
	if host.isClosed() {
		return AuditOutcomeCancelled, action.ErrHostClosed
	}
	if callerCtx != nil && errors.Is(callerCtx.Err(), context.Canceled) {
		return AuditOutcomeCancelled, context.Canceled
	}
	if requestCtx != nil && errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
		return AuditOutcomeTimedOut, action.ErrActionTimeout
	}
	if invocationCtx != nil && errors.Is(invocationCtx.Err(), context.DeadlineExceeded) {
		return AuditOutcomeTimedOut, action.ErrActionTimeout
	}
	return AuditOutcomeCancelled, context.Canceled
}

func authenticate(fn AuthenticateFunc, ctx context.Context, caller Caller) (identity Identity, err error) {
	defer func() {
		if recover() != nil {
			identity = Identity{}
			err = action.ErrUnauthenticated
		}
	}()
	identity, err = fn(ctx, caller)
	if err != nil || !identity.valid() {
		return Identity{}, action.ErrUnauthenticated
	}
	return identity, nil
}

func authorize(fn AuthorizeFunc, ctx context.Context, identity Identity, definition action.Definition, input json.RawMessage) (err error) {
	defer func() {
		if recover() != nil {
			err = action.ErrAuthorizationUnavailable
		}
	}()
	return fn(ctx, identity, definition, input)
}

func (host *Host) resolveInstance(ctx context.Context, moduleID, instanceID string) (InstanceStatus, error) {
	if instanceID == "" {
		return InstanceStatus{ModuleID: moduleID, State: InstanceReady, Available: true, GenerationID: host.deps.GenerationID}, nil
	}
	if host.deps.ResolveInstance != nil {
		status, err := resolveInstance(host.deps.ResolveInstance, ctx, moduleID, instanceID)
		if err != nil {
			return InstanceStatus{}, action.ErrInstanceUnavailable
		}
		return status, nil
	}
	for _, key := range []string{moduleID + "\x00" + instanceID, moduleID + "/" + instanceID, instanceID} {
		if status, ok := host.deps.Instances[key]; ok {
			return status, nil
		}
		if status, ok := host.deps.InstanceStates[key]; ok {
			return status, nil
		}
	}
	return InstanceStatus{}, fmt.Errorf("%w: %s/%s was not resolved", action.ErrInstanceUnavailable, moduleID, instanceID)
}

func resolveInstance(fn InstanceResolver, ctx context.Context, moduleID, instanceID string) (status InstanceStatus, err error) {
	defer func() {
		if recover() != nil {
			status = InstanceStatus{}
			err = errors.New("instance resolver panicked")
		}
	}()
	return fn(ctx, moduleID, instanceID)
}

func instanceReady(status InstanceStatus, generationID string) bool {
	// Generation identity is an exact match whenever either side has one. An
	// instance from a different generation (or an un-attested instance when
	// this Host has a sealed identity) is never considered active.
	if status.GenerationID != generationID {
		return false
	}
	// Available is a legacy convenience bit, not an override for an explicit
	// non-ready state. A stale resolver must not revive an inactive instance by
	// claiming Available=true.
	if status.State != "" && status.State != InstanceReady {
		return false
	}
	if status.Available {
		return true
	}
	return status.State == InstanceReady
}

func instanceMatches(status InstanceStatus, moduleID, instanceID, generationID string) bool {
	if status.ModuleID != moduleID || status.ID != instanceID {
		return false
	}
	if generationID != "" && status.GenerationID != generationID {
		return false
	}
	return instanceReady(status, generationID)
}

func (host *Host) instanceActive(ctx context.Context, moduleID, instanceID, generationID string) bool {
	if host == nil || host.isClosed() {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	status, err := host.resolveInstance(ctx, moduleID, instanceID)
	if err != nil {
		return false
	}
	return instanceMatches(status, moduleID, instanceID, generationID)
}

func (host *Host) authorizeBridge(ctx context.Context, identity Identity, request BridgeRequest) error {
	if host == nil || host.deps.AuthorizeBridge == nil {
		return action.ErrAuthorizationUnavailable
	}
	if request.Kind != BridgeRun && request.Kind != BridgeTool {
		return action.ErrAuthorizationUnavailable
	}
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = action.ErrAuthorizationUnavailable
			}
		}()
		err = host.deps.AuthorizeBridge(ctx, identity, request)
	}()
	if err != nil {
		return action.ErrAuthorizationUnavailable
	}
	return nil
}

type providerHost struct {
	parent       *Host
	definition   action.Definition
	identity     Identity
	instanceID   string
	generationID string
	ctx          context.Context
	mu           sync.Mutex
	secrets      []string
}

func (host *providerHost) ModuleID() string { return host.definition.Owner }

// active is checked at every capability boundary, not just before the
// Provider goroutine starts. A Provider that ignores cancellation must not be
// able to keep resolving Secrets or trigger Run/Tool side effects after the
// Host has timed out or been closed.
func (host *providerHost) active() error {
	if host == nil || host.parent == nil {
		return action.ErrHostClosed
	}
	if host.parent.isClosed() {
		return action.ErrHostClosed
	}
	if host.ctx != nil {
		if err := host.ctx.Err(); err != nil {
			return err
		}
	}
	if !host.parent.instanceActive(host.ctx, host.definition.Owner, host.instanceID, host.generationID) {
		return action.ErrInstanceUnavailable
	}
	return nil
}

func (host *providerHost) Grant(grant module.Grant) error {
	if err := host.active(); err != nil {
		return err
	}
	if !host.definitionGrantDeclared(grant) {
		return fmt.Errorf("%w: %s did not request %s", action.ErrGrantDenied, host.definition.ID, grant)
	}
	host.parent.mu.RLock()
	registered, ok := host.parent.providers[actionKey(host.definition.Owner, host.definition.ID)]
	host.parent.mu.RUnlock()
	if !ok {
		return action.ErrHostClosed
	}
	if _, ok := registered.grants[grant]; !ok {
		return fmt.Errorf("%w: %s", action.ErrGrantDenied, grant)
	}
	return nil
}

func (host *providerHost) HasGrant(grant module.Grant) bool { return host.Grant(grant) == nil }

func (host *providerHost) definitionGrantDeclared(grant module.Grant) bool {
	for _, declared := range host.definition.RequiredGrants {
		if declared == grant {
			return true
		}
	}
	return false
}

func (host *providerHost) Secret(name string) (string, error) {
	if err := host.active(); err != nil {
		return "", err
	}
	if err := host.Grant(module.GrantSecretRead); err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("actionhost: Secret name is required")
	}
	host.parent.mu.RLock()
	registered, ok := host.parent.providers[actionKey(host.definition.Owner, host.definition.ID)]
	host.parent.mu.RUnlock()
	if !ok {
		return "", action.ErrHostClosed
	}
	if binding, ok := registered.grants[module.GrantSecretRead]; ok {
		// Secret grants are name-scoped. An absent or empty names constraint is
		// intentionally no authority; a broad secret.read grant must never turn
		// into arbitrary environment/configuration access.
		names := binding.Constraints["names"]
		if len(names) == 0 || !contains(names, name) {
			return "", action.ErrGrantDenied
		}
	} else {
		return "", action.ErrGrantDenied
	}
	var (
		value string
		err   error
	)
	resolver := host.parent.deps.ResolveSecret
	if resolver != nil {
		value, err = resolver(host.ctx, host.definition.Owner, host.instanceID, name)
	} else if host.parent.deps.SecretResolver != nil {
		value, err = host.parent.deps.SecretResolver.Resolve(host.ctx, host.definition.Owner, host.instanceID, name)
	} else if host.parent.deps.ResolveSecretValue != nil {
		value, err = host.parent.deps.ResolveSecretValue(host.ctx, name)
	} else {
		err = errors.New("actionhost: Secret resolver is unavailable")
	}
	if err != nil {
		return "", action.ErrSecretUnavailable
	}
	if strings.TrimSpace(value) == "" {
		return "", action.ErrSecretUnavailable
	}
	host.mu.Lock()
	host.secrets = append(host.secrets, value)
	host.mu.Unlock()
	return value, nil
}

func (host *providerHost) secretValues() []string {
	host.mu.Lock()
	defer host.mu.Unlock()
	return append([]string(nil), host.secrets...)
}

func (host *providerHost) Settings() json.RawMessage {
	if host.active() != nil {
		return json.RawMessage("{}")
	}
	if host.parent.deps.Settings == nil {
		return json.RawMessage("{}")
	}
	value, err := host.parent.deps.Settings(host.ctx, host.identity, host.definition.Owner, host.instanceID)
	if err != nil || len(bytes.TrimSpace(value)) == 0 {
		return json.RawMessage("{}")
	}
	if len(value) > host.parent.deps.MaxOutputBytes {
		return json.RawMessage("{}")
	}
	if _, err := decodeJSON(value); err != nil {
		return json.RawMessage("{}")
	}
	if redacted := logging.Redact(string(value)); redacted != string(value) || containsSecretValueString(string(value), host.secretValues()) {
		return json.RawMessage("{}")
	}
	return append(json.RawMessage(nil), value...)
}

func (host *providerHost) StartRun(ctx context.Context, request action.RunRequest) (action.RunResult, error) {
	if err := host.active(); err != nil {
		return action.RunResult{}, err
	}
	request.SessionID = strings.TrimSpace(request.SessionID)
	if host.identity.SessionID == "" || request.SessionID == "" || request.SessionID != host.identity.SessionID || len(request.SessionID) > maxBridgeIDBytes || containsControl(request.SessionID) {
		// Session ownership comes from the authenticated identity, never from
		// the Provider's RunRequest. The empty identity is fail-closed.
		return action.RunResult{}, action.ErrUnauthenticated
	}
	if len(request.Text) > maxRunTextBytes || containsSecretValueString(request.Text, host.secretValues()) {
		return action.RunResult{}, action.ErrSecretLeak
	}
	// Providers cannot choose a stronger mode/profile through this bridge.
	request.Mode, request.PolicyProfile = "", ""
	if err := host.parent.authorizeBridge(host.ctx, host.identity, BridgeRequest{Kind: BridgeRun, Action: cloneDefinition(host.definition), Run: request}); err != nil {
		return action.RunResult{}, action.ErrRunDenied
	}
	if err := host.active(); err != nil {
		return action.RunResult{}, action.ErrRunDenied
	}
	if host.parent.deps.StartRun == nil {
		return action.RunResult{}, action.ErrRunDenied
	}
	bridgeCtx, release := host.scopedContext(ctx)
	defer release()
	result, err := host.parent.deps.StartRun(bridgeCtx, request)
	if err != nil {
		return action.RunResult{}, action.ErrRunDenied
	}
	if containsSecretValueString(result.ID, host.secretValues()) || containsSecretValueString(result.Status, host.secretValues()) {
		return action.RunResult{}, action.ErrSecretLeak
	}
	return result, nil
}

// Run is a convenience alias for StartRun with the same authoritative path.
func (host *providerHost) Run(ctx context.Context, sessionID, text string) (action.RunResult, error) {
	return host.StartRun(ctx, action.RunRequest{SessionID: sessionID, Text: text})
}

func (host *providerHost) InvokeTool(ctx context.Context, id string, input json.RawMessage) (json.RawMessage, error) {
	if err := host.active(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" || len(id) > maxBridgeIDBytes || containsControl(id) {
		return nil, action.ErrToolDenied
	}
	if _, allowed := host.parent.deps.AllowedTools[id]; !allowed {
		return nil, action.ErrToolDenied
	}
	if len(input) > host.parent.deps.MaxInputBytes || len(input) == 0 || !json.Valid(input) {
		return nil, action.ErrToolDenied
	}
	if containsSecretValueString(string(input), host.secretValues()) {
		return nil, action.ErrSecretLeak
	}
	if err := host.parent.authorizeBridge(host.ctx, host.identity, BridgeRequest{Kind: BridgeTool, Action: cloneDefinition(host.definition), ToolID: id, Input: append(json.RawMessage(nil), input...)}); err != nil {
		return nil, action.ErrToolDenied
	}
	if err := host.active(); err != nil {
		return nil, action.ErrToolDenied
	}
	if host.parent.deps.InvokeTool == nil {
		return nil, action.ErrToolDenied
	}
	bridgeCtx, release := host.scopedContext(ctx)
	defer release()
	result, err := host.parent.deps.InvokeTool(bridgeCtx, id, append(json.RawMessage(nil), input...))
	if err != nil {
		return nil, action.ErrToolDenied
	}
	if len(result) > host.parent.deps.MaxOutputBytes || len(result) == 0 || !json.Valid(result) {
		return nil, action.ErrToolDenied
	}
	if containsSecretValueString(string(result), host.secretValues()) {
		return nil, action.ErrSecretLeak
	}
	return append(json.RawMessage(nil), result...), nil
}

// scopedContext prevents a Provider from escaping the Host-owned action
// deadline by passing context.Background (or another unrelated context) to a
// Run/Tool bridge. Provider cancellation is still honored when supplied.
func (host *providerHost) scopedContext(ctx context.Context) (context.Context, func()) {
	if ctx == nil {
		return host.ctx, func() {}
	}
	bridgeCtx, cancel := context.WithCancel(host.ctx)
	bridgeCtx = WithIdentity(bridgeCtx, host.identity)
	stop := context.AfterFunc(ctx, cancel)
	return bridgeCtx, func() {
		stop()
		cancel()
	}
}

func (host *Host) isClosed() bool {
	if host == nil {
		return true
	}
	host.mu.RLock()
	closed := host.closed
	host.mu.RUnlock()
	return closed
}

// ExecuteTool is a convenience alias for InvokeTool with the same governed
// ToolHost path.
func (host *providerHost) ExecuteTool(ctx context.Context, id string, input json.RawMessage) (json.RawMessage, error) {
	return host.InvokeTool(ctx, id, input)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (host *Host) safeError(err error, secrets []string) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED_SECRET]")
		}
	}
	text = logging.Redact(text)
	if len(text) > maxAuditError {
		text = text[:maxAuditError] + "…"
	}
	return text
}

func findSecret(payload []byte, secrets []string) bool {
	for _, secret := range secrets {
		if secret != "" && bytes.Contains(payload, []byte(secret)) {
			return true
		}
	}
	return false
}

func containsSecretValue(value any, secrets []string) bool {
	switch typed := value.(type) {
	case string:
		for _, secret := range secrets {
			if secret != "" && strings.Contains(typed, secret) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsSecretValue(item, secrets) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if containsSecretValue(item, secrets) {
				return true
			}
		}
	}
	return false
}

// containsCredential catches the same high-confidence credential/key-value
// forms that the ToolHost redaction boundary removes. A Control Action result
// is rejected rather than returned with a silently altered shape: callers
// must fix the Provider instead of accidentally treating a redacted token as
// a usable value.
func containsCredential(payload []byte) bool {
	redacted := logging.Redact(string(payload))
	return strings.Contains(redacted, "[REDACTED_SECRET]") || strings.Contains(redacted, "[REDACTED]")
}

func authorizationPublicError(definition action.Definition) error {
	if definition.RequiresApproval || definition.ApprovalRequired {
		return action.ErrApprovalRequired
	}
	return action.ErrAuthorizationUnavailable
}

func containsSecretValueString(value string, secrets []string) bool {
	if value == "" {
		return false
	}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(value, secret) {
			return true
		}
	}
	return false
}

func (host *Host) denyAudit(ctx context.Context, registered *registeredAction, identity *Identity, actionID string, inputBytes int, err error, started time.Time) error {
	if registered == nil {
		return err
	}
	if auditErr := host.recordAudit(ctx, registered, identity, actionID, nil, nil, AuditOutcomeDenied, "decision", err, started, inputBytes, 0); auditErr != nil {
		return auditErr
	}
	return err
}

func (host *Host) completionAudit(ctx context.Context, registered *registeredAction, identity *Identity, actionID string, input, output []byte, outcome AuditOutcome, err error, started time.Time) error {
	if auditErr := host.recordAudit(ctx, registered, identity, actionID, input, output, outcome, "completion", err, started, len(input), len(output)); auditErr != nil {
		return auditErr
	}
	return err
}

// recordAudit is mandatory and durable from the Host's perspective: a sink
// failure or panic is itself a fail-closed error. It hashes payload bytes only
// and emits bounded trusted metadata, never provider error causes or raw
// identity claims.
func (host *Host) recordAudit(ctx context.Context, registered *registeredAction, identity *Identity, actionID string, input, output []byte, outcome AuditOutcome, phase string, err error, started time.Time, sizes ...int) (auditErr error) {
	sink := host.deps.Audit
	if sink == nil {
		sink = host.deps.AuditSink
	}
	if sink == nil {
		return action.ErrAuditUnavailable
	}
	inputBytes, outputBytes := len(input), len(output)
	if len(sizes) == 2 {
		if input == nil {
			inputBytes = sizes[0]
		}
		if output == nil {
			outputBytes = sizes[1]
		}
	}
	identityID := ""
	if identity != nil {
		identityID = identity.ID
	}
	record := AuditRecord{
		InvocationID: host.auditInvocationID(ctx, started),
		Phase:        phase,
		ActionID:     auditIdentifier(actionID),
		CallerID:     auditIdentifier(identityID),
		Outcome:      outcome,
		InputBytes:   boundedSize(inputBytes),
		OutputBytes:  boundedSize(outputBytes),
		DurationMS:   maxInt64(0, time.Since(started).Milliseconds()),
	}
	if registered != nil {
		record.ModuleID = auditIdentifier(registered.definition.Owner)
		record.ActionID = auditIdentifier(registered.definition.ID)
		record.InstanceID = auditIdentifier(registered.definition.InstanceID)
		record.Effect = registered.definition.Effect
		record.Event = "action." + string(registered.definition.Effect)
	}
	if err != nil {
		record.Error = auditError(err)
	}
	if len(input) > 0 {
		digest := sha256.Sum256(input)
		record.InputSHA256 = hex.EncodeToString(digest[:])
	} else if inputBytes > 0 {
		record.InputSHA256 = digestPlaceholder(inputBytes, actionID)
	}
	if len(output) > 0 {
		digest := sha256.Sum256(output)
		record.OutputSHA256 = hex.EncodeToString(digest[:])
	} else if outputBytes > 0 {
		record.OutputSHA256 = digestPlaceholder(outputBytes, actionID)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// Auditing is part of the action admission/deadline budget. Preserve the
	// caller/request cancellation and use the Host timeout as the upper bound
	// when the caller supplied no deadline; a slow sink must never turn a
	// bounded action into an unbounded request.
	auditBudget := 2 * time.Second
	if host != nil && host.deps.Timeout > 0 && host.deps.Timeout < auditBudget {
		auditBudget = host.deps.Timeout
	}
	auditCtx, cancel := context.WithTimeout(ctx, auditBudget)
	defer cancel()
	defer func() {
		if recover() != nil {
			auditErr = action.ErrAuditUnavailable
		}
	}()
	if err := sink.Record(auditCtx, record); err != nil {
		return action.ErrAuditUnavailable
	}
	return nil
}

func auditError(err error) string {
	switch {
	case errors.Is(err, action.ErrUnauthenticated):
		return action.ErrUnauthenticated.Error()
	case errors.Is(err, action.ErrGenerationUnavailable):
		return action.ErrGenerationUnavailable.Error()
	case errors.Is(err, action.ErrInstanceUnavailable):
		return action.ErrInstanceUnavailable.Error()
	case errors.Is(err, action.ErrGrantDenied):
		return action.ErrGrantDenied.Error()
	case errors.Is(err, action.ErrApprovalRequired):
		return action.ErrApprovalRequired.Error()
	case errors.Is(err, action.ErrAuthorizationUnavailable):
		return action.ErrAuthorizationUnavailable.Error()
	case errors.Is(err, action.ErrSecretLeak):
		return action.ErrSecretLeak.Error()
	case errors.Is(err, action.ErrProviderPanic):
		return action.ErrProviderPanic.Error()
	case errors.Is(err, action.ErrProviderFailed):
		return action.ErrProviderFailed.Error()
	case errors.Is(err, action.ErrAuditUnavailable):
		return action.ErrAuditUnavailable.Error()
	case errors.Is(err, action.ErrSecretUnavailable):
		return action.ErrSecretUnavailable.Error()
	case errors.Is(err, action.ErrRunDenied):
		return action.ErrRunDenied.Error()
	case errors.Is(err, action.ErrToolDenied):
		return action.ErrToolDenied.Error()
	case errors.Is(err, action.ErrConcurrencyLimit):
		return action.ErrConcurrencyLimit.Error()
	case errors.Is(err, action.ErrActionTimeout):
		return action.ErrActionTimeout.Error()
	case errors.Is(err, action.ErrHostClosed):
		return action.ErrHostClosed.Error()
	case errors.Is(err, action.ErrInvalidInput):
		return action.ErrInvalidInput.Error()
	case errors.Is(err, action.ErrInvalidOutput):
		return action.ErrInvalidOutput.Error()
	default:
		return "controlaction: action failed"
	}
}

func boundedSize(value int) int {
	if value < 0 {
		return 0
	}
	if value > maxAuditPayload {
		return maxAuditPayload
	}
	return value
}

func maxInt64(value, floor int64) int64 {
	if value < floor {
		return floor
	}
	return value
}

func auditIdentifier(value string) string {
	value = logging.Redact(strings.TrimSpace(value))
	const maxIdentifierBytes = 256
	if len(value) > maxIdentifierBytes {
		return value[:maxIdentifierBytes] + "…"
	}
	return value
}

// digestPlaceholder produces a stable non-secret correlation token when the
// audit API is given only bounded sizes at an early failure. Successful calls
// use actual payload digests below; the placeholder intentionally cannot be
// confused with an input/output content hash.
func digestPlaceholder(size int, actionID string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("size:%d action:%s", size, actionID)))
	return "size:" + hex.EncodeToString(digest[:8])
}

func compileSchema(raw json.RawMessage, id, kind string) (*jsonschema.Schema, error) {
	doc, err := decodeJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("%s schema is invalid JSON: %v", kind, err)
	}
	if _, ok := doc.(map[string]any); !ok {
		return nil, fmt.Errorf("%s schema must be an object", kind)
	}
	if err := rejectExternalRefs(doc); err != nil {
		return nil, fmt.Errorf("%s schema: %v", kind, err)
	}
	location := "action://" + strings.ReplaceAll(id, "/", "_") + "/" + kind
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(location, doc); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(location)
	if err != nil {
		return nil, err
	}
	return compiled, nil
}

func rejectExternalRefs(value any) error {
	var walk func(any) error
	walk = func(node any) error {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "$ref" {
					ref, ok := child.(string)
					if !ok || (!strings.HasPrefix(ref, "#") && !strings.HasPrefix(ref, "action://")) {
						return errors.New("external $ref is not allowed")
					}
				}
				if err := walk(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(value)
}

func decodeJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func schemaError(err error) string {
	if err == nil {
		return "schema validation failed"
	}
	// jsonschema validation errors may echo the offending scalar value. That
	// value is caller/provider data and can be a credential even when it does
	// not match one of the kernel's high-confidence redaction patterns. Keep
	// the taxonomy and a stable reason while dropping the value entirely.
	return "schema validation failed"
}
