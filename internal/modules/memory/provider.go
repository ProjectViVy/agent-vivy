package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/observer"
	"github.com/ProjectViVy/agent-vivy/bml"
)

// errUnavailable is the explicit sync-plane failure when the composition
// never opened the memory service — never a fabricated page or accepted
// receipt.
var errUnavailable = errors.New("memory: service unavailable")

// metadataPrefix namespaces every candidate metadata key the provider emits
// (MEM-1A plan: keys prefixed vivy.memory-bml.).
const metadataPrefix = "vivy.memory-bml."

// runCompletedEvent is the only run-event type the ingest path records;
// every other delivered type is acknowledged as completed with no write.
const runCompletedEvent = "run.completed"

// maxHistoryContentBytes bounds the projected history-record content; the
// event's most structured fields lead, so truncation cuts into the summary
// tail first.
const maxHistoryContentBytes = 8 << 10

// maxIDPartBytes bounds the run id embedded in deterministic record and
// receipt ids.
const maxIDPartBytes = 128

// Provider is the single sync-plane provider: one identity serves both the
// std/context-source@v1 recall contract and the std/observer/run@v1 ingest
// contract (receipt-aware), so the sealed manifest lists stay consistent.
type Provider struct{}

var (
	_ contextsource.Provider      = (*Provider)(nil)
	_ observer.RunProvider        = (*Provider)(nil)
	_ observer.ReceiptRunProvider = (*Provider)(nil)
)

// NewProvider is the generated binding's constructor.
func NewProvider() *Provider { return &Provider{} }

// ID satisfies both manifest lists with one identity.
func (*Provider) ID() string { return ProviderID }

// Query resolves the open service at call time and projects FTS5 recall onto
// the Candidate envelope per VIVY-MEMORY-PROFILE.md §record-envelope. The
// cursor is a decimal offset into the re-executed search; the page size is
// the host's Limit.
func (*Provider) Query(ctx context.Context, req contextsource.Request) (contextsource.Page, error) {
	service := Active()
	if service == nil {
		return contextsource.Page{}, errUnavailable
	}
	offset, err := recallOffset(req.Cursor)
	if err != nil {
		return contextsource.Page{}, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = int(defaultRecallLimit)
	}
	var sessionID *string
	if trimmed := strings.TrimSpace(req.SessionID); trimmed != "" {
		sessionID = &trimmed
	}
	hits, err := service.Recall(ctx, req.Query, sessionID, uint32(offset+limit+1))
	if err != nil {
		return contextsource.Page{}, err
	}
	if offset > len(hits) {
		offset = len(hits)
	}
	end := offset + limit
	if end > len(hits) {
		end = len(hits)
	}
	candidates := make([]contextsource.Candidate, 0, end-offset)
	for _, hit := range hits[offset:end] {
		candidates = append(candidates, candidateFromHit(hit))
	}
	next := ""
	if len(hits) > end {
		next = strconv.Itoa(end)
	}
	return contextsource.NewPage(candidates, next), nil
}

// ObserveRun delegates to the receipt-aware path so both observer contracts
// share one ingest decision.
func (p *Provider) ObserveRun(ctx context.Context, event observer.RunEvent) error {
	_, err := p.ObserveRunWithReceipt(ctx, event)
	return err
}

// ObserveRunWithReceipt ingests run.completed into a history record keyed by
// the EventID and reports the truthful disposition. The receipt id is
// deterministic per EventID and never empty — the observer host rejects an
// empty one, and a redelivered event replays the same receipt without
// writing twice. A failed state always pairs with a non-nil error so the
// host retains its cursor.
func (*Provider) ObserveRunWithReceipt(ctx context.Context, event observer.RunEvent) (observer.DeliveryReceipt, error) {
	receiptID := ingestReceiptID(event.ID)
	fail := func(err error) (observer.DeliveryReceipt, error) {
		return observer.NewDeliveryReceipt(event.ID, receiptID, observer.DeliveryFailed), err
	}
	service := Active()
	if service == nil {
		return fail(errUnavailable)
	}
	if event.Type != runCompletedEvent {
		return observer.NewDeliveryReceipt(event.ID, receiptID, observer.DeliveryCompleted), nil
	}
	if event.ID.RunID == "" {
		return fail(errors.New("memory: run.completed event has empty run id"))
	}
	var payload runCompletedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fail(fmt.Errorf("memory: decode run.completed payload: %w", err))
	}
	_, _, err := service.AppendHistory(ctx, bml.AppendHistoryInput{
		ID:         ingestRecordID(event.ID),
		Content:    historyContent(event, payload),
		Evidence:   []bml.EvidenceRef{runEvidence(event)},
		SourceID:   event.ID.String(),
		SessionID:  payload.SessionID,
		CapturedAt: time.UnixMilli(event.CreatedAt).UTC(),
	})
	if err != nil {
		return fail(err)
	}
	return observer.NewDeliveryReceipt(event.ID, receiptID, observer.DeliveryCompleted), nil
}

// runCompletedPayload mirrors the manifest's AllowedPayloadFields for
// run.completed; the host already projects the payload down to those fields.
type runCompletedPayload struct {
	Outcome     string `json:"outcome"`
	Summary     string `json:"summary"`
	View        string `json:"view"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

// historyContent renders the bounded record projection: structural fields
// first, the free-text summary last so truncation clips its tail.
func historyContent(event observer.RunEvent, payload runCompletedPayload) string {
	var b strings.Builder
	b.WriteString("run ")
	b.WriteString(event.ID.RunID)
	b.WriteString(" completed")
	for _, field := range [][2]string{
		{"outcome", payload.Outcome},
		{"session_id", payload.SessionID},
		{"workspace_id", payload.WorkspaceID},
		{"tenant_id", payload.TenantID},
		{"view", payload.View},
		{"summary", payload.Summary},
	} {
		if field[1] != "" {
			fmt.Fprintf(&b, "\n%s: %s", field[0], field[1])
		}
	}
	return truncateUTF8(b.String(), maxHistoryContentBytes)
}

// runEvidence carries the run-journal pointer the record attests to.
func runEvidence(event observer.RunEvent) bml.EvidenceRef {
	uri := "run:" + event.ID.RunID
	return bml.EvidenceRef{
		ID:        uri,
		Source:    bml.EvidenceSourceExperienceJournal,
		URI:       uri,
		CreatedAt: time.UnixMilli(event.CreatedAt).UTC(),
	}
}

// ingestRecordID is the deterministic record id for one EventID — the
// idempotency key bml's insert-only put enforces.
func ingestRecordID(id observer.EventID) string {
	return "run-history-" + sanitizeIDPart(id.RunID) + "-" + strconv.FormatInt(id.Seq, 10)
}

// ingestReceiptID is the deterministic receipt id for one EventID, so a
// host retry sees the identical receipt.
func ingestReceiptID(id observer.EventID) string {
	return ProviderID + "/" + sanitizeIDPart(id.RunID) + "/" + strconv.FormatInt(id.Seq, 10)
}

// sanitizeIDPart keeps record/receipt ids to a portable alphabet; the RunID
// is already non-empty when this is called on the ingest path.
func sanitizeIDPart(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return truncateUTF8(b.String(), maxIDPartBytes)
}

// recallOffset parses the decimal offset cursor; absent is the first page.
func recallOffset(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("memory: invalid cursor %q", cursor)
	}
	return offset, nil
}

// candidateFromHit maps one stored record onto the Candidate envelope:
// record id, decimal revision label, millisecond effective time, and the
// provider score from the record's basis-point confidence.
func candidateFromHit(hit bml.SearchHit) contextsource.Candidate {
	record := hit.Record.Record
	candidate := contextsource.Candidate{
		SourceID:   ProviderID,
		ContentID:  record.ID,
		MediaType:  "text/markdown",
		Content:    record.Content,
		Confidence: float64(record.ConfidenceBPS) / float64(bml.MaxConfidenceBPS),
		Version:    strconv.FormatInt(hit.Record.Revision, 10),
		UpdatedAt:  record.EffectiveAt.UnixMilli(),
		Treatment:  contextsource.TreatmentCompetitive,
		Metadata: map[string]string{
			metadataPrefix + "kind":       string(record.Kind),
			metadataPrefix + "trust":      string(record.Trust),
			metadataPrefix + "provenance": string(record.Provenance.Source),
		},
	}
	if len(record.EvidenceRefs) > 0 {
		uris := make([]string, 0, len(record.EvidenceRefs))
		for _, ref := range record.EvidenceRefs {
			uris = append(uris, ref.URI)
		}
		candidate.Metadata[metadataPrefix+"evidence"] = strings.Join(uris, ",")
	}
	if record.ExpiresAt != nil {
		candidate.ValidUntil = record.ExpiresAt.UnixMilli()
	}
	return contextsource.NewCandidate(candidate)
}

func truncateUTF8(value string, budget int) string {
	if len(value) <= budget {
		return value
	}
	end := budget
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}
