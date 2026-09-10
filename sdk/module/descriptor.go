// Package module defines the pure-data contracts shared by the v1 SDK and
// Assembly Compiler. Descriptors describe build inputs; they never execute
// initialization or assign their own trust or support state.
package module

import (
	"fmt"
	"regexp"
	"strings"
)

const APIVersionV1 = "vivy.module/v1"

var (
	moduleIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?/[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
	semverPattern   = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	sha256Pattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	portPattern     = regexp.MustCompile(`^(?:std|core)/[a-z0-9]+(?:[/-][a-z0-9]+)*@v[1-9][0-9]*$`)
)

// Descriptor is the canonical side-effect-free declaration for one Module.
type Descriptor struct {
	APIVersion      string        `json:"apiVersion" yaml:"apiVersion"`
	Module          Identity      `json:"module" yaml:"module"`
	Source          Source        `json:"source" yaml:"source"`
	Provides        []PortRef     `json:"provides" yaml:"provides"`
	Requires        []Requirement `json:"requires" yaml:"requires"`
	Optional        []Requirement `json:"optional,omitempty" yaml:"optional,omitempty"`
	Conflicts       []Conflict    `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
	RequestedGrants []Grant       `json:"requestedGrants" yaml:"requestedGrants"`
	I18N            *I18N         `json:"i18n,omitempty" yaml:"i18n,omitempty"`
	Lifecycle       Lifecycle     `json:"lifecycle" yaml:"lifecycle"`
}

type Identity struct {
	ID      string `json:"id" yaml:"id"`
	Version string `json:"version" yaml:"version"`
}

type Source struct {
	Ref    string `json:"ref" yaml:"ref"`
	SHA256 string `json:"sha256" yaml:"sha256"`
}

// PortRef names a versioned Port and, for a provided capability, its stable
// namespaced provider identity.
type PortRef struct {
	Port string `json:"port" yaml:"port"`
	ID   string `json:"id,omitempty" yaml:"id,omitempty"`
}

type Requirement struct {
	PortRef  `yaml:",inline"`
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
}

type Conflict struct {
	Module string `json:"module" yaml:"module"`
}

type Grant string

// GrantBinding is the exact compiler-approved runtime capability. Constraints
// remain attached so focused Hosts can enforce the same decision that Inspect
// reports instead of degrading it to a boolean.
type GrantBinding struct {
	Name        Grant
	Constraints map[string][]string
}

const (
	GrantFSRead         Grant = "fs.read"
	GrantFSWrite        Grant = "fs.write"
	GrantChannelPoll    Grant = "channel.poll"
	GrantChannelWebhook Grant = "channel.webhook"
	GrantChannelListen  Grant = "channel.listen"
	GrantChannelA2A     Grant = "channel.a2a"
	GrantSecretRead     Grant = "secret.read"
	GrantProcSpawn      Grant = "proc.spawn"
	GrantTTY            Grant = "tty"
	GrantArgv           Grant = "argv"
	GrantRPCClient      Grant = "rpc.client"
	GrantNetClient      Grant = "net.client"
)

func (grant Grant) Valid() bool {
	switch grant {
	case GrantFSRead, GrantFSWrite,
		GrantChannelPoll, GrantChannelWebhook, GrantChannelListen, GrantChannelA2A,
		GrantSecretRead, GrantProcSpawn, GrantTTY, GrantArgv, GrantRPCClient, GrantNetClient:
		return true
	default:
		return false
	}
}

// Validate checks only Descriptor-local facts. Trust, Port support, source
// availability, graph resolution, and effective Grants remain compiler-owned.
func (descriptor Descriptor) Validate() error {
	if descriptor.APIVersion != APIVersionV1 {
		return fmt.Errorf("unsupported apiVersion %s (want %s)", descriptor.APIVersion, APIVersionV1)
	}
	if descriptor.Module.ID == "" {
		return fmt.Errorf("module.id is required")
	}
	if !moduleIDPattern.MatchString(descriptor.Module.ID) {
		return fmt.Errorf("module.id must be lowercase and namespace-qualified: %q", descriptor.Module.ID)
	}
	if !semverPattern.MatchString(descriptor.Module.Version) {
		return fmt.Errorf("module.version is not semantic versioning: %q", descriptor.Module.Version)
	}
	if strings.TrimSpace(descriptor.Source.Ref) == "" {
		return fmt.Errorf("source.ref is required for %s", descriptor.Module.ID)
	}
	if !sha256Pattern.MatchString(descriptor.Source.SHA256) {
		return fmt.Errorf("invalid source sha256 for %s", descriptor.Module.ID)
	}
	if err := validatePortRefs("provides", descriptor.Provides); err != nil {
		return err
	}
	for _, provided := range descriptor.Provides {
		if strings.TrimSpace(provided.ID) == "" {
			return fmt.Errorf("provides id is required for %s", provided.Port)
		}
	}
	if err := validateRequirements("requires", descriptor.Requires); err != nil {
		return err
	}
	if err := validateRequirements("optional", descriptor.Optional); err != nil {
		return err
	}
	seenConflicts := make(map[string]struct{}, len(descriptor.Conflicts))
	for _, conflict := range descriptor.Conflicts {
		if !moduleIDPattern.MatchString(conflict.Module) {
			return fmt.Errorf("invalid conflict module %q", conflict.Module)
		}
		if _, exists := seenConflicts[conflict.Module]; exists {
			return fmt.Errorf("duplicate conflict module %s", conflict.Module)
		}
		seenConflicts[conflict.Module] = struct{}{}
	}
	seenGrants := make(map[Grant]struct{}, len(descriptor.RequestedGrants))
	for _, grant := range descriptor.RequestedGrants {
		if !grant.Valid() {
			return fmt.Errorf("unknown requested grant %q", grant)
		}
		if _, exists := seenGrants[grant]; exists {
			return fmt.Errorf("duplicate requested grant %q", grant)
		}
		seenGrants[grant] = struct{}{}
	}
	if descriptor.I18N == nil {
		if descriptor.providesUI() {
			return fmt.Errorf("i18n is required for UI Module %s", descriptor.Module.ID)
		}
	} else if err := descriptor.I18N.Validate(); err != nil {
		return err
	}
	return descriptor.Lifecycle.Validate()
}

func (descriptor Descriptor) providesUI() bool {
	for _, provided := range descriptor.Provides {
		if provided.Port == "std/ui-extension@v1" || provided.Port == "std/ui-root@v1" {
			return true
		}
	}
	return false
}

func validatePortRefs(field string, refs []PortRef) error {
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !portPattern.MatchString(ref.Port) {
			return fmt.Errorf("%s port %q is not a versioned Port", field, ref.Port)
		}
		key := ref.Port + "\x00" + ref.ID
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate %s port %s id %s", field, ref.Port, ref.ID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateRequirements(field string, requirements []Requirement) error {
	refs := make([]PortRef, 0, len(requirements))
	for _, requirement := range requirements {
		if requirement.Provider != "" && !moduleIDPattern.MatchString(requirement.Provider) {
			return fmt.Errorf("invalid %s provider module %q", field, requirement.Provider)
		}
		refs = append(refs, requirement.PortRef)
	}
	return validatePortRefs(field, refs)
}
