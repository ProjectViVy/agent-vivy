package bundles

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"agent-vivy/internal/bundlecontract"
)

func TestSafeOriginLocalPaths(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "private", "local repo")
	cases := []struct {
		name, source, want string
	}{
		{"relative", "./parent/repo", "repo"},
		{"native absolute", absolute, "local repo"},
		{"absolute forward slashes", filepath.ToSlash(absolute), "local repo"},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases,
			struct{ name, source, want string }{"drive backslashes", `C:\private\local repo`, "local repo"},
			struct{ name, source, want string }{"drive forward slashes", "c:/private/local repo", "local repo"},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := safeOrigin(tc.source)
			if err != nil || got != tc.want {
				t.Fatalf("safeOrigin(%q) = %q, %v; want %q", tc.source, got, err, tc.want)
			}
		})
	}
}

func TestSafeOriginHTTP(t *testing.T) {
	for _, source := range []string{"https://example.invalid/repo", "http://example.invalid/local%20repo"} {
		got, err := safeOrigin(source)
		if err != nil || got != source {
			t.Fatalf("safeOrigin(%q) = %q, %v", source, got, err)
		}
	}
}

func TestSafeOriginRejectsUnsafeSources(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "repo")
	for _, source := range []string{
		"", "-repo", absolute + "\x00", absolute + "\n", absolute + "\r",
		"https://example.invalid/repo\n", "https://example.invalid/repo\r", "https://example.invalid/repo\x00",
		"https://user@example.invalid/repo", "https://user:secret@example.invalid/repo",
		"https://example.invalid/repo?token=secret", "https://example.invalid/repo#secret", "https:///repo",
		"ssh://example.invalid/repo", "git://example.invalid/repo", "file:///repo", "ext::command",
	} {
		t.Run(source, func(t *testing.T) {
			got, err := safeOrigin(source)
			var invalid *bundlecontract.Error
			if got != "" || !errors.As(err, &invalid) || invalid.Code != "invalid_manifest" || invalid.Field != "git_origin" {
				t.Fatalf("safeOrigin(%q) = %q, %v; want invalid git_origin", source, got, err)
			}
		})
	}
}
