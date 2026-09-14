package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/toolhost"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/port/pretool"
)

type executableFailureMatrix struct {
	SchemaVersion string
	Cases         []executableFailureMatrixCase
}

type executableFailureMatrixCase struct {
	ID         string
	Layer      string
	EvidenceID string
	Diagnostic string
}

// TestGenerationFailureMatrixExecutesEveryCase is the executable owner of the
// release matrix. The JSON index drives every subtest; invalid whole-
// Generation inputs must return their stable diagnostic before Output exists.
func TestGenerationFailureMatrixExecutesEveryCase(t *testing.T) {
	raw, err := os.ReadFile("conformance/testdata/failure-matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var matrix executableFailureMatrix
	if err := decoder.Decode(&matrix); err != nil {
		t.Fatal(err)
	}
	if matrix.SchemaVersion != "vivy.conformance/failure-matrix-v1" {
		t.Fatalf("schemaVersion = %q", matrix.SchemaVersion)
	}
	known := matrixExecutionCaseIDs()
	if len(known) != len(matrix.Cases) {
		t.Fatalf("matrix has %d rows but %d executable handlers", len(matrix.Cases), len(known))
	}
	for _, testCase := range matrix.Cases {
		testCase := testCase
		t.Run(testCase.ID, func(t *testing.T) {
			if _, ok := known[testCase.ID]; !ok {
				t.Fatalf("matrix row %s has no executable handler", testCase.ID)
			}
			root := t.TempDir()
			output := filepath.Join(root, "generation")
			observed, valid := executeMatrixCase(t, testCase.ID, root, output)
			observed = redactMatrixDiagnostic(observed, root, output)
			if os.Getenv("VIVY_P9_PRINT_MATRIX") == "1" {
				t.Logf("VIVY_P9_MATRIX=%s=%q", testCase.ID, observed)
			}
			if observed != testCase.Diagnostic {
				t.Fatalf("observed diagnostic = %q, want exact redacted diagnostic %q", observed, testCase.Diagnostic)
			}
			if valid {
				return
			}
			if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
				t.Fatalf("invalid Generation emitted formal artifact: %v", statErr)
			}
		})
	}
}

func matrixExecutionCaseIDs() map[string]struct{} {
	ids := []string{
		"bad-source-hash", "catalog-byte-limit", "catalog-duplicate-key", "catalog-message-limit",
		"catalog-missing-english", "catalog-placeholder-drift", "catalog-placeholder-limit",
		"catalog-symlink-escape", "catalog-unit-limit", "catalog-unknown-field", "catalog-wrong-owner",
		"dependency-cycle", "duplicate-exclusive-provider", "grant-denied", "legacy-v0-input",
		"middleware-timeout", "missing-provider", "module-conflict", "no-formal-artifact",
		"protected-tool-override", "startup-rollback", "ui-root-conflict",
		"unsupported-eino-capability", "unused-provider", "valid-deterministic-rebuild",
	}
	known := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		known[id] = struct{}{}
	}
	return known
}

func executeMatrixCase(t *testing.T, id, root, output string) (string, bool) {
	t.Helper()
	switch {
	case strings.HasPrefix(id, "catalog-"):
		return executeCatalogMatrixCase(t, id, root, output), false
	case id == "middleware-timeout":
		return executeMiddlewareTimeout(t), false
	case id == "startup-rollback":
		return executeAnchoredGoTest(t, "./sdk/internal/assembly", "TestGeneratedBinderCompilesAndRollsBackLifecycle"), false
	case id == "valid-deterministic-rebuild":
		first, err := Pack(context.Background(), packOptions{Recipe: "../../recipes/minimal.vivy.yml", Output: filepath.Join(root, "first")})
		if err != nil {
			t.Fatal(err)
		}
		second, err := Pack(context.Background(), packOptions{Recipe: "../../recipes/minimal.vivy.yml", Output: filepath.Join(root, "second")})
		if err != nil {
			t.Fatal(err)
		}
		firstRaw, err := os.ReadFile(filepath.Join(first.Directory, "generation.json"))
		if err != nil {
			t.Fatal(err)
		}
		secondRaw, err := os.ReadFile(filepath.Join(second.Directory, "generation.json"))
		if err != nil {
			t.Fatal(err)
		}
		if first.Manifest.GenerationID != second.Manifest.GenerationID || !bytes.Equal(firstRaw, secondRaw) {
			t.Fatal("repeat Pack changed canonical Manifest or Generation identity")
		}
		return "identical canonical Manifest and Generation ID", true
	default:
		return executeAssemblyMatrixCase(t, id, root, output), false
	}
}

type matrixModuleSource struct {
	ID, Ref, SHA256, Dir string
}

func executeAssemblyMatrixCase(t *testing.T, id, root, output string) string {
	t.Helper()
	const minimal = "vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox"
	var recipe string
	var sources []matrixModuleSource
	switch id {
	case "legacy-v0-input":
		recipe = "apiVersion: vivy.generation/v0\nmodules: [" + minimal + "]\n"
	case "no-formal-artifact":
		recipe = "apiVersion: vivy.generation/v1\nmodules: [" + minimal + ", missing/module]\n"
	case "duplicate-exclusive-provider":
		recipe = "apiVersion: vivy.generation/v1\nmodules: [" + minimal + ", vivy/face-host, vivy/headless, vivy/tui]\nexclusive:\n  std/face@v1: vivy/headless\n"
	case "bad-source-hash":
		source := writeMatrixModule(t, root, "fixture/bad-hash", "provides: []\n", true)
		sources = []matrixModuleSource{source}
		recipe = matrixRecipe([]string{source.ID}, sources)
	case "dependency-cycle":
		a := writeMatrixModule(t, root, "fixture/cycle-a", "provides: []\nlifecycle: {scope: generation, after: [fixture/cycle-b]}\n", false)
		b := writeMatrixModule(t, root, "fixture/cycle-b", "provides: []\nlifecycle: {scope: generation, after: [fixture/cycle-a]}\n", false)
		sources = []matrixModuleSource{a, b}
		recipe = matrixRecipe([]string{a.ID, b.ID}, sources)
	case "missing-provider":
		source := writeMatrixModule(t, root, "fixture/context-host", "provides: []\nrequires: [{port: std/context-source@v1, provider: fixture/missing}]\n", false)
		sources = []matrixModuleSource{source}
		recipe = matrixRecipe([]string{source.ID}, sources)
	case "module-conflict":
		a := writeMatrixModule(t, root, "fixture/conflict-a", "provides: []\nconflicts: [{module: fixture/conflict-b}]\n", false)
		b := writeMatrixModule(t, root, "fixture/conflict-b", "provides: []\n", false)
		sources = []matrixModuleSource{a, b}
		recipe = matrixRecipe([]string{a.ID, b.ID}, sources)
	case "protected-tool-override":
		source := writeMatrixModule(t, root, "fixture/override", "provides: [{port: std/tool@v1, id: bash}]\n", false)
		sources = []matrixModuleSource{source}
		recipe = matrixRecipe([]string{source.ID}, sources)
	case "unsupported-eino-capability":
		source := writeMatrixModule(t, root, "fixture/eino-graph", "provides: [{port: std/eino-graph@v1, id: fixture.eino-graph}]\n", false)
		sources = []matrixModuleSource{source}
		recipe = matrixRecipe([]string{source.ID}, sources)
	case "unused-provider":
		source := writeMatrixModule(t, root, "fixture/provider", "provides: [{port: std/tool@v1, id: fixture.tool}]\n", false)
		sources = []matrixModuleSource{source}
		recipe = matrixRecipe([]string{source.ID}, sources)
	case "grant-denied":
		source := writeMatrixModule(t, root, "fixture/process-tool", "provides: [{port: std/tool@v1, id: fixture.process}]\nrequires: [{port: core/tool-host@v1, provider: vivy/tool-host}]\nrequestedGrants: [proc.spawn]\n", false)
		sources = []matrixModuleSource{source}
		recipe = matrixRecipe([]string{source.ID}, sources)
	case "ui-root-conflict":
		a := writeMatrixModule(t, root, "fixture/root-a", "provides: [{port: std/ui-root@v1, id: fixture.root-a}]\n", false)
		b := writeMatrixModule(t, root, "fixture/root-b", "provides: [{port: std/ui-root@v1, id: fixture.root-b}]\n", false)
		sources = []matrixModuleSource{a, b}
		recipe = matrixRecipe([]string{"vivy/presentation-host", a.ID, b.ID}, sources)
	default:
		t.Fatalf("unhandled assembly matrix case %s", id)
	}
	dirs := make([]string, 0, len(sources))
	for _, source := range sources {
		dirs = append(dirs, source.Dir)
	}
	return runMatrixPack(t, root, output, recipe, dirs)
}

func runMatrixPack(t *testing.T, root, output, recipe string, sources []string) string {
	t.Helper()
	recipePath := filepath.Join(root, "recipe.yml")
	if err := os.WriteFile(recipePath, []byte(recipe), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Pack(context.Background(), packOptions{Recipe: recipePath, Output: output, Sources: sources})
	if err == nil {
		t.Fatal("invalid matrix Generation unexpectedly packed")
	}
	return err.Error()
}

func writeMatrixModule(t *testing.T, root, id, descriptorTail string, badHash bool) matrixModuleSource {
	t.Helper()
	slug := strings.NewReplacer("/", "-", "_", "-").Replace(id)
	dir := filepath.Join(root, "sources", slug)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/vivy/matrix/"+slug+"\n\ngo 1.26.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "module.go"), []byte("package fixture\n\nfunc New() any { return nil }\nfunc NewProvider() any { return nil }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(descriptorTail, "std/ui-") {
		if err := os.MkdirAll(filepath.Join(dir, "i18n"), 0o700); err != nil {
			t.Fatal(err)
		}
		catalog := matrixCatalogWithUnit("plugin."+id+".title", []string{}, map[string]string{"en": "Matrix UI"})
		if err := os.WriteFile(filepath.Join(dir, "i18n", "catalog.json"), catalog, 0o600); err != nil {
			t.Fatal(err)
		}
		descriptorTail += "i18n: {catalog: i18n/catalog.json, default_locale: en, locales: [en]}\n"
	}
	const placeholder = "0000000000000000000000000000000000000000000000000000000000000000"
	ref := "file:matrix-" + slug
	sha256 := placeholder
	if badHash {
		sha256 = "not-a-sha256"
	}
	descriptor := fmt.Sprintf("apiVersion: vivy.module/v1\nmodule: {id: %s, version: 1.0.0}\nsource: {ref: %s, sha256: %s}\n%slifecycle: {scope: generation}\n", id, ref, sha256, descriptorTail)
	if strings.Contains(descriptorTail, "lifecycle:") {
		descriptor = fmt.Sprintf("apiVersion: vivy.module/v1\nmodule: {id: %s, version: 1.0.0}\nsource: {ref: %s, sha256: %s}\n%s", id, ref, sha256, descriptorTail)
	}
	if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	if !badHash {
		var err error
		sha256, err = assemblyv1.HashSourceTree(dir, placeholder)
		if err != nil {
			t.Fatal(err)
		}
		descriptor = strings.Replace(descriptor, placeholder, sha256, 1)
		if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(descriptor), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return matrixModuleSource{ID: id, Ref: ref, SHA256: sha256, Dir: dir}
}

func matrixRecipe(moduleIDs []string, sources []matrixModuleSource) string {
	var builder strings.Builder
	builder.WriteString("apiVersion: vivy.generation/v1\nmodules:\n")
	allModuleIDs := []string{"vivy/loop", "vivy/model", "vivy/tool-host", "vivy/storage", "vivy/checkpoint", "vivy/credential", "vivy/sandbox"}
	allModuleIDs = append(allModuleIDs, moduleIDs...)
	seen := make(map[string]struct{}, len(allModuleIDs))
	for _, id := range allModuleIDs {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		builder.WriteString("  - " + id + "\n")
	}
	if len(sources) == 0 {
		return builder.String()
	}
	builder.WriteString("sources:\n")
	for _, source := range sources {
		builder.WriteString(fmt.Sprintf("  %s: {ref: %s, sha256: %s}\n", source.ID, source.Ref, source.SHA256))
	}
	return builder.String()
}

type matrixPreToolProvider struct {
	id       string
	evaluate func(context.Context, pretool.Request) (pretool.Decision, error)
}

func (provider matrixPreToolProvider) ID() string { return provider.id }

func (provider matrixPreToolProvider) Evaluate(ctx context.Context, request pretool.Request) (pretool.Decision, error) {
	return provider.evaluate(ctx, request)
}

func executeMiddlewareTimeout(t *testing.T) string {
	t.Helper()
	host, err := toolhost.New(toolhost.Config{
		MiddlewareTimeout: time.Millisecond,
		Middleware: []pretool.Provider{matrixPreToolProvider{
			id: "matrix/slow",
			evaluate: func(ctx context.Context, _ pretool.Request) (pretool.Decision, error) {
				<-ctx.Done()
				return pretool.Decision{}, ctx.Err()
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.ApplyMiddleware(context.Background(), toolhost.Request{ID: "matrix.tool", Args: json.RawMessage(`{}`)}, nil)
	if err == nil {
		t.Fatal("timed-out Middleware unexpectedly passed")
	}
	return err.Error()
}

func executeAnchoredGoTest(t *testing.T, packagePath, testName string) string {
	t.Helper()
	command := exec.Command("go", "test", "-v", packagePath, "-run", "^"+testName+"$", "-count=1")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "VIVY_P9_EMIT_DIAGNOSTIC=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("anchored test %s failed: %v\n%s", testName, err, output)
	}
	const marker = "VIVY_P9_DIAGNOSTIC="
	index := bytes.Index(output, []byte(marker))
	if index < 0 {
		t.Fatalf("anchored test %s did not emit its production diagnostic:\n%s", testName, output)
	}
	diagnostic := output[index+len(marker):]
	if newline := bytes.IndexByte(diagnostic, '\n'); newline >= 0 {
		diagnostic = diagnostic[:newline]
	}
	return strings.TrimSpace(string(diagnostic))
}

var matrixSnapshotPathPattern = regexp.MustCompile(`(?:[A-Za-z]:)?/[^\s;]*vivy-(?:source-snapshot|sources)-[^/\s;]+`)

func redactMatrixDiagnostic(observed, root, output string) string {
	observed = filepath.ToSlash(observed)
	for _, replacement := range []struct{ from, to string }{
		{filepath.ToSlash(output), "<output>"},
		{filepath.ToSlash(root), "<tmp>"},
	} {
		observed = strings.ReplaceAll(observed, replacement.from, replacement.to)
	}
	observed = matrixSnapshotPathPattern.ReplaceAllString(observed, "<source-snapshot>")
	return strings.TrimSpace(observed)
}

func executeCatalogMatrixCase(t *testing.T, id, root, output string) string {
	t.Helper()
	const oldDigest = "c106d108fd1510eba19351b3eed7d87338fe777188d75ed942db61443d72b8cc"
	source := filepath.Join(root, "full-ui-module")
	if err := copySourceTree("testdata/full-ui-module", source); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(source, "i18n", "catalog.json")
	var catalog []byte
	switch id {
	case "catalog-byte-limit":
		catalog = []byte(strings.Repeat("x", (1<<20)+1))
	case "catalog-duplicate-key":
		catalog = []byte(`{"apiVersion":"vivy.i18n/v1","apiVersion":"vivy.i18n/v1","units":{}}`)
	case "catalog-message-limit":
		catalog = matrixCatalogWithUnit("plugin.fixture/full-ui.selected", []string{}, map[string]string{"en": strings.Repeat("x", 8193)})
	case "catalog-missing-english":
		catalog = matrixCatalogWithUnit("plugin.fixture/full-ui.selected", []string{}, map[string]string{"zh": "缺少英文"})
	case "catalog-placeholder-drift":
		catalog = matrixCatalogWithUnit("plugin.fixture/full-ui.selected", []string{"count"}, map[string]string{"en": "No declared placeholder here"})
	case "catalog-placeholder-limit":
		placeholders := make([]string, 33)
		for index := range placeholders {
			placeholders[index] = fmt.Sprintf("p%02d", index)
		}
		sort.Strings(placeholders)
		catalog = matrixCatalogWithUnit("plugin.fixture/full-ui.selected", placeholders, map[string]string{"en": "too many"})
	case "catalog-symlink-escape":
		outside := filepath.Join(root, "outside-catalog.json")
		if err := os.WriteFile(outside, []byte(`{"apiVersion":"vivy.i18n/v1","units":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(catalogPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, catalogPath); err != nil {
			t.Fatal(err)
		}
		recipeRaw, err := os.ReadFile("testdata/full-ui.vivy.yml")
		if err != nil {
			t.Fatal(err)
		}
		return runMatrixPack(t, root, output, string(recipeRaw), []string{source})
	case "catalog-unit-limit":
		units := make(map[string]any, 4097)
		for index := 0; index < 4097; index++ {
			units[fmt.Sprintf("plugin.fixture/full-ui.unit-%04d", index)] = map[string]any{
				"description": "unit", "placeholders": []string{}, "messages": map[string]string{"en": "unit"},
			}
		}
		catalog, _ = json.Marshal(map[string]any{"apiVersion": "vivy.i18n/v1", "units": units})
	case "catalog-unknown-field":
		catalog = []byte(`{"apiVersion":"vivy.i18n/v1","unknown":true,"units":{}}`)
	case "catalog-wrong-owner":
		catalog = matrixCatalogWithUnit("plugin.other.selected", []string{}, map[string]string{"en": "wrong owner"})
	default:
		t.Fatalf("unhandled catalog matrix case %s", id)
	}
	if err := os.WriteFile(catalogPath, catalog, 0o600); err != nil {
		t.Fatal(err)
	}
	descriptorPath := filepath.Join(source, "vivy-module.yaml")
	descriptor, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	newDigest, err := assemblyv1.HashSourceTree(source, oldDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptorPath, bytes.ReplaceAll(descriptor, []byte(oldDigest), []byte(newDigest)), 0o600); err != nil {
		t.Fatal(err)
	}
	recipe, err := os.ReadFile("testdata/full-ui.vivy.yml")
	if err != nil {
		t.Fatal(err)
	}
	recipe = bytes.ReplaceAll(recipe, []byte(oldDigest), []byte(newDigest))
	return runMatrixPack(t, root, output, string(recipe), []string{source})
}

func matrixCatalogWithUnit(key string, placeholders []string, messages map[string]string) []byte {
	encoded, _ := json.Marshal(map[string]any{
		"apiVersion": "vivy.i18n/v1",
		"units": map[string]any{key: map[string]any{
			"description": "matrix unit", "placeholders": placeholders, "messages": messages,
		}},
	})
	return encoded
}
