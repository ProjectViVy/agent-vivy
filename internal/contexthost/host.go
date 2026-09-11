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
	ErrInvalidSource   = errors.New("contexthost: invalid source")
	ErrDuplicateSource = errors.New("contexthost: duplicate source")
)

type AuthorizeFunc func(context.Context, string, Request) error
type TokenEstimator func(string) int

type Config struct {
	Sources           []contextsource.Provider
	Authorize         AuthorizeFunc
	SourceTimeout     time.Duration
	MaxCandidateBytes int
	MaxTotalBytes     int
	MaxCandidates     int
	TokenEstimator    TokenEstimator
}

type Request struct {
	Query       string
	SessionID   string
	WorkspaceID string
	TokenBudget int
	// ByteBudget is the final model-facing budget available to the result.
	// Zero means that the caller has no additional byte budget. The runtime
	// supplies CandidateBytes when framing overhead must be accounted for.
	ByteBudget int
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
}

type Host struct {
	sources           []contextsource.Provider
	authorize         AuthorizeFunc
	sourceTimeout     time.Duration
	maxCandidateBytes int
	maxTotalBytes     int
	maxCandidates     int
	estimate          TokenEstimator
}

type sourceOutcome struct {
	page contextsource.Page
	err  error
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

	sources, err := validateSources(cfg.Sources)
	if err != nil {
		return nil, err
	}
	return &Host{
		sources:           sources,
		authorize:         cfg.Authorize,
		sourceTimeout:     timeout,
		maxCandidateBytes: maxCandidateBytes,
		maxTotalBytes:     maxTotalBytes,
		maxCandidates:     maxCandidates,
		estimate:          estimator,
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
			SessionID:   request.SessionID,
			WorkspaceID: request.WorkspaceID,
			Cursor:      cursor,
			Limit:       host.sourcePageLimit(request.PerSourceLimit),
		})
		if err != nil {
			result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
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
				result.DroppedInvalid++
				continue
			}
			if len(candidate.Content) > host.maxCandidateBytes {
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
			result.DroppedBudget++
			continue
		}
		if request.ByteBudget > 0 && result.Bytes+candidateBytes > request.ByteBudget {
			result.DroppedBudget++
			continue
		}
		if request.TokenBudget > 0 && result.Tokens+candidate.Tokens > request.TokenBudget {
			result.DroppedBudget++
			continue
		}
		result.Candidates = append(result.Candidates, candidate)
		result.Bytes += candidateBytes
		result.Tokens += candidate.Tokens
	}
	return result, nil
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
