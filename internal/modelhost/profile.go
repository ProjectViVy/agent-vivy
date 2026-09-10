// Package modelhost owns the internal, build-selected Provider Profile
// registry. It contains no Eino imports or executable provider factories;
// internal/provider is the quarantined adapter layer.
package modelhost

import (
	"errors"
	"fmt"
	"sort"

	"agent-vivy/sdk/port/providerprofile"
)

type CapabilityState string

const (
	CapabilitySupported          CapabilityState = "SUPPORTED"
	CapabilityDeferredIndefinite CapabilityState = "DEFERRED-INDEFINITE"
)

var (
	ErrHostRequired       = errors.New("ModelHost is required")
	ErrProfileNotFound    = errors.New("provider Profile is not compiled")
	ErrAdapterUnavailable = errors.New("provider adapter is unavailable")
)

type Capabilities map[string]CapabilityState

type Host struct {
	profiles     map[string]providerprofile.Profile
	capabilities Capabilities
}

func New(profiles []providerprofile.Profile, capabilities Capabilities) (*Host, error) {
	host := &Host{
		profiles:     make(map[string]providerprofile.Profile, len(profiles)),
		capabilities: make(Capabilities, len(capabilities)),
	}
	for family, state := range capabilities {
		if family == "" || (state != CapabilitySupported && state != CapabilityDeferredIndefinite) {
			return nil, fmt.Errorf("modelhost: invalid adapter capability %q=%q", family, state)
		}
		host.capabilities[family] = state
	}
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil {
			return nil, fmt.Errorf("modelhost: Profile %q: %w", profile.ID, err)
		}
		if _, duplicate := host.profiles[profile.ID]; duplicate {
			return nil, fmt.Errorf("modelhost: duplicate Profile id %q", profile.ID)
		}
		if _, known := host.capabilities[profile.AdapterFamily]; !known {
			return nil, fmt.Errorf("modelhost: Profile %q names unsupported adapter family %q", profile.ID, profile.AdapterFamily)
		}
		host.profiles[profile.ID] = profile.Clone()
	}
	return host, nil
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

func (host *Host) ResolveExecutable(id string) (providerprofile.Profile, error) {
	if host == nil {
		return providerprofile.Profile{}, fmt.Errorf("modelhost: %w: %q", ErrProfileNotFound, id)
	}
	profile, ok := host.profiles[id]
	if !ok {
		return providerprofile.Profile{}, fmt.Errorf("modelhost: %w: %q", ErrProfileNotFound, id)
	}
	if host.capabilities[profile.AdapterFamily] != CapabilitySupported {
		return providerprofile.Profile{}, fmt.Errorf("modelhost: %w: Profile %q family %q is %s", ErrAdapterUnavailable, id, profile.AdapterFamily, host.capabilities[profile.AdapterFamily])
	}
	return profile.Clone(), nil
}

func (host *Host) Capability(family string) (CapabilityState, bool) {
	if host == nil {
		return "", false
	}
	state, ok := host.capabilities[family]
	return state, ok
}
