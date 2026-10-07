package view

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/sdk/tui/surface"
)

func writeTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "img.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetectImageProtocol(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}
	cases := []struct {
		vars map[string]string
		want string
	}{
		{map[string]string{"KITTY_WINDOW_ID": "1"}, imageProtoKitty},
		{map[string]string{"TERM": "xterm-kitty"}, imageProtoKitty},
		{map[string]string{"TERM_PROGRAM": "iTerm.app"}, imageProtoITerm2},
		{map[string]string{"TERM_PROGRAM": "WezTerm"}, imageProtoKitty},
		{map[string]string{"TERM": "xterm-256color"}, imageProtoNone},
		{map[string]string{}, imageProtoNone},
	}
	for _, tc := range cases {
		if got := detectImageProtocol(env(tc.vars)); got != tc.want {
			t.Fatalf("vars %v = %q, want %q", tc.vars, got, tc.want)
		}
	}
	if got := resolveImageProtocol("off", env(map[string]string{"KITTY_WINDOW_ID": "1"})); got != imageProtoNone {
		t.Fatal("off must disable")
	}
	if got := resolveImageProtocol("on", env(map[string]string{})); got != imageProtoKitty {
		t.Fatal("on without detection must force kitty")
	}
	if got := resolveImageProtocol("auto", env(map[string]string{})); got != imageProtoNone {
		t.Fatal("auto without detection must stay off")
	}
}

func TestKittyTransmitChunks(t *testing.T) {
	// 8192+ bytes of base64 forces continuation chunks.
	data := make([]byte, 8192)
	seq := kittyTransmitSequence(7, data)
	if !strings.Contains(seq, "i=7,m=1;") {
		t.Fatal("missing continuation chunk")
	}
	if !strings.HasSuffix(seq, "\x1b\\") || !strings.Contains(seq, "m=0;") {
		t.Fatal("missing final chunk")
	}
	if strings.Contains(seq, "\n") {
		t.Fatal("sequence must be single-line")
	}
	small := kittyTransmitSequence(3, []byte("hi"))
	if !strings.Contains(small, "i=3,m=0;aGk=") {
		t.Fatalf("small payload = %q", small)
	}
}

func TestItermSequenceAndCap(t *testing.T) {
	path := writeTestPNG(t, 8, 8)
	seq := itermFileSequence(path)
	if !strings.HasPrefix(seq, "\x1b]1337;File=inline=1") || !strings.HasSuffix(seq, "\x07") {
		t.Fatalf("iterm seq = %q", seq[:60])
	}
	if got := itermFileSequence(filepath.Join(t.TempDir(), "missing.png")); got != "" {
		t.Fatal("missing file must give empty sequence")
	}
}

func TestImageCellRows(t *testing.T) {
	if got := imageCellRows(1000, 500, 60); got != 18 {
		t.Fatalf("landscape 1000x500@60 = %d, want capped 18", got)
	}
	if got := imageCellRows(100, 50, 60); got != 18 {
		t.Fatalf("100x50@60 = %d, want 18 (cap)", got)
	}
	if got := imageCellRows(1000, 50, 10); got != 1 {
		t.Fatalf("wide thin = %d, want >=1", got)
	}
	if got := imageCellRows(0, 50, 10); got != 0 {
		t.Fatalf("zero width = %d, want 0", got)
	}
}

func TestRenderImageAttachmentKitty(t *testing.T) {
	path := writeTestPNG(t, 20, 10)
	d := g3Driver()
	m := New(d, Options{Images: "on"})
	// Simulate the transmit step.
	cmds := m.imageTransmitCmds()
	if len(cmds) != 0 {
		t.Fatal("no image attachments yet")
	}
	d.messages["s1"] = append(d.messages["s1"], surface.Message{
		ID: "m6", Role: surface.RoleAssistant,
		Attachments: []surface.Attachment{{Path: path, Name: "img.png", MimeType: "image/png"}},
	})
	cmds = m.imageTransmitCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected one transmit cmd, got %d", len(cmds))
	}
	if id := m.imageState.byPath[path]; id == 0 {
		t.Fatal("image id not assigned")
	}
	att := surface.Attachment{Path: path, MimeType: "image/png"}
	lines := m.renderImageAttachment(att, 80)
	if lines == nil || !strings.Contains(lines[0], "\x1b_Ga=p,i=") {
		t.Fatalf("no placement: %v", lines)
	}
	// Second call must not re-transmit.
	if cmds := m.imageTransmitCmds(); len(cmds) != 0 {
		t.Fatal("re-transmitted")
	}
	// Missing image falls back to chip (nil).
	if lines := m.renderImageAttachment(surface.Attachment{Path: "/nonexistent.png", MimeType: "image/png"}, 80); lines != nil {
		t.Fatal("missing file must fall back")
	}
	// Non-image ignored.
	if lines := m.renderImageAttachment(surface.Attachment{Path: path, MimeType: "text/plain"}, 80); lines != nil {
		t.Fatal("non-image must fall back")
	}
}

func TestRenderImageAttachmentOffAndIterm(t *testing.T) {
	path := writeTestPNG(t, 10, 10)
	d := g3Driver()
	// off: no protocol, always nil.
	m := New(d, Options{Images: "off"})
	if got := m.renderImageAttachment(surface.Attachment{Path: path, MimeType: "image/png"}, 80); got != nil {
		t.Fatal("off must not render")
	}
	// iterm2 forced via env injection is not reachable through Options; test
	// the state directly.
	m = New(d, Options{Images: "off"})
	m.imageState.protocol = imageProtoITerm2
	lines := m.renderImageAttachment(surface.Attachment{Path: path, MimeType: "image/png"}, 80)
	if lines == nil || !strings.Contains(lines[0], "\x1b]1337;File=") {
		t.Fatalf("iterm render = %v", lines)
	}
}
