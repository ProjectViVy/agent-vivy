package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/storage"
	"github.com/ProjectViVy/laputa/laputa/actmem"
)

// This is a durable intent, never a receipt. Only the bound C1 lookup/commit can
// authorize retirement. Bodies remain in their existing retained source stores.
type recapPending struct {
	Version int
	Fold    recapFold
	Request cognitivecontract.RecapCommit
}

func recapPendingPrefix(binding storage.SubjectHistoryBinding) string {
	raw, _ := json.Marshal(binding)
	return "recap/pending/v1/" + sha256Hex(raw) + "/"
}
func recapPendingKey(p *recapPending) string {
	return recapPendingPrefix(p.Fold.Binding) + p.Request.OperationID
}

func (s *Service) persistRecapPending(ctx context.Context, p *recapPending) error {
	if s.deps.ToolEvidence == nil {
		return ErrRecapUnavailable
	}
	if _, ok := s.deps.ToolEvidence.(storage.BlobPrefixLister); !ok {
		return ErrRecapUnavailable
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	key := recapPendingKey(p)
	old, found, err := s.deps.ToolEvidence.Get(ctx, key)
	if err != nil {
		return err
	}
	if found && string(old) != string(raw) {
		return fmt.Errorf("%w: immutable pending identity mismatch", ErrRecapUnavailable)
	}
	if !found {
		if err = s.deps.ToolEvidence.Put(ctx, key, raw); err != nil {
			return err
		}
	}
	stored, found, err := s.deps.ToolEvidence.Get(ctx, key)
	if err != nil {
		return err
	}
	if !found || string(stored) != string(raw) {
		return fmt.Errorf("%w: pending read verification failed", ErrRecapUnavailable)
	}
	return nil
}
func matchesRecapPending(p *recapPending, h cognitivecontract.RecapHead) bool {
	r := p.Request
	return h.Revision == r.BaseRevision+1 && h.Binding == r.Binding && h.OperationID == r.OperationID && h.Text == r.Text && h.Cause == r.Cause && h.Sources == r.Sources && h.RestoredFrom == r.RestoredFrom && h.InvalidationEpoch == p.Fold.Head.InvalidationEpoch && h.CoveredThrough == r.Sources.Through && h.OperationDigest != ""
}
func (s *Service) lookupRecapPending(ctx context.Context, p *recapPending) (cognitivecontract.RecapHead, bool, error) {
	h, found, err := s.deps.Recap.LookupRecapOperation(ctx, p.Request.OperationID)
	if err != nil {
		return cognitivecontract.RecapHead{}, false, fmt.Errorf("%w: %w", ErrRecapCommitPending, err)
	}
	if !found {
		return cognitivecontract.RecapHead{}, false, nil
	}
	if !matchesRecapPending(p, h) {
		return cognitivecontract.RecapHead{}, false, fmt.Errorf("%w: operation receipt mismatch", ErrRecapUnavailable)
	}
	// Cleanup is advisory after a positive receipt. A failed delete leaves a
	// resolvable exact intent and can never turn this durable success into failure.
	_ = s.deps.ToolEvidence.Delete(ctx, recapPendingKey(p))
	return h, true, nil
}
func (s *Service) commitRecapPending(ctx context.Context, p *recapPending) (cognitivecontract.RecapHead, error) {
	s.recapCommitMu.Lock()
	defer s.recapCommitMu.Unlock()
	if err := s.validateRecapFold(ctx, &p.Fold); err != nil {
		return cognitivecontract.RecapHead{}, err
	}
	if err := s.persistRecapPending(ctx, p); err != nil {
		return cognitivecontract.RecapHead{}, err
	}
	h, err := s.deps.Recap.CommitRecap(ctx, p.Request)
	if err == nil {
		if !matchesRecapPending(p, h) {
			return cognitivecontract.RecapHead{}, ErrRecapCommitPending
		}
		_ = s.deps.ToolEvidence.Delete(context.WithoutCancel(ctx), recapPendingKey(p))
		return h, nil
	}
	resolved, found, lookupErr := s.lookupRecapPending(context.WithoutCancel(ctx), p)
	if lookupErr != nil {
		return cognitivecontract.RecapHead{}, lookupErr
	}
	if found {
		return resolved, nil
	}
	if actmem.CodeOf(err) == "recap_conflict" {
		if deleteErr := s.deps.ToolEvidence.Delete(context.WithoutCancel(ctx), recapPendingKey(p)); deleteErr != nil {
			return cognitivecontract.RecapHead{}, fmt.Errorf("%w: rejected intent cleanup: %v", ErrRecapCommitPending, deleteErr)
		}
		return cognitivecontract.RecapHead{}, errRecapCaptureChanged
	}
	return cognitivecontract.RecapHead{}, fmt.Errorf("%w: %w", ErrRecapCommitPending, err)
}

func (s *Service) resolvePendingRecaps(ctx context.Context, binding storage.SubjectHistoryBinding) error {
	s.recapCommitMu.Lock()
	defer s.recapCommitMu.Unlock()
	if s.deps.ToolEvidence == nil {
		return ErrRecapUnavailable
	}
	lister, ok := s.deps.ToolEvidence.(storage.BlobPrefixLister)
	if !ok {
		return ErrRecapUnavailable
	}
	keys, err := lister.ListPrefix(ctx, recapPendingPrefix(binding))
	if err != nil {
		return err
	}
	for _, key := range keys {
		raw, found, err := s.deps.ToolEvidence.Get(ctx, key)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		var p recapPending
		if err = json.Unmarshal(raw, &p); err != nil || p.Version != 1 || p.Fold.Binding != binding || recapPendingKey(&p) != key || p.Request.OperationID == "" || p.Request.Sources.Digest != p.Fold.Receipt {
			return ErrRecapUnavailable
		}
		if _, found, err := s.lookupRecapPending(ctx, &p); err != nil {
			return err
		} else if found {
			continue
		}
		// Authoritative negative resolution is necessary but insufficient: exact
		// request replay requires all original Finalize checks to still pass.
		if err = s.validateRecapFold(ctx, &p.Fold); err != nil {
			return fmt.Errorf("%w: pending source/base changed: %w", ErrRecapCommitPending, err)
		}
		rebuilt := p.Fold
		if err = s.populateRecapFold(ctx, &rebuilt); err != nil {
			return err
		}
		if rebuilt.Receipt != p.Fold.Receipt || rebuilt.InputDigest != p.Fold.InputDigest || !reflect.DeepEqual(rebuilt.CorrectionRefs, p.Fold.CorrectionRefs) {
			return ErrRecapCommitPending
		}
		if err = s.checkRecapSummaryCapacity(ctx, rebuilt.input, false); err != nil {
			return err
		}
		h, err := s.deps.Recap.CommitRecap(ctx, p.Request)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrRecapCommitPending, err)
		}
		if !matchesRecapPending(&p, h) {
			return ErrRecapCommitPending
		}
		_ = s.deps.ToolEvidence.Delete(ctx, key)
	}
	return nil
}
