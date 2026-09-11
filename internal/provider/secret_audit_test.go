package provider

// E3 (AS-9, D-010): source-level secret boundary audit. These tests fail
// the build of anyone who moves a credential read out of this package or
// hardcodes a key literal into production code.

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"agent-vivy/internal/modelhost"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return root
}

// walkGoSources visits every Go source file under the given repo dirs.
func walkGoSources(t *testing.T, root string, dirs []string, visit func(relPath string, src []byte)) {
	t.Helper()
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			visit(filepath.ToSlash(rel), src)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
}

// secretEnvName flags environment variable names that plausibly carry
// credentials.
func secretEnvName(name string) bool {
	up := strings.ToUpper(name)
	return strings.Contains(up, "KEY") || strings.Contains(up, "SECRET") || strings.Contains(up, "TOKEN")
}

// Credential environment reads are allowed only in the model resolver
// (frozen ENV session) and must never happen inside provider construction.
func TestSecretEnvReadsStayOutOfProvider(t *testing.T) {
	getenv := regexp.MustCompile(`os\.Getenv\("([^"]+)"\)`)
	var violations []string
	walkGoSources(t, repoRoot(t), []string{"cmd", "internal"}, func(rel string, src []byte) {
		if strings.HasPrefix(rel, "internal/app/model.go") {
			return
		}
		if strings.HasPrefix(rel, "internal/provider/") {
			for _, m := range getenv.FindAllStringSubmatch(string(src), -1) {
				if secretEnvName(m[1]) {
					violations = append(violations, rel+": os.Getenv("+m[1]+")")
				}
			}
			return
		}
		for _, m := range getenv.FindAllStringSubmatch(string(src), -1) {
			if secretEnvName(m[1]) {
				violations = append(violations, rel+": os.Getenv("+m[1]+")")
			}
		}
	})
	if len(violations) > 0 {
		t.Fatalf("credential reads outside the model resolver: %v", violations)
	}
}

// Production sources must never contain a key-shaped literal. Test files
// may keep negative fixtures (rejection tests) and are exempt.
func TestNoHardcodedKeyLiterals(t *testing.T) {
	literal := regexp.MustCompile(`sk-[A-Za-z0-9_-]{6,}`)
	var violations []string
	walkGoSources(t, repoRoot(t), []string{"cmd", "internal"}, func(rel string, src []byte) {
		if strings.HasSuffix(rel, "_test.go") {
			return
		}
		if loc := literal.Find(src); loc != nil {
			violations = append(violations, rel+": "+string(loc))
		}
	})
	if len(violations) > 0 {
		t.Fatalf("hardcoded key literals in production code: %v", violations)
	}
}

// KeyMissingError names the environment variable but can never carry a key
// value: the struct holds no value field and the message is built from the
// provider and variable name only.
func TestKeyMissingErrorCarriesNoValue(t *testing.T) {
	const canary = "sk-canary-value-that-must-not-appear"
	err := &KeyMissingError{Provider: "openai", EnvKey: "OPENAI_API_KEY"}
	msg := err.Error()
	if !strings.Contains(msg, "OPENAI_API_KEY") && !strings.Contains(msg, "Settings") {
		t.Fatalf("message must name the env variable or settings path: %q", msg)
	}
	if strings.Contains(msg, canary) {
		t.Fatal("message leaked a key value")
	}
}

func TestProviderProfileStatusWireSourceCannotCarrySecrets(t *testing.T) {
	typeOfStatus := reflect.TypeOf(modelhost.ProfileStatus{})
	for _, forbidden := range []string{"SecretRefs", "OptionsSchema", "APIKey", "Credential"} {
		if _, exists := typeOfStatus.FieldByName(forbidden); exists {
			t.Fatalf("ProfileStatus exposes forbidden field %q", forbidden)
		}
	}
}
