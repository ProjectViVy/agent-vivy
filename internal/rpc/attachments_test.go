package rpc

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveProjectAttachmentsRejectsUnsafeNames(t *testing.T) {
	root := t.TempDir()
	writeTestAttachment(t, filepath.Join(root, "inside.png"), testPNGBytes())

	cases := []struct {
		name  string
		path  string
		cause error
		want  string
	}{
		{name: "parent traversal", path: "../inside.png", cause: errAttachmentPathTraversal, want: "path traversal"},
		{name: "mixed separator traversal", path: `sub\..\inside.png`, cause: errAttachmentPathTraversal, want: "path traversal"},
		{name: "embedded traversal", path: `sub/../../inside.png`, cause: errAttachmentPathTraversal, want: "path traversal"},
		{name: "unix absolute", path: `/tmp/inside.png`, cause: errAttachmentPathAbsolute, want: "project-relative"},
		{name: "windows absolute", path: `C:\tmp\inside.png`, cause: errAttachmentPathAbsolute, want: "project-relative"},
		{name: "windows drive relative", path: `C:inside.png`, cause: errAttachmentPathAbsolute, want: "project-relative"},
		{name: "unc", path: `\\server\share\inside.png`, cause: errAttachmentPathAbsolute, want: "project-relative"},
		{name: "nul", path: "inside\x00.png", cause: errAttachmentPathNUL, want: "invalid character"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveProjectAttachments(root, []string{tc.path})
			if err == nil {
				t.Fatal("unsafe path unexpectedly resolved")
			}
			if !errors.Is(err, tc.cause) {
				t.Fatalf("error = %v, want cause %v", err, tc.cause)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err, tc.want)
			}
			if strings.Contains(err.Error(), tc.path) || strings.Contains(err.Error(), root) {
				t.Fatalf("error leaked filesystem input: %q", err)
			}
		})
	}
}

func TestResolveProjectAttachmentsNormalizesSeparatorsAndSniffsContent(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestAttachment(t, filepath.Join(root, "nested", "photo.bin"), testJPEGBytes())

	got, err := resolveProjectAttachments(root, []string{`nested\photo.bin`})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 1 || got[0].Path != "nested/photo.bin" || got[0].Name != "photo.bin" || got[0].MimeType != "image/jpeg" {
		t.Fatalf("resolved = %+v", got)
	}
	if !bytes.Equal(got[0].Data, testJPEGBytes()) || got[0].Size != int64(len(testJPEGBytes())) {
		t.Fatalf("resolved bytes/size = %d/%d", len(got[0].Data), got[0].Size)
	}
}

func TestResolveProjectAttachmentsRejectsDirectorySpoofAndOversize(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir.png"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestAttachment(t, filepath.Join(root, "spoof.png"), []byte("this is not an image"))
	oversize := append(testPNGBytes(), make([]byte, maxAttachmentBytes+1-len(testPNGBytes()))...)
	writeTestAttachment(t, filepath.Join(root, "large.png"), oversize)

	cases := []struct {
		name  string
		path  string
		cause error
	}{
		{name: "directory", path: "dir.png", cause: errAttachmentPathDirectory},
		{name: "mime spoof", path: "spoof.png", cause: errAttachmentPathNotImage},
		{name: "oversize", path: "large.png", cause: errAttachmentPathTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveProjectAttachments(root, []string{tc.path})
			if err == nil || !errors.Is(err, tc.cause) {
				t.Fatalf("error = %v, want cause %v", err, tc.cause)
			}
			if strings.Contains(err.Error(), tc.path) || strings.Contains(err.Error(), root) {
				t.Fatalf("error leaked path: %q", err)
			}
		})
	}
}

func TestResolveProjectAttachmentsRejectsTooManyAndUnconfiguredRoot(t *testing.T) {
	root := t.TempDir()
	paths := []string{"a.png", "b.png", "c.png", "d.png", "e.png"}
	_, err := resolveProjectAttachments(root, paths)
	if err == nil || !strings.Contains(err.Error(), "at most 4") {
		t.Fatalf("count error = %v", err)
	}
	if _, err := resolveProjectAttachments("", []string{"a.png"}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unconfigured root error = %v", err)
	}
}

func TestResolveProjectAttachmentsAcceptsExactCountAndSizeLimits(t *testing.T) {
	root := t.TempDir()
	paths := make([]string, 0, maxAttachmentCount)
	for index := 0; index < maxAttachmentCount; index++ {
		name := fmt.Sprintf("image-%d.png", index)
		data := append(testPNGBytes(), make([]byte, maxAttachmentBytes-len(testPNGBytes()))...)
		writeTestAttachment(t, filepath.Join(root, name), data)
		paths = append(paths, name)
	}
	got, err := resolveProjectAttachments(root, paths)
	if err != nil {
		t.Fatalf("resolve exact limits: %v", err)
	}
	if len(got) != maxAttachmentCount {
		t.Fatalf("resolved count = %d, want %d", len(got), maxAttachmentCount)
	}
	for _, attachment := range got {
		if attachment.Size != maxAttachmentBytes || len(attachment.Data) != maxAttachmentBytes {
			t.Fatalf("resolved exact-size attachment = %d/%d", attachment.Size, len(attachment.Data))
		}
	}
}

func TestResolveProjectAttachmentsContainsSymlinkTargets(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTestAttachment(t, filepath.Join(root, "inside.png"), testPNGBytes())
	writeTestAttachment(t, filepath.Join(outside, "outside.png"), testPNGBytes())

	insideLink := filepath.Join(root, "inside-link.png")
	outsideLink := filepath.Join(root, "outside-link.png")
	if err := os.Symlink(filepath.Join(root, "inside.png"), insideLink); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "outside.png"), outsideLink); err != nil {
		t.Skipf("second symlink creation unavailable: %v", err)
	}
	if _, err := resolveProjectAttachments(root, []string{"inside-link.png"}); err == nil || !strings.Contains(err.Error(), "file cannot be opened") {
		t.Fatalf("in-root absolute symlink error = %v", err)
	}
	if _, err := resolveProjectAttachments(root, []string{"outside-link.png"}); err == nil || !strings.Contains(err.Error(), "escapes the project") {
		t.Fatalf("outside symlink error = %v", err)
	}
}

func TestResolveProjectAttachmentsSupportsAllWhitelistedMagic(t *testing.T) {
	cases := []struct {
		mime string
		data []byte
	}{
		{mime: "image/png", data: testPNGBytes()},
		{mime: "image/jpeg", data: testJPEGBytes()},
		{mime: "image/gif", data: []byte("GIF89a")},
		{mime: "image/webp", data: []byte("RIFFxxxxWEBP")},
	}
	for _, tc := range cases {
		if got := sniffAttachmentMIME(tc.data); got != tc.mime {
			t.Errorf("sniff(%s) = %q", tc.mime, got)
		}
	}
	if got := sniffAttachmentMIME([]byte("not an image")); got != "" {
		t.Fatalf("spoof sniff = %q", got)
	}
}

func TestSafeAttachmentNameRemovesTerminalControlsAndBoundsLength(t *testing.T) {
	if got := safeAttachmentName("photo\x1b[31m.png"); strings.ContainsRune(got, '\x1b') || got != "photo[31m.png" {
		t.Fatalf("unsafe display name = %q", got)
	}
	if got := safeAttachmentName("\r\n\t"); got != "image" {
		t.Fatalf("empty sanitized display name = %q", got)
	}
	if got := safeAttachmentName(strings.Repeat("图", 300)); len([]rune(got)) != 256 {
		t.Fatalf("bounded display name length = %d", len([]rune(got)))
	}
}

func writeTestAttachment(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func testPNGBytes() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
}

func testJPEGBytes() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
}
