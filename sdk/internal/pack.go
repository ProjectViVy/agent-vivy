package sdk

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"agent-vivy/internal/domain"
)

const fileRefPrefix = "file:"

// Artifact is the pack output beside the new EXE.
type Artifact struct {
	ID             string                 `json:"id"`
	ArtifactSHA256 string                 `json:"artifact_sha256"`
	SourceRef      string                 `json:"source_ref"`
	Recipe         domain.AssemblyRecipe  `json:"recipe"`
	Phase          domain.GenerationPhase `json:"phase"`
	Tools          []artifactTool         `json:"tools,omitempty"`
}

type artifactTool struct {
	Name     string `json:"name"`
	Readonly bool   `json:"readonly"`
}

type packOptions struct {
	With []string
	Out  string
}

func parsePackArgs(args []string) (packOptions, error) {
	var opt packOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--with":
			if i+1 >= len(args) {
				return packOptions{}, fmt.Errorf("sdk: --with needs a plugin name or directory")
			}
			i++
			opt.With = append(opt.With, args[i])
		case "--out":
			if i+1 >= len(args) {
				return packOptions{}, fmt.Errorf("sdk: --out needs a directory")
			}
			i++
			opt.Out = args[i]
		default:
			return packOptions{}, fmt.Errorf("sdk: unknown pack flag %q", args[i])
		}
	}
	return opt, nil
}

// Pack verifies named plugins and links them into a new species EXE.
func Pack(opt packOptions) (Artifact, error) {
	if len(opt.With) == 0 {
		return Artifact{}, fmt.Errorf("sdk: pack requires --with <plugin>")
	}
	root, err := findModuleRoot(".")
	if err != nil {
		return Artifact{}, err
	}
	type packedPlugin struct {
		dir     string
		name    string
		pkg     string
		impPath string
		man     manifest
	}
	var selected []packedPlugin
	var tools []artifactTool
	var names []string
	for _, spec := range opt.With {
		dir, err := resolvePluginDir(root, spec)
		if err != nil {
			return Artifact{}, err
		}
		rep, err := Verify(dir)
		if err != nil {
			return Artifact{}, err
		}
		if !rep.OK {
			return Artifact{}, fmt.Errorf("sdk: verify %s failed: %s", dir, strings.Join(rep.Issues, "; "))
		}
		man, err := loadManifest(dir)
		if err != nil {
			return Artifact{}, err
		}
		pkg, err := pluginPackageName(dir)
		if err != nil {
			return Artifact{}, err
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return Artifact{}, err
		}
		selected = append(selected, packedPlugin{
			dir: dir, name: man.Name, pkg: pkg,
			impPath: "agent-vivy/" + filepath.ToSlash(rel),
			man:     man,
		})
		names = append(names, man.Name)
		for _, tool := range man.Tools {
			tools = append(tools, artifactTool{Name: tool.Name, Readonly: tool.Effect == "read"})
		}
	}
	goBin, err := goToolchain()
	if err != nil {
		return Artifact{}, err
	}
	id := newGenerationID()
	outDir := opt.Out
	if outDir == "" {
		outDir = filepath.Join(root, "dist", id)
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return Artifact{}, fmt.Errorf("sdk: create out dir: %w", err)
	}
	exeName := "vivy"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exePath := filepath.Join(outDir, exeName)
	gens := make([]packedPluginForGen, 0, len(selected))
	for _, p := range selected {
		gens = append(gens, packedPluginForGen{pkg: p.pkg, impPath: p.impPath})
	}
	registerSrc := generateRegister(gens)
	tmp, err := os.MkdirTemp("", "vivy-pack-*")
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	overlaySrc := filepath.Join(tmp, "zz_register.go")
	if err := os.WriteFile(overlaySrc, []byte(registerSrc), 0o600); err != nil {
		return Artifact{}, err
	}
	liveRegister := filepath.Join(root, "internal", "generated", "plugins", "zz_register.go")
	overlayDoc := map[string]map[string]string{
		"Replace": {liveRegister: overlaySrc},
	}
	overlayJSON, err := json.Marshal(overlayDoc)
	if err != nil {
		return Artifact{}, err
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlayJSON, 0o600); err != nil {
		return Artifact{}, err
	}
	cmd := exec.Command(goBin, "build", "-overlay", overlayPath, "-o", exePath, "./cmd/vivy")
	cmd.Dir = root
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(exePath)
		return Artifact{}, fmt.Errorf("sdk: pack build failed: %s", strings.TrimSpace(string(out)))
	}
	sum, err := hashFile(exePath)
	if err != nil {
		return Artifact{}, err
	}
	absExe, err := filepath.Abs(exePath)
	if err != nil {
		absExe = exePath
	}
	art := Artifact{
		ID:             id,
		ArtifactSHA256: sum,
		SourceRef:      fileRefPrefix + absExe,
		Recipe: domain.AssemblyRecipe{
			Loop:    "eino",
			World:   "sandbox",
			Plugins: names,
		},
		Phase: domain.GenerationBuilt,
		Tools: tools,
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(outDir, "generation.json"), raw, 0o600); err != nil {
		return Artifact{}, err
	}
	return art, nil
}

func InspectArtifact(dir string) (Artifact, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "generation.json"))
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: inspect-artifact: %w", err)
	}
	var art Artifact
	if err := json.Unmarshal(raw, &art); err != nil {
		return Artifact{}, fmt.Errorf("sdk: inspect-artifact: %w", err)
	}
	if art.ID == "" || art.ArtifactSHA256 == "" || art.SourceRef == "" {
		return Artifact{}, fmt.Errorf("sdk: inspect-artifact: incomplete generation.json")
	}
	return art, nil
}

func findModuleRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(data), "module agent-vivy") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("sdk: species source tree not found")
		}
		dir = parent
	}
}

func resolvePluginDir(root, spec string) (string, error) {
	candidates := []string{spec, filepath.Join(root, spec), filepath.Join(root, "plugins", spec)}
	for _, candidate := range candidates {
		info, err := os.Stat(filepath.Join(candidate, "vivy-plugin.json"))
		if err == nil && !info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("sdk: plugin %q not found", spec)
}

func pluginPackageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.PackageClauseOnly)
		if err != nil {
			return "", err
		}
		if file.Name.Name == "main" {
			return "", fmt.Errorf("sdk: %s is package main", dir)
		}
		return file.Name.Name, nil
	}
	return "", fmt.Errorf("sdk: no Go package in %s", dir)
}

func generateRegister(plugins []packedPluginForGen) string {
	var b strings.Builder
	b.WriteString("// Code generated by vivy-sdk pack. DO NOT EDIT.\n\npackage plugins\n\nimport (\n")
	for _, p := range plugins {
		fmt.Fprintf(&b, "\t%s %q\n", p.pkg, p.impPath)
	}
	b.WriteString("\t\"agent-vivy/sdk/plugin\"\n)\n\nfunc Register() []plugin.Plugin {\n\treturn []plugin.Plugin{\n")
	for _, p := range plugins {
		fmt.Fprintf(&b, "\t\t%s.New(),\n", p.pkg)
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}

type packedPluginForGen struct {
	pkg     string
	impPath string
}

func goToolchain() (string, error) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}
	if _, err := os.Stat(goBin); err != nil {
		return "", fmt.Errorf("sdk: go toolchain missing")
	}
	return goBin, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func newGenerationID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("gen_%x", time.Now().UnixNano())
	}
	return "gen_" + hex.EncodeToString(b[:])
}
