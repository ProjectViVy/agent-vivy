package runtime

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

const (
	historyCursorVersion       = 1
	historyCursorDecodedMax    = 64 << 10
	historyCursorTransportMax  = 48 << 10
	historyCursorPartSeparator = "."
	historyPhaseMessages       = "messages"
	historyPhaseEvents         = "events"
	historyPhaseDone           = "done"
)

var errHistoryCursorStale = errors.New("stale_cursor")

type historyScopeContextKey struct{}
type historyOperatorContextKey struct{}

// WithHistoryScope attaches the host-resolved scope of the current run. The
// browser and model never get to manufacture this value; admission code owns
// the call site and HistoryService validates it again on every page.
func WithHistoryScope(ctx context.Context, scope domain.AcceptedHistoryScope) context.Context {
	return context.WithValue(ctx, historyScopeContextKey{}, scope)
}

// WithHistoryOperator marks a server-derived operator inspection request. It
// allows an operator to select source sessions explicitly while model tools
// remain limited to their current or admitted task scope.
func WithHistoryOperator(ctx context.Context) context.Context {
	return context.WithValue(ctx, historyOperatorContextKey{}, true)
}

func historyScopeFromContext(ctx context.Context) (domain.AcceptedHistoryScope, bool) {
	scope, ok := ctx.Value(historyScopeContextKey{}).(domain.AcceptedHistoryScope)
	return scope, ok
}

func historyOperatorFromContext(ctx context.Context) bool {
	operator, _ := ctx.Value(historyOperatorContextKey{}).(bool)
	return operator
}

// HistoryService is the runtime-owned, bounded projection over the storage
// history query contract. It deliberately knows no SQL and exposes no raw
// storage payloads.
type HistoryService struct {
	query    storage.HistoryQueryStore
	sessions storage.SessionStore
	limits   domain.ContinuityLimits
	key      []byte
	keyMu    sync.RWMutex
}

type HistoryOperations = tools.HistoryOperations

var _ HistoryOperations = (*HistoryService)(nil)

// NewHistoryService constructs the single history authority used by both
// readonly model tools and inspection RPCs.
func NewHistoryService(query storage.HistoryQueryStore, sessionStores ...storage.SessionStore) *HistoryService {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// crypto/rand failure is exceptionally unlikely. A process-local,
		// time-varying fallback still guarantees that a future process cannot
		// accept an old cursor; the cursor remains an opaque capability.
		digest := sha256.Sum256([]byte(fmt.Sprintf("vivy-history-%d", time.Now().UnixNano())))
		copy(key, digest[:])
	}
	var sessions storage.SessionStore
	if len(sessionStores) > 0 {
		sessions = sessionStores[0]
	}
	return &HistoryService{
		query:    query,
		sessions: sessions,
		limits:   domain.DefaultContinuityLimits(),
		key:      key,
	}
}

// SetLimits is intended for a composition root that has a narrower effective
// transport budget. Values larger than SC-D4 are still capped by this service.
func (s *HistoryService) SetLimits(limits domain.ContinuityLimits) {
	if s == nil {
		return
	}
	defaults := domain.DefaultContinuityLimits()
	if limits == (domain.ContinuityLimits{}) {
		limits = defaults
	}
	if limits.SearchQueryBytes <= 0 || limits.SearchQueryBytes > defaults.SearchQueryBytes {
		limits.SearchQueryBytes = defaults.SearchQueryBytes
	}
	if limits.SearchPageDefault <= 0 || limits.SearchPageDefault > defaults.SearchPageDefault {
		limits.SearchPageDefault = defaults.SearchPageDefault
	}
	if limits.SearchPageMax <= 0 || limits.SearchPageMax > defaults.SearchPageMax {
		limits.SearchPageMax = defaults.SearchPageMax
	}
	if limits.ReadPageDefault <= 0 || limits.ReadPageDefault > defaults.ReadPageDefault {
		limits.ReadPageDefault = defaults.ReadPageDefault
	}
	if limits.ReadPageMax <= 0 || limits.ReadPageMax > defaults.ReadPageMax {
		limits.ReadPageMax = defaults.ReadPageMax
	}
	if limits.CandidateRecords <= 0 || limits.CandidateRecords > defaults.CandidateRecords {
		limits.CandidateRecords = defaults.CandidateRecords
	}
	if limits.CandidateBytes <= 0 || limits.CandidateBytes > defaults.CandidateBytes {
		limits.CandidateBytes = defaults.CandidateBytes
	}
	if limits.ResultItemBytes <= 0 || limits.ResultItemBytes > defaults.ResultItemBytes {
		limits.ResultItemBytes = defaults.ResultItemBytes
	}
	if limits.ResultPageBytes <= 0 || limits.ResultPageBytes > defaults.ResultPageBytes {
		limits.ResultPageBytes = defaults.ResultPageBytes
	}
	s.keyMu.Lock()
	s.limits = limits
	s.keyMu.Unlock()
}

func (s *HistoryService) effectiveLimits() domain.ContinuityLimits {
	s.keyMu.RLock()
	limits := s.limits
	s.keyMu.RUnlock()
	if limits == (domain.ContinuityLimits{}) {
		return domain.DefaultContinuityLimits()
	}
	return limits
}

// Capabilities returns the effective model/RPC limits. It is deliberately a
// value projection so callers cannot mutate the service's authority.
func (s *HistoryService) Capabilities(context.Context) (tools.HistoryCapabilities, error) {
	if s == nil {
		return tools.HistoryCapabilities{}, errors.New("history service unavailable")
	}
	limits := s.effectiveLimits()
	return tools.HistoryCapabilities{
		Kinds: []string{
			string(domain.SourceKindMessage),
			string(domain.SourceKindToolCall),
			string(domain.SourceKindToolResult),
			string(domain.SourceKindEvent),
			string(domain.SourceKindSummary),
		},
		Filters: []string{"query", "session_ids", "from", "to", "kinds"},
		Limits: tools.HistoryCapabilityLimits{
			SearchQueryBytes: limits.SearchQueryBytes,
			SearchPageDefault: limits.SearchPageDefault,
			SearchPageMax: limits.SearchPageMax,
			ReadPageDefault: limits.ReadPageDefault,
			ReadPageMax: limits.ReadPageMax,
			CandidateRecords: limits.CandidateRecords,
			CandidateBytes: limits.CandidateBytes,
			ResultItemBytes: limits.ResultItemBytes,
			ResultPageBytes: limits.ResultPageBytes,
		},
	}, nil
}

// Search implements the HistoryOperations search boundary.
func (s *HistoryService) Search(ctx context.Context, request domain.HistorySearchRequest) (domain.HistoryPage, error) {
	if s == nil {
		return historyStatusPage(domain.HistoryStatusUnavailable, "history_unavailable"), nil
	}
	limits := s.effectiveLimits()
	if err := request.Validate(limits); err != nil {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, err.Error()), nil
	}
	if request.ArtifactID != "" || request.TaskID != "" {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, "unsupported_filter"), nil
	}
	if s.query == nil {
		return historyStatusPage(domain.HistoryStatusUnavailable, "history_unavailable"), nil
	}
	return s.runSearch(ctx, request, limits)
}

// Read implements the bounded source-selection read boundary. Reference IDs
// are intentionally not resolved by T3; T5 owns destination snapshots.
func (s *HistoryService) Read(ctx context.Context, request domain.HistoryReadRequest) (domain.HistoryPage, error) {
	if s == nil {
		return historyStatusPage(domain.HistoryStatusUnavailable, "history_unavailable"), nil
	}
	limits := s.effectiveLimits()
	if err := request.Validate(limits); err != nil {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, err.Error()), nil
	}
	if request.ReferenceID != "" {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, "reference_lookup_unavailable"), nil
	}
	if s.query == nil || request.Selection == nil {
		return historyStatusPage(domain.HistoryStatusUnavailable, "history_unavailable"), nil
	}
	return s.runRead(ctx, request, limits)
}

// Trace exposes only immediate source/provenance metadata. Recursive lookup
// of references and future deliverables belongs to their owning stories.
func (s *HistoryService) Trace(ctx context.Context, request domain.HistoryTraceRequest) (domain.HistoryPage, error) {
	if s == nil {
		return historyStatusPage(domain.HistoryStatusUnavailable, "history_unavailable"), nil
	}
	limits := s.effectiveLimits()
	if err := request.Validate(limits); err != nil {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, err.Error()), nil
	}
	if request.ReferenceID != "" || request.DeliverableID != "" {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, "unsupported_trace_selector"), nil
	}
	if request.SourceRef == nil {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, "source_ref_required"), nil
	}
	if s.query == nil {
		return historyStatusPage(domain.HistoryStatusUnavailable, "history_unavailable"), nil
	}
	current := tools.SessionIDFromContext(ctx)
	if current == "" {
		return historyStatusPage(domain.HistoryStatusForbidden, "missing_authority"), nil
	}
	if !s.sessionAllowed(ctx, request.SourceRef.SessionID, current) {
		return historyStatusPage(domain.HistoryStatusForbidden, "out_of_scope"), nil
	}
	selection := domain.HistorySelection{SourceSessionID: request.SourceRef.SessionID, Refs: []domain.SourceRef{*request.SourceRef}}
	page, err := s.Read(ctx, domain.HistoryReadRequest{Selection: &selection, Limit: 1})
	if err != nil {
		return domain.HistoryPage{}, err
	}
	if page.Status == string(domain.HistoryStatusOK) && len(page.Items) == 1 {
		page.Warnings = append(page.Warnings, "immediate_provenance_only")
		return s.boundPage(page, limits), nil
	}
	return page, nil
}

func (s *HistoryService) runSearch(ctx context.Context, request domain.HistorySearchRequest, limits domain.ContinuityLimits) (domain.HistoryPage, error) {
	sessions, scopeHash, page, err := s.authorizeSessions(ctx, request.SessionIDs)
	if err != nil {
		return domain.HistoryPage{}, err
	}
	if page.Status != "" {
		return page, nil
	}
	filterHash := historyFilterHash("search", request, scopeHash)
	state := historyCursorState{Version: historyCursorVersion, Destination: string(tools.SessionIDFromContext(ctx)), ScopeHash: scopeHash, FilterHash: filterHash, Phase: historyPhaseMessages}
	if request.Cursor != "" {
		state, err = s.decodeCursor(request.Cursor)
		if errors.Is(err, errHistoryCursorStale) {
			return historyStatusPage(domain.HistoryStatusConflict, "stale_cursor"), nil
		}
		if err != nil {
			return historyStatusPage(domain.HistoryStatusInvalidArgument, "invalid_cursor"), nil
		}
		if state.Destination != string(tools.SessionIDFromContext(ctx)) || state.ScopeHash != scopeHash || state.FilterHash != filterHash {
			return historyStatusPage(domain.HistoryStatusForbidden, "cursor_scope_mismatch"), nil
		}
		if err := s.reauthorizeCursor(ctx, state.Cut); err != nil {
			return historyStatusPage(domain.HistoryStatusForbidden, "cursor_scope_unavailable"), nil
		}
	} else {
		state.Cut, err = s.query.CaptureHistoryCut(ctx, sessions)
		if err != nil {
			return historyStoragePage(err), nil
		}
	}
	limit := request.Limit
	if limit == 0 {
		limit = limits.SearchPageDefault
	}
	searchLimits := limits
	searchLimits.ReadPageDefault = limits.SearchPageDefault
	searchLimits.ReadPageMax = limits.SearchPageMax
	items, next, incomplete, redacted, truncated, warnings, err := s.scan(ctx, state, limit, searchLimits, func(candidate storage.HistoryCandidate, item domain.HistoryItem) bool {
		if candidate.Unavailable || item.Text == "" {
			return false
		}
		if !historyKindMatches(request.Kinds, item.Ref.Kind, candidate.Ref.Kind) {
			return false
		}
		if request.From != 0 && item.Ref.CreatedAt < request.From {
			return false
		}
		if request.To != 0 && item.Ref.CreatedAt > request.To {
			return false
		}
		if request.Query == "" {
			return true
		}
		return strings.Contains(strings.ToLower(item.Text), strings.ToLower(request.Query))
	})
	if err != nil {
		return domain.HistoryPage{}, err
	}
	status := domain.HistoryStatusOK
	if incomplete || truncated {
		status = domain.HistoryStatusPartial
	}
	page = domain.HistoryPage{Status: status, Items: items, Truncated: incomplete || truncated, Redacted: redacted, Warnings: warnings}
	if !next.done() {
		page.NextCursor, err = s.encodeCursor(next)
		if err != nil {
			return historyStatusPage(domain.HistoryStatusUnavailable, "cursor_unavailable"), nil
		}
	}
	return s.boundPage(page, limits), nil
}

func (s *HistoryService) runRead(ctx context.Context, request domain.HistoryReadRequest, limits domain.ContinuityLimits) (domain.HistoryPage, error) {
	selection := *request.Selection
	current := tools.SessionIDFromContext(ctx)
	if current == "" || !s.sessionAllowed(ctx, selection.SourceSessionID, current) {
		return historyStatusPage(domain.HistoryStatusForbidden, "out_of_scope"), nil
	}
	sessions, scopeHash, page, err := s.authorizeSessions(ctx, []domain.SessionID{selection.SourceSessionID})
	if err != nil {
		return domain.HistoryPage{}, err
	}
	if page.Status != "" {
		return page, nil
	}
	filterHash := historyFilterHash("read", request, scopeHash)
	state := historyCursorState{Version: historyCursorVersion, Destination: string(current), ScopeHash: scopeHash, FilterHash: filterHash, Phase: historyPhaseMessages}
	if request.Cursor != "" {
		state, err = s.decodeCursor(request.Cursor)
		if errors.Is(err, errHistoryCursorStale) {
			return historyStatusPage(domain.HistoryStatusConflict, "stale_cursor"), nil
		}
		if err != nil {
			return historyStatusPage(domain.HistoryStatusInvalidArgument, "invalid_cursor"), nil
		}
		if state.Destination != string(current) || state.ScopeHash != scopeHash || state.FilterHash != filterHash {
			return historyStatusPage(domain.HistoryStatusForbidden, "cursor_scope_mismatch"), nil
		}
		if err := s.reauthorizeCursor(ctx, state.Cut); err != nil {
			return historyStatusPage(domain.HistoryStatusForbidden, "cursor_scope_unavailable"), nil
		}
	} else {
		state.Cut, err = s.query.CaptureHistoryCut(ctx, sessions)
		if err != nil {
			return historyStoragePage(err), nil
		}
	}
	limit := request.Limit
	if limit == 0 {
		limit = limits.ReadPageDefault
	}
	items, next, incomplete, redacted, truncated, warnings, err := s.scan(ctx, state, limit, limits, func(candidate storage.HistoryCandidate, item domain.HistoryItem) bool {
		if item.Ref.SessionID != selection.SourceSessionID {
			return false
		}
		if selection.RunRange != nil {
			if candidate.Ref.RunID != selection.RunRange.RunID {
				return false
			}
			if candidate.Ref.EventSeq < selection.RunRange.FromSeq || candidate.Ref.EventSeq > selection.RunRange.ToSeq {
				return false
			}
			return candidate.Ref.EventSeq != 0
		}
		for _, ref := range selection.Refs {
			if historyRefMatches(ref, item.Ref) {
				return true
			}
		}
		return false
	})
	if err != nil {
		return domain.HistoryPage{}, err
	}
	status := domain.HistoryStatusOK
	if len(items) == 0 && !incomplete && !truncated {
		status = domain.HistoryStatusNotFound
	}
	if hasUnavailable(items) {
		status = domain.HistoryStatusPartial
		warnings = append(warnings, "unavailable_record")
	}
	if incomplete || truncated {
		status = domain.HistoryStatusPartial
	}
	page = domain.HistoryPage{Status: status, Items: items, Truncated: incomplete || truncated, Redacted: redacted, Warnings: warnings}
	if !next.done() {
		page.NextCursor, err = s.encodeCursor(next)
		if err != nil {
			return historyStatusPage(domain.HistoryStatusUnavailable, "cursor_unavailable"), nil
		}
	}
	if status == domain.HistoryStatusOK && page.NextCursor == "" && len(items) > 0 && !hasUnavailable(items) {
		page.SelectionDigest = CanonicalHistorySelectionDigest(items)
	}
	return s.boundPage(page, limits), nil
}

type historyCursorState struct {
	Version     int                  `json:"version"`
	Destination string               `json:"destination"`
	ScopeHash   string               `json:"scope_hash"`
	FilterHash  string               `json:"filter_hash"`
	Cut         storage.HistoryCut  `json:"cut"`
	Phase       string               `json:"phase"`
	After       storage.HistoryPosition `json:"after"`
}

func (c historyCursorState) done() bool { return c.Phase == historyPhaseDone }

func (s *HistoryService) scan(ctx context.Context, state historyCursorState, limit int, limits domain.ContinuityLimits, match func(storage.HistoryCandidate, domain.HistoryItem) bool) ([]domain.HistoryItem, historyCursorState, bool, bool, bool, []string, error) {
	items := make([]domain.HistoryItem, 0, limit)
	warnings := []string{}
	redacted := false
	incomplete := false
	truncated := false
	for len(items) < limit && state.Phase != historyPhaseDone {
		stream := storage.HistoryStreamMessage
		if state.Phase == historyPhaseEvents {
			stream = storage.HistoryStreamRunEvent
		}
		candidates, err := s.query.QueryHistoryPage(ctx, state.Cut, state.After, storage.HistoryQueryOptions{Stream: stream, Limit: limit - len(items), Limits: limits})
		if err != nil {
			return nil, state, false, false, false, nil, err
		}
		for _, candidate := range candidates.Records {
			item, ok := projectHistoryCandidateWithLimit(candidate, limits.ResultItemBytes)
			if !ok {
				continue
			}
			if item.Redacted {
				redacted = true
			}
			if match(candidate, item) {
				items = append(items, item)
				if len(items) >= limit {
					break
				}
			}
		}
		if candidates.Next.IsZero() {
			if candidates.HasMore || candidates.ScanIncomplete {
				warnings = append(warnings, "cursor_progress_unavailable")
				incomplete = true
				break
			}
			state = advanceHistoryPhase(state)
			continue
		}
		state.After = candidates.Next
		if candidates.ScanIncomplete {
			incomplete = true
			warnings = append(warnings, "scan_incomplete")
			break
		}
		if candidates.HasMore {
			if len(items) >= limit {
				truncated = true
				break
			}
			continue
		}
		state = advanceHistoryPhase(state)
	}
	if state.Phase != historyPhaseDone && len(items) >= limit {
		truncated = true
	}
	return items, state, incomplete, redacted, truncated, warnings, nil
}

func advanceHistoryPhase(state historyCursorState) historyCursorState {
	switch state.Phase {
	case historyPhaseMessages:
		state.Phase = historyPhaseEvents
		state.After = storage.HistoryPosition{}
	case historyPhaseEvents:
		state.Phase = historyPhaseDone
		state.After = storage.HistoryPosition{}
	default:
		state.Phase = historyPhaseDone
	}
	return state
}

func (s *HistoryService) authorizeSessions(ctx context.Context, requested []domain.SessionID) ([]domain.SessionID, string, domain.HistoryPage, error) {
	current := tools.SessionIDFromContext(ctx)
	if current == "" || !utf8.ValidString(string(current)) {
		return nil, "", historyStatusPage(domain.HistoryStatusForbidden, "missing_authority"), nil
	}
	allowed := []domain.SessionID{current}
	if scope, ok := historyScopeFromContext(ctx); ok {
		if err := scope.Validate(); err != nil || scope.DestinationSessionID != current {
			return nil, "", historyStatusPage(domain.HistoryStatusForbidden, "invalid_scope"), nil
		}
		allowed = append([]domain.SessionID(nil), scope.SourceSessionIDs...)
	}
	if len(requested) == 0 {
		requested = allowed
	}
	requested = domain.CanonicalSessionIDs(requested)
	if len(requested) == 0 {
		return nil, "", historyStatusPage(domain.HistoryStatusInvalidArgument, "invalid_session_scope"), nil
	}
	allowedSet := make(map[domain.SessionID]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}
	if !historyOperatorFromContext(ctx) {
		for _, id := range requested {
			if _, ok := allowedSet[id]; !ok {
				return nil, "", historyStatusPage(domain.HistoryStatusForbidden, "out_of_scope"), nil
			}
		}
	}
	if s.sessions != nil {
		for _, id := range requested {
			if _, err := s.sessions.GetSession(ctx, id); err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return nil, "", historyStatusPage(domain.HistoryStatusNotFound, "session_not_found"), nil
				}
				return nil, "", domain.HistoryPage{}, err
			}
		}
	}
	hash, err := domain.ScopeHash(current, requested)
	if err != nil {
		return nil, "", historyStatusPage(domain.HistoryStatusInvalidArgument, "invalid_session_scope"), nil
	}
	return requested, hash, domain.HistoryPage{}, nil
}

func (s *HistoryService) sessionAllowed(ctx context.Context, id, current domain.SessionID) bool {
	if id == "" || current == "" {
		return false
	}
	if id == current {
		return true
	}
	if scope, ok := historyScopeFromContext(ctx); ok && scope.DestinationSessionID == current {
		for _, source := range scope.SourceSessionIDs {
			if source == id {
				return true
			}
		}
	}
	return historyOperatorFromContext(ctx)
}

func (s *HistoryService) reauthorizeCursor(ctx context.Context, cut storage.HistoryCut) error {
	if err := cut.Validate(); err != nil {
		return err
	}
	ids := make([]domain.SessionID, 0, len(cut.Sessions))
	for _, session := range cut.Sessions {
		ids = append(ids, session.SessionID)
		if !s.sessionAllowed(ctx, session.SessionID, tools.SessionIDFromContext(ctx)) {
			return errors.New("history cursor source is no longer authorized")
		}
	}
	if s.sessions != nil {
		for _, id := range ids {
			if _, err := s.sessions.GetSession(ctx, id); err != nil {
				return err
			}
		}
	}
	if len(ids) > 0 && s.query != nil {
		if _, err := s.query.CaptureHistoryCut(ctx, ids); err != nil {
			return err
		}
	}
	return nil
}

func (s *HistoryService) keyCopy() []byte {
	s.keyMu.RLock()
	defer s.keyMu.RUnlock()
	return append([]byte(nil), s.key...)
}

func (s *HistoryService) encodeCursor(cursor historyCursorState) (string, error) {
	cursor.Version = historyCursorVersion
	raw, err := json.Marshal(cursor)
	if err != nil || len(raw) > historyCursorDecodedMax {
		return "", errHistoryCursorStale
	}
	mac := hmac.New(sha256.New, s.keyCopy())
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + historyCursorPartSeparator + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *HistoryService) decodeCursor(encoded string) (historyCursorState, error) {
	if len(encoded) == 0 || len(encoded) > historyCursorTransportMax {
		return historyCursorState{}, errHistoryCursorStale
	}
	parts := strings.Split(encoded, historyCursorPartSeparator)
	if len(parts) != 2 {
		return historyCursorState{}, errHistoryCursorStale
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(raw) == 0 || len(raw) > historyCursorDecodedMax {
		return historyCursorState{}, errHistoryCursorStale
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return historyCursorState{}, errHistoryCursorStale
	}
	mac := hmac.New(sha256.New, s.keyCopy())
	_, _ = mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return historyCursorState{}, errHistoryCursorStale
	}
	var cursor historyCursorState
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Version != historyCursorVersion || cursor.Phase == "" {
		return historyCursorState{}, errHistoryCursorStale
	}
	if err := cursor.Cut.Validate(); err != nil {
		return historyCursorState{}, errHistoryCursorStale
	}
	if err := cursor.After.Validate(); err != nil {
		return historyCursorState{}, errHistoryCursorStale
	}
	return cursor, nil
}

func projectHistoryCandidate(candidate storage.HistoryCandidate) (domain.HistoryItem, bool) {
	return projectHistoryCandidateWithLimit(candidate, domain.DefaultContinuityLimits().ResultItemBytes)
}

func projectHistoryCandidateWithLimit(candidate storage.HistoryCandidate, maximum int) (domain.HistoryItem, bool) {
	if candidate.Ref.SessionID == "" {
		return domain.HistoryItem{}, false
	}
	if candidate.Ref.Kind == string(domain.SourceKindMessage) {
		// Modern assistant/tool rows are deterministic projections of their
		// journal events. Their event forms are canonical; only ordinary user
		// messages and legacy non-projected rows are emitted here.
		if strings.HasPrefix(candidate.Ref.MessageID, "msgp_") {
			return domain.HistoryItem{}, false
		}
		author := candidate.Author
		if author != domain.HistoryAuthorUser && author != domain.HistoryAuthorAssistant && author != domain.HistoryAuthorTool {
			return domain.HistoryItem{}, false
		}
		if candidate.Unavailable {
			return domain.HistoryItem{Ref: candidate.Ref, Author: author, Truncated: true}, true
		}
		text, redacted, truncated := sanitizeHistoryText(candidate.Text, maximum)
		return domain.HistoryItem{Ref: candidate.Ref, Author: author, Text: text, Redacted: redacted, Truncated: truncated || candidate.Truncated}, true
	}
	if !historyEventAllowed(candidate.EventType) {
		return domain.HistoryItem{}, false
	}
	item, ok := projectHistoryEvent(candidate, maximum)
	return item, ok
}

func projectHistoryEvent(candidate storage.HistoryCandidate, maximum int) (domain.HistoryItem, bool) {
	author := domain.HistoryAuthorAssistant
	kind := string(domain.SourceKindEvent)
	switch candidate.EventType {
	case domain.EventToolRequested:
		author = domain.HistoryAuthorTool
		kind = string(domain.SourceKindToolCall)
	case domain.EventToolFinished:
		author = domain.HistoryAuthorTool
		kind = string(domain.SourceKindToolResult)
	case domain.EventModelCompleted:
		author = domain.HistoryAuthorAssistant
		kind = string(domain.SourceKindMessage)
	}
	ref := candidate.Ref
	ref.Kind = kind
	if candidate.Unavailable || candidate.PayloadVersion < 0 {
		return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
	}
	if !knownHistoryPayloadVersion(candidate.EventType, candidate.PayloadVersion) {
		return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
	}
	text := candidate.Text
	switch candidate.EventType {
	case domain.EventToolRequested:
		var payload payloadToolRequested
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
		}
		args, _ := json.Marshal(payload.Args)
		text = payload.ToolName + " " + string(args)
	case domain.EventToolFinished:
		var payload payloadToolFinished
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
		}
		if payload.Error != "" {
			text = payload.Error
		} else {
			text = payload.Result
		}
	case domain.EventModelCompleted:
		if candidate.PayloadVersion == 2 {
			return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
		}
		var payload payloadModelCompleted
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
		}
		text = payload.Content
	case domain.EventContextCompacted:
		var payload payloadContextCompacted
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
		}
		text = fmt.Sprintf("compaction %s: %d -> %d tokens", payload.Mode, payload.BeforeTokens, payload.AfterTokens)
	case domain.EventSessionTruncated:
		var payload payloadSessionTruncated
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
		}
		text = fmt.Sprintf("session truncated at %s (%s)", payload.CutoffMessageID, payload.Reason)
	case domain.EventSessionForked:
		var payload payloadSessionForked
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return domain.HistoryItem{Ref: ref, Author: author, Truncated: true}, true
		}
		text = fmt.Sprintf("session forked from %s at %s", payload.ParentSessionID, payload.ForkPointMessageID)
	}
	safe, redacted, truncated := sanitizeHistoryText(text, maximum)
	return domain.HistoryItem{Ref: ref, Author: author, Text: safe, Redacted: redacted, Truncated: truncated || candidate.Truncated}, true
}

func sanitizeHistoryText(text string, maximum int) (string, bool, bool) {
	if !utf8.ValidString(text) {
		return "", false, true
	}
	redacted := tools.RedactSensitive(text)
	truncated := false
	if len(redacted) > maximum {
		redacted = truncateUTF8(redacted, maximum)
		truncated = true
	}
	return redacted, redacted != text, truncated
}

func truncateUTF8(text string, maximum int) string {
	if maximum <= 0 || len(text) <= maximum {
		return text
	}
	cut := maximum
	for cut > 0 && !utf8.ValidString(text[:cut]) {
		cut--
	}
	return text[:cut]
}

func historyEventAllowed(eventType domain.EventType) bool {
	switch eventType {
	case domain.EventModelCompleted, domain.EventToolRequested, domain.EventToolFinished,
		domain.EventContextCompacted, domain.EventSessionTruncated, domain.EventSessionForked:
		return true
	default:
		return false
	}
}

func knownHistoryPayloadVersion(eventType domain.EventType, version int) bool {
	if version == 0 {
		return true
	}
	if eventType == domain.EventModelCompleted {
		return version == 1 || version == 2
	}
	return version == 1
}

func historyKindMatches(requested []string, projected, original string) bool {
	if len(requested) == 0 {
		return true
	}
	for _, kind := range requested {
		if kind == projected || kind == original || (kind == string(domain.SourceKindEvent) && original == string(domain.SourceKindEvent)) {
			return true
		}
	}
	return false
}

func historyRefMatches(expected, actual domain.SourceRef) bool {
	if expected.SessionID != actual.SessionID || expected.RunID != actual.RunID || expected.MessageID != actual.MessageID || expected.EventSeq != actual.EventSeq {
		return false
	}
	if expected.Kind == actual.Kind {
		return true
	}
	return expected.Kind == string(domain.SourceKindEvent) && (actual.Kind == string(domain.SourceKindToolCall) || actual.Kind == string(domain.SourceKindToolResult) || actual.Kind == string(domain.SourceKindMessage))
}

func hasUnavailable(items []domain.HistoryItem) bool {
	for _, item := range items {
		if item.Truncated && item.Text == "" {
			return true
		}
	}
	return false
}

func historyStoragePage(err error) domain.HistoryPage {
	if errors.Is(err, storage.ErrHistoryNarrowScope) {
		return historyStatusPage(domain.HistoryStatusInvalidArgument, "narrow_scope")
	}
	if errors.Is(err, storage.ErrNotFound) {
		return historyStatusPage(domain.HistoryStatusNotFound, "session_not_found")
	}
	return historyStatusPage(domain.HistoryStatusUnavailable, "history_unavailable")
}

func historyStatusPage(status domain.HistoryStatus, reason string) domain.HistoryPage {
	page := domain.HistoryPage{Status: string(status)}
	if reason != "" {
		page.Reason = &reason
	}
	return page
}

func (s *HistoryService) boundPage(page domain.HistoryPage, limits domain.ContinuityLimits) domain.HistoryPage {
	for len(page.Items) > 0 {
		encoded, err := json.Marshal(page)
		if err == nil && len(encoded) <= limits.ResultPageBytes {
			return page
		}
		page.Items = page.Items[:len(page.Items)-1]
		page.Truncated = true
		page.Status = string(domain.HistoryStatusPartial)
		page.SelectionDigest = ""
		warning := "result_budget"
		if !containsString(page.Warnings, warning) {
			page.Warnings = append(page.Warnings, warning)
		}
	}
	return page
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type historyFilterEnvelope struct {
	Kind       string                     `json:"kind"`
	Search     *domain.HistorySearchRequest `json:"search,omitempty"`
	Read       *domain.HistoryReadRequest   `json:"read,omitempty"`
	ScopeHash  string                     `json:"scope_hash"`
}

func historyFilterHash(kind string, request any, scopeHash string) string {
	var encoded []byte
	switch typed := request.(type) {
	case domain.HistorySearchRequest:
		copy := typed
		copy.Cursor = ""
		copy.Limit = 0
		encoded, _ = json.Marshal(historyFilterEnvelope{Kind: kind, Search: &copy, ScopeHash: scopeHash})
	case domain.HistoryReadRequest:
		copy := typed
		copy.Cursor = ""
		copy.Limit = 0
		encoded, _ = json.Marshal(historyFilterEnvelope{Kind: kind, Read: &copy, ScopeHash: scopeHash})
	default:
		encoded, _ = json.Marshal(historyFilterEnvelope{Kind: kind, ScopeHash: scopeHash})
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", digest[:])
}

// CanonicalHistorySelectionDigest is shared by history/read and the later
// reference preview/attach implementation. Display labels and capture times
// are intentionally absent from this representation.
func CanonicalHistorySelectionDigest(items []domain.HistoryItem) string {
	type digestItem struct {
		Ref        domain.SourceRef   `json:"ref"`
		Author     string             `json:"author"`
		Text       string             `json:"text"`
		SourceRefs []domain.SourceRef `json:"source_refs,omitempty"`
		Redacted   bool               `json:"redacted"`
		Truncated  bool               `json:"truncated"`
	}
	canonical := make([]digestItem, 0, len(items))
	for _, item := range items {
		canonical = append(canonical, digestItem{Ref: item.Ref, Author: item.Author, Text: item.Text, SourceRefs: item.SourceRefs, Redacted: item.Redacted, Truncated: item.Truncated})
	}
	encoded, _ := json.Marshal(struct {
		Version int          `json:"version"`
		Items   []digestItem `json:"items"`
	}{Version: 1, Items: canonical})
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", digest[:])
}

func sortHistoryItems(items []domain.HistoryItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Ref.SessionID != items[j].Ref.SessionID {
			return items[i].Ref.SessionID < items[j].Ref.SessionID
		}
		if items[i].Ref.RunID != items[j].Ref.RunID {
			return items[i].Ref.RunID < items[j].Ref.RunID
		}
		if items[i].Ref.EventSeq != items[j].Ref.EventSeq {
			return items[i].Ref.EventSeq < items[j].Ref.EventSeq
		}
		return items[i].Ref.MessageID < items[j].Ref.MessageID
	})
}
