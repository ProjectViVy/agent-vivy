package divacognitive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/cognitivecontract"

	"github.com/ProjectViVy/inofy"
	"github.com/ProjectViVy/laputa/garden/agentapi"
	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"
	laputainofy "github.com/ProjectViVy/laputa/laputa/evolution/inofy"
)

// Identity fixed by composition, never by payload. DIVA embeds exactly one
// owner; the profile scope is personal until a host-selected workspace
// exists.
const (
	profileID     = "diva"
	agentID       = "vivy-diva"
	platformID    = "embedded"
	workspaceID   = ""
	destinationID = agentapi.BackendMentle
)

// Open is the sealed Assembly factory for core/cognitive-factory@v1. It
// opens the one Garden owner entirely inside the host data directory, arms
// the capture/source adapters that need no memory backend, and returns the
// bound bundle. A selected-but-unarmed composition fails closed.
func Open(ctx context.Context, in cognitivecontract.FactoryInput) (cognitivecontract.Bundle, error) {
	dataRoot := in.Config.DataDirectory()
	if dataRoot == "" || !filepath.IsAbs(dataRoot) {
		return nil, fmt.Errorf("diva-cognitive: host data directory must be absolute")
	}
	gardenRoot := filepath.Join(dataRoot, "garden")
	owner, err := agentapi.Open(ctx, agentapi.Config{
		PersonaDir: filepath.Join(gardenRoot, "persona"),
		PalacePath: filepath.Join(gardenRoot, "palace", "palace.db"),
		ModelsDir:  filepath.Join(gardenRoot, "models"),
		StateDB:    filepath.Join(gardenRoot, "garden.db"),
		ProfileID:  profileID,
		AgentID:    agentID,
		Platform:   platformID,
		Principal:  agentapi.PrincipalUser,
	})
	if err != nil {
		return nil, fmt.Errorf("diva-cognitive: open garden owner: %w", err)
	}
	b := &bundle{owner: owner, strategyDigest: strategyDigest()}
	b.scope = laputaevolution.Scope{
		SubjectID:   profileID,
		Kind:        laputaevolution.ScopePersonal,
		WorkspaceID: workspaceID,
	}
	src, err := owner.BindEvolutionSource(b.scope, destinationID)
	if err != nil {
		_ = owner.Close()
		return nil, fmt.Errorf("diva-cognitive: bind evolution source: %w", err)
	}
	if b.strategyDigest == "" {
		_ = owner.Close()
		return nil, fmt.Errorf("diva-cognitive: strategy definition unavailable")
	}
	b.sourceID = src.SourceID
	b.source = boundSource{highWatermark: src.HighWatermark}
	b.sink = boundSink{client: owner}
	b.mission = boundMission{revision: src.MissionRevision}
	return b, nil
}

func strategyDigest() string {
	def, err := laputainofy.Definition()
	if err != nil {
		return ""
	}
	digest, err := inofy.DefinitionDigest(def)
	if err != nil {
		return ""
	}
	return digest
}

type bundle struct {
	owner          *agentapi.Client
	scope          laputaevolution.Scope
	sourceID       string
	strategyDigest string

	source  boundSource
	sink    boundSink
	mission boundMission

	mu       sync.Mutex
	policy   laputaevolution.TriggerPolicy
	ports    *agentapi.EvolutionPorts
	portErr  error
	control  cognitivecontract.ControlPort
	attached bool
	closed   bool
}

// Prepare returns the session-frozen primary context projection. The input
// session is bound at call time so no caller can select a foreign session.
func (b *bundle) Prepare(ctx context.Context, in cognitivecontract.PrimaryContextInput) (cognitivecontract.PreparedPrimaryContext, error) {
	if b.closed {
		return cognitivecontract.PreparedPrimaryContext{}, errors.New("diva-cognitive: bundle closed")
	}
	human, err := b.owner.BindHumanSession(string(in.SessionID), in.WorkspaceID)
	if err != nil {
		return cognitivecontract.PreparedPrimaryContext{}, err
	}
	frozen, err := human.ReadFrozen(ctx)
	if err != nil {
		var apiErr *agentapi.Error
		if !errors.As(err, &apiErr) || apiErr.Code != "not_found" {
			return cognitivecontract.PreparedPrimaryContext{}, err
		}
		// ADR-0012 captures the Frozen Core at session start; for a session
		// that never ran a recall the capture is still absent, so the first
		// Prepare performs it through the same SessionProvider.Get path a
		// bootstrap recall uses, then re-reads the persisted snapshot.
		bound, bindErr := b.owner.BindSession(string(in.SessionID))
		if bindErr != nil {
			return cognitivecontract.PreparedPrimaryContext{}, bindErr
		}
		if _, bootErr := bound.Bootstrap(ctx, agentapi.BootstrapRequest{}); bootErr != nil {
			return cognitivecontract.PreparedPrimaryContext{}, fmt.Errorf("diva-cognitive: session-start frozen capture: %w", bootErr)
		}
		if frozen, err = human.ReadFrozen(ctx); err != nil {
			return cognitivecontract.PreparedPrimaryContext{}, err
		}
	}
	// Reject v1/corrupt/oversize snapshots explicitly: the strict v2
	// validator also enforces the seven-slot roster and per-slot caps.
	if err := frozen.Validate(); err != nil {
		return cognitivecontract.PreparedPrimaryContext{}, fmt.Errorf("diva-cognitive: frozen core invalid: %w", err)
	}
	raw, err := json.Marshal(frozen)
	if err != nil {
		return cognitivecontract.PreparedPrimaryContext{}, err
	}
	sum := sha256.Sum256(raw)
	var text strings.Builder
	for _, section := range frozen.Sections {
		text.WriteString("# ")
		text.WriteString(string(section.Kind))
		text.WriteString("\n")
		text.WriteString(section.Content)
		text.WriteString("\n")
	}
	// Required authority never truncates: an over-budget FrozenCore is a
	// hard admission failure, not silent evidence loss.
	if in.BudgetBytes > 0 && text.Len() > in.BudgetBytes {
		return cognitivecontract.PreparedPrimaryContext{}, fmt.Errorf(
			"diva-cognitive: frozen core %d bytes exceeds authority budget %d", text.Len(), in.BudgetBytes)
	}
	return cognitivecontract.PreparedPrimaryContext{
		Frozen: frozen,
		Digest: hex.EncodeToString(sum[:]),
		Text:   text.String(),
	}, nil
}

// ResolveBinding produces the binding stamped into the admitted run input.
// MissionRevision is re-read at resolve time; the runtime re-verifies it at
// admission through CheckMissionRevision.
func (b *bundle) ResolveBinding(ctx context.Context) (laputaevolution.RunBinding, error) {
	if b.closed {
		return laputaevolution.RunBinding{}, errors.New("diva-cognitive: bundle closed")
	}
	revision, err := b.mission.revision(ctx)
	if err != nil {
		return laputaevolution.RunBinding{}, err
	}
	policyJSON, err := json.Marshal(b.policySnapshot())
	if err != nil {
		return laputaevolution.RunBinding{}, err
	}
	sum := sha256.Sum256(policyJSON)
	return laputaevolution.RunBinding{
		SubjectID:       b.scope.SubjectID,
		WorkspaceID:     b.scope.WorkspaceID,
		DestinationID:   destinationID,
		PolicyRevision:  hex.EncodeToString(sum[:]),
		StrategyDigest:  b.strategyDigest,
		MissionRevision: revision,
	}, nil
}

// BoundDomain returns the one bound Domain for the persisted binding. The
// selected memory backend is resolved here — a selected-but-unavailable
// writer fails this call, not capture/source arming.
func (b *bundle) BoundDomain(ctx context.Context, binding laputaevolution.RunBinding) (laputaevolution.Domain, error) {
	if b.closed {
		return nil, errors.New("diva-cognitive: bundle closed")
	}
	if binding.SubjectID != b.scope.SubjectID || binding.WorkspaceID != b.scope.WorkspaceID || binding.DestinationID != destinationID {
		return nil, fmt.Errorf("diva-cognitive: foreign binding rejected")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.portErr != nil {
		return nil, b.portErr
	}
	if b.ports == nil {
		ports, err := b.owner.BindEvolution(b.scope, destinationID)
		if err != nil {
			b.portErr = err
			return nil, fmt.Errorf("diva-cognitive: bind evolution domain: %w", err)
		}
		b.ports = &ports
	}
	return b.ports.Domain, nil
}

func (b *bundle) SourceID() string { return b.sourceID }

func (b *bundle) Source() cognitivecontract.Source         { return b.source }
func (b *bundle) Sink() cognitivecontract.CaptureSink      { return b.sink }
func (b *bundle) Mission() cognitivecontract.MissionSource { return b.mission }
func (b *bundle) Policy() laputaevolution.TriggerPolicy    { return b.policySnapshot() }

func (b *bundle) policySnapshot() laputaevolution.TriggerPolicy {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.policy
}

func (b *bundle) setPolicy(policy laputaevolution.TriggerPolicy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.policy = policy
}

// AttachRuntime stores the single armed control cell used by the generated
// action providers. It is single-use: a second attach or an attach after
// close fails instead of replacing the live cell.
func (b *bundle) AttachRuntime(control cognitivecontract.ControlPort) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("diva-cognitive: bundle closed")
	}
	if b.attached {
		return errors.New("diva-cognitive: runtime already attached")
	}
	b.control = control
	b.attached = true
	return nil
}

func (b *bundle) armed() (cognitivecontract.ControlPort, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, cognitivecontract.ErrUnarmed
	}
	if !b.attached || b.control == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	return b.control, nil
}

// Dispatcher returns the bundle's armed dispatch surface; the app-side
// ActionHost facade returns it to the generated action providers.
func (b *bundle) Dispatcher() cognitivecontract.Dispatcher { return dispatch{bundle: b} }

func (b *bundle) Close() error {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	return b.owner.Close()
}

func decodeInput(input json.RawMessage, out any) error {
	dec := json.NewDecoder(strings.NewReader(string(input)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("diva-cognitive: input: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("diva-cognitive: trailing data after input")
	}
	return nil
}

// boundSink is the host-owned durable capture seam: every run event is
// submitted under a per-call bound session so receipt identity is never
// caller-supplied.
type boundSink struct{ client *agentapi.Client }

func (s boundSink) Capture(ctx context.Context, cap cognitivecontract.Capture) (cognitivecontract.CaptureReceipt, error) {
	bound, err := s.client.BindSession(cap.SessionID)
	if err != nil {
		return cognitivecontract.CaptureReceipt{}, fmt.Errorf("diva-cognitive: bind session: %w", err)
	}
	phase, err := capturePhase(cap.Phase)
	if err != nil {
		return cognitivecontract.CaptureReceipt{}, err
	}
	sum := sha256.Sum256([]byte(cap.Content))
	receipt, err := bound.Capture(ctx, agentapi.CaptureRequest{
		Phase:       phase,
		Content:     cap.Content,
		ContentHash: "sha256:" + hex.EncodeToString(sum[:]),
		Provenance:  provenanceOf(cap),
		OccurredAt:  time.UnixMilli(cap.OccurredAt).UTC(),
	})
	if err != nil {
		return cognitivecontract.CaptureReceipt{}, fmt.Errorf("diva-cognitive: capture: %w", err)
	}
	return cognitivecontract.CaptureReceipt{
		IngestionID: receipt.IngestionID,
		Seq:         receipt.Seq,
		Status:      receipt.Status,
	}, nil
}

func capturePhase(phase string) (agentapi.CapturePhase, error) {
	switch phase {
	case "completed":
		return agentapi.CaptureCompleted, nil
	case "failed":
		return agentapi.CaptureFailed, nil
	case "canceled", "cancelled":
		return agentapi.CaptureCanceled, nil
	default:
		return "", fmt.Errorf("diva-cognitive: unknown capture phase %q", phase)
	}
}

func provenanceOf(cap cognitivecontract.Capture) agentapi.CaptureProvenance {
	runID, eventSeq := splitEventID(cap.EventID)
	return agentapi.CaptureProvenance{
		RunID:    runID,
		EventSeq: eventSeq,
	}
}

// splitEventID parses the runtime EventID "<run_id>:<journal_seq>"; a
// malformed id yields the zero provenance and the durable store rejects or
// stamps it verbatim.
func splitEventID(eventID string) (string, uint64) {
	idx := strings.LastIndex(eventID, ":")
	if idx < 0 {
		return eventID, 0
	}
	seq, err := strconv.ParseUint(eventID[idx+1:], 10, 64)
	if err != nil {
		return eventID, 0
	}
	return eventID[:idx], seq
}

type boundSource struct {
	highWatermark func(context.Context) (uint64, error)
}

func (s boundSource) HighWatermark(ctx context.Context) (uint64, error) {
	return s.highWatermark(ctx)
}

type boundMission struct {
	revision func(context.Context) (uint64, error)
}

func (m boundMission) MissionRevision(ctx context.Context) (uint64, error) {
	return m.revision(ctx)
}
