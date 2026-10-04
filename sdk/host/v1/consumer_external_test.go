package host_test

// External-consumer proof (W3-1): a module outside agent-vivy compiles
// against sdk/host/v1 alone. The Go toolchain already refuses to resolve
// agent-vivy/internal/* from outside the module, so a successful build of a
// consumer importing only the host package is the no-leak evidence.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var replaceLine = regexp.MustCompile(`^\s*replace\s+(\S+)\s*=>\s*(\S+)`)

func repoRoot(t *testing.T) string {
	t.Helper()
	// sdk/host/v1 -> repo root is three directories up.
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestExternalModuleConsumesPublicAPI(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var goDirective string
	var replaces []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			goDirective = line
			continue
		}
		match := replaceLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		target := match[2]
		if strings.HasPrefix(target, ".") {
			target = filepath.Join(root, target)
		}
		replaces = append(replaces, "replace "+match[1]+" => "+target)
	}
	if goDirective == "" || len(replaces) == 0 {
		t.Fatal("failed to harvest go directive/replaces from agent-vivy go.mod")
	}

	dir := t.TempDir()
	gomod := strings.Join(append([]string{
		"module vivy-external-consumer-test",
		"",
		goDirective,
		"",
		"require agent-vivy v0.0.0",
		"",
		"replace agent-vivy => " + root,
	}, replaces...), "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600); err != nil {
		t.Fatal(err)
	}
	main := `package main

import (
	"context"

	host "agent-vivy/sdk/host/v1"
)

var (
	_ func(context.Context, host.Options) (*host.Host, error)                                  = host.Open
	_ func(*host.Host, context.Context, string, []byte) ([]byte, error)                        = nil
	_ host.Options
	_ host.Notification
	_ host.EventBatch
	_ host.Error
)

func main() {}
`
	// Exercise the public surface exactly as a consumer writes it; json.RawMessage
	// parameters keep the signature honest without dragging encoding/json into
	// assertions here.
	main = strings.Replace(main, "[]byte) ([]byte, error)", "json.RawMessage) (json.RawMessage, error)", 1)
	main = strings.Replace(main, `import (
	"context"
`, "import (\n\t\"context\"\n\t\"encoding/json\"\n", 1)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		cmd := exec.Command(goBin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("mod", "tidy")
	run("build", ".")
}
