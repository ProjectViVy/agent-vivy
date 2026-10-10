// Package cognitivecontract defines the narrow core contracts between the
// sealed Assembly and one selected cognitive Module factory. It carries no
// optional Garden implementation and no internal/runtime import: the
// generated RuntimeAssembly binds a typed Factory value, and App asserts it
// strictly (agent-diva backend-separation-contracts C2-6).
package cognitivecontract

import (
	"context"
	"encoding/json"
	"errors"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	controlaction "agent-vivy/sdk/port/controlaction"

	laputaevolution "github.com/dashimaki/laputa/evolution"
)

// Factory is emitted into a generated RuntimeAssembly only when the
// composition selected core/cognitive-factory@v1. App unwraps the typed value
// through the generated CognitiveFactoryValue accessor and invokes it once.
type Factory func(context.Context, FactoryInput) (Bundle, error)

// FactoryInput carries the existing trusted config, the sealed Generation
// identity, and the owned storage dependency the durable trigger record is
// persisted under. It introduces no second storage DTO scheme.
type FactoryInput struct {
	// Config is the trusted embedder config; the factory derives every
	// Garden-owned path and identity from it.
	Config config.Config
	// GenerationID is the sealed Generation identity proven by the
	// composition; a selected factory may refuse an empty identity.
	GenerationID string
	// Store is the durable trigger/policy record owned by the host. A nil
	// Store keeps StartCognitiveWorkflow usable while automatic and manual
	// entry stay unavailable.
	Store storage.SnapshotStore
}

// ErrUnarmed fails closed when a bound port is reached before its runtime
// callback is attached or after the owner is closed.
var ErrUnarmed = errors.New("cognitivecontract: binding is not armed")

// Capture is the host-owned capture request for one terminal primary run.
// EventID is the stable redelivery key ("<run_id>:<journal_seq>"): the
// sink's dedupe boundary.
type Capture struct {
	SubjectID   string
	WorkspaceID string
	SessionID   string
	RunID       domain.RunID
	EventID     string
	Phase       string // completed | failed | canceled
	Content     string
	UserContent string // trusted admitted, redacted user source; never assistant/tool/system
	OccurredAt  int64  // unix ms
}

// CaptureReceipt is the durable acceptance returned by the bound capture
// surface. Seq is the committed-activity ledger position the evolution
// watermark advances against.
type CaptureReceipt struct {
	IngestionID string
	Seq         uint64
	Status      string
}

// CaptureSink is the ViVy-owned port to the bound capture surface.
// Implementations must return the original receipt on redelivery.
type CaptureSink interface {
	Capture(ctx context.Context, capture Capture) (CaptureReceipt, error)
}

// SessionFinalizer is the owned capture sink's optional host lifecycle port.
// Producer admission and terminal Observer delivery must be sealed/drained
// before it archives the original session's captured activity.
type SessionFinalizer interface {
	FinalizeSession(context.Context, string) error
}

// Source reports the committed-activity watermark of the bound input source.
// The evolution window's Through is read here so a crash between capture and
// wake cannot fabricate or lose input.
type Source interface {
	HighWatermark(ctx context.Context) (uint64, error)
}

// MissionSource reports the current authority Mission revision. The bound
// Domain (or its persona adapter) implements it; 0 means unassigned.
type MissionSource interface {
	MissionRevision(ctx context.Context) (uint64, error)
}

// PrimaryContextInput is the per-run authority preparation request. The
// session/run pair is already admitted; BudgetBytes bounds the projected
// authority text.
type PrimaryContextInput struct {
	SessionID   domain.SessionID
	RunID       domain.RunID
	WorkspaceID string
	BudgetBytes int
}

// PreparedPrimaryContext is the durable session FrozenCore v2 authority plus
// its digest and projected text. There is no new snapshot store: Garden
// remains the snapshot authority and reopened sessions read the same
// snapshot.
type PreparedPrimaryContext struct {
	Frozen laputaevolution.FrozenCoreV2
	Digest string
	Text   string
}

// Bundle is the single owner of the selected cognitive capability. The same
// generated adapters arm its ports: domain capture/source before observer
// recovery, primary preparation and runtime callbacks after Service, Close
// in reverse ownership order.
type Bundle interface {
	// Prepare projects durable FrozenCore v2 for one admitted primary run.
	Prepare(ctx context.Context, in PrimaryContextInput) (PreparedPrimaryContext, error)
	// ResolveBinding returns the per-admission run binding; source scope and
	// destination are fixed by the selected composition.
	ResolveBinding(ctx context.Context) (laputaevolution.RunBinding, error)
	// BoundDomain returns the persisted-binding Domain guard sharing the
	// host authority gate with human writes.
	BoundDomain(ctx context.Context, binding laputaevolution.RunBinding) (laputaevolution.Domain, error)
	// SourceID names the bound committed-activity source on every Window.
	SourceID() string
	// Source is the bound input watermark reader.
	Source() Source
	// Sink is the bound capture surface for terminal primary runs.
	Sink() CaptureSink
	// Mission is the authority Mission revision reader.
	Mission() MissionSource
	// Policy seeds the durable trigger policy on first load.
	Policy() laputaevolution.TriggerPolicy
	// AttachRuntime binds the runtime callbacks once. A selected bundle that
	// cannot arm never reports ready; a second attach fails.
	AttachRuntime(port ControlPort) error
	// Close releases the owned Garden runtime after the Service and observer
	// admission paths stop.
	Close() error
}

// ControlPort owns the cognitive control callbacks the armed bundle exposes
// to its compiled human actions: state read, policy CAS, manual trigger and
// cancellation of the current active strategy run.
type ControlPort interface {
	GetState(ctx context.Context) (ControlState, error)
	SetPolicyCAS(ctx context.Context, policy laputaevolution.TriggerPolicy, baseRevision uint64) (ControlState, error)
	Trigger(ctx context.Context) (ControlState, error)
	Cancel(ctx context.Context, runID domain.RunID) (ControlState, error)
}

// ControlState is the narrow control projection mapped verbatim to the
// ledger's CapabilityStatus.cognition: no second scheduler or policy store.
type ControlState struct {
	Enabled        bool
	MinIntervalMS  int64
	PolicyRevision uint64
	Eligibility    *laputaevolution.Eligibility
	ActiveRunID    string
	SourceID       string
	Watermark      uint64
	PendingThrough uint64
	Phase          string
	BlockReason    string
}

// Dispatcher is the armed dispatch surface the generated cognitive action
// providers call after Host authentication. The bound bundle arms it through
// AttachRuntime; every action not yet implemented fails closed with an
// unavailable error, never a success placeholder.
type Dispatcher interface {
	Invoke(ctx context.Context, actionID string, input json.RawMessage) (json.RawMessage, error)
}

// DispatcherProvider is implemented by the bound bundle so the ActionHost
// facade can hand the armed dispatcher to the generated providers.
type DispatcherProvider interface {
	Dispatcher() Dispatcher
}

// ActionHost is the private facade made available to the sealed T1 cognitive
// action owner only. The generated action providers resolve the armed
// dispatcher through it; an unarmed or foreign host fails closed. The facade
// is constructed by App during the same sealed composition step that calls
// Bundle.AttachRuntime.
type ActionHost interface {
	controlaction.Host
	// Cognitive returns the armed dispatcher for this host, or an
	// unavailable error when the selected composition has not attached its
	// runtime callbacks (DN-4C owns the full dispatch built on top of it).
	Cognitive() (Dispatcher, error)
}
