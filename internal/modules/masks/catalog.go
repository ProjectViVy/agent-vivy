// Package masks owns the immutable built-in mask catalog and its prompt
// framing assets. Custom definitions are supplied by Core Storage in later
// seams; this package never scans a directory or writes a catalog row.
package masks

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/maskcontract"
)

const DefaultGenerationID = "mask-catalog/1"

// The only runtime asset input is this package-owned embedded filesystem.
// Production code never accepts an external path for the built-in catalog.
//
//go:embed prompts/*.md
var embeddedAssets embed.FS

type Catalog struct {
	generationID string
	definitions  []maskcontract.Definition
	frame        string
	frameDigest  string
}

type builtinSpec struct {
	id          string
	name        string
	description string
	path        string
}

var builtinSpecs = []builtinSpec{
	{
		id:          maskcontract.BuiltinProgrammerID,
		name:        "Programmer",
		description: "Scoped coding help with tests and clear changes.",
		path:        "prompts/programmer.md",
	},
	{
		id:          maskcontract.BuiltinResearcherID,
		name:        "Researcher",
		description: "Evidence first; separate findings, inference, and uncertainty.",
		path:        "prompts/researcher.md",
	},
	{
		id:          maskcontract.BuiltinWriterID,
		name:        "Writer",
		description: "Audience and format aware writing grounded in supplied facts.",
		path:        "prompts/writer.md",
	},
}

// NewCatalog constructs the pure built-in catalog from the package's embedded
// bytes. An empty generation ID uses the local catalog default; a compiled
// Assembly may provide its Generation identity explicitly.
func NewCatalog(generationID string) (Catalog, error) {
	return loadCatalog(embeddedAssets, generationID)
}

// loadCatalog is a deterministic filesystem seam used by same-package tests
// to prove that missing assets and duplicate IDs fail closed. Runtime callers
// use NewCatalog, whose filesystem is the immutable embedded asset set.
func loadCatalog(assets fs.FS, generationID string) (Catalog, error) {
	generationID = strings.TrimSpace(generationID)
	if generationID == "" {
		generationID = DefaultGenerationID
	}
	if !validGenerationID(generationID) {
		return Catalog{}, errors.New("mask catalog: invalid generation id")
	}
	frameBytes, err := fs.ReadFile(assets, "prompts/mask-frame.md")
	if err != nil {
		return Catalog{}, fmt.Errorf("mask catalog: read mask frame: %w", err)
	}
	frame := string(frameBytes)
	if err := validateAssetBody(frame, "mask frame"); err != nil {
		return Catalog{}, err
	}
	return loadCatalogDefinitions(assets, generationID, frameBytes, builtinSpecs)
}

func loadCatalogDefinitions(assets fs.FS, generationID string, frameBytes []byte, specs []builtinSpec) (Catalog, error) {
	definitions := make([]maskcontract.Definition, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if _, exists := seen[spec.id]; exists {
			return Catalog{}, fmt.Errorf("mask catalog: duplicate built-in id %q", spec.id)
		}
		seen[spec.id] = struct{}{}
		bodyBytes, err := fs.ReadFile(assets, spec.path)
		if err != nil {
			return Catalog{}, fmt.Errorf("mask catalog: read %s: %w", spec.path, err)
		}
		body := string(bodyBytes)
		if err := validateAssetBody(body, spec.name+" body"); err != nil {
			return Catalog{}, err
		}
		definition := maskcontract.Definition{
			ID:           spec.id,
			Name:         spec.name,
			Description:  spec.description,
			Body:         body,
			Revision:     maskcontract.BuiltinRevision,
			Digest:       maskcontract.DefinitionDigest(spec.id, spec.name, body),
			BuiltIn:      true,
			GenerationID: generationID,
		}
		if err := maskcontract.ValidateDefinition(definition); err != nil {
			return Catalog{}, fmt.Errorf("mask catalog: validate %s: %w", spec.id, err)
		}
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].ID < definitions[j].ID })

	frameSum := sha256.Sum256(frameBytes)
	return Catalog{
		generationID: generationID,
		definitions:  definitions,
		frame:        string(frameBytes),
		frameDigest:  hex.EncodeToString(frameSum[:]),
	}, nil
}

// Definitions returns owned copies in lexical ID order.
func (c Catalog) Definitions() []maskcontract.Definition {
	return append([]maskcontract.Definition(nil), c.definitions...)
}

// List is a short alias used by catalog consumers that do not need to
// distinguish the immutable built-in set from a later merged catalog.
func (c Catalog) List() []maskcontract.Definition {
	return c.Definitions()
}

// Get returns one built-in definition and an existence bit. The definition is
// a value, so a caller cannot mutate the catalog through this method.
func (c Catalog) Get(id string) (maskcontract.Definition, bool) {
	for _, definition := range c.definitions {
		if definition.ID == id {
			return definition, true
		}
	}
	return maskcontract.Definition{}, false
}

// Resolve is the pure built-in resolver used by the provider service. Custom
// IDs and unknown/reserved IDs are not resolved here.
func (c Catalog) Resolve(ctx context.Context, id string) (maskcontract.Definition, error) {
	if err := ctx.Err(); err != nil {
		return maskcontract.Definition{}, err
	}
	definition, ok := c.Get(id)
	if !ok {
		return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeNotFound, nil)
	}
	return definition, nil
}

// Snapshot resolves a built-in and binds its immutable definition to a
// session selection revision for a run capture.
func (c Catalog) Snapshot(ctx context.Context, id string, selectionRevision int64) (maskcontract.Snapshot, error) {
	if selectionRevision < 0 {
		return maskcontract.Snapshot{}, maskcontract.NewError(maskcontract.CodeInvalidMask, errors.New("selection revision must not be negative"))
	}
	definition, err := c.Resolve(ctx, id)
	if err != nil {
		return maskcontract.Snapshot{}, err
	}
	return maskcontract.Snapshot{
		ID:                 definition.ID,
		Name:               definition.Name,
		Body:               definition.Body,
		Digest:             definition.Digest,
		DefinitionRevision: definition.Revision,
		SelectionRevision:  selectionRevision,
		GenerationID:       definition.GenerationID,
	}, nil
}

// PromptAssets returns the immutable mask framing text and its SHA-256 digest.
func (c Catalog) PromptAssets() (frame string, digest string) {
	return c.frame, c.frameDigest
}

func (c Catalog) GenerationID() string { return c.generationID }

func validateAssetBody(value, label string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("mask catalog: %s is not valid UTF-8", label)
	}
	operation := maskcontract.CreateRequest{
		OperationID: "00000000-0000-4000-8000-000000000001",
		Name:        "asset",
		Body:        value,
	}
	if _, err := maskcontract.NormalizeCreate(operation); err != nil {
		return fmt.Errorf("mask catalog: invalid %s: %w", label, err)
	}
	return nil
}

func validGenerationID(value string) bool {
	if len(value) == 0 || len(value) > maskcontract.MaxIDBytes {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] >= utf8.RuneSelf || value[index] < 0x20 || value[index] == 0x7f {
			return false
		}
	}
	return true
}
