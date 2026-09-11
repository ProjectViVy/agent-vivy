package assembly

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	uiSDK "agent-vivy/sdk/ui"
)

func TestGenerateUIAssemblyIsDeterministicAcrossInputEnumerationOrder(t *testing.T) {
	first := uiAssemblyInputForTest()
	second := uiAssemblyInputForTest()
	second.Roots = append([]UIModule(nil), first.Roots...)
	second.Extensions = []UIModule{first.Extensions[1], first.Extensions[0]}
	second.ExtensionOrder = []string{"fixture/extension-a", "fixture/extension-b"}

	gotFirst, err := GenerateUIAssembly(first)
	if err != nil {
		t.Fatalf("GenerateUIAssembly(first) error = %v", err)
	}
	gotSecond, err := GenerateUIAssembly(second)
	if err != nil {
		t.Fatalf("GenerateUIAssembly(second) error = %v", err)
	}
	if !bytes.Equal(gotFirst.Source, gotSecond.Source) {
		t.Fatalf("generated imports changed with input enumeration order\nfirst:\n%s\nsecond:\n%s", gotFirst.Source, gotSecond.Source)
	}
	if !bytes.Equal(gotFirst.Manifest.Canonical(), gotSecond.Manifest.Canonical()) {
		t.Fatalf("UI provenance changed with input enumeration order\nfirst: %s\nsecond: %s", gotFirst.Manifest.Canonical(), gotSecond.Manifest.Canonical())
	}
}

func TestGenerateUIAssemblyRejectsTwoRoots(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Roots = append(input.Roots, UIModule{
		ID:                 "fixture/root-b",
		ModuleID:           "fixture/root-b",
		Entry:              "./root-b.tsx",
		SourceHash:         testDigest("root-b-source"),
		DependencyLockHash: testDigest("root-b-lock"),
		AssetHash:          testDigest("root-b-asset"),
	})

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorDuplicateRoot, "one selected root")
}

func TestGenerateUIAssemblyRejectsAmbiguousExtensionOrder(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.ExtensionOrder = nil

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorAmbiguousOrder, "ambiguous extension order")
}

func TestGenerateUIAssemblyRequiresRecipeOrderForSingleExtension(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions = input.Extensions[:1]
	input.ExtensionOrder = nil

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorAmbiguousOrder, "ExtensionOrder is required")
}

func TestGenerateUIAssemblyRequiresRecipeOrderForUniquelyChainedExtensions(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.ExtensionOrder = nil
	input.Extensions[0].After = []string{"fixture/extension-b"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorAmbiguousOrder, "ExtensionOrder is required")
}

func TestGenerateUIAssemblyRejectsIncompleteRecipeOrder(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.ExtensionOrder = []string{"fixture/extension-a"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorAmbiguousOrder, "omits a selected extension")
}

func TestGenerateUIAssemblyRejectsExtraRecipeOrder(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.ExtensionOrder = []string{"fixture/extension-a", "fixture/extension-b", "fixture/missing"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorMissingTarget, "names a missing target")
}

func TestGenerateUIAssemblyRejectsDuplicateRecipeOrder(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.ExtensionOrder = []string{"fixture/extension-a", "fixture/extension-a"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorAmbiguousOrder, "contains a duplicate")
}

func TestGenerateUIAssemblyAcceptsCanonicalModuleIDsInRecipeOrder(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].ModuleID = "fixture/module-a"
	input.Extensions[1].ModuleID = "fixture/module-b"
	input.ExtensionOrder = []string{"fixture/module-b", "fixture/module-a"}

	result, err := GenerateUIAssembly(input)
	if err != nil {
		t.Fatalf("GenerateUIAssembly() error = %v", err)
	}
	want := []string{"fixture/extension-b", "fixture/extension-a"}
	if !reflect.DeepEqual(result.Manifest.Extensions, want) {
		t.Fatalf("ordered extensions = %v, want %v", result.Manifest.Extensions, want)
	}
}

func TestGenerateUIAssemblyRejectsMissingRelationshipTarget(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].Before = []string{"fixture/missing"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorMissingTarget, "missing target")
}

func TestGenerateUIAssemblyRejectsFloatingPackageDependency(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].Dependencies = map[string]string{"react": "^19.2.0"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorFloatingDependency, "floating package dependency")
}

func TestGenerateUIAssemblyRejectsRemoteEntryURL(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].Entry = "https://example.invalid/plugin.js"

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorRemoteEntry, "remote entry URL")
}

func TestGenerateUIAssemblyRejectsEntriesOutsideSelectedModuleSource(t *testing.T) {
	for _, entry := range []string{
		"../outside.tsx",
		"./../../outside.tsx",
		"react",
		"@scope/ui",
		"./src/../index.tsx",
		"./src//index.tsx",
		"./src/index.tsx?raw",
		"./src/index.tsx#fragment",
		" ./src/index.tsx",
		"./src/index.tsx ",
		".\\src\\index.tsx",
		"C:\\workspace\\plugin\\index.tsx",
		"//cdn.example.invalid/plugin.js",
	} {
		t.Run(entry, func(t *testing.T) {
			input := uiAssemblyInputForTest()
			input.Extensions[0].Entry = entry

			_, err := GenerateUIAssembly(input)
			assertUIAssemblyError(t, err, UIAssemblyErrorRemoteEntry, "source-confined")
		})
	}
}

func TestGenerateUIAssemblyAcceptsNormalizedSourceRelativeEntry(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].Entry = "./src/components/index.tsx"

	result, err := GenerateUIAssembly(input)
	if err != nil {
		t.Fatalf("GenerateUIAssembly() error = %v", err)
	}
	if !strings.Contains(string(result.Source), `"./src/components/index.tsx"`) {
		t.Fatalf("generated Assembly omitted valid source-relative import:\n%s", result.Source)
	}
}

func TestGenerateUIAssemblyRejectsSDKVersionMismatch(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.SDKVersion = "1.0.1"

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorSDKVersionMismatch, "pinned UI SDK")
}

func TestGenerateUIAssemblyRejectsSDKPinDriftAtGeneration(t *testing.T) {
	pin, err := uiSDK.PinnedPackage()
	if err != nil {
		t.Fatal(err)
	}
	input := uiAssemblyInputForTest()
	input.SDKVersion = pin.Version + ".drift"

	_, err = GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorSDKVersionMismatch, "pinned UI SDK")
}

func TestGenerateUIAssemblyRejectsRootRelationships(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Roots[0].Replaces = []string{"fixture/extension-a"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorRootRelationship, "root relationship")
}

func TestGenerateUIAssemblyRejectsExtensionRelationshipsTargetingRoot(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*UIModule)
	}{
		{name: "before", set: func(module *UIModule) { module.Before = []string{"fixture/root"} }},
		{name: "after", set: func(module *UIModule) { module.After = []string{"fixture/root"} }},
		{name: "replaces", set: func(module *UIModule) { module.Replaces = []string{"fixture/root"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := uiAssemblyInputForTest()
			test.set(&input.Extensions[0])

			_, err := GenerateUIAssembly(input)
			assertUIAssemblyError(t, err, UIAssemblyErrorRootRelationship, "targeting root")
		})
	}
}

func TestGenerateUIAssemblyRejectsBlankRelationshipTargets(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*UIModule)
	}{
		{name: "before", set: func(module *UIModule) { module.Before = []string{" \t"} }},
		{name: "after", set: func(module *UIModule) { module.After = []string{"\n"} }},
		{name: "replaces", set: func(module *UIModule) { module.Replaces = []string{"  "} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := uiAssemblyInputForTest()
			test.set(&input.Extensions[0])

			_, err := GenerateUIAssembly(input)
			assertUIAssemblyError(t, err, UIAssemblyErrorMissingTarget, "blank relationship target")
		})
	}
}

func TestGenerateUIAssemblySupportsMultipleProvidersFromOneModule(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].ModuleID = "fixture/shared-module"
	input.Extensions[1].ModuleID = "fixture/shared-module"
	input.ExtensionOrder = []string{"fixture/extension-b", "fixture/extension-a"}

	result, err := GenerateUIAssembly(input)
	if err != nil {
		t.Fatalf("GenerateUIAssembly() error = %v", err)
	}
	want := []string{"fixture/extension-b", "fixture/extension-a"}
	if !reflect.DeepEqual(result.Manifest.Extensions, want) {
		t.Fatalf("ordered providers = %v, want %v", result.Manifest.Extensions, want)
	}
}

func TestGenerateUIAssemblyRejectsAmbiguousModuleIDOrderAlias(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].ModuleID = "fixture/shared-module"
	input.Extensions[1].ModuleID = "fixture/shared-module"
	input.ExtensionOrder = []string{"fixture/shared-module"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorAmbiguousOrder, "ambiguous ModuleID")
}

func TestGenerateUIAssemblyRejectsConflictingCompatibilityAliases(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].Entry = "./new-entry.tsx"
	input.Extensions[0].ImportPath = "./old-entry.tsx"

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorConflictingAlias, "conflicting entry aliases")
}

func TestGenerateUIAssemblyRejectsConflictingHashAliases(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].AssetHash = testDigest("new-asset")
	input.Extensions[0].FinalAssetHash = testDigest("old-asset")

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorConflictingAlias, "conflicting asset hash aliases")
}

func TestGenerateUIAssemblyRejectsConflictingGlobalHashAliases(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].DependencyLockHash = ""
	input.Extensions[0].LockHash = ""
	input.DependencyLockHashes = map[string]string{"fixture/extension-a": testDigest("new-lock")}
	input.LockHashes = map[string]string{"fixture/extension-a": testDigest("old-lock")}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorConflictingAlias, "conflicting dependency lock hash aliases")
}

func TestGenerateUIAssemblyRejectsPartialSingularRoot(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Roots = nil
	input.Root = UIModule{Port: UIRootPort}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorInvalidInput, "provider id is required")
}

func TestGenerateUIAssemblyManifestBindsAllExplicitProvenance(t *testing.T) {
	result, err := GenerateUIAssembly(uiAssemblyInputForTest())
	if err != nil {
		t.Fatalf("GenerateUIAssembly() error = %v", err)
	}
	manifest := result.Manifest
	if manifest.SDKPackage != UIAssemblySDKPackageName {
		t.Fatalf("SDKPackage = %q, want %q", manifest.SDKPackage, UIAssemblySDKPackageName)
	}
	if manifest.SDKVersion != UIAssemblySDKVersion {
		t.Fatalf("SDKVersion = %q, want %s", manifest.SDKVersion, UIAssemblySDKVersion)
	}
	for _, id := range []string{"fixture/root", "fixture/extension-a", "fixture/extension-b"} {
		if manifest.SourceHashes[id] == "" {
			t.Fatalf("source hash for %s missing from manifest: %#v", id, manifest.SourceHashes)
		}
		if manifest.DependencyLockHashes[id] == "" {
			t.Fatalf("dependency lock hash for %s missing from manifest: %#v", id, manifest.DependencyLockHashes)
		}
		if manifest.AssetHashes[id] == "" {
			t.Fatalf("final asset hash for %s missing from manifest: %#v", id, manifest.AssetHashes)
		}
	}
	for _, required := range []string{"sourceHashes", "dependencyLockHashes", "sdkVersion", "assetHashes"} {
		if !strings.Contains(string(result.Source), required) {
			t.Fatalf("generated assembly missing provenance field %q:\n%s", required, result.Source)
		}
	}
}

func TestGenerateUIAssemblyEmbedsValidatedCatalogProjection(t *testing.T) {
	catalog := CatalogManifest{
		Module:        "fixture/catalog",
		APIVersion:    "vivy.i18n/v1",
		SchemaVersion: "vivy.i18n/v1",
		Path:          "i18n/catalog.json",
		DefaultLocale: "en",
		Locales:       []string{"en", "zh"},
		Units: map[string]CatalogUnit{
			"plugin.fixture/catalog.results": {
				Description:  "Result count",
				Placeholders: []string{"count"},
				Messages:     map[string]string{"en": "{{count}} results", "zh": "{{count}} 个结果"},
				Short:        map[string]string{"en": "{{count}} results"},
			},
		},
	}
	catalog.Digest = catalogProjectionDigest(catalog)
	input := uiAssemblyInputForTest()
	input.Catalogs = []CatalogManifest{catalog}
	result, err := GenerateUIAssembly(input)
	if err != nil {
		t.Fatalf("GenerateUIAssembly() error = %v", err)
	}
	if len(result.Manifest.Catalogs) != 1 || result.Manifest.Catalogs[0].Units["plugin.fixture/catalog.results"].Messages["en"] != "{{count}} results" {
		t.Fatalf("catalog projection was not sealed: %#v", result.Manifest.Catalogs)
	}
	if !strings.Contains(string(result.Source), "plugin.fixture/catalog.results") || !strings.Contains(string(result.Source), "{{count}} results") {
		t.Fatalf("generated assembly omitted catalog body:\n%s", result.Source)
	}
}

func TestGenerateUIAssemblyRejectsMalformedOrAmbiguousCatalogs(t *testing.T) {
	valid := CatalogManifest{
		Module:        "fixture/catalog",
		APIVersion:    "vivy.i18n/v1",
		SchemaVersion: "vivy.i18n/v1",
		DefaultLocale: "en",
		Locales:       []string{"en"},
		Units: map[string]CatalogUnit{
			"plugin.fixture/catalog.message": {
				Description:  "Message",
				Placeholders: []string{},
				Messages:     map[string]string{"en": "Message"},
			},
		},
	}
	valid.Digest = catalogProjectionDigest(valid)
	tests := []struct {
		name string
		set  func(*UIAssemblyInput)
		code UIAssemblyErrorCode
		want string
	}{
		{name: "missing units", set: func(input *UIAssemblyInput) {
			copy := valid
			copy.Units = nil
			copy.Digest = catalogProjectionDigest(copy)
			input.Catalogs = []CatalogManifest{copy}
		}, code: UIAssemblyErrorInvalidInput, want: "translation units"},
		{name: "wrong owner", set: func(input *UIAssemblyInput) {
			copy := valid
			copy.Units = map[string]CatalogUnit{"plugin.other.message": valid.Units["plugin.fixture/catalog.message"]}
			copy.Digest = catalogProjectionDigest(copy)
			input.Catalogs = []CatalogManifest{copy}
		}, code: UIAssemblyErrorInvalidInput, want: "outside owner namespace"},
		{name: "wrong digest", set: func(input *UIAssemblyInput) {
			copy := valid
			copy.Digest = testDigest("wrong-catalog")
			input.Catalogs = []CatalogManifest{copy}
		}, code: UIAssemblyErrorMissingHash, want: "digest"},
		{name: "conflicting api aliases", set: func(input *UIAssemblyInput) {
			copy := valid
			copy.SchemaVersion = "vivy.i18n/v1"
			copy.APIVersion = "vivy.i18n/v2"
			copy.Digest = catalogProjectionDigest(copy)
			input.Catalogs = []CatalogManifest{copy}
		}, code: UIAssemblyErrorConflictingAlias, want: "conflicting catalog apiVersion aliases"},
		{name: "invalid module namespace", set: func(input *UIAssemblyInput) {
			copy := valid
			copy.Module = "../catalog"
			copy.Units = map[string]CatalogUnit{"plugin.../catalog.message": valid.Units["plugin.fixture/catalog.message"]}
			copy.Digest = catalogProjectionDigest(copy)
			input.Catalogs = []CatalogManifest{copy}
		}, code: UIAssemblyErrorInvalidInput, want: "module namespace"},
		{name: "duplicate translation key", set: func(input *UIAssemblyInput) {
			second := valid
			second.Module = "fixture/other"
			second.Units = map[string]CatalogUnit{"plugin.fixture/catalog.message": valid.Units["plugin.fixture/catalog.message"]}
			second.Digest = catalogProjectionDigest(second)
			input.Catalogs = []CatalogManifest{valid, second}
		}, code: UIAssemblyErrorDuplicateID, want: "translation key"},
		{name: "duplicate module", set: func(input *UIAssemblyInput) { input.Catalogs = []CatalogManifest{valid, valid} }, code: UIAssemblyErrorDuplicateID, want: "selected more than once"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := uiAssemblyInputForTest()
			test.set(&input)
			_, err := GenerateUIAssembly(input)
			assertUIAssemblyError(t, err, test.code, test.want)
		})
	}
}

func TestGenerateUIAssemblyOmitsUnselectedModulesFromMinimalBuild(t *testing.T) {
	input := UIAssemblyInput{SDKVersion: UIAssemblySDKVersion}
	result, err := GenerateUIAssembly(input)
	if err != nil {
		t.Fatalf("GenerateUIAssembly(minimal) error = %v", err)
	}
	source := string(result.Source)
	for _, omitted := range []string{"fixture/root", "fixture/extension-a", "fixture/extension-b", "root.tsx", "extension-a.tsx", "extension-b.tsx"} {
		if strings.Contains(source, omitted) {
			t.Fatalf("minimal generated assembly retained omitted module %q:\n%s", omitted, source)
		}
	}
	if len(result.Manifest.SourceHashes) != 0 || len(result.Manifest.DependencyLockHashes) != 0 || len(result.Manifest.AssetHashes) != 0 {
		t.Fatalf("minimal manifest retained omitted module provenance: %#v", result.Manifest)
	}
}

func TestGenerateUIAssemblyRejectsIncompleteExplicitProvenance(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].AssetHash = ""

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorMissingHash, "asset hash")
}

func TestGenerateUIAssemblyRejectsConflictingExplicitOrder(t *testing.T) {
	input := uiAssemblyInputForTest()
	input.Extensions[0].Before = []string{"fixture/extension-b"}
	input.ExtensionOrder = []string{"fixture/extension-b", "fixture/extension-a"}

	_, err := GenerateUIAssembly(input)
	assertUIAssemblyError(t, err, UIAssemblyErrorAmbiguousOrder, "order conflict")
}

func TestGenerateNonEmptyUIAssemblyTypechecks(t *testing.T) {
	result, err := GenerateUIAssembly(uiAssemblyInputForTest())
	if err != nil {
		t.Fatalf("GenerateUIAssembly() error = %v", err)
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "assembly.ts"), result.Source, 0o600); err != nil {
		t.Fatal(err)
	}
	rootSource := `import type { UIRoot } from "@vivy/ui-sdk";
export const root: UIRoot = { id: "fixture/root", render: () => null };
`
	extensionASource := `import type { UIExtension } from "@vivy/ui-sdk";
export const extensionA: UIExtension = { id: "fixture/extension-a", install: () => undefined };
`
	extensionBSource := `import type { UIExtension } from "@vivy/ui-sdk";
export const extensionB: UIExtension = { id: "fixture/extension-b", install: () => undefined };
`
	for name, source := range map[string]string{"root.tsx": rootSource, "extension-a.tsx": extensionASource, "extension-b.tsx": extensionBSource} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := `{
  "compilerOptions": {
    "module": "ESNext",
    "target": "ES2022",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "allowImportingTsExtensions": true,
    "strict": true,
    "noEmit": true,
    "baseUrl": ".",
    "paths": {"@vivy/ui-sdk": ["` + filepath.ToSlash(filepath.Join(repositoryRoot, "sdk/ui/src")) + `"]}
  },
  "include": ["./assembly.ts", "./root.tsx", "./extension-a.tsx", "./extension-b.tsx"]
}
`
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join(repositoryRoot, "ui", "node_modules", ".bin", "tsc")
	command := exec.Command(tsc, "-p", filepath.Join(root, "tsconfig.json"))
	command.Dir = repositoryRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("non-empty generated Assembly did not typecheck: %v\n%s", err, output)
	}
}

func TestTask7BuildsAndInspectsEveryFixtureGeneration(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		name     string
		input    UIAssemblyInput
		selected []string
		omitted  []string
		sources  map[string]string
	}{
		{
			name: "default",
			input: UIAssemblyInput{
				SDKVersion: UIAssemblySDKVersion,
				Roots: []UIModule{{
					ID: "fixture/default-root", ModuleID: "fixture/default", Port: UIRootPort,
					Entry: "./default-root.tsx", Export: "root",
					SourceHash: testDigest("task7-default-source"), DependencyLockHash: testDigest("task7-default-lock"), AssetHash: testDigest("task7-default-asset"),
				}},
			},
			selected: []string{"fixture/default-root", "./default-root.tsx"},
			omitted:  []string{"fixture/extension", "fixture/replacement-root", "./extension.tsx", "./replacement-root.tsx"},
			sources:  map[string]string{"default-root.tsx": `import type { UIRoot } from "@vivy/ui-sdk"; export const root: UIRoot = { id: "fixture/default-root", render: () => null };`},
		},
		{
			name: "extension",
			input: UIAssemblyInput{
				SDKVersion: UIAssemblySDKVersion,
				Roots: []UIModule{{
					ID: "fixture/extension-root", ModuleID: "fixture/extension", Port: UIRootPort,
					Entry: "./extension-root.tsx", Export: "root",
					SourceHash: testDigest("task7-extension-root-source"), DependencyLockHash: testDigest("task7-extension-root-lock"), AssetHash: testDigest("task7-extension-root-asset"),
				}},
				Extensions: []UIModule{{
					ID: "fixture/extension", ModuleID: "fixture/extension", Port: UIExtensionPort,
					Entry: "./extension.tsx", Export: "extension",
					SourceHash: testDigest("task7-extension-source"), DependencyLockHash: testDigest("task7-extension-lock"), AssetHash: testDigest("task7-extension-asset"),
				}},
				ExtensionOrder: []string{"fixture/extension"},
			},
			selected: []string{"fixture/extension-root", "fixture/extension", "./extension-root.tsx", "./extension.tsx"},
			omitted:  []string{"fixture/default-root", "fixture/replacement-root", "./default-root.tsx", "./replacement-root.tsx"},
			sources: map[string]string{
				"extension-root.tsx": `import type { UIRoot } from "@vivy/ui-sdk"; export const root: UIRoot = { id: "fixture/extension-root", render: () => null };`,
				"extension.tsx":      `import type { UIExtension } from "@vivy/ui-sdk"; export const extension: UIExtension = { id: "fixture/extension", install: () => undefined };`,
			},
		},
		{
			name: "replacement-root",
			input: UIAssemblyInput{
				SDKVersion: UIAssemblySDKVersion,
				Roots: []UIModule{{
					ID: "fixture/replacement-root", ModuleID: "fixture/replacement", Port: UIRootPort,
					Entry: "./replacement-root.tsx", Export: "root",
					SourceHash: testDigest("task7-replacement-root-source"), DependencyLockHash: testDigest("task7-replacement-root-lock"), AssetHash: testDigest("task7-replacement-root-asset"),
				}},
				Extensions: []UIModule{{
					ID: "fixture/replacement-extension", ModuleID: "fixture/replacement", Port: UIExtensionPort,
					Entry: "./replacement-extension.tsx", Export: "extension",
					SourceHash: testDigest("task7-replacement-extension-source"), DependencyLockHash: testDigest("task7-replacement-extension-lock"), AssetHash: testDigest("task7-replacement-extension-asset"),
				}},
				ExtensionOrder: []string{"fixture/replacement-extension"},
			},
			selected: []string{"fixture/replacement-root", "fixture/replacement-extension", "./replacement-root.tsx", "./replacement-extension.tsx"},
			omitted:  []string{"fixture/default-root", "fixture/extension", "./default-root.tsx", "./extension.tsx"},
			sources: map[string]string{
				"replacement-root.tsx":      `import type { UIRoot } from "@vivy/ui-sdk"; export const root: UIRoot = { id: "fixture/replacement-root", render: () => null };`,
				"replacement-extension.tsx": `import type { UIExtension } from "@vivy/ui-sdk"; export const extension: UIExtension = { id: "fixture/replacement-extension", install: () => undefined };`,
			},
		},
		{
			name:     "minimal",
			input:    UIAssemblyInput{SDKVersion: UIAssemblySDKVersion},
			selected: nil,
			omitted:  []string{"fixture/default-root", "fixture/extension", "fixture/replacement-root", "./default-root.tsx", "./extension.tsx", "./replacement-root.tsx"},
			sources:  map[string]string{},
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			result, generateErr := GenerateUIAssembly(fixture.input)
			if generateErr != nil {
				t.Fatalf("GenerateUIAssembly(%s) error = %v", fixture.name, generateErr)
			}
			source := string(result.Source)
			for _, selected := range fixture.selected {
				if !strings.Contains(source, selected) {
					t.Fatalf("%s generated Assembly omitted selected input %q:\n%s", fixture.name, selected, source)
				}
			}
			for _, omitted := range fixture.omitted {
				if strings.Contains(source, omitted) {
					t.Fatalf("%s generated Assembly retained omitted input %q:\n%s", fixture.name, omitted, source)
				}
			}
			if !bytes.Equal(result.Source, result.Generated) || !bytes.Equal(result.Source, result.Code) {
				t.Fatalf("%s generated Assembly compatibility spellings diverged", fixture.name)
			}
			for _, id := range fixture.selected {
				if strings.HasPrefix(id, "./") {
					continue
				}
				if result.Manifest.SourceHashes[id] == "" || result.Manifest.DependencyLockHashes[id] == "" || result.Manifest.AssetHashes[id] == "" {
					t.Fatalf("%s omitted sealed hash provenance for selected %s: %#v", fixture.name, id, result.Manifest)
				}
			}

			canonicalRecipe := []byte(fmt.Sprintf(`{"apiVersion":"vivy.generation/v1","profile":%q,"ui":%s}`, fixture.name, result.Manifest.Canonical()))
			sealed, raw, sealErr := SealManifest(AssemblyPlan{}, SealInputs{
				SpecificationVersion: "vivy.assembly/v1",
				CompilerVersion:      "task7-fixture-compiler",
				SDKVersion:           UIAssemblySDKVersion,
				CanonicalRecipe:      canonicalRecipe,
				UIArtifacts:          map[string]string{"ui/dist": testDigest("task7-" + fixture.name + "-dist")},
				UI:                   &result.Manifest,
			})
			if sealErr != nil {
				t.Fatalf("SealManifest(%s) error = %v", fixture.name, sealErr)
			}
			inspected, inspectErr := InspectManifest(raw)
			if inspectErr != nil {
				t.Fatalf("InspectManifest(%s) error = %v", fixture.name, inspectErr)
			}
			if inspected.UI == nil || inspected.GenerationID != sealed.GenerationID || inspected.UI.Root != result.Manifest.Root || !equalStrings(inspected.UI.Extensions, result.Manifest.Extensions) || !reflect.DeepEqual(inspected.UI.SourceHashes, result.Manifest.SourceHashes) || !reflect.DeepEqual(inspected.UI.DependencyLockHashes, result.Manifest.DependencyLockHashes) || !reflect.DeepEqual(inspected.UI.AssetHashes, result.Manifest.AssetHashes) || inspected.UI.SDKPackage != result.Manifest.SDKPackage || inspected.UI.SDKVersion != result.Manifest.SDKVersion {
				t.Fatalf("InspectManifest(%s) changed sealed UI projection: inspected=%#v generated=%#v", fixture.name, inspected.UI, result.Manifest)
			}
			if fixture.name == "minimal" && (inspected.UI.Root != "" || len(inspected.UI.Extensions) != 0 || len(inspected.UI.SourceHashes) != 0 || len(inspected.UI.DependencyLockHashes) != 0 || len(inspected.UI.AssetHashes) != 0) {
				t.Fatalf("minimal InspectManifest retained selected UI state: %#v", inspected.UI)
			}
			task7TypecheckGeneratedUI(t, repositoryRoot, result.Source, fixture.sources)
		})
	}
}

func TestTask7FailedUIBuildHasNoPublishedGeneration(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	input := UIAssemblyInput{
		SDKVersion: UIAssemblySDKVersion,
		Roots: []UIModule{{
			ID: "fixture/build-failure-root", ModuleID: "fixture/build-failure", Port: UIRootPort,
			Entry: "./root.tsx", Export: "root", SourceHash: testDigest("task7-build-failure-source"), DependencyLockHash: testDigest("task7-build-failure-lock"), AssetHash: testDigest("task7-build-failure-asset"),
		}},
	}
	result, err := GenerateUIAssembly(input)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "assembly.ts"), result.Source, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "root.tsx"), []byte(`import type { UIRoot } from "@vivy/ui-sdk"; export const root: UIRoot = { id: "fixture/build-failure-root", render: () => null };`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "generation.ts"), []byte(`export const intentionallyBroken = ;`), 0o600); err != nil {
		t.Fatal(err)
	}
	config := task7TypeScriptConfig(repositoryRoot, []string{"./assembly.ts", "./root.tsx", "./generation.ts"})
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join(repositoryRoot, "ui", "node_modules", ".bin", "tsc")
	command := exec.Command(tsc, "-p", filepath.Join(root, "tsconfig.json"))
	command.Dir = repositoryRoot
	if output, buildErr := command.CombinedOutput(); buildErr == nil {
		t.Fatal("broken UI fixture unexpectedly typechecked")
	} else if len(output) == 0 {
		t.Fatalf("broken UI fixture failed without a compiler diagnostic: %v", buildErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "generation.json")); !os.IsNotExist(statErr) {
		t.Fatalf("failed UI build published generation.json: %v", statErr)
	}
}

func task7TypecheckGeneratedUI(t *testing.T, repositoryRoot string, generated []byte, sources map[string]string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "assembly.ts"), generated, 0o600); err != nil {
		t.Fatal(err)
	}
	include := []string{"./assembly.ts"}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		include = append(include, "./"+name)
	}
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(task7TypeScriptConfig(repositoryRoot, include)), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join(repositoryRoot, "ui", "node_modules", ".bin", "tsc")
	command := exec.Command(tsc, "-p", filepath.Join(root, "tsconfig.json"))
	command.Dir = repositoryRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated UI Assembly did not typecheck: %v\n%s", err, output)
	}
}

func task7TypeScriptConfig(repositoryRoot string, include []string) string {
	return fmt.Sprintf(`{
  "compilerOptions": {
    "module": "ESNext",
    "target": "ES2022",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "allowImportingTsExtensions": true,
    "strict": true,
    "noEmit": true,
    "baseUrl": ".",
    "paths": {"@vivy/ui-sdk": ["%s"]}
  },
  "include": %s
}
`, filepath.ToSlash(filepath.Join(repositoryRoot, "sdk/ui/src")), mustJSON(include))
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func TestUIAssemblySDKPinMatchesPublicPackage(t *testing.T) {
	pin, err := uiSDK.PinnedPackage()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "ui", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var packageJSON struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &packageJSON); err != nil {
		t.Fatal(err)
	}
	if packageJSON.Name != pin.Name || packageJSON.Version != pin.Version {
		t.Fatalf("UI SDK pin = %s@%s, want %s@%s", UIAssemblySDKPackageName, UIAssemblySDKVersion, packageJSON.Name, packageJSON.Version)
	}
	if UIAssemblySDKPackageName != pin.Name || UIAssemblySDKVersion != pin.Version {
		t.Fatalf("assembly UI SDK pin = %s@%s, want embedded %s@%s", UIAssemblySDKPackageName, UIAssemblySDKVersion, pin.Name, pin.Version)
	}
}

func uiAssemblyInputForTest() UIAssemblyInput {
	return UIAssemblyInput{
		SDKVersion: UIAssemblySDKVersion,
		Roots: []UIModule{{
			ID:                 "fixture/root",
			ModuleID:           "fixture/root",
			Port:               UIRootPort,
			Entry:              "./root.tsx",
			Export:             "root",
			SourceHash:         testDigest("root-source"),
			DependencyLockHash: testDigest("root-lock"),
			AssetHash:          testDigest("root-asset"),
		}},
		Extensions: []UIModule{{
			ID:                 "fixture/extension-a",
			ModuleID:           "fixture/extension-a",
			Port:               UIExtensionPort,
			Entry:              "./extension-a.tsx",
			Export:             "extensionA",
			SourceHash:         testDigest("extension-a-source"),
			DependencyLockHash: testDigest("extension-a-lock"),
			AssetHash:          testDigest("extension-a-asset"),
		}, {
			ID:                 "fixture/extension-b",
			ModuleID:           "fixture/extension-b",
			Port:               UIExtensionPort,
			Entry:              "./extension-b.tsx",
			Export:             "extensionB",
			SourceHash:         testDigest("extension-b-source"),
			DependencyLockHash: testDigest("extension-b-lock"),
			AssetHash:          testDigest("extension-b-asset"),
		}},
		ExtensionOrder: []string{"fixture/extension-a", "fixture/extension-b"},
	}
}

func testDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func assertUIAssemblyError(t *testing.T, err error, code UIAssemblyErrorCode, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("GenerateUIAssembly() succeeded, want %s", code)
	}
	var assemblyErr *UIAssemblyError
	if !errors.As(err, &assemblyErr) {
		t.Fatalf("error = %T %v, want *UIAssemblyError(%s)", err, err, code)
	}
	if assemblyErr.Code != code {
		t.Fatalf("error code = %s, want %s (%v)", assemblyErr.Code, code, err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}
