//go:build unix

package rpc

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestResolveProjectAttachmentsRejectsFIFOBeforeOpen(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "image.png"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveProjectAttachments(root, []string{"image.png"})
	if err == nil || !errors.Is(err, errAttachmentPathDirectory) || len(resolved) != 0 {
		t.Fatalf("FIFO resolved: attachments=%d err=%v", len(resolved), err)
	}
}

func TestResolveProjectAttachmentsRejectsFIFOReplacementWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "image.png")
	if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G'}, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := resolveProjectAttachmentsWithHooks(root, []string{"image.png"}, attachmentResolveHooks{
		beforeOpen: func(_ int, _ string) {
			if removeErr := os.Remove(path); removeErr != nil {
				t.Fatal(removeErr)
			}
			if fifoErr := syscall.Mkfifo(path, 0o600); fifoErr != nil {
				t.Fatal(fifoErr)
			}
		},
	})
	if err == nil || (!errors.Is(err, errAttachmentPathChanged) && !errors.Is(err, errAttachmentPathDirectory)) {
		t.Fatalf("expected raced FIFO replacement rejection, got %v", err)
	}
}
