package assembly

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-vivy/sdk/module"
)

// Trust is assigned by the build-owned Source Catalog, never by a Module.
type Trust string

const (
	TrustT1 Trust = "T1"
	TrustT2 Trust = "T2"
)

type SourceRecord struct {
	Descriptor module.Descriptor
	Trust      Trust
	Binding    GoBinding
	Root       string
	Ref        string
}

// GoBinding is build metadata owned by the Source Catalog. It is not part of
// a Module Descriptor and therefore cannot redirect its own generated import.
type GoBinding struct {
	ImportPath                   string
	Package                      string
	Constructor                  string
	ProviderConstructor          string
	ProviderCollection           bool
	DiagnosticObserver           bool
	LanguageServerStatusProvider bool
	PreToolProvider              bool
	RunObserverProvider          bool
	DiagnosticObserverV1Provider bool
	StatusProvider               bool
	// Typed P4 bindings keep Source/Host composition explicit in generated
	// Assembly. They are build metadata, never Module-controlled redirects.
	ContextSourceProvider bool
	SkillSourceProvider   bool
	MCPHostProvider       bool
}

// SourceCatalog is the sole authority that binds a Module ID to source bytes
// and an in-process trust classification.
type SourceCatalog struct {
	records map[string]SourceRecord
}

func NewSourceCatalog(records []SourceRecord) (SourceCatalog, error) {
	catalog := SourceCatalog{records: make(map[string]SourceRecord, len(records))}
	for _, record := range records {
		id := record.Descriptor.Module.ID
		if id == "" {
			return SourceCatalog{}, fmt.Errorf("source catalog module id is required")
		}
		if record.Trust != TrustT1 && record.Trust != TrustT2 {
			return SourceCatalog{}, fmt.Errorf("source catalog has invalid Trust %q for %s", record.Trust, id)
		}
		if record.Root != "" {
			if record.Ref == "" {
				return SourceCatalog{}, fmt.Errorf("source catalog ref is required for %s", id)
			}
			if record.Descriptor.Source.Ref != record.Ref {
				return SourceCatalog{}, fmt.Errorf("source ref mismatch for %s: got %s, want %s", id, record.Descriptor.Source.Ref, record.Ref)
			}
			digest, err := HashSourceTree(record.Root, record.Descriptor.Source.SHA256)
			if err != nil {
				return SourceCatalog{}, fmt.Errorf("verify source for %s: %w", id, err)
			}
			if digest != record.Descriptor.Source.SHA256 {
				return SourceCatalog{}, fmt.Errorf("source hash mismatch for %s: got %s, want %s", id, digest, record.Descriptor.Source.SHA256)
			}
		}
		if _, exists := catalog.records[id]; exists {
			return SourceCatalog{}, fmt.Errorf("ambiguous source for module %s", id)
		}
		catalog.records[id] = cloneSourceRecord(record)
	}
	return catalog, nil
}

// HashSourceTree hashes a deterministic path/content stream. The Descriptor's
// own declared digest is normalized so the source can carry its content
// address without introducing a circular hash dependency.
func HashSourceTree(root, declaredDigest string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source tree contains symbolic link %s", path)
		}
		if entry.Type().IsRegular() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			if filepath.ToSlash(rel) == "generated/assembly/zz_default.go" {
				return nil
			}
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if declaredDigest != "" {
			body = []byte(strings.ReplaceAll(string(body), declaredDigest, strings.Repeat("0", sha256.Size*2)))
		}
		_, _ = io.WriteString(h, filepath.ToSlash(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(body)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (catalog SourceCatalog) Resolve(moduleID string) (SourceRecord, error) {
	record, ok := catalog.records[moduleID]
	if !ok {
		return SourceRecord{}, fmt.Errorf("missing source for module %s", moduleID)
	}
	return cloneSourceRecord(record), nil
}

func cloneSourceRecord(record SourceRecord) SourceRecord {
	descriptor := record.Descriptor
	descriptor.Provides = append([]module.PortRef(nil), descriptor.Provides...)
	descriptor.Requires = append([]module.Requirement(nil), descriptor.Requires...)
	descriptor.Optional = append([]module.Requirement(nil), descriptor.Optional...)
	descriptor.Conflicts = append([]module.Conflict(nil), descriptor.Conflicts...)
	descriptor.RequestedGrants = append([]module.Grant(nil), descriptor.RequestedGrants...)
	descriptor.Lifecycle.After = append([]string(nil), descriptor.Lifecycle.After...)
	if descriptor.I18N != nil {
		catalog := *descriptor.I18N
		catalog.Locales = append([]string(nil), catalog.Locales...)
		descriptor.I18N = &catalog
	}
	record.Descriptor = descriptor
	return record
}
