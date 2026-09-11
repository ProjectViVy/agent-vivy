package skillhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/skillsource"
)

const (
	defaultSourceTimeout       = 750 * time.Millisecond
	defaultMaxSkillBytes       = 512 << 10
	defaultMaxSkills           = 256
	defaultMaxAlwaysSkillBytes = 4000
	defaultMaxAlwaysTotalBytes = 2000
)

var (
	ErrInvalidSource      = errors.New("skillhost: invalid source")
	ErrDuplicateSource    = errors.New("skillhost: duplicate source")
	ErrDuplicateSkillID   = errors.New("skillhost: duplicate skill id")
	ErrInvalidSkill       = errors.New("skillhost: invalid skill")
	ErrSkillNotFound      = errors.New("skillhost: skill not found")
	ErrAmbiguousSkillName = errors.New("skillhost: ambiguous skill name")
	ErrSkillDenied        = errors.New("skillhost: skill authorization denied")
	ErrSkillInactive      = errors.New("skillhost: skill is outside activation scope")
	ErrSkillTooLarge      = errors.New("skillhost: skill exceeds content budget")
	ErrSkillChanged       = errors.New("skillhost: skill metadata changed during resolution")
)

var skillIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Config struct {
	Sources             []skillsource.Provider
	SourceTimeout       time.Duration
	MaxSkillBytes       int
	MaxSkills           int
	MaxAlwaysSkillBytes int
	MaxAlwaysTotalBytes int
	// ActiveIDs is the host-owned activation scope. A non-nil value means
	// only the listed stable IDs may be projected, regardless of source
	// metadata. An empty but non-nil slice therefore activates nothing.
	ActiveIDs []string
	// Authorize is evaluated by SkillHost after source discovery and before
	// content resolution. Source text and metadata never grant authority.
	Authorize AuthorizeFunc
}

// AuthorizeFunc is the SkillHost authority seam. The source ID is not
// authority; the stable skill ID and request identity are the inputs to the
// host-owned decision.
type AuthorizeFunc func(context.Context, string, Request) error

type Request struct {
	SessionID   string
	WorkspaceID string
	// ActiveIDs optionally narrows the configured Host activation scope for
	// one request. IDs are matched literally; names and case-folded aliases
	// are never accepted as scope grants.
	ActiveIDs []string
}

type Summary struct {
	skillsource.Summary
	SourceID     string
	ProvenanceID string
}

type ResolvedSkill struct {
	skillsource.Skill
	SourceID     string
	ContentHash  string
	ProvenanceID string
}

type Failure struct {
	SourceID string
	Cause    error
}

type Host struct {
	sources             []skillsource.Provider
	sourceTimeout       time.Duration
	maxSkillBytes       int
	maxSkills           int
	maxAlwaysSkillBytes int
	maxAlwaysTotalBytes int
	activeIDs           map[string]struct{}
	activeIDsSet        bool
	authorize           AuthorizeFunc

	mu       sync.RWMutex
	failures []Failure
}

type catalogEntry struct {
	summary skillsource.Summary
	source  skillsource.Provider
}

type catalog struct {
	byID   map[string]catalogEntry
	byName map[string][]catalogEntry
}

type listOutcome struct {
	items []skillsource.Summary
	err   error
}

type getOutcome struct {
	skill skillsource.Skill
	err   error
}

func New(cfg Config) (*Host, error) {
	timeout := cfg.SourceTimeout
	if timeout <= 0 {
		timeout = defaultSourceTimeout
	}
	maxBytes := cfg.MaxSkillBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxSkillBytes
	}
	maxSkills := cfg.MaxSkills
	if maxSkills <= 0 {
		maxSkills = defaultMaxSkills
	}
	maxAlwaysSkillBytes := cfg.MaxAlwaysSkillBytes
	if maxAlwaysSkillBytes <= 0 {
		maxAlwaysSkillBytes = defaultMaxAlwaysSkillBytes
	}
	maxAlwaysTotalBytes := cfg.MaxAlwaysTotalBytes
	if maxAlwaysTotalBytes <= 0 {
		maxAlwaysTotalBytes = defaultMaxAlwaysTotalBytes
	}
	seen := make(map[string]struct{}, len(cfg.Sources))
	sources := make([]skillsource.Provider, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		if source == nil || source.ID() == "" || strings.TrimSpace(source.ID()) != source.ID() {
			return nil, ErrInvalidSource
		}
		id := source.ID()
		key := strings.ToLower(id)
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateSource, id)
		}
		seen[key] = struct{}{}
		sources = append(sources, source)
	}
	activeIDs := make(map[string]struct{}, len(cfg.ActiveIDs))
	for _, id := range cfg.ActiveIDs {
		if strings.TrimSpace(id) != id || !skillIDPattern.MatchString(id) {
			return nil, fmt.Errorf("%w: invalid active skill ID %q", ErrInvalidSkill, id)
		}
		if _, duplicate := activeIDs[id]; duplicate {
			return nil, fmt.Errorf("%w: duplicate active skill ID %s", ErrInvalidSkill, id)
		}
		activeIDs[id] = struct{}{}
	}
	return &Host{
		sources: sources, sourceTimeout: timeout, maxSkillBytes: maxBytes, maxSkills: maxSkills,
		maxAlwaysSkillBytes: maxAlwaysSkillBytes, maxAlwaysTotalBytes: maxAlwaysTotalBytes,
		activeIDs: activeIDs, activeIDsSet: cfg.ActiveIDs != nil, authorize: cfg.Authorize,
	}, nil
}

func (host *Host) List(ctx context.Context, request Request) ([]Summary, error) {
	if err := validateRequestScope(request); err != nil {
		return nil, err
	}
	catalog, failures, err := host.catalog(ctx, request)
	host.setFailures(failures)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(catalog.byID))
	for key := range catalog.byID {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Summary, 0, len(catalog.byID))
	for _, key := range keys {
		entry := catalog.byID[key]
		if !entry.summary.Available || !host.active(entry.summary, request) {
			continue
		}
		if err := host.authorizeSkill(ctx, entry.summary, request); err != nil {
			host.appendFailure(Failure{SourceID: entry.source.ID(), Cause: err})
			continue
		}
		out = append(out, Summary{
			Summary:      cloneSummary(entry.summary),
			SourceID:     entry.source.ID(),
			ProvenanceID: summaryProvenance(entry.source.ID(), entry.summary),
		})
	}
	return out, nil
}

func (host *Host) Get(ctx context.Context, request Request, id string) (ResolvedSkill, error) {
	if err := validateRequestScope(request); err != nil {
		return ResolvedSkill{}, err
	}
	catalog, failures, err := host.catalog(ctx, request)
	host.setFailures(failures)
	if err != nil {
		return ResolvedSkill{}, err
	}
	entry, err := catalog.resolve(id)
	if err != nil {
		return ResolvedSkill{}, err
	}
	if !entry.summary.Available {
		return ResolvedSkill{}, fmt.Errorf("%w: %s", ErrSkillNotFound, id)
	}
	if !host.active(entry.summary, request) {
		return ResolvedSkill{}, fmt.Errorf("%w: %s", ErrSkillInactive, entry.summary.ID)
	}
	if err := host.authorizeSkill(ctx, entry.summary, request); err != nil {
		return ResolvedSkill{}, err
	}

	skill, err := host.getFromSource(ctx, entry.source, skillsource.Request{
		SessionID:   request.SessionID,
		WorkspaceID: request.WorkspaceID,
	}, entry.summary.ID)
	if err != nil {
		host.appendFailure(Failure{SourceID: entry.source.ID(), Cause: err})
		return ResolvedSkill{}, err
	}
	skill = skill.Clone()
	if !sameIdentity(entry.summary, skill) {
		return ResolvedSkill{}, fmt.Errorf("%w: %s", ErrSkillChanged, entry.summary.ID)
	}
	if !skill.Available {
		return ResolvedSkill{}, fmt.Errorf("%w: %s", ErrSkillNotFound, entry.summary.ID)
	}
	if err := validateSkill(skill.Summary()); err != nil {
		return ResolvedSkill{}, err
	}
	if strings.TrimSpace(skill.Content) == "" {
		return ResolvedSkill{}, fmt.Errorf("%w: %s has empty content", ErrInvalidSkill, entry.summary.ID)
	}
	// Redaction is part of the model-facing projection. Enforce the byte
	// budget after it, because replacement markers may be larger than the
	// source token that triggered them.
	skill.Content = tools.RedactSensitive(skill.Content)
	if len(skill.Content) > host.maxSkillBytes {
		return ResolvedSkill{}, fmt.Errorf("%w: %s", ErrSkillTooLarge, entry.summary.ID)
	}
	content := skill.Content
	contentHash := hashText(content)
	sourceID := entry.source.ID()
	return ResolvedSkill{
		Skill:        skill,
		SourceID:     sourceID,
		ContentHash:  contentHash,
		ProvenanceID: resolvedProvenance(sourceID, skill, contentHash),
	}, nil
}

// Always resolves and projects enabled always-skills through the Host. The
// byte budgets include redaction and the exact section framing emitted to the
// model, so the adapter cannot expand the final projection after the Host has
// approved it.
func (host *Host) Always(ctx context.Context, request Request) (string, error) {
	items, err := host.List(ctx, request)
	if err != nil {
		return "", err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	remaining := host.maxAlwaysTotalBytes
	sections := make([]string, 0, len(items))
	for _, item := range items {
		if !item.Always || remaining <= 0 {
			continue
		}
		resolved, err := host.Get(ctx, request, item.ID)
		if err != nil {
			if errors.Is(err, ErrSkillTooLarge) || errors.Is(err, ErrSkillNotFound) || errors.Is(err, ErrSkillInactive) || errors.Is(err, ErrSkillDenied) {
				continue
			}
			return "", err
		}
		if len(resolved.Content) > host.maxAlwaysSkillBytes {
			continue
		}
		header := fmt.Sprintf("### Skill: %s\n\n", resolved.Name)
		separator := ""
		if len(sections) > 0 {
			separator = "\n\n---\n\n"
		}
		available := remaining - len(separator) - len(header)
		perSkillAvailable := host.maxAlwaysSkillBytes - len(header)
		if perSkillAvailable < available {
			available = perSkillAvailable
		}
		if available <= 0 {
			continue
		}
		body := truncateUTF8Bytes(resolved.Content, available)
		if strings.TrimSpace(body) == "" {
			continue
		}
		section := header + body
		if len(separator)+len(section) > remaining {
			// truncateUTF8Bytes is conservative; retain this guard as a
			// fail-closed assertion if framing changes later.
			continue
		}
		sections = append(sections, section)
		remaining -= len(separator) + len(section)
	}
	return strings.Join(sections, "\n\n---\n\n"), nil
}

func (host *Host) Failures() []Failure {
	host.mu.RLock()
	defer host.mu.RUnlock()
	return append([]Failure(nil), host.failures...)
}

func (host *Host) catalog(ctx context.Context, request Request) (*catalog, []Failure, error) {
	entries := &catalog{byID: make(map[string]catalogEntry), byName: make(map[string][]catalogEntry)}
	failures := make([]Failure, 0)
	count := 0
	for _, source := range host.sources {
		items, err := host.listSource(ctx, source, skillsource.Request{
			SessionID:   request.SessionID,
			WorkspaceID: request.WorkspaceID,
		})
		if err != nil {
			failures = append(failures, Failure{SourceID: source.ID(), Cause: err})
			continue
		}
		for _, raw := range items {
			summary := cloneSummary(raw)
			if err := validateSkill(summary); err != nil {
				return nil, failures, err
			}
			key := strings.ToLower(summary.ID)
			if previous, duplicate := entries.byID[key]; duplicate {
				return nil, failures, fmt.Errorf("%w: %s from %s and %s", ErrDuplicateSkillID, summary.ID, previous.source.ID(), source.ID())
			}
			if count >= host.maxSkills {
				return nil, failures, fmt.Errorf("%w: catalog exceeds %d skills", ErrInvalidSkill, host.maxSkills)
			}
			entry := catalogEntry{summary: summary, source: source}
			entries.byID[key] = entry
			entries.byName[summary.Name] = append(entries.byName[summary.Name], entry)
			count++
		}
	}
	return entries, failures, nil
}

func (catalog *catalog) resolve(query string) (catalogEntry, error) {
	if catalog == nil || query == "" || strings.TrimSpace(query) != query {
		return catalogEntry{}, fmt.Errorf("%w: %s", ErrSkillNotFound, query)
	}
	// Stable IDs are exact and win over display-name aliases. This preserves
	// identity when a display name happens to equal another skill's name.
	if entry, ok := catalog.byID[strings.ToLower(query)]; ok && entry.summary.ID == query {
		return entry, nil
	}
	// A case-folded ID is only accepted when it is the literal stored ID. IDs
	// differing only by case are rejected while building the catalog, so this
	// branch intentionally does not alias them.
	if candidates := catalog.byName[query]; len(candidates) == 1 {
		return candidates[0], nil
	} else if len(candidates) > 1 {
		return catalogEntry{}, fmt.Errorf("%w: %s", ErrAmbiguousSkillName, query)
	}
	return catalogEntry{}, fmt.Errorf("%w: %s", ErrSkillNotFound, query)
}

func (host *Host) active(summary skillsource.Summary, request Request) bool {
	if host.activeIDsSet {
		if _, ok := host.activeIDs[summary.ID]; !ok {
			return false
		}
	}
	if request.ActiveIDs == nil {
		return true
	}
	for _, id := range request.ActiveIDs {
		if id == summary.ID {
			return true
		}
	}
	return false
}

func validateRequestScope(request Request) error {
	for _, id := range request.ActiveIDs {
		if strings.TrimSpace(id) != id || !skillIDPattern.MatchString(id) {
			return fmt.Errorf("%w: invalid request activation ID %q", ErrSkillInactive, id)
		}
	}
	return nil
}

func (host *Host) authorizeSkill(ctx context.Context, summary skillsource.Summary, request Request) error {
	if host.authorize == nil {
		return nil
	}
	if err := host.authorize(ctx, summary.ID, request); err != nil {
		if errors.Is(err, ErrSkillDenied) {
			return fmt.Errorf("%w: %s", ErrSkillDenied, summary.ID)
		}
		return fmt.Errorf("%w: %s: %v", ErrSkillDenied, summary.ID, err)
	}
	return nil
}

func (host *Host) listSource(ctx context.Context, source skillsource.Provider, request skillsource.Request) ([]skillsource.Summary, error) {
	callCtx, cancel := context.WithTimeout(ctx, host.sourceTimeout)
	defer cancel()
	outcomes := make(chan listOutcome, 1)
	go func() {
		outcome := listOutcome{}
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.err = fmt.Errorf("skill source panic: %v", recovered)
			}
			outcomes <- outcome
		}()
		outcome.items, outcome.err = source.List(callCtx, request)
	}()
	select {
	case <-callCtx.Done():
		return nil, callCtx.Err()
	case outcome := <-outcomes:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := callCtx.Err(); err != nil {
			return nil, err
		}
		return outcome.items, outcome.err
	}
}

func (host *Host) getFromSource(ctx context.Context, source skillsource.Provider, request skillsource.Request, id string) (skillsource.Skill, error) {
	callCtx, cancel := context.WithTimeout(ctx, host.sourceTimeout)
	defer cancel()
	outcomes := make(chan getOutcome, 1)
	go func() {
		outcome := getOutcome{}
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.err = fmt.Errorf("skill source panic: %v", recovered)
			}
			outcomes <- outcome
		}()
		outcome.skill, outcome.err = source.Get(callCtx, request, id)
	}()
	select {
	case <-callCtx.Done():
		return skillsource.Skill{}, callCtx.Err()
	case outcome := <-outcomes:
		if err := ctx.Err(); err != nil {
			return skillsource.Skill{}, err
		}
		if err := callCtx.Err(); err != nil {
			return skillsource.Skill{}, err
		}
		return outcome.skill, outcome.err
	}
}

func validateSkill(summary skillsource.Summary) error {
	id := summary.ID
	name := summary.Name
	if strings.TrimSpace(id) != id || strings.TrimSpace(name) != name || strings.TrimSpace(summary.Version) != summary.Version || strings.TrimSpace(summary.SourceHash) != summary.SourceHash ||
		!skillIDPattern.MatchString(id) || !skillIDPattern.MatchString(name) || summary.Version == "" || summary.SourceHash == "" {
		return fmt.Errorf("%w: invalid identity/version for %q", ErrInvalidSkill, id)
	}
	if strings.TrimSpace(summary.Context) != summary.Context {
		return fmt.Errorf("%w: invalid context for %s", ErrInvalidSkill, id)
	}
	switch summary.Context {
	case "", "fork", "fork_with_context":
	default:
		return fmt.Errorf("%w: unsupported context %q for %s", ErrInvalidSkill, summary.Context, id)
	}
	for _, dependency := range summary.Dependencies {
		if strings.TrimSpace(dependency) != dependency || !skillIDPattern.MatchString(dependency) {
			return fmt.Errorf("%w: invalid dependency %q for %s", ErrInvalidSkill, dependency, id)
		}
	}
	return nil
}

func sameIdentity(summary skillsource.Summary, skill skillsource.Skill) bool {
	return summary.ID == skill.ID && summary.Name == skill.Name && summary.Version == skill.Version && summary.SourceHash == skill.SourceHash
}

func cloneSummary(summary skillsource.Summary) skillsource.Summary {
	skill := skillsource.Skill{
		ID: summary.ID, Name: summary.Name, Description: summary.Description,
		Version: summary.Version, SourceHash: summary.SourceHash,
		Available: summary.Available, DisabledReason: summary.DisabledReason,
		Dependencies: summary.Dependencies, Always: summary.Always,
		UserInvocable: summary.UserInvocable, DeclaredTools: summary.DeclaredTools,
		Context: summary.Context, Agent: summary.Agent, Model: summary.Model,
		Metadata: summary.Metadata,
	}.Clone()
	return skill.Summary()
}

func summaryProvenance(sourceID string, summary skillsource.Summary) string {
	return hashFields(sourceID, summary.ID, summary.Name, summary.Version, summary.SourceHash)
}

func resolvedProvenance(sourceID string, skill skillsource.Skill, contentHash string) string {
	return hashFields(sourceID, skill.ID, skill.Name, skill.Version, skill.SourceHash, contentHash)
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func hashFields(fields ...string) string {
	hash := sha256.New()
	for _, field := range fields {
		_, _ = hash.Write([]byte(strings.TrimSpace(field)))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	cut := value[:maxBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func (host *Host) setFailures(failures []Failure) {
	host.mu.Lock()
	host.failures = append([]Failure(nil), failures...)
	host.mu.Unlock()
}

func (host *Host) appendFailure(failure Failure) {
	host.mu.Lock()
	host.failures = append(host.failures, failure)
	host.mu.Unlock()
}

// StableSummaries is useful for status/inspection projections without loading
// Skill content. It is deliberately derived from List and carries no source
// mutation authority.
func StableSummaries(items []Summary) []Summary {
	out := append([]Summary(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
