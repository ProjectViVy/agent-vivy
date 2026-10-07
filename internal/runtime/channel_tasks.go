package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// ErrChannelTaskUnavailable reports channel-task admission without the
// atomic store and prompt-snapshot machinery it requires.
var ErrChannelTaskUnavailable = errors.New("runtime: channel task admission is not wired")

// channelTaskAdmissionGraceWindow bounds how long an uncommitted provisional
// resource may linger before the startup sweep reaps it. The window must
// exceed the maximum admission attempt duration; uncertain commits resolve
// via the durable receipt inside it.
const channelTaskAdmissionGraceWindow = 30 * time.Minute

// SweepChannelTaskOrphans is the runtime startup sweep for provisional
// channel-task resources (design 6.2 cleanup contract): workspace dirs that
// are empty, unlisted in runs, and older than the grace window are reaped.
// Frozen-persona orphans need the pinned laputa ListFrozenSessions
// enumeration, which is an owner-review pin bump; until it lands the sweep
// only covers workspaces. Call once at boot before accepting submissions.
func (s *Service) SweepChannelTaskOrphans(ctx context.Context) error {
	alloc, _ := s.deps.Workspaces.(AdmissionWorkspaceAllocator)
	if alloc == nil || s.deps.Runs == nil {
		return nil
	}
	return alloc.SweepAdmissionOrphans(ctx, func(runID domain.RunID) (bool, error) {
		_, err := s.deps.Runs.GetRun(ctx, runID)
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	}, time.Now().Add(-channelTaskAdmissionGraceWindow))
}

// ChannelTaskAdmission carries the host-validated A2A envelope through
// RunWithOptions. It is incompatible with every other admission axis: the
// channel-task commit transaction owns its own ordering, idempotency and
// evidence.
type ChannelTaskAdmission struct {
	Scope     domain.ChannelTaskScope
	MessageID string
	InputHash [32]byte
	// RunID is allocated by the caller before the durable commit so retries
	// and the channel adapter can name the same accepted run.
	RunID domain.RunID
	// NewSession, when non-nil, is the candidate Session the commit inserts
	// iff it is still absent; nil submits to an already-owned context.
	NewSession *domain.Session
	// Out receives the winning receipt — the fresh commit's or the original
	// winner's on a racing replay.
	Out *domain.ChannelTaskReceipt
}

// channelTaskMaxParts / channelTaskMaxTextBytes bound one external message's
// native cost before it can reach admission work.
const (
	channelTaskMaxParts     = 16
	channelTaskMaxTextBytes = 64 << 10
	channelTaskHashContract = "channel-task/v1"
	channelTaskOpSubmit     = "submit"
	channelTaskChannelName  = "a2a"
	channelTaskMaxIDBytes   = 256
)

// SubmitChannelTask admits one external message through the native primary
// run path (design §6.1–6.2). It returns the durable receipt — the fresh
// one, or the original winner's when the same scoped message already
// committed. A different body under the same message id is ErrConflict; a
// distinct message on a busy owned context is ErrWorkRunConflict.
func (s *Service) SubmitChannelTask(ctx context.Context, in domain.ChannelTaskInput) (domain.ChannelTaskReceipt, error) {
	if s.deps.ChannelTasks == nil || s.deps.Admission == nil {
		return domain.ChannelTaskReceipt{}, ErrChannelTaskUnavailable
	}
	if err := validateChannelTaskInput(in); err != nil {
		return domain.ChannelTaskReceipt{}, err
	}
	hash, err := channelTaskInputHash(in)
	if err != nil {
		return domain.ChannelTaskReceipt{}, err
	}
	// Cheap receipt resolution before any busy check, quota or allocation:
	// an accepted retry returns the original identity without touching run
	// admission. The commit transaction repeats this under the scope lock.
	receipt, found, err := s.deps.ChannelTasks.FindChannelTaskReceipt(ctx, in.Scope, in.MessageID)
	if err != nil {
		return domain.ChannelTaskReceipt{}, fmt.Errorf("runtime: read channel task receipt: %w", err)
	}
	if found {
		if receipt.InputHash != hash {
			return domain.ChannelTaskReceipt{}, storage.ErrConflict
		}
		return receipt, nil
	}

	var candidate *domain.Session
	sessionID := in.SessionID
	if sessionID == "" {
		candidate = &domain.Session{ID: domain.SessionID(newPrefixedID("sess_")), CreatedAt: time.Now().UnixMilli()}
		sessionID = candidate.ID
	}
	runID := in.RunID
	if runID == "" {
		runID = newRunID()
	}
	admission := &ChannelTaskAdmission{
		Scope: in.Scope, MessageID: in.MessageID, InputHash: hash,
		RunID: runID, NewSession: candidate, Out: &receipt,
	}
	options := RunOptions{
		Provenance: &domain.Provenance{
			Source: domain.SourceChannel, Channel: channelTaskChannelName,
			ChannelMessageID: in.MessageID,
		},
		ChannelTask: admission,
	}
	_, err = s.runWithAdmissionGate(ctx, sessionID, strings.Join(in.Parts, "\n\n"), options, nil, false)
	if admission.Out.RunID != "" {
		// Either this call committed or a racing identical commit did; Out
		// always names the winner.
		return *admission.Out, nil
	}
	if err != nil {
		// A conflict or busy may be a racing identical commit resolving
		// first: prefer the durable receipt when it now exists.
		if resolved, found, rerr := s.deps.ChannelTasks.FindChannelTaskReceipt(ctx, in.Scope, in.MessageID); rerr == nil && found {
			if resolved.InputHash == hash {
				return resolved, nil
			}
			return domain.ChannelTaskReceipt{}, storage.ErrConflict
		}
		return domain.ChannelTaskReceipt{}, err
	}
	// Defensive: the commit branch always fills Out, so this is only
	// reachable if the store accepted without writing the receipt.
	return domain.ChannelTaskReceipt{}, fmt.Errorf("runtime: channel task admission committed no receipt")
}

func validateChannelTaskInput(in domain.ChannelTaskInput) error {
	if in.Scope.InstanceKey == "" || in.Scope.PrincipalID == "" {
		return fmt.Errorf("runtime: channel task scope is required")
	}
	if !validChannelTaskID(in.MessageID) || (in.SessionID != "" && !validChannelTaskID(string(in.SessionID))) ||
		(in.RunID != "" && !validChannelTaskID(string(in.RunID))) {
		return fmt.Errorf("runtime: channel task id outside native limits")
	}
	if len(in.Parts) == 0 || len(in.Parts) > channelTaskMaxParts {
		return fmt.Errorf("runtime: channel task carries %d parts, want 1-%d", len(in.Parts), channelTaskMaxParts)
	}
	total := 0
	for _, part := range in.Parts {
		if !utf8.ValidString(part) {
			return fmt.Errorf("runtime: channel task part is not valid UTF-8")
		}
		total += len(part)
	}
	if total > channelTaskMaxTextBytes {
		return fmt.Errorf("runtime: channel task text %d bytes exceeds %d", total, channelTaskMaxTextBytes)
	}
	return nil
}

func validChannelTaskID(id string) bool {
	if id == "" || len(id) > channelTaskMaxIDBytes || !utf8.ValidString(id) {
		return false
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// channelTaskInputHash is the fixed-field receipt fingerprint (design §6
// canonical hash): contract tag, operation, the original optional context
// selector, ordered normalized text parts and the fixed task profile.
// Transport fields (JSON-RPC id, wait mode) never enter it.
func channelTaskInputHash(in domain.ChannelTaskInput) ([32]byte, error) {
	canonical := struct {
		Contract string           `json:"contract"`
		Op       string           `json:"operation"`
		Context  domain.SessionID `json:"context"`
		Parts    []string         `json:"parts"`
		Profile  string           `json:"profile"`
	}{
		Contract: channelTaskHashContract,
		Op:       channelTaskOpSubmit,
		Context:  in.SessionID,
		Parts:    append([]string(nil), in.Parts...),
		Profile:  "fixed",
	}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return [32]byte{}, fmt.Errorf("runtime: encode channel task hash input: %w", err)
	}
	return sha256.Sum256(raw), nil
}

// frozenSessionDiscarder is the optional loser-cleanup seam a cognitive
// bundle gains with the pinned-laputa DiscardSession extension (A2A-02.2,
// owner review). Until the pin carries it, orphan frozen rows age out via
// the startup sweep instead.
type frozenSessionDiscarder interface {
	DiscardFrozenSession(ctx context.Context, sessionID string) error
}

// discardFrozenCandidate releases a losing candidate's provisional FrozenCore
// row when the bound bundle exposes the seam. Nil-safe and optional: a bundle
// without it leaves the row for the restart sweep.
func (s *Service) discardFrozenCandidate(ctx context.Context, sessionID domain.SessionID) {
	b := s.deps.Cognitive
	if b == nil || b.Primary == nil {
		return
	}
	if discarder, ok := b.Primary.(frozenSessionDiscarder); ok {
		if err := discarder.DiscardFrozenSession(ctx, string(sessionID)); err != nil {
			// Log-only: the startup sweep remains the bounded backstop.
			slog.WarnContext(ctx, "channel task loser frozen row retained", "session", sessionID, "err", err)
		}
	}
}
