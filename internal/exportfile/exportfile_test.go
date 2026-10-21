package exportfile

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWritePublishesDistinctArtifacts(t *testing.T) {
	dir := t.TempDir()
	one, err := Write(dir, "session", ".html", []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := Write(dir, "session", ".html", []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if one == two || !strings.HasPrefix(one, "session-") || !strings.HasSuffix(one, ".html") {
		t.Fatalf("identity reused: %q %q", one, two)
	}
	body, err := Read(context.Background(), dir, one)
	if err != nil || string(body) != "first" {
		t.Fatalf("first artifact overwritten: %q %v", body, err)
	}
	body, err = Read(context.Background(), dir, two)
	if err != nil || string(body) != "second" {
		t.Fatalf("second artifact: %q %v", body, err)
	}
}

func TestReadRejectsInvalidAndOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", ".", "..", "a/b", "a\\b", "bad name"} {
		if _, err := Read(context.Background(), dir, name); !errors.Is(err, ErrName) {
			t.Fatalf("name %q: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "large.html"), bytes.Repeat([]byte("x"), MaxBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), dir, "large.html"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize: %v", err)
	}
}

func TestReadRejectsLinkedFile(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Skip("symlink refusal still requires the independent Linux owner check")
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(context.Background(), dir, "linked.html"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("link followed: %v", err)
	}
}

func TestReadRefusesMutationDuringRead(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "in-place", true: "replace"}[replace], func(t *testing.T) {
			dir := t.TempDir()
			name, err := Write(dir, "session", ".html", []byte("initial bytes"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = read(context.Background(), dir, name, func() {
				full := filepath.Join(dir, name)
				if replace {
					if err := os.Remove(full); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(full, []byte("changed bytes longer"), 0600); err != nil {
					t.Fatal(err)
				}
			})
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("mutation accepted: %v", err)
			}
		})
	}
}

func TestWriteBoundsAndReadCancellation(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "session", ".html", make([]byte, MaxBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("write oversize: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected write mutated root: %v %v", entries, err)
	}
	name, err := Write(dir, "session", ".html", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, dir, name); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	body, err := Read(context.Background(), dir, name)
	if err != nil || len(body) != 0 {
		t.Fatalf("empty: %q %v", body, err)
	}
}
