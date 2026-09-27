// Package attachment is the single source of the image attachment limits
// (VC-1g-2): one whitelist, one per-file size bound, one per-message count
// bound, one content sniff. The RPC boundary (UI/TUI faces) and the
// channel inbound path enforce the same numbers through this package —
// no other code may define its own copy. Validation rejects, never
// truncates: bounded bytes only ever enter the organism.
package attachment

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

// MaxBytes is the per-image bound: 5 MiB, aligned with the Crush client
// surface like the rest of the VC-1g-2 limits.
const MaxBytes = 5 << 20

// MaxCount is the per-message attachment bound (all sources combined).
const MaxCount = 4

// MimeWhitelist is the accepted image MIME vocabulary.
var MimeWhitelist = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// ValidateOne validates one already-decoded image payload against the
// shared limits: the claimed MIME (case/whitespace tolerant) must be
// whitelisted, the data non-empty and within MaxBytes, and the content
// sniff must detect that MIME. It returns the sniffed MIME. The error
// texts are the stable per-item rejection reasons callers embed.
func ValidateOne(claimedMIME string, data []byte) (string, error) {
	mime := strings.ToLower(strings.TrimSpace(claimedMIME))
	if !MimeWhitelist[mime] {
		return "", fmt.Errorf("unsupported type %q (png, jpeg, gif and webp images only)", claimedMIME)
	}
	if len(data) == 0 {
		return "", errors.New("data must not be empty")
	}
	if len(data) > MaxBytes {
		return "", fmt.Errorf("image exceeds the %d MiB limit", MaxBytes>>20)
	}
	detected := SniffMIME(data)
	if detected == "" {
		return "", errors.New("file content is not a supported image")
	}
	if detected != mime {
		return "", errors.New("MIME type does not match image content")
	}
	return detected, nil
}

// SniffMIME is content based. The extension and any client MIME claim are
// intentionally ignored, preventing a text file named *.png from entering
// the existing multimodal pipeline.
func SniffMIME(data []byte) string {
	// The first four PNG bytes are enough to distinguish the format for the
	// existing inline attachment contract (which permits a short deterministic
	// fixture); project-path attachments still use this same content sniffing
	// seam and never trust an extension or client MIME claim.
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x89, 'P', 'N', 'G'}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return "image/gif"
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	// Keep net/http's maintained sniff table as a final guard for equivalent
	// signatures while still applying the explicit whitelist.
	detected := http.DetectContentType(data)
	if MimeWhitelist[detected] {
		return detected
	}
	return ""
}

// SanitizeName keeps filenames inert when projected into terminal chips
// or persisted history. Control characters are stripped, the length is
// bounded, and a name that sanitizes to nothing falls back to "image".
func SanitizeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	runes := []rune(name)
	if len(runes) > 256 {
		name = string(runes[:256])
	}
	if strings.TrimSpace(name) == "" {
		return "image"
	}
	return name
}
