package assembly

import (
	"fmt"

	"agent-vivy/internal/sourcehash"
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
	// RootlessFixture admits a T2 record without a source root for compiler test
	// fixtures only. Production pack operations must provide a root before
	// source provenance can be sealed.
	RootlessFixture bool
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
	// ContextSourceRequired is build-owned availability policy. It is sealed
	// into the Generation and cannot be selected by Provider code.
	ContextSourceRequired bool
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
	return sourcehash.Tree(root, declaredDigest)
}

// BindSourceHashes derives source provenance from the exact roots selected for
// a pack operation. Descriptors may carry a legacy digest for circular-content
// normalization, but that value is never trusted as an input or compared by
// the compiler.
func BindSourceHashes(plan *AssemblyPlan, catalog SourceCatalog) error {
	if plan == nil {
		return fmt.Errorf("source hash binding requires a plan")
	}
	for index := range plan.Modules {
		resolved := &plan.Modules[index]
		record, err := catalog.Resolve(resolved.Descriptor.Module.ID)
		if err != nil {
			return err
		}
		if record.Root == "" {
			return fmt.Errorf("source root is required to bind hash for %s", resolved.Descriptor.Module.ID)
		}
		digest, err := HashSourceTree(record.Root, record.Descriptor.Source.SHA256)
		if err != nil {
			return fmt.Errorf("hash source for %s: %w", resolved.Descriptor.Module.ID, err)
		}
		resolved.Descriptor.Source.SHA256 = digest
	}
	return nil
}

// VerifyBoundSourceHashes detects source mutation after pack provenance was
// bound. It is a pack consistency check, not a compile-time package gate.
func VerifyBoundSourceHashes(plan AssemblyPlan, catalog SourceCatalog) error {
	for _, resolved := range plan.Modules {
		record, err := catalog.Resolve(resolved.Descriptor.Module.ID)
		if err != nil {
			return err
		}
		if record.Root == "" {
			return fmt.Errorf("source root is required to verify hash for %s", resolved.Descriptor.Module.ID)
		}
		actual, err := HashSourceTree(record.Root, record.Descriptor.Source.SHA256)
		if err != nil {
			return fmt.Errorf("verify source for %s: %w", resolved.Descriptor.Module.ID, err)
		}
		if actual != resolved.Descriptor.Source.SHA256 {
			return fmt.Errorf("source changed during pack for %s: got %s, want %s", resolved.Descriptor.Module.ID, actual, resolved.Descriptor.Source.SHA256)
		}
	}
	return nil
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
