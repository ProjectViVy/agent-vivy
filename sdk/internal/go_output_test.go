package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type goOutputFixture struct {
	Args           []string
	Stdout, Stderr string
	ExitCode       int
}

// Re-execute this test binary as go, without a shell or a live module network.
// TestMain handles Go's arguments before the testing package parses its flags.
func TestMain(m *testing.M) {
	if raw := os.Getenv("VIVY_SDK_GO_OUTPUT_FIXTURE"); raw != "" {
		var commands []goOutputFixture
		if err := json.Unmarshal([]byte(raw), &commands); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		for _, command := range commands {
			if reflect.DeepEqual(os.Args[1:], command.Args) {
				fmt.Fprint(os.Stdout, command.Stdout)
				fmt.Fprint(os.Stderr, command.Stderr)
				os.Exit(command.ExitCode)
			}
		}
		fmt.Fprintf(os.Stderr, "unexpected Go fixture arguments: %q\n", os.Args[1:])
		os.Exit(99)
	}
	os.Exit(m.Run())
}

func installGoOutputFixture(t *testing.T, commands ...goOutputFixture) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), body, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(commands)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("VIVY_SDK_GO_OUTPUT_FIXTURE", string(raw))
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
}

const goOutputDiagnostic = "go: downloading example/dependency v1.2.3\n"

func assertGoOutputFailure(t *testing.T, err error) {
	t.Helper()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 7 {
		t.Fatalf("error = %v, want wrapped subprocess exit code 7", err)
	}
	for _, diagnostic := range []string{"partial stdout", "stderr failure cause"} {
		if !strings.Contains(err.Error(), diagnostic) {
			t.Errorf("error = %v, missing %q", err, diagnostic)
		}
	}
}

func TestSourceGoBindingOutput(t *testing.T) {
	for _, tc := range []struct {
		name, stdout, wantPackage string
	}{
		{"success with diagnostics", "example/module/provider provider\n", "provider"},
		{"main package", "example/module/provider main\n", ""},
		{"empty output", "", ""},
		{"missing package", "example/module/provider\n", ""},
		{"extra field", "example/module/provider provider extra\n", ""},
		{"multiple packages", "example/module/provider provider\nexample/module/other other\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installGoOutputFixture(t, goOutputFixture{
				Args:   []string{"list", "-f", "{{.ImportPath}} {{.Name}}", "."},
				Stdout: tc.stdout, Stderr: goOutputDiagnostic,
			})
			importPath, packageName, err := sourceGoBinding(t.TempDir())
			if tc.wantPackage == "" {
				if err == nil || !strings.Contains(err.Error(), "expected one importable Go package") || importPath != "" || packageName != "" {
					t.Fatalf("binding = %q, %q, %v; want rejected output", importPath, packageName, err)
				}
				return
			}
			if err != nil || importPath != "example/module/provider" || packageName != tc.wantPackage {
				t.Fatalf("binding = %q, %q, %v; stderr must not become package data", importPath, packageName, err)
			}
		})
	}
}

func TestGoModulePathOutput(t *testing.T) {
	installGoOutputFixture(t, goOutputFixture{
		Args:   []string{"list", "-m", "-f", "{{.Path}}"},
		Stdout: "example/module\n", Stderr: goOutputDiagnostic,
	})
	modulePath, err := goModulePath(t.TempDir())
	if err != nil || modulePath != "example/module" {
		t.Fatalf("module path = %q, %v; stderr must not become module data", modulePath, err)
	}
}

func TestMergeSourceModulePinsOutput(t *testing.T) {
	f := newPackFixture(t)
	modfile := filepath.Join(f.out, "build.mod")
	const document = `{"Module":{"Path":"example/module"}}`
	installGoOutputFixture(t,
		goOutputFixture{Args: []string{"mod", "edit", "-json", "-modfile=" + filepath.Join(f.source, "go.mod")}, Stdout: document, Stderr: goOutputDiagnostic},
		goOutputFixture{Args: []string{"mod", "edit", "-json", "-modfile=" + modfile}, Stdout: document, Stderr: goOutputDiagnostic},
	)
	if err := mergeSourceModulePins(f.root, "example/root", modfile, f.source, "example/module"); err != nil {
		t.Fatalf("merge JSON with stderr diagnostics: %v", err)
	}
}

func TestCanonicalResolvedModfileOutput(t *testing.T) {
	root := t.TempDir()
	modfile := filepath.Join(root, "build.mod")
	const document = `{"Module":{"Path":"example/module"}}`
	installGoOutputFixture(t, goOutputFixture{
		Args:   []string{"mod", "edit", "-json", "-modfile=" + modfile},
		Stdout: document, Stderr: goOutputDiagnostic,
	})
	canonical, err := canonicalResolvedModfile(root, modfile)
	if err != nil || string(canonical) != document {
		t.Fatalf("canonical modfile = %s, %v; stderr must not become JSON data", canonical, err)
	}
}

func TestSnapshotDependencyLocksOutput(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	modfile := filepath.Join(root, "build.mod")
	const buildList = "example/module\nexample/dependency v1.2.3\n"
	installGoOutputFixture(t,
		goOutputFixture{Args: []string{"list", "-mod=readonly", "-modfile=" + modfile, "-m", "-f", "{{.Path}} {{.Version}}", "all"}, Stdout: buildList, Stderr: goOutputDiagnostic},
		goOutputFixture{Args: []string{"mod", "edit", "-json", "-modfile=" + modfile}, Stdout: `{}`},
	)
	locks, err := snapshotDependencyLocks(root, modfile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := locks["resolved-build-list"], sha256Hex(bytes.TrimSpace([]byte(buildList))); got != want {
		t.Fatalf("build list digest = %s, want %s from stdout only", got, want)
	}
}

func TestParsedGoCommandFailuresRetainDiagnostics(t *testing.T) {
	for _, operation := range []string{"source binding", "module path", "source pins", "canonical modfile", "dependency locks"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			modfile := filepath.Join(root, "build.mod")
			var args []string
			var call func() error
			switch operation {
			case "source binding":
				args = []string{"list", "-f", "{{.ImportPath}} {{.Name}}", "."}
				call = func() error { _, _, err := sourceGoBinding(root); return err }
			case "module path":
				args = []string{"list", "-m", "-f", "{{.Path}}"}
				call = func() error { _, err := goModulePath(root); return err }
			case "source pins":
				args = []string{"mod", "edit", "-json", "-modfile=" + filepath.Join(root, "go.mod")}
				call = func() error { return mergeSourceModulePins(root, "example/root", modfile, root, "example/module") }
			case "canonical modfile":
				args = []string{"mod", "edit", "-json", "-modfile=" + modfile}
				call = func() error { _, err := canonicalResolvedModfile(root, modfile); return err }
			case "dependency locks":
				args = []string{"list", "-mod=readonly", "-modfile=" + modfile, "-m", "-f", "{{.Path}} {{.Version}}", "all"}
				call = func() error { _, err := snapshotDependencyLocks(root, modfile); return err }
			}
			installGoOutputFixture(t, goOutputFixture{Args: args, Stdout: "partial stdout\n", Stderr: "stderr failure cause\n", ExitCode: 7})
			assertGoOutputFailure(t, call())
		})
	}
}
