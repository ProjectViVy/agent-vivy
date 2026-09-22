package masks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"agent-vivy/internal/maskcontract"
)

func TestMaskCatalogEmbedsCanonicalBuiltIns(t *testing.T) {
	catalog, err := NewCatalog("generation-test")
	if err != nil {
		t.Fatal(err)
	}
	definitions := catalog.Definitions()
	if len(definitions) != 3 {
		t.Fatalf("built-in count = %d, want 3", len(definitions))
	}
	wantIDs := []string{
		maskcontract.BuiltinProgrammerID,
		maskcontract.BuiltinResearcherID,
		maskcontract.BuiltinWriterID,
	}
	for index, definition := range definitions {
		if definition.ID != wantIDs[index] || !definition.BuiltIn || definition.Revision != maskcontract.BuiltinRevision {
			t.Fatalf("definition[%d] = %+v", index, definition)
		}
		if definition.GenerationID != "generation-test" || definition.Digest != maskcontract.DefinitionDigest(definition.ID, definition.Name, definition.Body) {
			t.Fatalf("definition[%d] has invalid generation/digest: %+v", index, definition)
		}
		if definition.Body == "" || strings.TrimSpace(definition.Body) == "" {
			t.Fatalf("definition[%d] has empty body", index)
		}
	}
	for index := 1; index < len(definitions); index++ {
		if definitions[index-1].ID >= definitions[index].ID {
			t.Fatalf("catalog is not in lexical ID order: %+v", definitions)
		}
	}

	frame, digest := catalog.PromptAssets()
	sum := sha256.Sum256([]byte(frame))
	if digest != hex.EncodeToString(sum[:]) || frame == "" {
		t.Fatalf("prompt assets digest/frame mismatch: digest=%q frame=%q", digest, frame)
	}
	if !strings.Contains(frame, "Runtime rules") || !strings.Contains(frame, "explicit") || !strings.Contains(frame, "task requirements") {
		t.Fatalf("mask frame does not state authority/precedence: %q", frame)
	}
}

func TestMaskCatalogResolverAndOwnedReturns(t *testing.T) {
	catalog, err := NewCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	definitions := catalog.Definitions()
	original := definitions[0]
	definitions[0].Name = "mutated"
	definitions[0].Body = "mutated"
	got, ok := catalog.Get(original.ID)
	if !ok || got.Name != original.Name || got.Body != original.Body {
		t.Fatalf("caller mutation changed catalog: got %+v want %+v", got, original)
	}
	resolved, err := catalog.Resolve(context.Background(), original.ID)
	if err != nil || resolved != original {
		t.Fatalf("resolve = %+v, %v; want %+v", resolved, err, original)
	}
	if _, err := catalog.Resolve(context.Background(), "custom/00000000-0000-4000-8000-000000000001"); err == nil {
		t.Fatal("custom id unexpectedly resolved by built-in catalog")
	} else {
		var typed *maskcontract.Error
		if !errors.As(err, &typed) || typed.Code != maskcontract.CodeNotFound {
			t.Fatalf("unknown id error = %v", err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := catalog.Resolve(cancelled, original.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled resolve error = %v", err)
	}
}

func TestMaskCatalogMissingAssetFailsClosed(t *testing.T) {
	assets := fstest.MapFS{
		"prompts/mask-frame.md":  &fstest.MapFile{Data: []byte("frame")},
		"prompts/programmer.md":  &fstest.MapFile{Data: []byte("programmer")},
		"prompts/researcher.md":  &fstest.MapFile{Data: []byte("researcher")},
	}
	if _, err := loadCatalog(assets, "generation-test"); err == nil {
		t.Fatal("missing writer asset was accepted")
	} else if !strings.Contains(err.Error(), "writer.md") {
		t.Fatalf("missing asset error = %v", err)
	}
}

func TestMaskCatalogDuplicateIDsFailClosed(t *testing.T) {
	frame, err := fs.ReadFile(embeddedAssets, "prompts/mask-frame.md")
	if err != nil {
		t.Fatal(err)
	}
	specs := append([]builtinSpec(nil), builtinSpecs...)
	specs[1].id = specs[0].id
	if _, err := loadCatalogDefinitions(embeddedAssets, "generation-test", frame, specs); err == nil {
		t.Fatal("duplicate built-in ID was accepted")
	} else if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate ID error = %v", err)
	}
}

func TestMaskCatalogAssetBodiesPreserveLiteralText(t *testing.T) {
	const quotedID = maskcontract.BuiltinProgrammerID
	const quotedName = `Writer "quoted"`
	const literalBody = "A literal \"{system}\" and {{persona}} stay unchanged.\n"
	assets := fstest.MapFS{
		"prompts/mask-frame.md": &fstest.MapFile{Data: []byte("frame")},
		"prompts/programmer.md": &fstest.MapFile{Data: []byte(literalBody)},
	}
	specs := []builtinSpec{{
		id:          quotedID,
		name:        quotedName,
		description: "quoted name fixture",
		path:        "prompts/programmer.md",
	}}
	catalog, err := loadCatalogDefinitions(assets, "generation-test", []byte("frame"), specs)
	if err != nil {
		t.Fatal(err)
	}
	resolved, resolveErr := catalog.Resolve(context.Background(), quotedID)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	if resolved.Name != quotedName {
		t.Fatalf("quoted name = %q, want %q", resolved.Name, quotedName)
	}
	if resolved.Body != literalBody {
		t.Fatalf("literal body = %q, want %q", resolved.Body, literalBody)
	}
	wantDigest := maskcontract.DefinitionDigest(quotedID, quotedName, literalBody)
	if resolved.Digest != wantDigest {
		t.Fatalf("literal digest = %q, want %q", resolved.Digest, wantDigest)
	}
}
