// Package credential owns scoped, non-enumerable Secret reference resolution.
// Callers receive values only for references compiled into their Module scope.
package credential

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/providerprofile"
)

const (
	ID   = "vivy/credential"
	Port = "core/credential-resolver@v1"
)

var (
	ErrDenied  = errors.New("credential resolver: reference denied")
	ErrMissing = errors.New("credential resolver: referenced value is empty or unset")
)

type Resolver struct {
	scopes map[string]map[string]struct{}
}

func Compose(scopes map[string][]string) (*Resolver, error) {
	resolver := &Resolver{scopes: make(map[string]map[string]struct{}, len(scopes))}
	for moduleID, refs := range scopes {
		if strings.TrimSpace(moduleID) == "" || moduleID != strings.TrimSpace(moduleID) {
			return nil, fmt.Errorf("credential resolver: invalid Module id %q", moduleID)
		}
		allowed := make(map[string]struct{}, len(refs))
		for _, ref := range refs {
			if !config.ValidEnvKey(ref) {
				return nil, fmt.Errorf("credential resolver: invalid Secret reference %q for %s", ref, moduleID)
			}
			allowed[ref] = struct{}{}
		}
		resolver.scopes[moduleID] = allowed
	}
	return resolver, nil
}

func (resolver *Resolver) Resolve(moduleID, ref string) (string, error) {
	if resolver == nil {
		return "", ErrDenied
	}
	allowed, ok := resolver.scopes[moduleID]
	if !ok {
		return "", fmt.Errorf("%w for Module %q", ErrDenied, moduleID)
	}
	if _, ok := allowed[ref]; !ok {
		return "", fmt.Errorf("%w: Module %q cannot read %q", ErrDenied, moduleID, ref)
	}
	value, ok := os.LookupEnv(ref)
	if !ok || value == "" {
		return "", fmt.Errorf("%w: %q", ErrMissing, ref)
	}
	return value, nil
}

func (resolver *Resolver) IsSet(moduleID, ref string) bool {
	_, err := resolver.Resolve(moduleID, ref)
	return err == nil
}

// CompileScopes projects declarative Provider Profiles and Channel envelopes
// into per-Module allowlists. It copies names only; no value is read here.
func CompileScopes(profiles []providerprofile.Profile, channels config.Channels) map[string][]string {
	scopes := map[string][]string{"vivy/model": {}}
	for _, profile := range profiles {
		scopes["vivy/model"] = append(scopes["vivy/model"], profile.SecretRefs...)
	}
	for name, envelope := range channels {
		moduleID := "vivy/" + name
		if config.ValidEnvKey(envelope.TokenEnv) {
			scopes[moduleID] = append(scopes[moduleID], envelope.TokenEnv)
		}
		node := envelope.Settings
		if node.Kind != yaml.MappingNode {
			continue
		}
		for index := 0; index+1 < len(node.Content); index += 2 {
			key, value := node.Content[index], node.Content[index+1]
			if !strings.HasSuffix(key.Value, "_env") || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || !config.ValidEnvKey(value.Value) {
				continue
			}
			scopes[moduleID] = append(scopes[moduleID], value.Value)
		}
	}
	return scopes
}

func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   []module.PortRef{{Port: Port, ID: "vivy.credential-resolver"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
}
func (ownerModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return ownerInstance{}, nil
}

type ownerInstance struct{}

func (ownerInstance) Start(context.Context) error { return nil }
func (ownerInstance) Ready(context.Context) error { return nil }
func (ownerInstance) Stop(context.Context) error  { return nil }
func (ownerInstance) Close(context.Context) error { return nil }

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
