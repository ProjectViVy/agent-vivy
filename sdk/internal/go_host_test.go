package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
)

// goHostFixture copies the committed minimal host into a writable temp
// module. The checked-in fixture's replace only points agent-vivy at the
// repository root, so the copy must be adjusted to the staged layout.
func stageGoHostFixture(t *testing.T, vivyRoot string) string {
	t.Helper()
	source := filepath.Join("testdata", "go-host")
	destination := filepath.Join(t.TempDir(), "go-host")
	if err := copySourceTree(source, destination); err != nil {
		t.Fatalf("stage go-host fixture: %v", err)
	}
	modfile := filepath.Join(destination, "go.mod")
	raw, err := os.ReadFile(modfile)
	if err != nil {
		t.Fatalf("read fixture modfile: %v", err)
	}
	staged := strings.Replace(string(raw), "replace agent-vivy => ../../../..", "replace agent-vivy => "+vivyRoot, 1)
	if staged == string(raw) {
		t.Fatal("fixture modfile lacks the agent-vivy pin")
	}
	if err := os.WriteFile(modfile, []byte(staged), 0o600); err != nil {
		t.Fatalf("rewrite fixture modfile: %v", err)
	}
	return destination
}

// A consumer module sees only its own go.mod: agent-vivy's local replaces
// (ProjectViVy laputa modules, agent-vivy/* submodules, bml) are not
// inherited, so the fixture cannot compile until the pack target generates
// the consumer modfile with the full replacement closure (W3-4).
func TestGoHostFixtureRequiresGeneratedReplaceClosure(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	repoRoot, err = filepath.Abs(filepath.Join(repoRoot, ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := stageGoHostFixture(t, repoRoot)
	cmd := exec.Command("go", "build", "-o", filepath.Join(t.TempDir(), "go-host"), "./cmd/gohost")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("go-host fixture built without the generated replacement closure; replaces were inherited unexpectedly")
	}
	text := string(output)
	for _, missing := range []string{"ProjectViVy/laputa/laputa", "ProjectViVy/laputa/garden", "agent-vivy/bml", "agent-vivy/plugins/"} {
		if strings.Contains(text, missing) {
			return
		}
	}
	t.Fatalf("build failed for an unexpected reason, not the missing replace closure: %s", text)
}

func writeGoHostLock(t *testing.T, dir string, mutate func(map[string]any)) string {
	t.Helper()
	lock := map[string]any{
		"schema":  "diva.go-host-inputs/v1",
		"release": false,
		"host":    map[string]any{"modulePath": "example.com/vivy-go-host", "repository": "https://example.invalid/go-host", "commit": strings.Repeat("a", 40)},
		"sources": map[string]any{
			"vivy":   map[string]any{"repository": "https://github.com/ProjectViVy/agent-vivy", "commit": strings.Repeat("c", 40)},
			"laputa": map[string]any{"repository": "https://github.com/ProjectViVy/laputa", "commit": strings.Repeat("e", 40)},
			"inofy":  map[string]any{"module": "github.com/ProjectViVy/inofy", "version": "v0.0.0-20260930141905-71e2c9bbe47d", "commit": "71e2c9bbe47d"},
		},
		"recipe": map[string]any{"path": "recipes/diva.vivy.yml"},
		"tools":  map[string]any{"go": "go1.26.4"},
	}
	if mutate != nil {
		mutate(lock)
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vivy-sources.lock.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func makeGoHostDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/vivy-go-host\n\ngo 1.26.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"cmd/diva", "dist", "agent-diva-gui/dist"} {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(full, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(full, "index.html"), []byte("<html/>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPackGoHostRejectsHostFlagsOnOtherTargets(t *testing.T) {
	for _, target := range []string{"executable", "shared"} {
		_, err := parsePackArgs([]string{"--recipe", "r.yml", "--output", "o", "--target", target, "--host-dir", "h"})
		if err == nil {
			t.Fatalf("host flag accepted on %s target", target)
		}
	}
}

func TestPackGoHostRequiresEveryHostFlag(t *testing.T) {
	_, err := parsePackArgs([]string{"--recipe", "r.yml", "--output", "o", "--target", "go-host", "--host-dir", "h", "--host-package", "./cmd/diva"})
	if err == nil || !strings.Contains(err.Error(), "--host-") {
		t.Fatalf("missing host flags not rejected: %v", err)
	}
}

func TestPackGoHostRejectsUnsafeHostPaths(t *testing.T) {
	hostDir := makeGoHostDir(t)
	lock := writeGoHostLock(t, t.TempDir(), nil)
	base := []string{"--recipe", "recipes/diva.vivy.yml", "--output", filepath.Join(t.TempDir(), "out"), "--target", "go-host", "--host-dir", hostDir, "--host-lock", lock}
	for name, bad := range map[string]struct{ flag, value string }{
		"host-package-absolute":  {"--host-package", "/abs/cmd"},
		"host-package-traversal": {"--host-package", "../escape"},
		"host-assets-traversal":  {"--host-assets", "../escape"},
	} {
		args := append(append([]string{}, base...), bad.flag, bad.value)
		if bad.flag == "--host-package" {
			args = append(args, "--host-assets", "dist")
		} else {
			args = append(args, "--host-package", "./cmd/diva")
		}
		if _, err := parsePackArgs(args); err == nil {
			t.Fatalf("%s not rejected", name)
		}
	}
}

func TestPackGoHostRejectsMissingInputs(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	lock := writeGoHostLock(t, t.TempDir(), nil)
	// host-dir without go.mod
	badHost := t.TempDir()
	_, err := parsePackArgs([]string{"--recipe", "recipes/diva.vivy.yml", "--output", outDir, "--target", "go-host", "--host-dir", badHost, "--host-package", "./cmd/diva", "--host-assets", "dist", "--host-lock", lock})
	if err == nil {
		t.Fatal("host-dir without go.mod accepted")
	}
	// missing lock file
	hostDir := makeGoHostDir(t)
	_, err = parsePackArgs([]string{"--recipe", "recipes/diva.vivy.yml", "--output", outDir, "--target", "go-host", "--host-dir", hostDir, "--host-package", "./cmd/diva", "--host-assets", "dist", "--host-lock", filepath.Join(t.TempDir(), "missing.json")})
	if err == nil {
		t.Fatal("missing host-lock accepted")
	}
	// wrong lock schema
	badLock := writeGoHostLock(t, t.TempDir(), func(lock map[string]any) { lock["schema"] = "diva.other/v9" })
	_, err = parsePackArgs([]string{"--recipe", "recipes/diva.vivy.yml", "--output", outDir, "--target", "go-host", "--host-dir", hostDir, "--host-package", "./cmd/diva", "--host-assets", "dist", "--host-lock", badLock})
	if err == nil {
		t.Fatal("lock with unknown schema accepted")
	}
}

func TestParsePackGoHostHappyPath(t *testing.T) {
	hostDir := makeGoHostDir(t)
	lock := writeGoHostLock(t, t.TempDir(), nil)
	o, err := parsePackArgs([]string{"--recipe", "recipes/diva.vivy.yml", "--output", "out", "--target", "go-host", "--host-dir", hostDir, "--host-package", "./cmd/diva", "--host-assets", "agent-diva-gui/dist", "--host-lock", lock})
	if err != nil {
		t.Fatalf("valid go-host args rejected: %v", err)
	}
	if o.Target != "go-host" || o.GoHost.Package != "cmd/diva" || o.GoHost.Assets != "agent-diva-gui/dist" || o.GoHost.Dir != hostDir || o.GoHost.Lock != lock {
		t.Fatalf("host options not normalized: %+v", o)
	}
}

// goHostUniverse builds the minimal sealed universe the go-host pack target
// consumes: a git-committed host fixture, and a lock computed from the live
// agent-vivy and laputa worktrees (release is off, so local dirt is allowed).
func goHostUniverse(t *testing.T) (packOptions, string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	laputaRoot, err := filepath.Abs(filepath.Join(repoRoot, "..", "laputa"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(laputaRoot, ".git")); err != nil {
		t.Skip("go-host pack tests need the laputa sibling checkout")
	}
	hostDir := stageGoHostFixture(t, repoRoot)
	run := func(args ...string) string {
		out, err := gitOutput(hostDir, args...)
		if err != nil {
			t.Fatalf("fixture git %v: %v", args, err)
		}
		return out
	}
	run("init", "-q")
	run("remote", "add", "origin", "https://example.invalid/go-host")
	run("-c", "user.name=test", "-c", "user.email=test@example.invalid", "add", "-A")
	// Fixed author/committer dates make the fixture commit SHA reproducible so
	// two unrelated staging locations seal the same hostBuild provenance.
	commit := exec.Command("git", "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	commit.Dir = hostDir
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("fixture commit: %v: %s", err, out)
	}
	sourcePin := func(dir string) goHostLockSource {
		head, err := gitHead(dir)
		if err != nil {
			t.Fatal(err)
		}
		return goHostLockSource{Repository: gitRemoteURL(dir), Commit: head}
	}
	recipe := filepath.Join(repoRoot, "recipes", "diva.vivy.yml")
	host := sourcePin(hostDir)
	vivy := sourcePin(repoRoot)
	laputa := sourcePin(laputaRoot)
	lockDir := t.TempDir()
	lockPath := filepath.Join(lockDir, "vivy-sources.lock.json")
	lock := map[string]any{
		"schema":  "diva.go-host-inputs/v1",
		"release": false,
		"host":    map[string]any{"modulePath": "example.com/vivy-go-host", "repository": "https://example.invalid/go-host", "commit": host.Commit},
		"sources": map[string]any{
			"vivy":   map[string]any{"repository": vivy.Repository, "commit": vivy.Commit},
			"laputa": map[string]any{"repository": laputa.Repository, "commit": laputa.Commit},
			"inofy":  map[string]any{"module": "github.com/ProjectViVy/inofy", "version": "v0.0.0-20260930141905-71e2c9bbe47d", "commit": "71e2c9bbe47d"},
		},
		"recipe": map[string]any{"path": "recipes/diva.vivy.yml"},
		"tools":  map[string]any{"go": "go1.26.4"},
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return packOptions{
		Recipe:      recipe,
		Output:      filepath.Join(t.TempDir(), "artifact"),
		Target:      "go-host",
		HostDir:     hostDir,
		HostPackage: "./cmd/gohost",
		HostAssets:  "agent-diva-gui/dist",
		HostLock:    lockPath,
	}, hostDir
}

// The pack target must compile the real fixture host under the generated
// consumer modfile and seal hostBuild provenance into the Generation.
func TestPackGoHostSealedExternalArtifact(t *testing.T) {
	opts, hostDir := goHostUniverse(t)
	artifact, err := Pack(context.Background(), opts)
	if err != nil {
		t.Fatalf("pack go-host: %v", err)
	}
	binaryName := artifactHostBinaryName(runtime.GOOS, "gohost")
	binary := filepath.Join(opts.Output, binaryName)
	info, err := os.Stat(binary)
	if err != nil || info.Size() == 0 {
		t.Fatalf("host executable missing: %v", err)
	}
	if artifact.Binary != binary {
		t.Fatalf("artifact binary %q, want %q", artifact.Binary, binary)
	}
	raw, err := os.ReadFile(filepath.Join(opts.Output, "generation.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := assemblyv1.InspectManifest(raw)
	if err != nil {
		t.Fatalf("sealed manifest invalid: %v", err)
	}
	if manifest.HostBuild == nil {
		t.Fatal("manifest missing hostBuild")
	}
	hostHead, _ := gitHead(hostDir)
	build := manifest.HostBuild
	if build.Schema != assemblyv1.HostBuildSchema || build.ModulePath != "example.com/vivy-go-host" || build.Package != "./cmd/gohost" || build.Source.Commit != hostHead {
		t.Fatalf("hostBuild provenance wrong: %+v", build)
	}
	binaryRaw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := generation.ExtractEmbeddedManifest(binaryRaw)
	if err != nil {
		t.Fatalf("embedded manifest missing from host binary: %v", err)
	}
	if !bytes.Equal(embedded, raw) {
		t.Fatal("embedded manifest does not match generation.json")
	}
	frontend := filepath.Join(opts.Output, "frontend")
	if _, err := os.Stat(filepath.Join(frontend, "index.html")); err != nil {
		t.Fatalf("frontend index missing: %v", err)
	}
	report, err := os.ReadFile(filepath.Join(opts.Output, "build-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(report, &decoded); err != nil {
		t.Fatalf("build report invalid: %v", err)
	}
	if decoded["schema"] != "vivy.go-host-report/v1" || decoded["generationId"] != manifest.GenerationID {
		t.Fatalf("build report identity wrong: %s", report)
	}
	if _, err := os.Stat(filepath.Join(opts.Output, "checksums.sha256")); !os.IsNotExist(err) {
		t.Fatalf("retired checksum artifact exists or stat failed: %v", err)
	}
	for _, field := range []string{"treeSHA256", "consumerModfileSHA256", "consumerSumSHA256", "dependencyLockSHA256"} {
		if bytes.Contains(raw, []byte(field)) || bytes.Contains(report, []byte(field)) {
			t.Fatalf("retired field %s remains in host artifact", field)
		}
	}
	inspected, err := InspectArtifact(opts.Output)
	if err != nil {
		t.Fatalf("inspect go-host artifact: %v", err)
	}
	if inspected.Binary != binary || inspected.Manifest.HostBuild == nil || inspected.Manifest.GenerationID != manifest.GenerationID {
		t.Fatalf("inspect returned wrong artifact: %+v", inspected.Manifest.HostBuild)
	}
}

// mutateGoHostLock rewrites the lock document produced by goHostUniverse.
func mutateGoHostLock(t *testing.T, lockPath string, mutate func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	mutate(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Native source pins still reject a different commit before staging.
func TestPackGoHostRejectsSourceCommitMismatch(t *testing.T) {
	opts, _ := goHostUniverse(t)
	mutateGoHostLock(t, opts.HostLock, func(doc map[string]any) {
		doc["sources"].(map[string]any)["vivy"].(map[string]any)["commit"] = strings.Repeat("0", 40)
	})
	if _, err := Pack(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "source commit") {
		t.Fatalf("source commit mismatch not rejected: %v", err)
	}
}

// Recipe identity is its explicit path, independent of its file bytes.
func TestPackGoHostRejectsRecipePathMismatch(t *testing.T) {
	opts, _ := goHostUniverse(t)
	mutateGoHostLock(t, opts.HostLock, func(doc map[string]any) {
		doc["recipe"].(map[string]any)["path"] = "recipes/minimal.vivy.yml"
	})
	if _, err := Pack(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "Recipe path") {
		t.Fatalf("Recipe path mismatch not rejected: %v", err)
	}
}

// Release mode refuses to build from a dirty or unpinned worktree.
func TestPackGoHostRejectsDirtyRelease(t *testing.T) {
	opts, hostDir := goHostUniverse(t)
	mutateGoHostLock(t, opts.HostLock, func(doc map[string]any) {
		doc["release"] = true
	})
	if err := os.WriteFile(filepath.Join(hostDir, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(context.Background(), opts); err == nil {
		t.Fatal("release pack accepted a dirty source tree")
	}
}

// Inspect validates the sidecar against the framed executable manifest.
// Frontend bytes carry no Vivy-computed content claim.
func TestInspectGoHostRejectsTamperedArtifact(t *testing.T) {
	opts, _ := goHostUniverse(t)
	if _, err := Pack(context.Background(), opts); err != nil {
		t.Fatalf("pack go-host: %v", err)
	}
	copyArtifact := func() string {
		dest := filepath.Join(t.TempDir(), "artifact")
		if err := copySourceTree(opts.Output, dest); err != nil {
			t.Fatal(err)
		}
		return dest
	}

	t.Run("frontend byte", func(t *testing.T) {
		dir := copyArtifact()
		target := filepath.Join(dir, "frontend", "index.html")
		body, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		body[0] ^= 0xFF
		if err := os.WriteFile(target, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectArtifact(dir); err != nil {
			t.Fatalf("frontend bytes incorrectly treated as an identity claim: %v", err)
		}
	})
	t.Run("sidecar manifest", func(t *testing.T) {
		dir := copyArtifact()
		target := filepath.Join(dir, "generation.json")
		body, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		tampered := bytes.Replace(body, []byte("\"vivy.go-host/v1\""), []byte("\"vivy.go-host/v9\""), 1)
		if bytes.Equal(tampered, body) {
			t.Fatal("generation.json lacks hostBuild schema marker")
		}
		if err := os.WriteFile(target, tampered, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectArtifact(dir); err == nil {
			t.Fatal("inspect accepted a tampered sidecar manifest")
		}
	})
	t.Run("embedded manifest", func(t *testing.T) {
		dir := copyArtifact()
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var binary string
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "gohost") {
				binary = filepath.Join(dir, entry.Name())
			}
		}
		if binary == "" {
			t.Fatal("host binary missing")
		}
		body, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		marker := []byte("VIVY_GENERATION_V1_BEGIN[")
		at := bytes.Index(body, marker)
		if at < 0 {
			t.Fatal("embedded manifest marker missing")
		}
		body[at+len(marker)+16] ^= 0x01
		if err := os.WriteFile(binary, body, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectArtifact(dir); err == nil {
			t.Fatal("inspect accepted a tampered embedded manifest")
		}
	})
}

// Each pack allocates a fresh opaque identity even when authoring inputs
// supply an existing Generation ID.
func TestPackGoHostAllocatesFreshGenerationID(t *testing.T) {
	first, _ := goHostUniverse(t)
	second, _ := goHostUniverse(t)
	first.GenerationID, second.GenerationID = "authoring-go-host", "authoring-go-host"
	firstArtifact, err := Pack(context.Background(), first)
	if err != nil {
		t.Fatalf("first pack: %v", err)
	}
	secondArtifact, err := Pack(context.Background(), second)
	if err != nil {
		t.Fatalf("second pack: %v", err)
	}
	ids := []string{firstArtifact.Manifest.GenerationID, secondArtifact.Manifest.GenerationID}
	if ids[0] == "" || ids[1] == "" || ids[0] == ids[1] || ids[0] == first.GenerationID || ids[1] == second.GenerationID {
		t.Fatalf("pack did not allocate distinct identities: %q", ids)
	}
	for _, artifact := range []Artifact{firstArtifact, secondArtifact} {
		ui, err := os.ReadFile(filepath.Join(artifact.Directory, "ui-assembly.ts"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(ui, []byte(artifact.Manifest.GenerationID)) {
			t.Fatal("UI and manifest Generation identity differ")
		}
		inspected, err := InspectArtifact(artifact.Directory)
		if err != nil {
			t.Fatal(err)
		}
		if inspected.Manifest.GenerationID != artifact.Manifest.GenerationID {
			t.Fatal("Inspect and manifest Generation identity differ")
		}
	}
}

// An existing nonempty output directory is never overwritten.
func TestPackGoHostRejectsOutputReuse(t *testing.T) {
	opts, _ := goHostUniverse(t)
	if _, err := Pack(context.Background(), opts); err != nil {
		t.Fatalf("pack go-host: %v", err)
	}
	if _, err := Pack(context.Background(), opts); err == nil {
		t.Fatal("pack overwrote an existing output directory")
	}
}

// The tracked lock must not pin its own DIVA commit: packing succeeds without
// host.commit and hostBuild records the live resolved identity.
func TestPackGoHostResolvesUnpinnedHostIdentity(t *testing.T) {
	opts, hostDir := goHostUniverse(t)
	mutateGoHostLock(t, opts.HostLock, func(doc map[string]any) {
		host := doc["host"].(map[string]any)
		delete(host, "commit")
	})
	if _, err := Pack(context.Background(), opts); err != nil {
		t.Fatalf("pack go-host with unpinned host identity: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(opts.Output, "generation.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := assemblyv1.InspectManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	head, err := gitHead(hostDir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.HostBuild == nil || manifest.HostBuild.Source.Commit != head {
		t.Fatalf("hostBuild source identity wrong: %+v", manifest.HostBuild)
	}
}

// Build input declarations retain native source identity without tree or
// dependency-lock digests.
func TestParseGoHostLockAcceptsNativeIdentity(t *testing.T) {
	lockPath := writeGoHostLock(t, t.TempDir(), nil)
	if _, err := parseGoHostLock(lockPath); err != nil {
		t.Fatalf("native source identity rejected: %v", err)
	}
}

func TestParseGoHostLockRejectsInvalidNativeIdentity(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"commit": func(doc map[string]any) {
			doc["sources"].(map[string]any)["vivy"].(map[string]any)["commit"] = "floating"
		},
		"Recipe traversal": func(doc map[string]any) { doc["recipe"].(map[string]any)["path"] = "../escape.yml" },
		"Recipe absolute":  func(doc map[string]any) { doc["recipe"].(map[string]any)["path"] = "/escape.yml" },
		"Recipe absent":    func(doc map[string]any) { delete(doc, "recipe") },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGoHostLock(writeGoHostLock(t, t.TempDir(), mutate)); err == nil {
				t.Fatal("invalid native identity accepted")
			}
		})
	}
}

// Keep remote identity and release cleanliness checks independent of a full
// product build so each refusal proves the intended owner.
func TestVerifyGoHostLockNativeIdentity(t *testing.T) {
	repo := func() string {
		dir := t.TempDir()
		for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://example.invalid/source"}, {"-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "fixture"}} {
			if _, err := gitOutput(dir, args...); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	vivy, laputa, host := repo(), repo(), repo()
	pin := func(dir string) goHostLockSource {
		head, err := gitHead(dir)
		if err != nil {
			t.Fatal(err)
		}
		return goHostLockSource{Repository: gitRemoteURL(dir), Commit: head}
	}
	var lock goHostLock
	lock.Sources.Vivy, lock.Sources.Laputa = pin(vivy), pin(laputa)
	lock.Host.Repository = gitRemoteURL(host)
	inputs := goHostInputs{Dir: host, LockDoc: lock}
	if err := verifyGoHostLock(inputs, vivy, laputa); err != nil {
		t.Fatal(err)
	}
	t.Run("remote", func(t *testing.T) {
		bad := inputs
		bad.LockDoc.Sources.Vivy.Repository = "https://example.invalid/other"
		if err := verifyGoHostLock(bad, vivy, laputa); err == nil || !strings.Contains(err.Error(), "source repository") {
			t.Fatalf("remote mismatch not rejected: %v", err)
		}
	})
	t.Run("commit", func(t *testing.T) {
		bad := inputs
		bad.LockDoc.Sources.Vivy.Commit = strings.Repeat("0", 40)
		if err := verifyGoHostLock(bad, vivy, laputa); err == nil || !strings.Contains(err.Error(), "source commit") {
			t.Fatalf("commit mismatch not rejected: %v", err)
		}
	})
	if err := os.WriteFile(filepath.Join(host, "untracked"), []byte("development bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyGoHostLock(inputs, vivy, laputa); err != nil {
		t.Fatalf("development input rejected: %v", err)
	}
	inputs.LockDoc.Release = true
	if err := verifyGoHostLock(inputs, vivy, laputa); err == nil || !strings.Contains(err.Error(), "dirty host") {
		t.Fatalf("dirty release not rejected by host owner: %v", err)
	}
}
