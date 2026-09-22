// Package defaults declares the build-owned T1 Source Catalog for the
// established Vivy body.
package defaults

import (
	"fmt"
	"path/filepath"
	"strings"

	maskmodule "agent-vivy/internal/modules/masks"
	"agent-vivy/internal/modules/optional"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/sourcehash"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/module"
)

type Binding struct {
	ImportPath, Package, Constructor, ProviderConstructor string
	ProviderCollection                                    bool
	MaskFactory                                           string
	ContextSourceProvider                                 bool
	SkillSourceProvider                                   bool
	MCPHostProvider                                       bool
}
type Record struct {
	Descriptor module.Descriptor
	Binding    Binding
}

func Catalog(repoRoot string) ([]Record, error) {
	digest, err := sourcehash.Tree(filepath.Join(repoRoot, "internal"), "")
	if err != nil {
		return nil, fmt.Errorf("default Source Catalog: %w", err)
	}
	source := module.Source{Ref: "file:internal", SHA256: digest}
	protectedPorts := make([]module.PortRef, 0, len(tools.AssemblyControlledToolNames()))
	for _, id := range tools.AssemblyControlledToolNames() {
		protectedPorts = append(protectedPorts, port("std/tool@v1", id))
	}
	maskProvides := []module.PortRef{port("core/mask-service@v1", "vivy.mask-service")}
	for _, actionID := range []string{
		maskmodule.ActionCatalogList,
		maskmodule.ActionCatalogGet,
		maskmodule.ActionCatalogCreate,
		maskmodule.ActionCatalogUpdate,
		maskmodule.ActionCatalogDelete,
		maskmodule.ActionSelectionGet,
		maskmodule.ActionSelectionSet,
	} {
		maskProvides = append(maskProvides, port("std/control-action@v1", actionID))
	}
	records := []Record{
		boundRecord("vivy/loop", "agent-vivy/internal/modules/loop", "loop", "NewModule", source, port("core/loop-driver@v1", "vivy.loop-driver")),
		boundRecord("vivy/model", "agent-vivy/internal/modules/model", "model", "NewModule", source, port("core/chat-model-host@v1", "vivy.chat-model-host")),
		boundRecord("vivy/storage", "agent-vivy/internal/modules/storage", "storage", "NewModule", source, port("core/storage-engine@v1", "vivy.storage-engine")),
		boundRecord("vivy/masks", "agent-vivy/internal/modules/masks", "masks", "NewModule", source, maskProvides...),
		boundRecord("vivy/checkpoint", "agent-vivy/internal/modules/checkpoint", "checkpoint", "NewModule", source, port("core/checkpoint-store@v1", "vivy.checkpoint-store")),
		boundRecord("vivy/credential", "agent-vivy/internal/modules/credential", "credential", "NewModule", source, port("core/credential-resolver@v1", "vivy.credential-resolver")),
		boundRecord("vivy/sandbox", "agent-vivy/internal/modules/sandbox", "sandbox", "NewModule", source, port("core/sandbox-backend@v1", "vivy.sandbox-backend")),
		record("vivy/protected-tools", "NewProtectedTools", source, protectedPorts...),
		record("vivy/context-source", "NewContextSource", source, port("std/context-source@v1", "vivy.project-context")),
		record("vivy/skill-source", "NewSkillSource", source, port("std/skill-source@v1", "vivy.default-skills")),
		record("vivy/channel-host", "NewChannelHost", source, port("core/channel-host@v1", "vivy.channel-host")),
		record("vivy/face-host", "NewFaceHost", source, port("core/face-host@v1", "vivy.face-host")),
		record("vivy/provider-profiles", "NewProviderProfiles", source, port("std/provider-profile@v1", provider.AdapterOpenAICompletions), port("std/provider-profile@v1", provider.AdapterOpenAIResponses), port("std/provider-profile@v1", provider.AdapterAnthropicMessages)),
	}
	for _, host := range optional.Catalog() {
		provided := []module.PortRef{port(host.Port, "vivy."+strings.TrimPrefix(host.ModuleID, "vivy/"))}
		if host.ModuleID == "vivy/mcp-host" {
			provided = append(provided, port("std/tool-world@v1", "mcp"))
		}
		records = append(records, record(host.ModuleID, host.Constructor, source, provided...))
	}
	for i := range records {
		switch records[i].Descriptor.Module.ID {
		case "vivy/masks":
			records[i].Binding.MaskFactory = "Open"
			records[i].Binding.ProviderConstructor = "ActionProviders"
			records[i].Binding.ProviderCollection = true
		case "vivy/protected-tools":
			records[i].Binding.ProviderConstructor = "ProtectedToolProviders"
			records[i].Binding.ProviderCollection = true
		case "vivy/mcp-host":
			records[i].Binding.ProviderConstructor = "NewMCPProvider"
			records[i].Binding.MCPHostProvider = true
		case "vivy/provider-profiles":
			records[i].Binding.ProviderConstructor = "ProviderProfiles"
			records[i].Binding.ProviderCollection = true
		case "vivy/context-source":
			records[i].Binding.ProviderConstructor = "ContextSourceProviders"
			records[i].Binding.ProviderCollection = true
			records[i].Binding.ContextSourceProvider = true
		case "vivy/skill-source":
			records[i].Binding.ProviderConstructor = "SkillSourceProviders"
			records[i].Binding.ProviderCollection = true
			records[i].Binding.SkillSourceProvider = true
		}
		switch records[i].Descriptor.Module.ID {
		case "vivy/protected-tools", "vivy/mcp-host":
			records[i].Descriptor.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}}
		case "vivy/context-source":
			records[i].Descriptor.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/context-host@v1"}, Provider: "vivy/context-host"}}
		case "vivy/skill-source":
			records[i].Descriptor.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/skill-host@v1"}, Provider: "vivy/skill-host"}}
		}
		if records[i].Descriptor.Module.ID == "vivy/provider-profiles" {
			records[i].Descriptor.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/chat-model-host@v1"}, Provider: "vivy/model"}}
		}
		if err := records[i].Descriptor.Validate(); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func record(id, constructor string, source module.Source, provides ...module.PortRef) Record {
	return Record{Descriptor: module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: id, Version: "1.0.0"}, Source: source, Provides: provides, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}, Binding: Binding{ImportPath: "agent-vivy/internal/modules/defaults", Package: "defaults", Constructor: constructor}}
}
func boundRecord(id, importPath, packageName, constructor string, source module.Source, provides ...module.PortRef) Record {
	record := record(id, constructor, source, provides...)
	record.Binding.ImportPath = importPath
	record.Binding.Package = packageName
	return record
}
func port(name, id string) module.PortRef { return module.PortRef{Port: name, ID: id} }
