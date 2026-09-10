// Package defaults declares the build-owned T1 Source Catalog for the
// established Vivy body.
package defaults

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/module"
)

type Binding struct {
	ImportPath, Package, Constructor, ProviderConstructor string
	ProviderCollection                                    bool
}
type Record struct {
	Descriptor module.Descriptor
	Binding    Binding
}

func Catalog(repoRoot string) ([]Record, error) {
	digest, err := hashTree(filepath.Join(repoRoot, "internal"))
	if err != nil {
		return nil, fmt.Errorf("default Source Catalog: %w", err)
	}
	source := module.Source{Ref: "file:internal", SHA256: digest}
	protectedPorts := make([]module.PortRef, 0, len(tools.AssemblyControlledToolNames()))
	for _, id := range tools.AssemblyControlledToolNames() {
		protectedPorts = append(protectedPorts, port("std/tool@v1", id))
	}
	records := []Record{
		record("vivy/kernel", "NewKernel", source, port("core/loop-driver@v1", "vivy.loop-driver"), port("core/chat-model-host@v1", "vivy.chat-model-host"), port("core/storage-engine@v1", "vivy.storage-engine"), port("core/checkpoint-store@v1", "vivy.checkpoint-store"), port("core/credential-resolver@v1", "vivy.credential-resolver"), port("core/sandbox-backend@v1", "vivy.sandbox-backend")),
		record("vivy/tool-host", "NewToolHost", source, port("core/tool-host@v1", "vivy.tool-host")),
		record("vivy/protected-tools", "NewProtectedTools", source, protectedPorts...),
		record("vivy/mcp-host", "NewMCPHost", source, port("core/mcp-host@v1", "vivy.mcp-host"), port("std/tool-world@v1", "mcp")),
		record("vivy/channel-host", "NewChannelHost", source, port("core/channel-host@v1", "vivy.channel-host")),
		record("vivy/face-host", "NewFaceHost", source, port("core/face-host@v1", "vivy.face-host")),
	}
	for i := range records {
		switch records[i].Descriptor.Module.ID {
		case "vivy/protected-tools":
			records[i].Binding.ProviderConstructor = "ProtectedToolProviders"
			records[i].Binding.ProviderCollection = true
		case "vivy/mcp-host":
			records[i].Binding.ProviderConstructor = "NewMCPProvider"
		}
		if records[i].Descriptor.Module.ID == "vivy/protected-tools" || records[i].Descriptor.Module.ID == "vivy/mcp-host" {
			records[i].Descriptor.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: "vivy/tool-host"}}
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
func port(name, id string) module.PortRef { return module.PortRef{Port: name, ID: id} }
func hashTree(root string) (string, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(rel) == "generated/assembly/zz_default.go" {
			return nil
		}
		if e.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, filepath.ToSlash(rel))
		_, _ = h.Write([]byte{0})
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
