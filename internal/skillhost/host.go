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

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/skillsource"
)

const (
	defaultSourceTimeout = 750 * time.Millisecond
	defaultMaxSkillBytes = 512 << 10
	defaultMaxSkills     = 256
)

var (
	ErrInvalidSource    = errors.New("skillhost: invalid source")
	ErrDuplicateSource  = errors.New("skillhost: duplicate source")
	ErrDuplicateSkillID = errors.New("skillhost: duplicate skill id")
	ErrInvalidSkill     = errors.New("skillhost: invalid skill")
	ErrSkillNotFound    = errors.New("skillhost: skill not found")
	ErrSkillTooLarge    = errors.New("skillhost: skill exceeds content budget")
	ErrSkillChanged     = errors.New("skillhost: skill metadata changed during resolution")
)

var skillIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Config struct {
	Sources       []skillsource.Provider
	SourceTimeout time.Duration
	MaxSkillBytes int
	MaxSkills     int
}

type Request struct {
	SessionID   string
	WorkspaceID string
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
	sources       []skillsource.Provider
	sourceTimeout time.Duration
	maxSkillBytes int
	maxSkills     int

	mu       sync.RWMutex
	failures []Failure
}

type catalogEntry struct {
	summary skillsource.Summary
	source  skillsource.Provider
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
	seen := make(map[string]struct{}, len(cfg.Sources))
	sources := make([]skillsource.Provider, 0, len(cfg.Sources))
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
	return &Host{sources: sources, sourceTimeout: timeout, maxSkillBytes: maxBytes, maxSkills: maxSkills}, nil
}

func (host *Host) List(ctx context.Context, request Request) ([]Summary, error) {
	entries, failures, err := host.catalog(ctx, request)
	host.setFailures(failures)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Summary, 0, len(entries))
	for _, key := range keys {
		entry := entries[key]
		if !entry.summary.Available {
			continue
		}
		out = append(out, Summary{
			Summary:      cloneSummary(entry.summary),
			SourceID:     strings.TrimSpace(entry.source.ID()),
			ProvenanceID: summaryProvenance(strings.TrimSpace(entry.source.ID()), entry.summary),
		})
	}
	return out, nil
}

func (host *Host) Get(ctx context.Context, request Request, id string) (ResolvedSkill, error) {
	entries, failures, err := host.catalog(ctx, request)
	host.setFailures(failures)
	if err != nil {
		return ResolvedSkill{}, err
	}
	key := strings.ToLower(strings.TrimSpace(id))
	entry, ok := entries[key]
	if !ok || !entry.summary.Available {
		return ResolvedSkill{}, fmt.Errorf("%w: %s", ErrSkillNotFound, id)
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
	if len(skill.Content) > host.maxSkillBytes {
		return ResolvedSkill{}, fmt.Errorf("%w: %s", ErrSkillTooLarge, entry.summary.ID)
	}
	if strings.TrimSpace(skill.Content) == "" {
		return ResolvedSkill{}, fmt.Errorf("%w: %s has empty content", ErrInvalidSkill, entry.summary.ID)
	}
	content := tools.RedactSensitive(skill.Content)
	skill.Content = content
	contentHash := hashText(content)
	sourceID := strings.TrimSpace(entry.source.ID())
	return ResolvedSkill{
		Skill:        skill,
		SourceID:     sourceID,
		ContentHash:  contentHash,
		ProvenanceID: resolvedProvenance(sourceID, skill, contentHash),
	}, nil
}

func (host *Host) Failures() []Failure {
	host.mu.RLock()
	defer host.mu.RUnlock()
	return append([]Failure(nil), host.failures...)
}

func (host *Host) catalog(ctx context.Context, request Request) (map[string]catalogEntry, []Failure, error) {
	entries := make(map[string]catalogEntry)
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
			if previous, duplicate := entries[key]; duplicate {
				return nil, failures, fmt.Errorf("%w: %s from %s and %s", ErrDuplicateSkillID, summary.ID, previous.source.ID(), source.ID())
			}
			if count >= host.maxSkills {
				return nil, failures, fmt.Errorf("%w: catalog exceeds %d skills", ErrInvalidSkill, host.maxSkills)
			}
			entries[key] = catalogEntry{summary: summary, source: source}
			count++
		}
	}
	return entries, failures, nil
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
		return outcome.skill, outcome.err
	}
}

func validateSkill(summary skillsource.Summary) error {
	id := strings.TrimSpace(summary.ID)
	name := strings.TrimSpace(summary.Name)
	if !skillIDPattern.MatchString(id) || !skillIDPattern.MatchString(name) || strings.TrimSpace(summary.Version) == "" || strings.TrimSpace(summary.SourceHash) == "" {
		return fmt.Errorf("%w: invalid identity/version for %q", ErrInvalidSkill, id)
	}
	switch strings.TrimSpace(summary.Context) {
	case "", "fork", "fork_with_context":
	default:
		return fmt.Errorf("%w: unsupported context %q for %s", ErrInvalidSkill, summary.Context, id)
	}
	for _, dependency := range summary.Dependencies {
		if !skillIDPattern.MatchString(strings.TrimSpace(dependency)) {
			return fmt.Errorf("%w: invalid dependency %q for %s", ErrInvalidSkill, dependency, id)
		}
	}
	return nil
}

func sameIdentity(summary skillsource.Summary, skill skillsource.Skill) bool {
	return strings.EqualFold(strings.TrimSpace(summary.ID), strings.TrimSpace(skill.ID)) &&
		strings.TrimSpace(summary.Name) == strings.TrimSpace(skill.Name) &&
		strings.TrimSpace(summary.Version) == strings.TrimSpace(skill.Version) &&
		strings.TrimSpace(summary.SourceHash) == strings.TrimSpace(skill.SourceHash)
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
	return hashFields(sourceID, summary.ID, summary.Version, summary.SourceHash)
}

func resolvedProvenance(sourceID string, skill skillsource.Skill, contentHash string) string {
	return hashFields(sourceID, skill.ID, skill.Version, skill.SourceHash, contentHash)
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
