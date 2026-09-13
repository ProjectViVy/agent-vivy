package contexthost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/contextsource"
)

const (
	defaultSourceTimeout     = 750 * time.Millisecond
	defaultMaxCandidateBytes = 1 << 20
	defaultMaxTotalBytes     = 4 << 20
	defaultMaxCandidates     = 64
	maxSourceIDBytes         = 256
	maxContentIDBytes        = 512
	maxMediaTypeBytes        = 128
	maxVersionBytes          = 256
	maxMetadataEntries       = 32
	maxMetadataKeyBytes      = 128
	maxMetadataValueBytes    = 1024
	maxMetadataTotalBytes    = 8 << 10
)

var (
	ErrInvalidSource           = errors.New("contexthost: invalid source")
	ErrDuplicateSource         = errors.New("contexthost: duplicate source")
	ErrResourceResolver        = errors.New("contexthost: resource resolver unavailable")
	ErrResourceVersionMismatch = errors.New("contexthost: resource version mismatch")
	ErrResourceScope           = errors.New("contexthost: resource scope mismatch")
	ErrRequiredContextBudget   = errors.New("contexthost: required context exceeds budget")
)

type AuthorizeFunc func(context.Context, string, Request) error
type TokenEstimator func(string) int

type Config struct {
	Sources []contextsource.Provider
	// RequiredSourceIDs is build-owned policy. A required Source outage or
	// authorization failure aborts preparation instead of becoming an optional
	// omission. Providers cannot mark themselves required at runtime.
	RequiredSourceIDs []string
	Authorize         AuthorizeFunc
	SourceTimeout     time.Duration
	MaxCandidateBytes int
	MaxTotalBytes     int
	MaxCandidates     int
	TokenEstimator    TokenEstimator
	Now               func() time.Time
}

type Request struct {
	Query       string
	TenantID    string
	SessionID   string
	WorkspaceID string
	TokenBudget int
	// ByteBudget is the final model-facing budget available to the result.
	// Zero means that the caller has no additional byte budget. The runtime
	// supplies CandidateBytes when framing overhead must be accounted for.
	ByteBudget int
	// EnforceByteBudget distinguishes an explicit zero-byte remainder from
	// the legacy zero value, which means no request-specific byte limit.
	EnforceByteBudget bool
	// CandidateBytes returns the bytes the caller will actually add to its
	// model input for one candidate, including provenance framing. When it is
	// nil, the redacted candidate body length is used.
	CandidateBytes func(Candidate) int
	PerSourceLimit int
	Cursors        map[string]string
}

type Candidate struct {
	contextsource.Candidate
	ProvenanceID string
	Tokens       int
}

type Failure struct {
	SourceID string
	Cause    error
}

type Result struct {
	Candidates       []Candidate
	Failures         []Failure
	NextCursors      map[string]string
	Tokens           int
	Bytes            int
	DroppedInvalid   int
	DroppedDuplicate int
	DroppedOversize  int
	DroppedBudget    int
	DroppedExpired   int
	View             View
}

type ViewItem struct {
	SourceID     string
	ContentID    string
	Version      string
	ProvenanceID string
	Treatment    contextsource.Treatment
}

type View struct {
	ID          string
	TenantID    string
	WorkspaceID string
	SessionID   string
	Items       []ViewItem
}

type Host struct {
	sources           []contextsource.Provider
	requiredSources   map[string]struct{}
	authorize         AuthorizeFunc
	sourceTimeout     time.Duration
	maxCandidateBytes int
	maxTotalBytes     int
	maxCandidates     int
	estimate          TokenEstimator
	now               func() time.Time
}

type sourceOutcome struct {
	page contextsource.Page
	err  error
}

type resourceOutcome struct {
	resource contextsource.Resource
	err      error
}

func New(cfg Config) (*Host, error) {
	timeout := cfg.SourceTimeout
	if timeout <= 0 {
		timeout = defaultSourceTimeout
	}
	maxCandidateBytes := cfg.MaxCandidateBytes
	if maxCandidateBytes <= 0 {
		maxCandidateBytes = defaultMaxCandidateBytes
	}
	maxTotalBytes := cfg.MaxTotalBytes
	if maxTotalBytes <= 0 {
		maxTotalBytes = defaultMaxTotalBytes
	}
	maxCandidates := cfg.MaxCandidates
	if maxCandidates <= 0 {
		maxCandidates = defaultMaxCandidates
	}
	estimator := cfg.TokenEstimator
	if estimator == nil {
		estimator = func(text string) int {
			if text == "" {
				return 0
			}
			return (len(text) + 3) / 4
		}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	sources, err := validateSources(cfg.Sources)
	if err != nil {
		return nil, err
	}
	required := make(map[string]struct{}, len(cfg.RequiredSourceIDs))
	selected := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		selected[source.ID()] = struct{}{}
	}
	for _, id := range cfg.RequiredSourceIDs {
		id = strings.TrimSpace(id)
		if _, ok := selected[id]; !ok {
			return nil, fmt.Errorf("%w: required source %s is not selected", ErrInvalidSource, id)
		}
		if _, duplicate := required[id]; duplicate {
			return nil, fmt.Errorf("%w: required source %s is duplicated", ErrInvalidSource, id)
		}
		required[id] = struct{}{}
	}
	return &Host{
		sources:           sources,
		requiredSources:   required,
		authorize:         cfg.Authorize,
		sourceTimeout:     timeout,
		maxCandidateBytes: maxCandidateBytes,
		maxTotalBytes:     maxTotalBytes,
		maxCandidates:     maxCandidates,
		estimate:          estimator,
		now:               now,
	}, nil
}

func (host *Host) Query(ctx context.Context, request Request) (Result, error) {
	if host == nil {
		return Result{}, ErrInvalidSource
	}
	return host.query(ctx, request, host.sources)
}

// QuerySources runs only the supplied ephemeral Sources through this Host's
// authorization, validation, redaction, ranking, and budgets. Runtime uses
// it for per-turn first-party snapshots so they share the exact Host path
// without re-querying configured remote Sources for every history row.
func (host *Host) QuerySources(ctx context.Context, request Request, sources ...contextsource.Provider) (Result, error) {
	if host == nil {
		return Result{}, ErrInvalidSource
	}
	validated, err := validateSources(sources)
	if err != nil {
		return Result{}, err
	}
	return host.query(ctx, request, validated)
}

func validateSources(input []contextsource.Provider) ([]contextsource.Provider, error) {
	seen := make(map[string]struct{}, len(input))
	sources := make([]contextsource.Provider, 0, len(input))
	for _, source := range input {
		if source == nil {
			return nil, ErrInvalidSource
		}
		id := source.ID()
		if !validBoundedIdentifier(id, maxSourceIDBytes, false) {
			return nil, fmt.Errorf("%w: source id", ErrInvalidSource)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateSource, id)
		}
		seen[id] = struct{}{}
		sources = append(sources, source)
	}
	return sources, nil
}

func (host *Host) query(ctx context.Context, request Request, sources []contextsource.Provider) (Result, error) {
	if request.ByteBudget < 0 || request.TokenBudget < 0 {
		return Result{}, errors.New("contexthost: request budgets must not be negative")
	}
	result := Result{NextCursors: make(map[string]string)}
	if err := ctx.Err(); err != nil {
		result.Failures = append(result.Failures, Failure{Cause: err})
		return result, nil
	}
	collected := make([]Candidate, 0)
	for _, source := range sources {
		sourceID := source.ID()
		if err := ctx.Err(); err != nil {
			result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
			result.Candidates = nil
			return result, nil
		}
		if host.authorize != nil {
			if err := host.authorize(ctx, sourceID, request); err != nil {
				result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
				if host.sourceRequired(sourceID) {
					return result, err
				}
				if ctx.Err() != nil {
					result.Candidates = nil
					return result, nil
				}
				continue
			}
		}
		if err := ctx.Err(); err != nil {
			result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
			result.Candidates = nil
			return result, nil
		}
		cursor := ""
		if request.Cursors != nil {
			cursor = request.Cursors[sourceID]
		}
		page, err := host.querySource(ctx, source, contextsource.Request{
			Query:       request.Query,
			TenantID:    request.TenantID,
			SessionID:   request.SessionID,
			WorkspaceID: request.WorkspaceID,
			Cursor:      cursor,
			Limit:       host.sourcePageLimit(request.PerSourceLimit),
		})
		if err != nil {
			result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
			if host.sourceRequired(sourceID) {
				return result, err
			}
			if ctx.Err() != nil {
				result.Candidates = nil
				return result, nil
			}
			continue
		}
		if err := ctx.Err(); err != nil {
			result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
			result.Candidates = nil
			return result, nil
		}
		if page.NextCursor != "" {
			result.NextCursors[sourceID] = page.NextCursor
		}
		pageLimit := host.sourcePageLimit(request.PerSourceLimit)
		if len(page.Candidates) > pageLimit {
			result.DroppedBudget += len(page.Candidates) - pageLimit
		}
		for index, raw := range page.Candidates {
			if index >= pageLimit {
				break
			}
			if err := ctx.Err(); err != nil {
				result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
				result.Candidates = nil
				return result, nil
			}
			candidate, ok := host.normalizeCandidate(sourceID, raw)
			if !ok {
				if strictTreatment(raw.Treatment) {
					return result, ErrInvalidSource
				}
				result.DroppedInvalid++
				continue
			}
			if candidate.ValidUntil > 0 && host.now().UnixMilli() > candidate.ValidUntil {
				result.DroppedExpired++
				continue
			}
			if candidate.Resource != nil {
				strict := strictTreatment(candidate.Treatment)
				candidate, err = host.resolveCandidate(ctx, source, request, candidate)
				if err != nil {
					result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
					if strict {
						return result, err
					}
					continue
				}
			}
			if len(candidate.Content) > host.maxCandidateBytes {
				if strictTreatment(candidate.Treatment) {
					return result, ErrRequiredContextBudget
				}
				result.DroppedOversize++
				continue
			}
			collected = append(collected, candidate)
		}
	}
	if err := ctx.Err(); err != nil {
		result.Failures = append(result.Failures, Failure{Cause: err})
		result.Candidates = nil
		return result, nil
	}

	// Stable sort preserves Recipe source order and each Source's own order
	// when confidence is equal. Sources cannot smuggle a second ordering
	// mechanism into the Host.
	sort.SliceStable(collected, func(i, j int) bool {
		if treatmentRank(collected[i].Treatment) != treatmentRank(collected[j].Treatment) {
			return treatmentRank(collected[i].Treatment) < treatmentRank(collected[j].Treatment)
		}
		return collected[i].Confidence > collected[j].Confidence
	})

	seenContent := make(map[string]struct{}, len(collected))
	for _, candidate := range collected {
		if err := ctx.Err(); err != nil {
			result.Failures = append(result.Failures, Failure{Cause: err})
			result.Candidates = nil
			result.Bytes = 0
			result.Tokens = 0
			return result, nil
		}
		dedupKey := contentHash(candidate.MediaType, candidate.Content)
		if candidate.Content == "" {
			// Empty snapshots carry identity even though their body has no
			// content hash. Two distinct files must remain explicit parts.
			dedupKey = candidate.SourceID + "\x00" + candidate.ContentID
		}
		if _, duplicate := seenContent[dedupKey]; duplicate {
			result.DroppedDuplicate++
			continue
		}
		seenContent[dedupKey] = struct{}{}
		if len(result.Candidates) >= host.maxCandidates {
			if strictTreatment(candidate.Treatment) {
				return result, ErrRequiredContextBudget
			}
			result.DroppedBudget++
			continue
		}
		candidateBytes := len(candidate.Content)
		if request.CandidateBytes != nil {
			candidateBytes = request.CandidateBytes(candidate)
		}
		if candidateBytes < 0 {
			result.DroppedInvalid++
			continue
		}
		if result.Bytes+candidateBytes > host.maxTotalBytes {
			if strictTreatment(candidate.Treatment) {
				return result, ErrRequiredContextBudget
			}
			result.DroppedBudget++
			continue
		}
		if (request.EnforceByteBudget || request.ByteBudget > 0) && result.Bytes+candidateBytes > request.ByteBudget {
			if strictTreatment(candidate.Treatment) {
				return result, ErrRequiredContextBudget
			}
			result.DroppedBudget++
			continue
		}
		if request.TokenBudget > 0 && result.Tokens+candidate.Tokens > request.TokenBudget {
			if strictTreatment(candidate.Treatment) {
				return result, ErrRequiredContextBudget
			}
			result.DroppedBudget++
			continue
		}
		result.Candidates = append(result.Candidates, candidate)
		result.Bytes += candidateBytes
		result.Tokens += candidate.Tokens
	}
	result.View = makeView(request, result.Candidates)
	return result, nil
}

func (host *Host) sourceRequired(sourceID string) bool {
	_, required := host.requiredSources[sourceID]
	return required
}

func strictTreatment(treatment contextsource.Treatment) bool {
	return treatment == contextsource.TreatmentRequired || treatment == contextsource.TreatmentReserved
}

func treatmentRank(treatment contextsource.Treatment) int {
	switch treatment {
	case contextsource.TreatmentRequired:
		return 0
	case contextsource.TreatmentReserved:
		return 1
	default:
		return 2
	}
}

func makeView(request Request, candidates []Candidate) View {
	view := View{TenantID: request.TenantID, WorkspaceID: request.WorkspaceID, SessionID: request.SessionID, Items: make([]ViewItem, 0, len(candidates))}
	hash := sha256.New()
	for _, value := range []string{request.TenantID, request.WorkspaceID, request.SessionID} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	for _, candidate := range candidates {
		item := ViewItem{SourceID: candidate.SourceID, ContentID: candidate.ContentID, Version: candidate.Version, ProvenanceID: candidate.ProvenanceID, Treatment: candidate.Treatment}
		view.Items = append(view.Items, item)
		for _, value := range []string{item.SourceID, item.ContentID, item.Version, item.ProvenanceID, string(item.Treatment)} {
			_, _ = hash.Write([]byte(value))
			_, _ = hash.Write([]byte{0})
		}
	}
	view.ID = hex.EncodeToString(hash.Sum(nil))
	return view
}

func (host *Host) sourcePageLimit(requestLimit int) int {
	limit := host.maxCandidates
	if requestLimit > 0 && requestLimit < limit {
		limit = requestLimit
	}
	return limit
}

func (host *Host) querySource(ctx context.Context, source contextsource.Provider, request contextsource.Request) (contextsource.Page, error) {
	if err := ctx.Err(); err != nil {
		return contextsource.Page{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, host.sourceTimeout)
	defer cancel()
	outcomes := make(chan sourceOutcome, 1)
	go func() {
		outcome := sourceOutcome{}
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.err = fmt.Errorf("context source panic: %v", recovered)
			}
			// The buffered, non-blocking handoff prevents a provider that
			// ignores cancellation and returns after the timeout from leaking
			// this worker while trying to report its late result.
			select {
			case outcomes <- outcome:
			default:
			}
		}()
		outcome.page, outcome.err = source.Query(callCtx, request)
	}()
	select {
	case <-callCtx.Done():
		return contextsource.Page{}, callCtx.Err()
	case outcome := <-outcomes:
		if err := ctx.Err(); err != nil {
			return contextsource.Page{}, err
		}
		if err := callCtx.Err(); err != nil {
			return contextsource.Page{}, err
		}
		return outcome.page, outcome.err
	}
}

func (host *Host) resolveCandidate(ctx context.Context, source contextsource.Provider, request Request, candidate Candidate) (Candidate, error) {
	reference := candidate.Resource
	if reference == nil {
		return candidate, nil
	}
	if !scopeMatches(reference.Scope, request) {
		return Candidate{}, ErrResourceScope
	}
	if host.authorize != nil {
		if err := host.authorize(ctx, source.ID(), request); err != nil {
			return Candidate{}, err
		}
	}
	resolver, ok := source.(contextsource.Resolver)
	if !ok {
		return Candidate{}, ErrResourceResolver
	}
	maxBytes := host.maxCandidateBytes
	if request.ByteBudget > 0 && request.ByteBudget < maxBytes {
		maxBytes = request.ByteBudget
	}
	callCtx, cancel := context.WithTimeout(ctx, host.sourceTimeout)
	defer cancel()
	outcomes := make(chan resourceOutcome, 1)
	resolveRequest := contextsource.ResolveRequest{
		Reference: reference.Clone(), TenantID: request.TenantID, WorkspaceID: request.WorkspaceID,
		SessionID: request.SessionID, MaxBytes: maxBytes,
	}
	go func() {
		outcome := resourceOutcome{}
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.err = fmt.Errorf("context resource resolver panic: %v", recovered)
			}
			select {
			case outcomes <- outcome:
			default:
			}
		}()
		outcome.resource, outcome.err = resolver.Resolve(callCtx, resolveRequest)
	}()
	var outcome resourceOutcome
	select {
	case <-callCtx.Done():
		return Candidate{}, callCtx.Err()
	case outcome = <-outcomes:
	}
	if outcome.err != nil {
		return Candidate{}, outcome.err
	}
	resource := outcome.resource.Clone()
	if reference.VersionMode == contextsource.VersionExact && resource.Reference.Version != reference.Version {
		return Candidate{}, ErrResourceVersionMismatch
	}
	if resource.Reference.URI != reference.URI || resource.Reference.MediaType != reference.MediaType || !scopeEqual(resource.Reference.Scope, reference.Scope) {
		return Candidate{}, ErrResourceScope
	}
	if len(resource.Content) > maxBytes {
		return Candidate{}, ErrRequiredContextBudget
	}
	raw := candidate.Candidate.Clone()
	raw.Content = string(resource.Content)
	raw.SizeHint = len(resource.Content)
	raw.MediaType = resource.Reference.MediaType
	raw.Version = resource.Reference.Version
	raw.Resource = &resource.Reference
	resolved, valid := host.normalizeCandidate(source.ID(), raw)
	if !valid {
		return Candidate{}, ErrInvalidSource
	}
	return resolved, nil
}

func scopeMatches(scope contextsource.Scope, request Request) bool {
	return (scope.TenantID == "" || scope.TenantID == request.TenantID) &&
		(scope.WorkspaceID == "" || scope.WorkspaceID == request.WorkspaceID) &&
		(scope.SessionID == "" || scope.SessionID == request.SessionID)
}

func scopeEqual(left, right contextsource.Scope) bool { return left == right }

func (host *Host) normalizeCandidate(sourceID string, raw contextsource.Candidate) (Candidate, bool) {
	candidate := raw.Clone()
	if candidate.SourceID == "" {
		candidate.SourceID = sourceID
	}
	if candidate.SourceID != sourceID || !validBoundedIdentifier(candidate.SourceID, maxSourceIDBytes, false) || !validBoundedIdentifier(candidate.ContentID, maxContentIDBytes, false) {
		return Candidate{}, false
	}
	if !validBoundedIdentifier(candidate.MediaType, maxMediaTypeBytes, true) || !validBoundedIdentifier(candidate.Version, maxVersionBytes, true) {
		return Candidate{}, false
	}
	if candidate.Treatment == "" {
		candidate.Treatment = contextsource.TreatmentCompetitive
	}
	if candidate.Treatment != contextsource.TreatmentCompetitive && candidate.Treatment != contextsource.TreatmentReserved && candidate.Treatment != contextsource.TreatmentRequired {
		return Candidate{}, false
	}
	if candidate.ValidUntil < 0 {
		return Candidate{}, false
	}
	if candidate.Resource != nil {
		reference := candidate.Resource
		if !validBoundedIdentifier(reference.URI, maxContentIDBytes, false) || !validBoundedIdentifier(reference.Version, maxVersionBytes, false) || !validBoundedIdentifier(reference.MediaType, maxMediaTypeBytes, false) || reference.SizeHint < 0 {
			return Candidate{}, false
		}
		if reference.VersionMode != contextsource.VersionExact && reference.VersionMode != contextsource.VersionBestEffort {
			return Candidate{}, false
		}
		if candidate.Version != "" && candidate.Version != reference.Version {
			return Candidate{}, false
		}
		if candidate.MediaType != "" && candidate.MediaType != reference.MediaType {
			return Candidate{}, false
		}
		candidate.Version = reference.Version
		candidate.MediaType = reference.MediaType
	}
	if !utf8.ValidString(candidate.Content) || !validMetadata(candidate.Metadata) {
		return Candidate{}, false
	}
	if math.IsNaN(candidate.Confidence) || math.IsInf(candidate.Confidence, 0) || candidate.Confidence < 0 || candidate.Confidence > 1 {
		return Candidate{}, false
	}
	candidate.Content = tools.RedactSensitive(candidate.Content)
	if candidate.Metadata != nil {
		redacted := make(map[string]string, len(candidate.Metadata))
		for key, value := range candidate.Metadata {
			redacted[key] = tools.RedactSensitive(value)
		}
		candidate.Metadata = redacted
	}
	if !validMetadata(candidate.Metadata) {
		return Candidate{}, false
	}
	candidate.SizeHint = len(candidate.Content)
	projected := Candidate{Candidate: candidate}
	projected.Tokens = host.estimate(candidate.Content)
	if projected.Tokens < 0 {
		return Candidate{}, false
	}
	projected.ProvenanceID = provenanceID(candidate)
	return projected, true
}

func validBoundedIdentifier(value string, maxBytes int, allowEmpty bool) bool {
	if value == "" {
		return allowEmpty
	}
	if len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	if strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || isBidiControl(r) {
			return false
		}
	}
	return true
}

func validMetadata(metadata map[string]string) bool {
	if len(metadata) > maxMetadataEntries {
		return false
	}
	total := 0
	for key, value := range metadata {
		if !validBoundedIdentifier(key, maxMetadataKeyBytes, false) || !validMetadataValue(value) {
			return false
		}
		total += len(key) + len(value)
		if total > maxMetadataTotalBytes {
			return false
		}
	}
	return true
}

func validMetadataValue(value string) bool {
	if len(value) > maxMetadataValueBytes || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || isBidiControl(r) {
			return false
		}
	}
	return true
}

func isBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

func provenanceID(candidate contextsource.Candidate) string {
	hash := sha256.New()
	for _, value := range []string{candidate.SourceID, candidate.ContentID, candidate.Version, candidate.MediaType, contentHash(candidate.MediaType, candidate.Content)} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func contentHash(mediaType, content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(mediaType) + "\x00" + content))
	return hex.EncodeToString(sum[:])
}
