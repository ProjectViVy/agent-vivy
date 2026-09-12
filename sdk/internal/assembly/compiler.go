// Package assembly compiles build-time Module selections into deterministic
// typed Assembly plans. It performs no runtime discovery or activation.
package assembly

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

const RecipeAPIVersionV1 = "vivy.generation/v1"

type Recipe struct {
	APIVersion     string                   `json:"apiVersion" yaml:"apiVersion"`
	Profile        string                   `json:"profile,omitempty" yaml:"profile,omitempty"`
	Modules        []string                 `json:"modules" yaml:"modules"`
	Sources        map[string]module.Source `json:"sources,omitempty" yaml:"sources,omitempty"`
	Exclusive      map[string]string        `json:"exclusive,omitempty" yaml:"exclusive,omitempty"`
	Order          map[string][]string      `json:"order,omitempty" yaml:"order,omitempty"`
	GrantApprovals []GrantApproval          `json:"grantApprovals,omitempty" yaml:"grantApprovals,omitempty"`
	UI             *UIAssemblyInput         `json:"ui,omitempty" yaml:"ui,omitempty"`
}

type ResolvedModule struct {
	Descriptor      module.Descriptor
	Trust           Trust
	Binding         GoBinding
	EffectiveGrants []EffectiveGrant
}

type PortEdge struct {
	Port     module.PortRef
	Provider string
	Consumer string
}

type AssemblyPlan struct {
	Modules              []ResolvedModule
	PortEdges            []PortEdge
	LifecycleOrder       []string
	OrderedContributions map[string][]string
}

type Compiler struct {
	Ports        port.Catalog
	Sources      SourceCatalog
	PortEvidence map[string]port.SupportEvidence
}

type diagnosticsError struct {
	diagnostics []string
}

func (err diagnosticsError) Error() string {
	return strings.Join(err.diagnostics, "; ")
}

func (c Compiler) Compile(ctx context.Context, recipe Recipe) (AssemblyPlan, error) {
	if err := ctx.Err(); err != nil {
		return AssemblyPlan{}, err
	}
	diagnostics := make([]string, 0)
	if recipe.APIVersion != RecipeAPIVersionV1 {
		diagnostics = append(diagnostics, fmt.Sprintf("unsupported recipe apiVersion %s (want %s)", recipe.APIVersion, RecipeAPIVersionV1))
	}

	selected := make(map[string]SourceRecord, len(recipe.Modules))
	resolved := make([]ResolvedModule, 0, len(recipe.Modules))
	for _, moduleID := range recipe.Modules {
		if err := ctx.Err(); err != nil {
			return AssemblyPlan{}, err
		}
		if _, exists := selected[moduleID]; exists {
			diagnostics = append(diagnostics, "duplicate module "+moduleID)
			continue
		}
		record, err := c.Sources.Resolve(moduleID)
		if err != nil {
			diagnostics = append(diagnostics, err.Error())
			continue
		}
		if record.Trust == TrustT2 && record.Root != "" {
			pin, ok := recipe.Sources[moduleID]
			if !ok {
				diagnostics = append(diagnostics, "missing authoritative source pin for "+moduleID)
			} else if pin.Ref != record.Ref || pin != record.Descriptor.Source {
				diagnostics = append(diagnostics, "source pin mismatch for "+moduleID)
			}
		}
		selected[moduleID] = record
		resolved = append(resolved, ResolvedModule{Descriptor: record.Descriptor, Trust: record.Trust, Binding: record.Binding})
		if err := record.Descriptor.Validate(); err != nil {
			diagnostics = append(diagnostics, err.Error())
		}
	}
	for moduleID := range recipe.Sources {
		if _, exists := selected[moduleID]; !exists {
			diagnostics = append(diagnostics, "source pin names unselected module "+moduleID)
		}
	}
	providers := make(map[string][]SourceRecord)
	providerIDs := make(map[string]string)
	for _, record := range selected {
		for _, provided := range record.Descriptor.Provides {
			providers[provided.Port] = append(providers[provided.Port], record)
			key := provided.Port + "\x00" + provided.ID
			if owner, duplicate := providerIDs[key]; duplicate {
				diagnostics = append(diagnostics, fmt.Sprintf("duplicate provider id %s for %s in %s and %s", provided.ID, provided.Port, owner, record.Descriptor.Module.ID))
			} else {
				providerIDs[key] = record.Descriptor.Module.ID
			}
			if record.Trust == TrustT2 && !strings.HasPrefix(provided.ID, strings.Split(record.Descriptor.Module.ID, "/")[0]+".") {
				diagnostics = append(diagnostics, fmt.Sprintf("T2 provider id %s for %s is outside module namespace %s", provided.ID, provided.Port, record.Descriptor.Module.ID))
			}
			if strings.HasPrefix(provided.Port, "core/") {
				if err := validateInternalProvider(provided, record.Descriptor.Module.ID, record.Trust); err != nil {
					diagnostics = append(diagnostics, err.Error())
				}
			}
			if provided.Port == "std/tool@v1" && record.Trust == TrustT2 && protectedToolIDs[provided.ID] {
				diagnostics = append(diagnostics, fmt.Sprintf("T2 module %s cannot claim protected Tool id %s", record.Descriptor.Module.ID, provided.ID))
			}
			if provided.Port == "std/tool-world@v1" && provided.ID == "mcp" {
				if !record.Binding.MCPHostProvider {
					diagnostics = append(diagnostics, fmt.Sprintf("MCP ToolWorld mcp requires typed MCPHostProvider binding on %s", record.Descriptor.Module.ID))
				}
				if !descriptorProvides(record.Descriptor, "core/mcp-host@v1") {
					diagnostics = append(diagnostics, fmt.Sprintf("MCP ToolWorld mcp requires core/mcp-host@v1 from %s", record.Descriptor.Module.ID))
				}
			}
		}
	}
	for _, record := range selected {
		if !record.Binding.MCPHostProvider {
			continue
		}
		if !descriptorProvides(record.Descriptor, "std/tool-world@v1") || !descriptorProvidesRef(record.Descriptor, module.PortRef{Port: "std/tool-world@v1", ID: "mcp"}) {
			diagnostics = append(diagnostics, fmt.Sprintf("typed MCPHostProvider %s must bind std/tool-world@v1 id mcp", record.Descriptor.Module.ID))
		}
		if !descriptorProvides(record.Descriptor, "core/mcp-host@v1") {
			diagnostics = append(diagnostics, fmt.Sprintf("typed MCPHostProvider %s requires core/mcp-host@v1", record.Descriptor.Module.ID))
		}
	}
	for portName, records := range providers {
		if strings.HasPrefix(portName, "core/") {
			continue
		}
		definition, ok := c.Ports.Lookup(module.PortRef{Port: portName})
		if !ok {
			diagnostics = append(diagnostics, "unknown Port "+portName)
			continue
		}
		if definition.Cardinality == port.CardinalityExclusive && len(records) > 1 {
			diagnostics = append(diagnostics, "duplicate provider for "+portName)
		}
		if definition.Cardinality == port.CardinalityExclusive && len(records) == 1 {
			selectedProvider, selectedExplicitly := recipe.Exclusive[portName]
			if !selectedExplicitly {
				diagnostics = append(diagnostics, "missing exclusive selection for "+portName)
			} else if selectedProvider != records[0].Descriptor.Module.ID {
				diagnostics = append(diagnostics, fmt.Sprintf("exclusive selection %s does not provide %s", selectedProvider, portName))
			}
		}
	}
	for portName, selectedProvider := range recipe.Exclusive {
		definition, ok := c.Ports.Lookup(module.PortRef{Port: portName})
		if !ok {
			diagnostics = append(diagnostics, "unknown exclusive Port "+portName)
			continue
		}
		if definition.Cardinality != port.CardinalityExclusive {
			diagnostics = append(diagnostics, "Port "+portName+" is not exclusive")
			continue
		}
		record, exists := selected[selectedProvider]
		if !exists || !descriptorProvides(record.Descriptor, portName) {
			diagnostics = append(diagnostics, fmt.Sprintf("unresolved exclusive selection %s for %s", selectedProvider, portName))
		}
	}

	moduleIDs := make([]string, 0, len(selected))
	for moduleID := range selected {
		moduleIDs = append(moduleIDs, moduleID)
	}
	graph := newDependencyGraph(moduleIDs)
	edges := make([]PortEdge, 0)
	used := make(map[string]struct{})
	for _, consumerID := range sortedRecordIDs(selected) {
		record := selected[consumerID]
		for _, requirement := range record.Descriptor.Requires {
			c.compileRequirement(requirement, consumerID, selected, graph, &edges, used, &diagnostics)
		}
		for _, requirement := range record.Descriptor.Optional {
			if _, exists := selected[requirement.Provider]; requirement.Provider != "" && exists {
				c.compileRequirement(requirement, consumerID, selected, graph, &edges, used, &diagnostics)
			}
		}
		for _, after := range record.Descriptor.Lifecycle.After {
			if _, exists := selected[after]; !exists {
				diagnostics = append(diagnostics, fmt.Sprintf("unresolved lifecycle.after %s for %s", after, consumerID))
				continue
			}
			graph.addEdge(after, consumerID)
		}
		for _, conflict := range record.Descriptor.Conflicts {
			if _, exists := selected[conflict.Module]; exists {
				diagnostics = append(diagnostics, fmt.Sprintf("module conflict: %s conflicts with %s", consumerID, conflict.Module))
			}
		}
		for _, provided := range record.Descriptor.Provides {
			if strings.HasPrefix(provided.Port, "core/") {
				continue
			}
			definition, ok := c.Ports.Lookup(provided)
			if !ok {
				continue
			}
			hostID := ""
			for _, requirement := range record.Descriptor.Requires {
				if requirement.Port == definition.Consumer.Port && (definition.Consumer.ID == "" || requirement.ID == definition.Consumer.ID) {
					hostID = requirement.Provider
					break
				}
			}
			if hostID != "" {
				used[providerKey(consumerID, provided.Port)] = struct{}{}
				edges = append(edges, PortEdge{Port: provided, Provider: consumerID, Consumer: hostID})
			}
		}
	}
	markSelectedUIProviders(recipe.UI, selected, providers, used, &diagnostics)
	markSelectedControlActionProviders(selected, providers, used)

	for portName, records := range providers {
		if strings.HasPrefix(portName, "core/") {
			continue
		}
		for _, record := range records {
			key := providerKey(record.Descriptor.Module.ID, portName)
			if _, exists := used[key]; !exists {
				diagnostics = append(diagnostics, fmt.Sprintf("unused provider %s for %s", record.Descriptor.Module.ID, portName))
			}
		}
	}

	ordered := make(map[string][]string, len(recipe.Order))
	for portName, records := range providers {
		definition, ok := c.Ports.Lookup(module.PortRef{Port: portName})
		if !ok || !definition.Ordered {
			continue
		}
		sequence, exists := recipe.Order[portName]
		if !exists {
			diagnostics = append(diagnostics, "missing order for "+portName)
			continue
		}
		listed := make(map[string]struct{}, len(sequence))
		for _, moduleID := range sequence {
			listed[moduleID] = struct{}{}
		}
		for _, record := range records {
			moduleID := record.Descriptor.Module.ID
			if _, exists := listed[moduleID]; !exists {
				diagnostics = append(diagnostics, fmt.Sprintf("missing order edge %s for %s", moduleID, portName))
			}
		}
	}
	for portName, sequence := range recipe.Order {
		definition, ok := c.Ports.Lookup(module.PortRef{Port: portName})
		if !ok {
			diagnostics = append(diagnostics, "unknown ordered Port "+portName)
			continue
		}
		if !definition.Ordered {
			diagnostics = append(diagnostics, "Port "+portName+" is not ordered")
			continue
		}
		seen := make(map[string]struct{}, len(sequence))
		for _, providerID := range sequence {
			record, exists := selected[providerID]
			if !exists || !descriptorProvides(record.Descriptor, portName) {
				diagnostics = append(diagnostics, fmt.Sprintf("unresolved order edge %s for %s", providerID, portName))
				continue
			}
			if _, duplicate := seen[providerID]; duplicate {
				diagnostics = append(diagnostics, fmt.Sprintf("duplicate order edge %s for %s", providerID, portName))
				continue
			}
			seen[providerID] = struct{}{}
		}
		ordered[portName] = append([]string(nil), sequence...)
	}

	for portName := range providers {
		if strings.HasPrefix(portName, "core/") {
			continue
		}
		if err := c.Ports.RequireSelectable(module.PortRef{Port: portName}, c.PortEvidence[portName]); err != nil {
			diagnostics = append(diagnostics, err.Error())
		}
	}
	for index := range resolved {
		allowed := c.allowedGrants(resolved[index].Descriptor)
		effective, err := calculateEffectiveGrants(
			resolved[index].Descriptor.Module.ID,
			resolved[index].Descriptor.RequestedGrants,
			allowed,
			resolved[index].Trust,
			recipe.GrantApprovals,
		)
		if err != nil {
			diagnostics = append(diagnostics, err.Error())
			continue
		}
		resolved[index].EffectiveGrants = effective
	}
	for _, record := range selected {
		for _, requirement := range append(append([]module.Requirement(nil), record.Descriptor.Requires...), record.Descriptor.Optional...) {
			if strings.HasPrefix(requirement.Port, "core/") {
				continue
			}
			if err := c.Ports.RequireSelectable(requirement.PortRef, c.PortEvidence[requirement.Port]); err != nil {
				diagnostics = append(diagnostics, err.Error())
			}
		}
	}

	lifecycleOrder, graphErr := graph.order()
	if graphErr != nil {
		diagnostics = append(diagnostics, graphErr.Error())
	}
	if len(diagnostics) > 0 {
		return AssemblyPlan{}, newDiagnosticsError(diagnostics)
	}
	sort.Slice(edges, func(i, j int) bool {
		left := edges[i].Port.Port + "\x00" + edges[i].Provider + "\x00" + edges[i].Consumer
		right := edges[j].Port.Port + "\x00" + edges[j].Provider + "\x00" + edges[j].Consumer
		return left < right
	})
	return AssemblyPlan{Modules: resolved, PortEdges: edges, LifecycleOrder: lifecycleOrder, OrderedContributions: ordered}, nil
}

// markSelectedUIProviders binds the compiler's explicit UI recipe projection
// to the frontend PresentationHost. Unlike backend Ports, the UI consumer is
// not a Go Module in the generated lifecycle graph, so its selection is
// recorded as a used provider without inventing a synthetic runtime edge.
func markSelectedUIProviders(input *UIAssemblyInput, selected map[string]SourceRecord, providers map[string][]SourceRecord, used map[string]struct{}, diagnostics *[]string) {
	if input == nil {
		return
	}
	contributions := make([]UIModule, 0, len(input.Roots)+len(input.Extensions)+1)
	contributions = append(contributions, input.Roots...)
	if uiModulePresent(input.Root) {
		contributions = append(contributions, input.Root)
	}
	contributions = append(contributions, input.Extensions...)
	for _, contribution := range contributions {
		moduleID := strings.TrimSpace(contribution.ModuleID)
		if moduleID == "" {
			moduleID = strings.TrimSpace(contribution.ID)
		}
		record, exists := selected[moduleID]
		if !exists {
			for _, candidate := range selected {
				if candidate.Descriptor.Module.ID == moduleID || descriptorProvidesRef(candidate.Descriptor, module.PortRef{Port: contribution.Port, ID: contribution.ID}) {
					record = candidate
					exists = true
					break
				}
			}
		}
		if !exists {
			*diagnostics = append(*diagnostics, fmt.Sprintf("UI Assembly provider %s is not selected in Recipe modules", contribution.ID))
			continue
		}
		portName := strings.TrimSpace(contribution.Port)
		if portName == "" {
			portName = UIRootPort
			for _, provided := range record.Descriptor.Provides {
				if provided.ID == contribution.ID && (provided.Port == UIExtensionPort || provided.Port == UIRootPort) {
					portName = provided.Port
					break
				}
			}
		}
		matched := false
		for _, provided := range providers[portName] {
			if provided.Descriptor.Module.ID == record.Descriptor.Module.ID && (contribution.ID == "" || descriptorProvidesRef(provided.Descriptor, module.PortRef{Port: portName, ID: contribution.ID})) {
				used[providerKey(record.Descriptor.Module.ID, portName)] = struct{}{}
				matched = true
				break
			}
		}
		if !matched {
			*diagnostics = append(*diagnostics, fmt.Sprintf("UI Assembly provider %s is not provided by selected module %s", contribution.ID, record.Descriptor.Module.ID))
		}
	}
}

// markSelectedControlActionProviders binds the compiler's implicit backend
// ActionHost consumer. Control Actions are invoked through the one kernel RPC
// and therefore have no hand-authored Module requirement edge; the selected
// provider still must be marked used so ordinary unused-provider diagnostics
// cannot be bypassed or accidentally reject a valid action set.
func markSelectedControlActionProviders(selected map[string]SourceRecord, providers map[string][]SourceRecord, used map[string]struct{}) {
	const controlActionPort = "std/control-action@v1"
	for _, record := range selected {
		for _, provided := range record.Descriptor.Provides {
			if provided.Port != controlActionPort {
				continue
			}
			for _, candidate := range providers[controlActionPort] {
				if candidate.Descriptor.Module.ID == record.Descriptor.Module.ID && candidate.Descriptor.Module.ID != "" {
					used[providerKey(record.Descriptor.Module.ID, controlActionPort)] = struct{}{}
					break
				}
			}
		}
	}
}

func (c Compiler) allowedGrants(descriptor module.Descriptor) []module.Grant {
	allowed := make(map[module.Grant]struct{})
	for _, provided := range descriptor.Provides {
		definition, ok := c.Ports.Lookup(provided)
		if !ok {
			continue
		}
		for _, grant := range definition.AllowedGrants {
			allowed[grant] = struct{}{}
		}
	}
	grants := make([]module.Grant, 0, len(allowed))
	for grant := range allowed {
		grants = append(grants, grant)
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i] < grants[j] })
	return grants
}

func (c Compiler) compileRequirement(
	requirement module.Requirement,
	consumerID string,
	selected map[string]SourceRecord,
	graph *dependencyGraph,
	edges *[]PortEdge,
	used map[string]struct{},
	diagnostics *[]string,
) {
	providerRecord, exists := selected[requirement.Provider]
	if requirement.Provider == "" || !exists || !descriptorProvidesRef(providerRecord.Descriptor, requirement.PortRef) {
		*diagnostics = append(*diagnostics, fmt.Sprintf("missing provider for %s: module %s requirement id %q names provider %q", requirement.Port, consumerID, requirement.ID, requirement.Provider))
		return
	}
	used[providerKey(requirement.Provider, requirement.Port)] = struct{}{}
	graph.addEdge(requirement.Provider, consumerID)
	*edges = append(*edges, PortEdge{Port: requirement.PortRef, Provider: requirement.Provider, Consumer: consumerID})
}

func descriptorProvidesRef(descriptor module.Descriptor, ref module.PortRef) bool {
	for _, provided := range descriptor.Provides {
		if provided.Port == ref.Port && (ref.ID == "" || provided.ID == ref.ID) {
			return true
		}
	}
	return false
}

var protectedToolIDs = map[string]bool{
	"ask_user": true, "list_dir": true, "read_file": true, "search_files": true,
	"write_file": true, "patch": true, "multiedit": true, "execute": true,
	"bash": true, "skills_list": true, "skill_view": true,
}

func descriptorProvides(descriptor module.Descriptor, portName string) bool {
	for _, provided := range descriptor.Provides {
		if provided.Port == portName {
			return true
		}
	}
	return false
}

func providerKey(moduleID, portName string) string {
	return moduleID + "\x00" + portName
}

func sortedRecordIDs(records map[string]SourceRecord) []string {
	ids := make([]string, 0, len(records))
	for id := range records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func newDiagnosticsError(diagnostics []string) error {
	sort.Strings(diagnostics)
	unique := diagnostics[:0]
	for _, diagnostic := range diagnostics {
		if len(unique) == 0 || unique[len(unique)-1] != diagnostic {
			unique = append(unique, diagnostic)
		}
	}
	return diagnosticsError{diagnostics: append([]string(nil), unique...)}
}
