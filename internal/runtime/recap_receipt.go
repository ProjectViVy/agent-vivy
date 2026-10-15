package runtime

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"

	"agent-vivy/internal/storage"
)

var ErrRecapUnavailable = errors.New("runtime: recap_unavailable")
var ErrRecapReconciliationRequired = errors.New("runtime: recap_reconciliation_required")

// The host alone interprets Sources.Digest. The owner stores this exact opaque
// envelope atomically with the text; source navigation and compaction reuse it.
const recapProjectionPolicy = 1

type recapReceipt struct {
	Version                  int                           `json:"v"`
	ProjectionPolicy         int                           `json:"projection_policy"`
	Binding                  storage.SubjectHistoryBinding `json:"binding"`
	Range                    recapReceiptRange             `json:"range"`
	Dispositions             recapReceiptDispositions      `json:"dispositions"`
	CorrectionRefs           []storage.SubjectDisposition  `json:"correction_refs"`
	CorrectionEvidenceDigest string                        `json:"correction_evidence_digest"`
}
type recapReceiptRange struct {
	After                uint64 `json:"after"`
	Through              uint64 `json:"through"`
	ExactOriginalsDigest string `json:"exact_originals_digest"`
}
type recapReceiptDispositions struct {
	Digest string `json:"digest"`
}

func validRecapDigest(v string) bool {
	raw, err := hex.DecodeString(v)
	return err == nil && len(raw) == 32 && v == hex.EncodeToString(raw)
}

func encodeRecapReceipt(receipt recapReceipt) (string, error) {
	// A JSON copy prevents sorting from mutating a captured producer snapshot.
	raw, err := json.Marshal(receipt)
	if err != nil {
		return "", ErrRecapUnavailable
	}
	var r recapReceipt
	if json.Unmarshal(raw, &r) != nil {
		return "", ErrRecapUnavailable
	}
	if err = validateRecapReceipt(r); err != nil {
		return "", err
	}
	for i := range r.CorrectionRefs {
		d := &r.CorrectionRefs[i]
		for j := range d.Targets {
			t := &d.Targets[j]
			type pair struct{ canonical, view string }
			pairs := make([]pair, len(t.MessageIDs))
			for k := range pairs {
				pairs[k] = pair{t.MessageIDs[k], t.ViewMessageIDs[k]}
			}
			sort.Slice(pairs, func(a, b int) bool {
				if pairs[a].canonical != pairs[b].canonical {
					return pairs[a].canonical < pairs[b].canonical
				}
				return pairs[a].view < pairs[b].view
			})
			for k, p := range pairs {
				t.MessageIDs[k], t.ViewMessageIDs[k] = p.canonical, p.view
			}
		}
		sort.Slice(d.Targets, func(a, b int) bool { return d.Targets[a].InteractionID < d.Targets[b].InteractionID })
	}
	sort.Slice(r.CorrectionRefs, func(a, b int) bool {
		if r.CorrectionRefs[a].SessionID != r.CorrectionRefs[b].SessionID {
			return r.CorrectionRefs[a].SessionID < r.CorrectionRefs[b].SessionID
		}
		return r.CorrectionRefs[a].MarkerID < r.CorrectionRefs[b].MarkerID
	})
	raw, err = json.Marshal(r)
	if err != nil {
		return "", ErrRecapUnavailable
	}
	return string(raw), nil
}
func decodeRecapReceipt(encoded string) (recapReceipt, error) {
	var r recapReceipt
	decoder := json.NewDecoder(bytes.NewBufferString(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&r) != nil {
		return r, ErrRecapUnavailable
	}
	if decoder.Decode(new(any)) != io.EOF {
		return r, ErrRecapUnavailable
	}
	if err := validateRecapReceipt(r); err != nil {
		return r, err
	}
	// Require canonical bytes, also rejecting duplicate JSON members.
	canonical, err := encodeRecapReceipt(r)
	if err != nil || canonical != encoded {
		return r, ErrRecapUnavailable
	}
	return r, nil
}
func validateRecapReceipt(r recapReceipt) error {
	if r.Version != 1 || r.ProjectionPolicy != recapProjectionPolicy || r.Binding.SubjectID == "" || r.Binding.DomainID == "" || r.Range.After > r.Range.Through || !validRecapDigest(r.Range.ExactOriginalsDigest) || !validRecapDigest(r.Dispositions.Digest) || !validRecapDigest(r.CorrectionEvidenceDigest) || r.CorrectionRefs == nil {
		return ErrRecapUnavailable
	}
	markers := map[string]map[uint64]bool{}
	for _, d := range r.CorrectionRefs {
		if d.SessionID == "" || d.MarkerID == 0 || d.Marker.SessionID != d.SessionID || (d.Marker.Reason != storage.TruncationRewind && d.Marker.Reason != storage.TruncationEdit) || len(d.Targets) == 0 {
			return ErrRecapUnavailable
		}
		key := string(d.SessionID)
		if markers[key] == nil {
			markers[key] = map[uint64]bool{}
		}
		if markers[key][d.MarkerID] {
			return ErrRecapUnavailable
		}
		markers[key][d.MarkerID] = true
		targets := map[string]bool{}
		for _, t := range d.Targets {
			if t.InteractionID == "" || t.Sequence == 0 || t.SourceSessionID == "" || t.RunID == "" || !validRecapDigest(t.SourceDigest) || len(t.MessageIDs) == 0 || len(t.MessageIDs) != len(t.ViewMessageIDs) || targets[t.InteractionID] {
				return ErrRecapUnavailable
			}
			targets[t.InteractionID] = true
			ids := map[string]bool{}
			views := map[string]bool{}
			for i, id := range t.MessageIDs {
				if id == "" || t.ViewMessageIDs[i] == "" || ids[id] || views[t.ViewMessageIDs[i]] {
					return ErrRecapUnavailable
				}
				ids[id] = true
				views[t.ViewMessageIDs[i]] = true
			}
		}
	}
	return nil
}

// validateRecapCorrectionReferences accepts only exact captured metadata and
// aligned message pairs actually selected for the summarizer. A later capture
// may contain more targets; historical receipts never expand to that larger set.
func validateRecapCorrectionReferences(refs []storage.SubjectDisposition, snapshot storage.SubjectContextSnapshot) error {
	for _, used := range refs {
		var captured *storage.SubjectDisposition
		for i := range snapshot.Dispositions {
			candidate := &snapshot.Dispositions[i]
			if candidate.SessionID == used.SessionID && candidate.MarkerID == used.MarkerID {
				captured = candidate
				break
			}
		}
		if captured == nil || captured.Marker != used.Marker {
			return ErrRecapUnavailable
		}
		for _, target := range used.Targets {
			var found *storage.SubjectDispositionTarget
			for i := range captured.Targets {
				candidate := &captured.Targets[i]
				if candidate.InteractionID == target.InteractionID {
					found = candidate
					break
				}
			}
			if found == nil || target.Sequence != found.Sequence || target.SourceSessionID != found.SourceSessionID || target.RunID != found.RunID || target.SourceDigest != found.SourceDigest || len(target.MessageIDs) != len(target.ViewMessageIDs) {
				return ErrRecapUnavailable
			}
			for i, id := range target.MessageIDs {
				matched := false
				for j, original := range found.MessageIDs {
					if original == id && j < len(found.ViewMessageIDs) && found.ViewMessageIDs[j] == target.ViewMessageIDs[i] {
						matched = true
						break
					}
				}
				if !matched {
					return ErrRecapUnavailable
				}
			}
		}
	}
	return nil
}
