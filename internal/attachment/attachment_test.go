package attachment

import (
	"strings"
	"testing"
)

func testPNGBytes() []byte {
	return []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
}

func testJPEGBytes() []byte {
	return []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0}
}

// TestValidateOneAcceptsWhitelistedImages: a claimed MIME that matches the
// content sniff passes and returns the sniffed MIME.
func TestValidateOneAcceptsWhitelistedImages(t *testing.T) {
	cases := []struct {
		claimed string
		data    []byte
		want    string
	}{
		{"image/png", testPNGBytes(), "image/png"},
		{" image/png ", testPNGBytes(), "image/png"}, // tolerant claim
		{"IMAGE/PNG", testPNGBytes(), "image/png"},
		{"image/jpeg", testJPEGBytes(), "image/jpeg"},
	}
	for _, tc := range cases {
		got, err := ValidateOne(tc.claimed, tc.data)
		if err != nil || got != tc.want {
			t.Fatalf("ValidateOne(%q) = %q, %v; want %q", tc.claimed, got, err, tc.want)
		}
	}
}

// TestValidateOneRejectsBadInput: every rejection reason, in contract
// order — whitelist, empty, size, sniff, mismatch.
func TestValidateOneRejectsBadInput(t *testing.T) {
	cases := []struct {
		name    string
		claimed string
		data    []byte
		wantErr string
	}{
		{"unwhitelisted claim", "image/bmp", testPNGBytes(), `unsupported type "image/bmp" (png, jpeg, gif and webp images only)`},
		{"empty data", "image/png", nil, "data must not be empty"},
		{"oversize", "image/png", append(testPNGBytes(), make([]byte, MaxBytes)...), "image exceeds the 5 MiB limit"},
		{"not an image", "image/png", []byte("not an image"), "file content is not a supported image"},
		{"mismatch", "image/jpeg", testPNGBytes(), "MIME type does not match image content"},
	}
	for _, tc := range cases {
		if _, err := ValidateOne(tc.claimed, tc.data); err == nil || err.Error() != tc.wantErr {
			t.Fatalf("%s: err = %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}

// TestSniffMIME pins the magic-byte table: exact families detect, text
// stays empty, and the detected value is always whitelist-checked.
func TestSniffMIME(t *testing.T) {
	cases := []struct {
		data []byte
		want string
	}{
		{testPNGBytes(), "image/png"},
		{testJPEGBytes(), "image/jpeg"},
		{[]byte("GIF89a...."), "image/gif"},
		{[]byte("GIF87a...."), "image/gif"},
		{[]byte("RIFF0000WEBPVP8 "), "image/webp"},
		{[]byte("plain text"), ""},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := SniffMIME(tc.data); got != tc.want {
			t.Fatalf("SniffMIME(%q) = %q, want %q", tc.data, got, tc.want)
		}
	}
}

// TestSanitizeName: control characters stripped, 256-rune bound, empty
// falls back to "image".
func TestSanitizeName(t *testing.T) {
	if got := SanitizeName("photo\x1b[31m.png"); got != "photo[31m.png" {
		t.Fatalf("SanitizeName control chars = %q", got)
	}
	if got := SanitizeName("\r\n\t"); got != "image" {
		t.Fatalf("SanitizeName empty fallback = %q", got)
	}
	if got := SanitizeName(strings.Repeat("图", 300)); len([]rune(got)) != 256 {
		t.Fatalf("SanitizeName bound = %d runes", len([]rune(got)))
	}
}
