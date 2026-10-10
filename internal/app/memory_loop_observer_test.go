package app

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
	laputaevolution "github.com/dashimaki/laputa/evolution"
)

func (f *memoryLoopFixture) cognitiveValue(ctx context.Context, action, sessionID string, extra map[string]any, dst any) error {
	input := map[string]any{"session_id": sessionID}
	for k, v := range extra {
		input[k] = v
	}
	args, err := json.Marshal(map[string]any{"module_id": "vivy/diva-cognitive", "action_id": action, "input": input})
	if err != nil {
		return err
	}
	raw, err := f.Call(ctx, "module.action.invoke", args)
	if err != nil {
		return err
	}
	var out struct {
		Status string          `json:"status"`
		Value  json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if out.Status != "ok" {
		return fmt.Errorf("%s: %s", action, raw)
	}
	return json.Unmarshal(out.Value, dst)
}

// Observe committed effects and the processed source window through public
// controls, then read the real canonical body. Never manufacture a receipt.
func (f *memoryLoopFixture) reflectedSnapshot(ctx context.Context, runID string) (memoryLoopSnapshot, bool, error) {
	snap, ready, err := f.snapshot(ctx, "canonical", runID)
	if err != nil || !ready {
		return snap, false, err
	}
	run, err := f.app.backend.GetRun(ctx, domain.RunID(runID))
	if err != nil {
		return snap, false, err
	}
	var status struct {
		Scope     laputaevolution.Scope `json:"scope"`
		Cognition struct {
			SourceID    string `json:"source_id"`
			Watermark   uint64 `json:"watermark"`
			Phase       string `json:"phase"`
			BlockReason string `json:"block_reason"`
		} `json:"cognition"`
	}
	if err := f.cognitiveValue(ctx, "diva.cognitive.status", string(run.SessionID), nil, &status); err != nil {
		return snap, false, err
	}
	snap.ProcessedThrough = status.Cognition.Watermark
	snap.SourceProviderID, snap.BoundScope = status.Cognition.SourceID, status.Scope
	if snap.ProcessedThrough < snap.CaptureSeq {
		return snap, false, nil
	}
	canonical, err := f.readOnlyDB("garden", "palace", "palace.db", "canonical.sqlite3")
	if err != nil {
		return snap, false, err
	}
	defer canonical.Close()
	want := memoryLoopUserText(snap.SourceBody)
	cursor := ""
	for {
		var page struct {
			Items []struct {
				OperationID string `json:"operation_id"`
				Kind        string `json:"kind"`
				Status      string `json:"status"`
				TargetRef   string `json:"target_ref"`
				Revision    uint64 `json:"revision"`
			} `json:"items"`
			Next string `json:"next_cursor"`
		}
		if err := f.cognitiveValue(ctx, "diva.cognitive.results.list", string(run.SessionID), map[string]any{"limit": 100, "cursor": cursor}, &page); err != nil {
			return snap, false, err
		}
		for _, item := range page.Items {
			if item.Kind != "memory_mutation" || item.Status != "applied" {
				continue
			}
			var body string
			var revision uint64
			var metadata, source []byte
			if err := canonical.QueryRowContext(ctx, `SELECT version,content,metadata_json,source_json FROM memories WHERE id=?`, item.TargetRef).Scan(&revision, &body, &metadata, &source); err != nil {
				return snap, false, err
			}
			if body != want {
				continue
			}
			if revision != item.Revision {
				return snap, false, fmt.Errorf("effect/canonical revision mismatch")
			}
			var refs struct {
				Sources []laputaevolution.SourceRef `json:"evolution_sources"`
			}
			var primary struct {
				URI      string `json:"uri"`
				Revision string `json:"revision"`
			}
			if err := json.Unmarshal(metadata, &refs); err != nil {
				return snap, false, err
			}
			if err := json.Unmarshal(source, &primary); err != nil {
				return snap, false, err
			}
			snap.CanonicalSources, snap.PrimarySourceURI, snap.PrimarySourceRevision = refs.Sources, primary.URI, primary.Revision
			snap.OperationID, snap.RecordID, snap.Revision, snap.CanonicalBody, snap.State = item.OperationID, item.TargetRef, revision, body, item.Status
			if err := canonical.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`).Scan(&snap.CanonicalCount); err != nil {
				return snap, false, err
			}
			return snap, true, nil
		}
		if page.Next == "" {
			return snap, false, fmt.Errorf("processed window has no matching applied memory effect: phase=%s reason=%s", status.Cognition.Phase, status.Cognition.BlockReason)
		}
		cursor = page.Next
	}
}
