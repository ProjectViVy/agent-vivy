package contexthost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/contextsource"
)

const (
	defaultSourceTimeout     = 750 * time.Millisecond
	defaultMaxCandidateBytes = 1 << 20
	defaultMaxTotalBytes     = 4 << 20
	defaultMaxCandidates     = 64
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
	Query          string
	SessionID      string
	WorkspaceID    string
	TokenBudget    int
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

	seen := make(map[string]struct{}, len(cfg.Sources))
	sources := make([]contextsource.Provider, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		if source == nil || strings.TrimSpace(source.ID()) == "" {
			return nil, ErrInvalidSource
		}
		id := strings.TrimSpace(source.ID())
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateSource, id)
		}
		seen[id] = struct{}{}
		sources = append(sources, source)
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
	result := Result{NextCursors: make(map[string]string)}
	collected := make([]Candidate, 0)
	for _, source := range host.sources {
		sourceID := strings.TrimSpace(source.ID())
		if host.authorize != nil {
			if err := host.authorize(ctx, sourceID, request); err != nil {
				result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
				continue
			}
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
			Limit:       request.PerSourceLimit,
		})
		if err != nil {
			result.Failures = append(result.Failures, Failure{SourceID: sourceID, Cause: err})
			continue
		}
		if page.NextCursor != "" {
			result.NextCursors[sourceID] = page.NextCursor
		}
		for _, raw := range page.Candidates {
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

	// Stable sort preserves Recipe source order and each Source's own order
	// when confidence is equal. Sources cannot smuggle a second ordering
	// mechanism into the Host.
	sort.SliceStable(collected, func(i, j int) bool {
		return collected[i].Confidence > collected[j].Confidence
	})

	seenContent := make(map[string]struct{}, len(collected))
	for _, candidate := range collected {
		dedupKey := contentHash(candidate.MediaType, candidate.Content)
		if _, duplicate := seenContent[dedupKey]; duplicate {
			result.DroppedDuplicate++
			continue
		}
		seenContent[dedupKey] = struct{}{}
		if len(result.Candidates) >= host.maxCandidates {
			result.DroppedBudget++
			continue
		}
		if result.Bytes+len(candidate.Content) > host.maxTotalBytes {
			result.DroppedBudget++
			continue
		}
		if request.TokenBudget > 0 && result.Tokens+candidate.Tokens > request.TokenBudget {
			result.DroppedBudget++
			continue
		}
		result.Candidates = append(result.Candidates, candidate)
		result.Bytes += len(candidate.Content)
		result.Tokens += candidate.Tokens
	}
	return result, nil
}

func (host *Host) querySource(ctx context.Context, source contextsource.Provider, request contextsource.Request) (contextsource.Page, error) {
	callCtx, cancel := context.WithTimeout(ctx, host.sourceTimeout)
	defer cancel()
	outcomes := make(chan sourceOutcome, 1)
	go func() {
		outcome := sourceOutcome{}
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.err = fmt.Errorf("context source panic: %v", recovered)
			}
			outcomes <- outcome
		}()
		outcome.page, outcome.err = source.Query(callCtx, request)
	}()
	select {
	case <-callCtx.Done():
		return contextsource.Page{}, callCtx.Err()
	case outcome := <-outcomes:
		return outcome.page, outcome.err
	}
}

func (host *Host) normalizeCandidate(sourceID string, raw contextsource.Candidate) (Candidate, bool) {
	candidate := raw.Clone()
	if candidate.SourceID == "" {
		candidate.SourceID = sourceID
	}
	if candidate.SourceID != sourceID || candidate.ContentID == "" || candidate.Content == "" {
		return Candidate{}, false
	}
	if candidate.Confidence < 0 || candidate.Confidence > 1 {
		return Candidate{}, false
	}
	candidate.Content = tools.RedactSensitive(candidate.Content)
	candidate.SizeHint = len(candidate.Content)
	projected := Candidate{Candidate: candidate}
	projected.Tokens = host.estimate(candidate.Content)
	if projected.Tokens < 0 {
		return Candidate{}, false
	}
	projected.ProvenanceID = provenanceID(candidate)
	return projected, true
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
