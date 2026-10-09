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

func TestDirtySourceSnapshotIncludesAddedFilesAndOmitsDeletedFiles(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	write("tracked.txt", "old")
	write(".gitignore", "ignored.txt\n")
	run("add", "tracked.txt", ".gitignore")
	if err := os.Remove(filepath.Join(dir, "tracked.txt")); err != nil {
		t.Fatal(err)
	}
	write("added.txt", "first")
	write("ignored.txt", "ignored")

	first, err := hashSourceTree(dir)
	if err != nil {
		t.Fatalf("hash dirty source tree: %v", err)
	}
	write("added.txt", "second")
	second, err := hashSourceTree(dir)
	if err != nil {
		t.Fatalf("hash changed dirty source tree: %v", err)
	}
	if first == second {
		t.Fatal("untracked source content was not sealed")
	}
	buildStage, err := os.MkdirTemp(dir, ".vivy-pack-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildStage, "zz_assembly.go"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	withStage, err := hashSourceTree(dir)
	if err != nil || withStage != second {
		t.Fatalf("SDK staging changed its own source pin: %s != %s, %v", withStage, second, err)
	}

	staged := t.TempDir()
	if err := stageSourceTree(dir, staged); err != nil {
		t.Fatalf("stage dirty source tree: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(staged, "added.txt")); err != nil || string(got) != "second" {
		t.Fatalf("added source was not staged: %q, %v", got, err)
	}
	for _, name := range []string{"tracked.txt", "ignored.txt", filepath.Base(buildStage)} {
		if _, err := os.Stat(filepath.Join(staged, name)); !os.IsNotExist(err) {
			t.Fatalf("%s unexpectedly staged: %v", name, err)
		}
	}
	run("add", filepath.Join(filepath.Base(buildStage), "zz_assembly.go"))
	withTrackedStage, err := hashSourceTree(dir)
	if err != nil || withTrackedStage == second {
		t.Fatalf("explicitly tracked source was excluded: %s, %v", withTrackedStage, err)
	}
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
		"host":    map[string]any{"modulePath": "example.com/vivy-go-host", "repository": "https://example.invalid/go-host", "commit": strings.Repeat("a", 40), "treeSHA256": strings.Repeat("b", 64)},
		"sources": map[string]any{
			"vivy":   map[string]any{"repository": "https://github.com/ProjectViVy/agent-vivy", "commit": strings.Repeat("c", 40), "treeSHA256": strings.Repeat("d", 64)},
			"laputa": map[string]any{"repository": "https://github.com/ProjectViVy/laputa", "commit": strings.Repeat("e", 40), "treeSHA256": strings.Repeat("f", 64)},
			"inofy":  map[string]any{"module": "github.com/ProjectViVy/inofy", "version": "v0.0.0-20260930141905-71e2c9bbe47d", "commit": "71e2c9bbe47d"},
		},
		"recipe": map[string]any{"path": "recipes/diva.vivy.yml", "sha256": strings.Repeat("0", 64)},
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

func TestPrepareConsumerModfileMapsHostLaputaReplaceToSnapshot(t *testing.T) {
	root := t.TempDir()
	repoRoot := filepath.Join(root, "vivy")
	laputaRoot := filepath.Join(root, "laputa")
	hostRoot := filepath.Join(root, "host-source")
	stageRoot := filepath.Join(root, "stage")
	for _, dir := range []string{repoRoot, laputaRoot, hostRoot, filepath.Join(hostRoot, "deps", "laputa", "garden")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(repoRoot, "go.mod"), "module agent-vivy\n\ngo 1.26.4\n")
	write(filepath.Join(laputaRoot, "go.mod"), "module github.com/ProjectViVy/laputa\n\ngo 1.26.4\n")
	hostMod := "module example.com/host\n\ngo 1.26.4\n\nreplace github.com/dashimaki/garden => ./deps/laputa/garden\n"
	write(filepath.Join(hostRoot, "go.mod"), hostMod)

	staged := &goHostStaged{
		Root: stageRoot,
		Vivy: filepath.Join(stageRoot, "vivy"),
		Host: filepath.Join(stageRoot, "host"),
		Deps: filepath.Join(stageRoot, "deps"),
	}
	for _, dir := range []string{
		staged.Vivy,
		staged.Host,
		filepath.Join(staged.Deps, "laputa", "garden"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(staged.Host, "go.mod"), hostMod)
	if err := prepareConsumerModfile(staged, repoRoot, laputaRoot, goHostInputs{Dir: hostRoot}); err != nil {
		t.Fatalf("prepare consumer modfile: %v", err)
	}
	replaces, err := localModfileReplaces(staged.Modfile)
	if err != nil {
		t.Fatalf("read consumer replacements: %v", err)
	}
	for _, replace := range replaces {
		if replace.Old == "github.com/dashimaki/garden" {
			if replace.New != "../deps/laputa/garden" {
				t.Fatalf("host Laputa replace = %q, want locked staged snapshot path", replace.New)
			}
			return
		}
	}
	t.Fatal("consumer modfile dropped the host's declared Laputa replacement")
}

// goHostUniverse builds the minimal sealed universe the go-host pack target
// consumes: a git-committed host fixture, and a lock computed from the live
// agent-vivy and laputa worktrees (release is off, so local dirt is sealed,
// not rejected).
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
		tree, err := hashSourceTree(dir)
		if err != nil {
			t.Fatal(err)
		}
		return goHostLockSource{Repository: "https://example.invalid/repo", Commit: head, TreeSHA256: tree}
	}
	recipe := filepath.Join(repoRoot, "recipes", "diva.vivy.yml")
	recipeRaw, err := os.ReadFile(recipe)
	if err != nil {
		t.Fatal(err)
	}
	host := sourcePin(hostDir)
	vivy := sourcePin(repoRoot)
	laputa := sourcePin(laputaRoot)
	lockDir := t.TempDir()
	lockPath := filepath.Join(lockDir, "vivy-sources.lock.json")
	lock := map[string]any{
		"schema":  "diva.go-host-inputs/v1",
		"release": false,
		"host":    map[string]any{"modulePath": "example.com/vivy-go-host", "repository": "https://example.invalid/go-host", "commit": host.Commit, "treeSHA256": host.TreeSHA256},
		"sources": map[string]any{
			"vivy":   map[string]any{"repository": vivy.Repository, "commit": vivy.Commit, "treeSHA256": vivy.TreeSHA256},
			"laputa": map[string]any{"repository": laputa.Repository, "commit": laputa.Commit, "treeSHA256": laputa.TreeSHA256},
			"inofy":  map[string]any{"module": "github.com/ProjectViVy/inofy", "version": "v0.0.0-20260930141905-71e2c9bbe47d", "commit": "71e2c9bbe47d"},
		},
		"recipe": map[string]any{"path": "recipes/diva.vivy.yml", "sha256": sha256Hex(recipeRaw)},
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
	digest, err := hashUIArtifactTree(frontend)
	if err != nil {
		t.Fatalf("frontend tree missing: %v", err)
	}
	if digest != build.Assets.SHA256 {
		t.Fatalf("frontend digest %s, hostBuild seals %s", digest, build.Assets.SHA256)
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
	checksums, err := os.ReadFile(filepath.Join(opts.Output, "checksums.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(checksums)), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("malformed checksum line %q", line)
		}
		body, err := os.ReadFile(filepath.Join(opts.Output, filepath.FromSlash(parts[1])))
		if err != nil {
			t.Fatalf("checksum references missing file %s", parts[1])
		}
		if sha256Hex(body) != parts[0] {
			t.Fatalf("checksum mismatch for %s", parts[1])
		}
		seen[parts[1]] = true
	}
	for _, required := range []string{binaryName, "generation.json", "frontend/index.html"} {
		if !seen[required] {
			t.Fatalf("checksums missing %s", required)
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

// A lock whose pinned source tree digest no longer matches the real tree must
// stop the pack before any build input is staged.
func TestPackGoHostRejectsSourceHashMismatch(t *testing.T) {
	opts, _ := goHostUniverse(t)
	mutateGoHostLock(t, opts.HostLock, func(doc map[string]any) {
		doc["sources"].(map[string]any)["vivy"].(map[string]any)["treeSHA256"] = strings.Repeat("0", 64)
	})
	if _, err := Pack(context.Background(), opts); err == nil {
		t.Fatal("pack accepted a stale source tree digest")
	} else if !strings.Contains(err.Error(), "treeSHA256") && !strings.Contains(err.Error(), "tree") {
		t.Fatalf("wrong rejection reason: %v", err)
	}
}

// Recipe bytes sealed into the lock cannot drift after the lock is written.
func TestPackGoHostRejectsRecipeDrift(t *testing.T) {
	opts, _ := goHostUniverse(t)
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	recipe := filepath.Join(repoRoot, "recipes", "diva.vivy.yml")
	original, err := os.ReadFile(recipe)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile(recipe, original, 0o644); err != nil {
			t.Fatalf("restore recipe: %v", err)
		}
	}()
	if err := os.WriteFile(recipe, append(append([]byte{}, original...), []byte("\n# drift\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(context.Background(), opts); err == nil {
		t.Fatal("pack accepted a recipe that drifted after the lock was written")
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

// The sealed artifact must reject tampering at Inspect time: a changed
// frontend byte, a rewritten sidecar manifest, or a binary without the
// matching embedded manifest each fail independently.
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
		if _, err := InspectArtifact(dir); err == nil {
			t.Fatal("inspect accepted a tampered frontend byte")
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

// The same normalized inputs must produce the same Generation ID and manifest
// from any checkout location; raw binary bytes may still differ by build path
// or toolchain metadata, which the build report records instead of promising
// bit-identical executables.
func TestPackGoHostReproducibleGenerationID(t *testing.T) {
	first, _ := goHostUniverse(t)
	second, _ := goHostUniverse(t)
	if _, err := Pack(context.Background(), first); err != nil {
		t.Fatalf("first pack: %v", err)
	}
	if _, err := Pack(context.Background(), second); err != nil {
		t.Fatalf("second pack: %v", err)
	}
	read := func(dir, name string) []byte {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	manifestA := read(first.Output, "generation.json")
	manifestB := read(second.Output, "generation.json")
	if !bytes.Equal(manifestA, manifestB) {
		t.Fatal("identical normalized inputs produced different Generation Manifests")
	}
	if !bytes.Equal(read(first.Output, "zz_assembly.go"), read(second.Output, "zz_assembly.go")) {
		t.Fatal("identical normalized inputs produced different generated Assembly")
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
// host.commit/host.treeSHA256 and hostBuild seals the live resolved identity.
func TestPackGoHostResolvesUnpinnedHostIdentity(t *testing.T) {
	opts, hostDir := goHostUniverse(t)
	mutateGoHostLock(t, opts.HostLock, func(doc map[string]any) {
		host := doc["host"].(map[string]any)
		delete(host, "commit")
		delete(host, "treeSHA256")
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
