package rpc

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		{name: "windows alternate data stream", path: `inside.png:secret`, cause: errAttachmentPathAbsolute, want: "project-relative"},
		{name: "nested alternate data stream", path: `nested\inside.png:$DATA`, cause: errAttachmentPathAbsolute, want: "project-relative"},
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

func TestResolveProjectAttachmentsRejectsNativeWindowsADS(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows ADS semantics")
	}
	root := t.TempDir()
	base := filepath.Join(root, "photo.png")
	writeTestAttachment(t, base, []byte("not an image"))
	ads := base + ":preview"
	if err := os.WriteFile(ads, testPNGBytes(), 0o600); err != nil {
		t.Skipf("ADS creation unavailable: %v", err)
	}
	resolved, err := resolveProjectAttachments(root, []string{"photo.png:preview"})
	if err == nil || !errors.Is(err, errAttachmentPathAbsolute) || len(resolved) != 0 {
		t.Fatalf("native ADS resolved: attachments=%d err=%v", len(resolved), err)
	}
}

func TestResolveProjectAttachmentsRejectsSensitivePaths(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{".env", "credentials.json", "secret.png", filepath.Join(".ssh", "avatar.png")} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		writeTestAttachment(t, full, testPNGBytes())
		_, err := resolveProjectAttachments(root, []string{path})
		if err == nil || !errors.Is(err, errAttachmentPathSensitive) || !strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("resolve sensitive %q = %v", path, err)
		}
		if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), root) {
			t.Fatalf("sensitive error leaked path: %q", err)
		}
	}
}

func TestValidateAttachmentPathIdentityRejectsReplacement(t *testing.T) {
	root := t.TempDir()
	name := "photo.png"
	path := filepath.Join(root, name)
	writeTestAttachment(t, path, testPNGBytes())
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(root, "original.png")); err != nil {
		t.Fatal(err)
	}
	writeTestAttachment(t, path, testJPEGBytes())
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer rootHandle.Close()
	if err := validateAttachmentPathIdentity(rootHandle, name, opened); !errors.Is(err, errAttachmentPathChanged) {
		t.Fatalf("replacement identity check = %v, want changed", err)
	}
}

func TestResolveProjectAttachmentsRejectsPostReadReplacement(t *testing.T) {
	root := t.TempDir()
	writeTestAttachment(t, filepath.Join(root, "photo.png"), testPNGBytes())
	var hookErr error
	resolved, err := resolveProjectAttachmentsWithHooks(root, []string{"photo.png"}, attachmentResolveHooks{
		beforePostCheck: func(_ int, clean string) {
			original := filepath.Join(root, clean)
			hookErr = os.Rename(original, filepath.Join(root, "original.png"))
			if hookErr == nil {
				hookErr = os.WriteFile(original, testJPEGBytes(), 0o600)
			}
		},
	})
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	if err == nil || !errors.Is(err, errAttachmentPathChanged) || len(resolved) != 0 {
		t.Fatalf("post-read replacement = attachments:%d err:%v, want no data/changed", len(resolved), err)
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
	relativeLink := filepath.Join(root, "relative-link.png")
	if err := os.Symlink(filepath.Join(root, "inside.png"), insideLink); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "outside.png"), outsideLink); err != nil {
		t.Skipf("second symlink creation unavailable: %v", err)
	}
	if err := os.Symlink("inside.png", relativeLink); err != nil {
		t.Skipf("relative symlink creation unavailable: %v", err)
	}
	if _, err := resolveProjectAttachments(root, []string{"inside-link.png"}); err == nil || !strings.Contains(err.Error(), "symlink paths are not allowed") {
		t.Fatalf("in-root absolute symlink error = %v", err)
	}
	if _, err := resolveProjectAttachments(root, []string{"outside-link.png"}); err == nil || !strings.Contains(err.Error(), "symlink paths are not allowed") {
		t.Fatalf("outside symlink error = %v", err)
	}
	if _, err := resolveProjectAttachments(root, []string{"relative-link.png"}); err == nil || !strings.Contains(err.Error(), "symlink paths are not allowed") {
		t.Fatalf("relative in-root symlink error = %v", err)
	}
}

func TestResolveProjectAttachmentsRejectsSensitiveSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	writeTestAttachment(t, filepath.Join(root, ".env"), testPNGBytes())
	if err := os.Symlink(filepath.Join(root, ".env"), filepath.Join(root, "avatar.png")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	_, err := resolveProjectAttachments(root, []string{"avatar.png"})
	if err == nil || !errors.Is(err, errAttachmentPathChanged) || !strings.Contains(err.Error(), "symlink paths are not allowed") {
		t.Fatalf("sensitive symlink target = %v", err)
	}
	if strings.Contains(err.Error(), root) || strings.Contains(err.Error(), ".env") {
		t.Fatalf("sensitive target leaked through public error: %q", err)
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
