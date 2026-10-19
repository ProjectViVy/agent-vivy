// Package modelhost owns the internal, build-selected Provider Profile
// registry. It contains no Eino imports or executable provider factories;
// internal/provider is the quarantined adapter layer.
package modelhost

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"agent-vivy/sdk/port/providerprofile"
)

var (
	ErrHostRequired    = errors.New("ModelHost is required")
	ErrProfileNotFound = errors.New("provider Profile is not compiled")
)

// ProfileState is the runtime-safe projection of one compiled Provider
// Profile. It deliberately describes availability without carrying Secret
// references or provider options across the control-plane boundary.
type ProfileState string

const (
	ProfileCompiled     ProfileState = "COMPILED"
	ProfileUnconfigured ProfileState = "UNCONFIGURED"
	ProfileReady        ProfileState = "READY"
	ProfileUnavailable  ProfileState = "UNAVAILABLE"
)

type ProfileStatus struct {
	ID            string
	AdapterFamily string
	EndpointClass providerprofile.EndpointClass
	ModelIDs      []string
	State         ProfileState
}

type Host struct {
	profiles map[string]providerprofile.Profile
	statusMu sync.RWMutex
	// unavailable records Profile ids whose adapter construction already
	// failed in this process.
	unavailable map[string]bool
}

// New indexes the compiled Profiles over the adapter families this build can
// construct. A Profile naming a family outside executableAdapters is a
// construction error: the Generation seals executable adapters, so a Profile
// that no adapter can serve must never become resolvable.
func New(profiles []providerprofile.Profile, executableAdapters []string) (*Host, error) {
	host := &Host{
		profiles:    make(map[string]providerprofile.Profile, len(profiles)),
		unavailable: make(map[string]bool),
	}
	executable := make(map[string]struct{}, len(executableAdapters))
	for _, family := range executableAdapters {
		if family == "" {
			continue
		}
		executable[family] = struct{}{}
	}
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil {
			return nil, fmt.Errorf("modelhost: Profile %q: %w", profile.ID, err)
		}
		if _, duplicate := host.profiles[profile.ID]; duplicate {
			return nil, fmt.Errorf("modelhost: duplicate Profile id %q", profile.ID)
		}
		if _, known := executable[profile.AdapterFamily]; !known {
			return nil, fmt.Errorf("modelhost: Profile %q names unsupported adapter family %q", profile.ID, profile.AdapterFamily)
		}
		host.profiles[profile.ID] = profile.Clone()
	}
	return host, nil
}

// MarkUnavailable records a failed adapter construction without retaining the
// provider error (which may include upstream or credential detail).
func (host *Host) MarkUnavailable(id string) {
	if host == nil {
		return
	}
	host.statusMu.Lock()
	defer host.statusMu.Unlock()
	if _, compiled := host.profiles[id]; compiled {
		host.unavailable[id] = true
	}
}

// MarkAvailable clears a previous adapter-construction failure after a
// successful construction. Status inspection itself never probes a network.
func (host *Host) MarkAvailable(id string) {
	if host == nil {
		return
	}
	host.statusMu.Lock()
	defer host.statusMu.Unlock()
	delete(host.unavailable, id)
}

// Statuses projects the compiled Generation profiles in stable ID order.
// activeReady comes from the configuration resolver; no Secret value enters
// this surface.
func (host *Host) Statuses(activeID string, activeReady bool) []ProfileStatus {
	if host == nil {
		return []ProfileStatus{}
	}
	host.statusMu.RLock()
	defer host.statusMu.RUnlock()
	statuses := make([]ProfileStatus, 0, len(host.profiles))
	for _, profile := range host.profiles {
		state := ProfileCompiled
		switch {
		case host.unavailable[profile.ID]:
			state = ProfileUnavailable
		case profile.ID == activeID && activeReady:
			state = ProfileReady
		case profile.ID == activeID:
			state = ProfileUnconfigured
		}
		statuses = append(statuses, ProfileStatus{
			ID: profile.ID, AdapterFamily: profile.AdapterFamily,
			EndpointClass: profile.EndpointClass,
			ModelIDs:      append([]string(nil), profile.ModelIDs...),
			State:         state,
		})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].ID < statuses[j].ID })
	return statuses
}

func (host *Host) Profiles() []providerprofile.Profile {
	if host == nil {
		return nil
	}
	profiles := make([]providerprofile.Profile, 0, len(host.profiles))
	for _, profile := range host.profiles {
		profiles = append(profiles, profile.Clone())
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles
}

// ResolveExecutable returns the compiled Profile id. Every Profile in a built
// Host was validated against the executable adapter set at construction, so
// an unknown id is the only remaining failure — the same fail-closed answer
// for a stored selection this build cannot serve.
func (host *Host) ResolveExecutable(id string) (providerprofile.Profile, error) {
	if host == nil {
		return providerprofile.Profile{}, fmt.Errorf("modelhost: %w: %q", ErrProfileNotFound, id)
	}
	profile, ok := host.profiles[id]
	if !ok {
		return providerprofile.Profile{}, fmt.Errorf("modelhost: %w: %q", ErrProfileNotFound, id)
	}
	return profile.Clone(), nil
}
